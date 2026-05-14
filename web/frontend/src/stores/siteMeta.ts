import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import axios from 'axios'
import { http, type Envelope } from '../api/http'

type PubSiteMeta = {
  brand_name?: string
  brand_icon_url?: string
  api_public_base_url?: string
  /** 注册推广说明（纯文本）；空或未返回则不展示 */
  register_promo_text?: string
}

/** 与后端 sitecfg.DefaultBrandIconURL 一致；接口返回该值时存为空串表示「无自定义」 */
const defaultPubIcon = '/tracehub-icon.svg'

export const useSiteMetaStore = defineStore('siteMeta', () => {
  const brandName = ref('')
  const brandIconUrl = ref('')
  /** 管理员配置的 OpenAI 兼容 Base URL（含 /v1）；空则 effectiveOpenaiApiBase 回退为当前站点 origin + /v1 */
  const apiPublicBaseUrl = ref('')
  /** 注册页 / 官网展示的推广文案（来自站点配置） */
  const registerPromoText = ref('')
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

  const effectiveOpenaiApiBase = computed(() => {
    const c = apiPublicBaseUrl.value.trim()
    if (c) return c.replace(/\/+$/, '')
    if (typeof window !== 'undefined') {
      return `${window.location.origin.replace(/\/$/, '')}/v1`
    }
    return '/v1'
  })

  async function pullPubSiteMeta(): Promise<void> {
    try {
      const { data } = await http.get<Envelope<PubSiteMeta>>('/api/pub/site-meta', {
        params: { _: Date.now() },
      })
      if (Number(data.code) === 0 && data.data) {
        if (typeof data.data.brand_name === 'string') {
          brandName.value = data.data.brand_name
        }
        if (typeof data.data.brand_icon_url === 'string') {
          const u = data.data.brand_icon_url.trim()
          brandIconUrl.value = u === defaultPubIcon ? '' : u
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
    apiPublicBaseUrl.value = ''
    registerPromoText.value = ''
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
    apiPublicBaseUrl,
    registerPromoText,
    loaded,
    displayName,
    displayIconUrl,
    effectiveOpenaiApiBase,
    ensureLoaded,
    refreshFromServer,
  }
})
