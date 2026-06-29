import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import axios from 'axios'
import { http, type Envelope } from '../api/http'

/** 与 /api/pub/site-meta 中 public_contact 一致 */
export type PublicContactPayload = {
  title?: string
  qr_items?: Array<{ label?: string; image_url?: string }>
  emails?: string[]
}

/** 与 /api/pub/site-meta 中 recharge_shops 一致 */
export type RechargeShopItem = {
  label: string
  url: string
  description?: string
  sort?: number
  show_qr?: boolean
  qr_image_url?: string
}

export type RechargeShopsPayload = {
  title?: string
  hint?: string
  shops?: RechargeShopItem[]
}

type PubSiteMeta = {
  brand_name?: string
  brand_icon_url?: string
  brand_favicon_url?: string
  brand_og_image_url?: string
  contact_card_url?: string
  app_version?: string
  api_public_base_url?: string
  /** 注册推广说明（纯文本）；空或未返回则不展示 */
  register_promo_text?: string
  /** 官网页脚「联系我们」；未返回或空则不展示 */
  public_contact?: PublicContactPayload | null
  /** 全站渠道声明模型去重数（百位取整+）；未返回或低于展示阈值则为空 */
  declared_model_count_badge?: string
  /** 在线支付总开关 */
  online_payment_enabled?: boolean
  /** 是否通过外部商铺充值（配置后替代在线支付展示） */
  recharge_via_external_shop?: boolean
  /** 外部充值商铺 */
  recharge_shops?: RechargeShopsPayload | null
  /** 全站页脚备案等多行文案 */
  beian_footer_lines?: string[]
}

function normalizePublicContact(raw: unknown): PublicContactPayload | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const title = typeof o.title === 'string' ? o.title.trim() : ''
  const qrRaw = Array.isArray(o.qr_items) ? o.qr_items : []
  const qr_items: Array<{ label: string; image_url: string }> = []
  for (const x of qrRaw) {
    if (!x || typeof x !== 'object') continue
    const r = x as Record<string, unknown>
    const label = typeof r.label === 'string' ? r.label.trim() : ''
    const image_url = typeof r.image_url === 'string' ? r.image_url.trim() : ''
    if (!image_url) continue
    qr_items.push({ label, image_url })
  }
  const emRaw = Array.isArray(o.emails) ? o.emails : []
  const emails: string[] = []
  for (const e of emRaw) {
    if (typeof e === 'string' && e.trim()) emails.push(e.trim())
  }
  if (!title && qr_items.length === 0 && emails.length === 0) return null
  return { title, qr_items, emails }
}

function normalizeRechargeShops(raw: unknown): RechargeShopsPayload | null {
  if (!raw || typeof raw !== 'object') return null
  const o = raw as Record<string, unknown>
  const title = typeof o.title === 'string' ? o.title.trim() : ''
  const hint = typeof o.hint === 'string' ? o.hint.trim() : ''
  const shopsRaw = Array.isArray(o.shops) ? o.shops : []
  const shops: RechargeShopItem[] = []
  for (const x of shopsRaw) {
    if (!x || typeof x !== 'object') continue
    const r = x as Record<string, unknown>
    const label = typeof r.label === 'string' ? r.label.trim() : ''
    const url = typeof r.url === 'string' ? r.url.trim() : ''
    if (!label || !url) continue
    const description = typeof r.description === 'string' ? r.description.trim() : ''
    const sort = typeof r.sort === 'number' ? r.sort : 0
    const show_qr = r.show_qr === true
    const qr_image_url = typeof r.qr_image_url === 'string' ? r.qr_image_url.trim() : ''
    shops.push({ label, url, description, sort, show_qr, qr_image_url: qr_image_url || undefined })
  }
  shops.sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0) || a.label.localeCompare(b.label))
  if (!shops.length) return null
  return { title, hint, shops }
}

function normalizeBeianFooterLines(raw: unknown): string[] {
  if (!Array.isArray(raw)) return []
  const out: string[] = []
  for (const x of raw) {
    if (typeof x === 'string') {
      const s = x.trim()
      if (s) out.push(s)
    }
  }
  return out
}

/** 与后端 sitecfg 默认一致；接口返回该值时存为空串表示「无自定义」 */
const defaultPubIcon = '/tracehub-icon.svg'
const defaultPubFavicon = '/tracehub-icon.svg'

function normalizeStoredBrandUrl(apiUrl: string | undefined, defaultUrl: string): string {
  if (typeof apiUrl !== 'string') return ''
  const u = apiUrl.trim()
  return u === defaultUrl ? '' : u
}

export const useSiteMetaStore = defineStore('siteMeta', () => {
  const brandName = ref('')
  const brandIconUrl = ref('')
  const brandFaviconUrl = ref('')
  const brandOgImageUrl = ref('')
  const contactCardUrl = ref('')
  const appVersion = ref('')
  /** 管理员配置的 OpenAI 兼容 Base URL（含 /v1）；空则 effectiveOpenaiApiBase 回退为当前站点 origin + /v1 */
  const apiPublicBaseUrl = ref('')
  /** 注册页 / 官网展示的推广文案（来自站点配置） */
  const registerPromoText = ref('')
  /** 官网页脚联系我们（来自站点配置） */
  const publicContact = ref<PublicContactPayload | null>(null)
  /** 全站渠道声明模型数展示文案（如 300+），来自 /api/pub/site-meta */
  const declaredModelCountBadge = ref('')
  const onlinePaymentEnabled = ref(false)
  const rechargeViaExternalShop = ref(false)
  const rechargeShops = ref<RechargeShopsPayload | null>(null)
  /** 全站页脚备案等多行文案 */
  const beianFooterLines = ref<string[]>([])
  const loaded = ref(false)
  let inflight: Promise<void> | null = null

  const displayName = computed(() => {
    const s = brandName.value.trim()
    return s || 'TraceHub'
  })

  const displayIconUrl = computed(() => {
    const u = brandIconUrl.value.trim()
    return u || defaultPubIcon
  })

  const displayFaviconUrl = computed(() => {
    const u = brandFaviconUrl.value.trim()
    return u || defaultPubFavicon
  })

  const displayOgImageUrl = computed(() => {
    const u = brandOgImageUrl.value.trim()
    return u || null
  })

  const displayContactCardUrl = computed(() => {
    const u = contactCardUrl.value.trim()
    return u || null
  })

  const displayVersion = computed(() => {
    const v = appVersion.value.trim()
    return v || '1.0.0'
  })

  const effectiveOpenaiApiBase = computed(() => {
    const c = apiPublicBaseUrl.value.trim()
    if (c) return c.replace(/\/+$/, '')
    if (typeof window !== 'undefined') {
      return `${window.location.origin.replace(/\/$/, '')}/v1`
    }
    return '/v1'
  })

  const hasPublicContact = computed(() => publicContact.value != null)
  const hasRechargeShops = computed(() => rechargeViaExternalShop.value && rechargeShops.value != null)
  const hasBeianFooter = computed(() => beianFooterLines.value.length > 0)

  async function pullPubSiteMeta(): Promise<void> {
    try {
      const { data } = await http.get<Envelope<PubSiteMeta>>('/api/pub/site-meta', {
        params: { _: Date.now() },
      })
      if (Number(data.code) === 0 && data.data) {
        if (typeof data.data.brand_name === 'string') {
          brandName.value = data.data.brand_name
        }
        brandIconUrl.value = normalizeStoredBrandUrl(data.data.brand_icon_url, defaultPubIcon)
        brandFaviconUrl.value = normalizeStoredBrandUrl(data.data.brand_favicon_url, defaultPubFavicon)
        brandOgImageUrl.value =
          typeof data.data.brand_og_image_url === 'string' ? data.data.brand_og_image_url.trim() : ''
        contactCardUrl.value =
          typeof data.data.contact_card_url === 'string' ? data.data.contact_card_url.trim() : ''
        if (typeof data.data.app_version === 'string') {
          appVersion.value = data.data.app_version.trim()
        } else {
          appVersion.value = ''
        }
        if (typeof data.data.api_public_base_url === 'string') {
          apiPublicBaseUrl.value = data.data.api_public_base_url.trim()
        } else {
          apiPublicBaseUrl.value = ''
        }
        if (typeof data.data.register_promo_text === 'string') {
          registerPromoText.value = data.data.register_promo_text.trim()
        } else {
          registerPromoText.value = ''
        }
        publicContact.value = normalizePublicContact(data.data.public_contact)
        beianFooterLines.value = normalizeBeianFooterLines(data.data.beian_footer_lines)
        if (typeof data.data.declared_model_count_badge === 'string') {
          declaredModelCountBadge.value = data.data.declared_model_count_badge.trim()
        } else {
          declaredModelCountBadge.value = ''
        }
        onlinePaymentEnabled.value = data.data.online_payment_enabled === true
        rechargeViaExternalShop.value = data.data.recharge_via_external_shop === true
        rechargeShops.value = normalizeRechargeShops(data.data.recharge_shops)
      }
    } catch (e: unknown) {
      if (!axios.isAxiosError(e) || e.response?.status !== 404) {
        /* 保持默认 */
      }
    }
  }

  async function ensureLoaded() {
    if (loaded.value) return
    if (inflight) return inflight
    inflight = (async () => {
      try {
        await pullPubSiteMeta()
      } finally {
        loaded.value = true
        inflight = null
      }
    })()
    return inflight
  }

  async function refreshFromServer() {
    if (inflight) {
      await inflight.catch(() => {})
    }
    loaded.value = false
    brandName.value = ''
    brandIconUrl.value = ''
    brandFaviconUrl.value = ''
    brandOgImageUrl.value = ''
    contactCardUrl.value = ''
    appVersion.value = ''
    apiPublicBaseUrl.value = ''
    registerPromoText.value = ''
    publicContact.value = null
    beianFooterLines.value = []
    declaredModelCountBadge.value = ''
    onlinePaymentEnabled.value = false
    rechargeViaExternalShop.value = false
    rechargeShops.value = null
    inflight = (async () => {
      try {
        await pullPubSiteMeta()
      } finally {
        loaded.value = true
        inflight = null
      }
    })()
    await inflight
  }

  return {
    brandName,
    brandIconUrl,
    brandFaviconUrl,
    brandOgImageUrl,
    contactCardUrl,
    appVersion,
    apiPublicBaseUrl,
    registerPromoText,
    publicContact,
    declaredModelCountBadge,
    onlinePaymentEnabled,
    rechargeViaExternalShop,
    rechargeShops,
    hasRechargeShops,
    hasPublicContact,
    beianFooterLines,
    hasBeianFooter,
    loaded,
    displayName,
    displayIconUrl,
    displayFaviconUrl,
    displayOgImageUrl,
    displayContactCardUrl,
    displayVersion,
    effectiveOpenaiApiBase,
    ensureLoaded,
    refreshFromServer,
  }
})
