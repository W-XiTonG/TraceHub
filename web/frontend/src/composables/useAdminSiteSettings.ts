import axios from 'axios'
import type { InjectionKey } from 'vue'
import { computed, reactive, ref } from 'vue'
import { useSmoothLoading } from './useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { useSiteMetaStore, type PublicContactPayload } from '../stores/siteMeta'

/** 与后端 sitecfg.DefaultBrandIconURL 一致 */
export const defaultBrandIconUrl = '/tracehub-icon.svg'

export type BrandLibIcon = {
  url: string
  uploaded_at?: number
  filename?: string
  is_builtin_default?: boolean
}

export type SiteSettingsPayload = {
  brand_name?: string
  brand_icon_url?: string
  login_captcha_enabled?: boolean
  login_captcha_file?: boolean
  login_captcha_db_set?: boolean
  login_captcha_db_value?: string
  operator_display_name?: string
  operator_display_file?: string
  default_new_user_balance?: number
  default_new_user_balance_file?: number
  default_new_user_balance_db_set?: boolean
  default_new_user_balance_db_value?: string
  register_promo_text?: string
  register_promo_text_file?: string
  register_promo_text_db_set?: boolean
  register_promo_text_db_value?: string
  no_bill_models?: string[]
  no_bill_models_file?: string[]
  no_bill_models_db_set?: boolean
  no_bill_models_db_value?: string
  api_public_base_url?: string
  public_contact?: PublicContactPayload | null
  upstream_model_probe_enabled?: boolean
  upstream_model_probe_interval_minutes?: number
  upstream_model_probe_enabled_db_set?: boolean
  upstream_model_probe_enabled_db_value?: string
  upstream_model_probe_interval_db_set?: boolean
  upstream_model_probe_interval_db_value?: string
  upstream_capability_fail_stale_hours?: number
  upstream_capability_fail_stale_hours_db_set?: boolean
  upstream_capability_fail_stale_hours_db_value?: string
  upstream_capability_probe_enabled?: boolean
  routing_strategy_default?: string
  routing_failover_enabled_default?: boolean
  routing_failover_max_attempts_default?: number
  routing_sticky_enabled_default?: boolean
  routing_auto_health_from_probe?: boolean
  routing_probe_fail_threshold?: number
  gateway_ip_blacklist?: string[]
  gateway_ip_blacklist_db_set?: boolean
  /** 以下三项来自运行配置 YAML，供全站计价换算等只读展示 */
  currency_quota_per_yuan?: number
  currency_quota_per_usd?: number
  pricing_cny_per_usd?: number
}

export function createAdminSiteSettings() {
  const { t } = useI18n()
  const siteMeta = useSiteMetaStore()

  const form = reactive({
    brandName: '',
    brandIconUrl: '',
    loginCaptcha: false,
    operatorDisplayName: '',
    defaultNewUserBalance: 0,
    registerPromoText: '',
    noBillModelsText: '',
    apiPublicBaseUrl: '',
    upstreamProbeEnabled: false,
    upstreamProbeInterval: 360,
    upstreamCapabilityFailStaleHours: 6,
    upstreamCapabilityProbeEnabled: false,
    routingStrategy: 'weighted_random',
    routingFailoverEnabled: false,
    routingFailoverMaxAttempts: 2,
    routingStickyEnabled: true,
    routingAutoHealthFromProbe: false,
    routingProbeFailThreshold: 3,
  })

  const contactTitle = ref('')
  const contactEmailsText = ref('')
  const contactQRs = ref<Array<{ label: string; image_url: string }>>([])
  const contactPickQrIndex = ref<number | null>(null)
  const uploadingContactRow = ref<number | null>(null)
  const { busy, visible: busyUi, run: runLoad } = useSmoothLoading()
  const saving = ref(false)
  const uploadingIcon = ref(false)
  const msg = ref('')
  const err = ref('')
  const brandIcons = ref<BrandLibIcon[]>([])
  const deletingBrandFilename = ref<string | null>(null)
  const resettingDefaults = ref(false)
  const brandPreviewNonce = ref(0)
  const defaultNewUserBalanceYaml = ref(0)
  const defaultNewUserBalanceDbOverride = ref(false)
  const registerPromoTextYaml = ref('')
  const registerPromoTextDbOverride = ref(false)
  const noBillModelsYaml = ref('')
  const noBillModelsDbOverride = ref(false)
  const upstreamProbeEnabledDbOverride = ref(false)
  const upstreamProbeIntervalDbOverride = ref(false)
  const gatewayIpBlacklistDbOverride = ref(false)
  const gatewayIpBlacklistRows = ref<Array<{ value: string }>>([])

  function addGatewayIpBlacklistRow() {
    gatewayIpBlacklistRows.value.push({ value: '' })
  }

  function removeGatewayIpBlacklistRow(idx: number) {
    gatewayIpBlacklistRows.value.splice(idx, 1)
  }

  const brandIconPreviewSrc = computed(() => {
    const u = form.brandIconUrl.trim()
    return u || defaultBrandIconUrl
  })

  const brandIconsForUi = computed((): BrandLibIcon[] => {
    const rest = brandIcons.value.filter((x) => x.url.trim() !== defaultBrandIconUrl)
    const head: BrandLibIcon = {
      url: defaultBrandIconUrl,
      uploaded_at: 0,
      filename: 'default',
      is_builtin_default: true,
    }
    return [head, ...rest]
  })

  const brandIconsUploadedOnly = computed(() => brandIcons.value)

  function httpErr(e: unknown): string {
    if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
      const d = e.response.data as { message?: string; code?: unknown }
      if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
      const c = Number(d.code)
      if (Number.isFinite(c) && c !== 0) return `${t('adminCommon.requestFail')} (${c})`
    }
    return e instanceof Error ? e.message : t('adminCommon.requestFail')
  }

  function isEnvelopeSuccess(body: unknown): body is { code: number } {
    if (!body || typeof body !== 'object' || !('code' in body)) return false
    const c = Number((body as { code: unknown }).code)
    return Number.isFinite(c) && c === 0
  }

  function pickInt64ish(v: unknown): number {
    if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v)
    if (typeof v === 'string' && v.trim() !== '') {
      const n = Number.parseInt(v.trim(), 10)
      if (Number.isFinite(n)) return n
    }
    return 0
  }

  function applyPayload(d: SiteSettingsPayload) {
    form.brandName = typeof d.brand_name === 'string' ? d.brand_name : ''
    form.brandIconUrl = typeof d.brand_icon_url === 'string' ? d.brand_icon_url : ''
    form.loginCaptcha = Boolean(d.login_captcha_enabled)
    form.operatorDisplayName = typeof d.operator_display_name === 'string' ? d.operator_display_name : ''
    form.defaultNewUserBalance = pickInt64ish(d.default_new_user_balance)
    defaultNewUserBalanceYaml.value = pickInt64ish(d.default_new_user_balance_file)
    defaultNewUserBalanceDbOverride.value = Boolean(d.default_new_user_balance_db_set)
    form.registerPromoText = typeof d.register_promo_text === 'string' ? d.register_promo_text : ''
    registerPromoTextYaml.value = typeof d.register_promo_text_file === 'string' ? d.register_promo_text_file : ''
    registerPromoTextDbOverride.value = Boolean(d.register_promo_text_db_set)
    const noBillList = Array.isArray(d.no_bill_models)
      ? d.no_bill_models.filter((x): x is string => typeof x === 'string' && x.trim())
      : []
    form.noBillModelsText = noBillList.join('\n')
    const yamlList = Array.isArray(d.no_bill_models_file)
      ? d.no_bill_models_file.filter((x): x is string => typeof x === 'string' && x.trim())
      : []
    noBillModelsYaml.value = yamlList.join('\n')
    noBillModelsDbOverride.value = Boolean(d.no_bill_models_db_set)
    form.apiPublicBaseUrl = typeof d.api_public_base_url === 'string' ? d.api_public_base_url : ''
    form.upstreamProbeEnabled = Boolean(d.upstream_model_probe_enabled)
    const iv = pickInt64ish(d.upstream_model_probe_interval_minutes)
    form.upstreamProbeInterval = iv >= 15 && iv <= 1440 ? iv : 360
    upstreamProbeEnabledDbOverride.value = Boolean(d.upstream_model_probe_enabled_db_set)
    upstreamProbeIntervalDbOverride.value = Boolean(d.upstream_model_probe_interval_db_set)
    const staleH = pickInt64ish(d.upstream_capability_fail_stale_hours)
    form.upstreamCapabilityFailStaleHours = staleH >= 0 && staleH <= 168 ? staleH : 6
    form.upstreamCapabilityProbeEnabled = Boolean(d.upstream_capability_probe_enabled)
    form.routingStrategy =
      typeof d.routing_strategy_default === 'string' && d.routing_strategy_default.trim()
        ? d.routing_strategy_default
        : 'weighted_random'
    form.routingFailoverEnabled = Boolean(d.routing_failover_enabled_default)
    form.routingFailoverMaxAttempts = pickInt64ish(d.routing_failover_max_attempts_default) || 2
    form.routingStickyEnabled = d.routing_sticky_enabled_default !== false
    form.routingAutoHealthFromProbe = Boolean(d.routing_auto_health_from_probe)
    form.routingProbeFailThreshold = pickInt64ish(d.routing_probe_fail_threshold) || 3
    const gwList = Array.isArray(d.gateway_ip_blacklist)
      ? d.gateway_ip_blacklist.filter((x): x is string => typeof x === 'string' && x.trim())
      : []
    gatewayIpBlacklistRows.value = gwList.map((ip) => ({ value: ip }))
    gatewayIpBlacklistDbOverride.value = Boolean(d.gateway_ip_blacklist_db_set)
    const pc = d.public_contact
    if (pc && typeof pc === 'object') {
      contactTitle.value = typeof pc.title === 'string' ? pc.title.trim() : ''
      contactEmailsText.value = Array.isArray(pc.emails)
        ? pc.emails.filter((e): e is string => typeof e === 'string' && e.trim()).join('\n')
        : ''
      contactQRs.value = Array.isArray(pc.qr_items)
        ? pc.qr_items
            .map((x) => {
              if (!x || typeof x !== 'object') return { label: '', image_url: '' }
              const o = x as { label?: unknown; image_url?: unknown }
              return {
                label: typeof o.label === 'string' ? o.label : '',
                image_url: typeof o.image_url === 'string' ? o.image_url : '',
              }
            })
            .filter((x) => x.image_url.trim() !== '')
        : []
    } else {
      contactTitle.value = ''
      contactEmailsText.value = ''
      contactQRs.value = []
    }
  }

  async function load() {
    err.value = ''
    msg.value = ''
    await runLoad(async () => {
      const { data } = await http.get<Envelope<SiteSettingsPayload>>('/api/admin/site-settings', {
        params: { _: Date.now() },
      })
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminCommon.loadFail'))
      applyPayload(data.data ?? {})
    }).catch((e: unknown) => {
      err.value = httpErr(e)
    })
  }

  async function loadBrandIcons() {
    try {
      const { data } = await http.get<Envelope<{ icons?: BrandLibIcon[] }>>('/api/admin/site-brand/icons', {
        params: { _: Date.now() },
      })
      if (!isEnvelopeSuccess(data) || !Array.isArray(data.data?.icons)) {
        brandIcons.value = []
        return
      }
      brandIcons.value = data.data.icons.filter((x) => x && typeof x.url === 'string' && x.url.trim())
    } catch {
      brandIcons.value = []
    }
  }

  function brandIconTitle(ico: BrandLibIcon): string {
    if (ico.is_builtin_default) return t('adminSystem.brandIconDefault')
    const name = typeof ico.filename === 'string' && ico.filename.trim() ? ico.filename.trim() : ico.url
    const ts = typeof ico.uploaded_at === 'number' && ico.uploaded_at > 0 ? ico.uploaded_at * 1000 : 0
    const when = ts ? new Date(ts).toLocaleString() : ''
    return when ? `${name}\n${when}` : name
  }

  function isBrandIconTileSelected(ico: BrandLibIcon): boolean {
    if (ico.is_builtin_default) {
      const u = form.brandIconUrl.trim()
      return u === '' || u === defaultBrandIconUrl
    }
    return form.brandIconUrl.trim() === ico.url.trim()
  }

  function selectBrandIcon(ico: BrandLibIcon) {
    contactPickQrIndex.value = null
    if (ico.is_builtin_default) {
      form.brandIconUrl = ''
      return
    }
    form.brandIconUrl = ico.url.trim()
  }

  async function confirmDeleteBrandIcon(ico: BrandLibIcon) {
    if (ico.is_builtin_default) return
    const fn = typeof ico.filename === 'string' ? ico.filename.trim() : ''
    if (!fn) return
    if (!globalThis.confirm(t('adminSystem.brandIconDeleteConfirm', { name: fn }))) return
    deletingBrandFilename.value = fn
    err.value = ''
    msg.value = ''
    try {
      const { data } = await http.delete<Envelope<{ cleared_config?: boolean }>>(
        `/api/admin/site-brand/icons/${encodeURIComponent(fn)}`,
      )
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.brandIconDeleteFail'))
      if (form.brandIconUrl.trim() === ico.url.trim()) {
        form.brandIconUrl = ''
      }
      await refresh()
      await siteMeta.refreshFromServer()
      msg.value = t('adminSystem.brandIconDeleted')
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      deletingBrandFilename.value = null
    }
  }

  function addContactQrRow() {
    contactPickQrIndex.value = null
    contactQRs.value.push({ label: '', image_url: '' })
  }

  function removeContactQrRow(i: number) {
    contactQRs.value.splice(i, 1)
    if (contactPickQrIndex.value === i) contactPickQrIndex.value = null
    else if (contactPickQrIndex.value !== null && contactPickQrIndex.value > i) contactPickQrIndex.value -= 1
  }

  function startPickContactLibrary(rowIdx: number) {
    contactPickQrIndex.value = contactPickQrIndex.value === rowIdx ? null : rowIdx
  }

  function applyContactLibraryIcon(ico: BrandLibIcon) {
    const idx = contactPickQrIndex.value
    if (idx === null || ico.is_builtin_default) return
    const row = contactQRs.value[idx]
    if (!row) return
    row.image_url = ico.url.trim()
    contactPickQrIndex.value = null
    msg.value = t('adminSystem.contactPickedFromLibrary')
  }

  async function uploadContactQr(ev: Event, rowIdx: number) {
    const input = ev.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    uploadingContactRow.value = rowIdx
    err.value = ''
    msg.value = ''
    try {
      const fd = new FormData()
      fd.append('file', file)
      const { data } = await http.post<Envelope<{ url?: string }>>('/api/admin/site-upload/contact-qr', fd)
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.contactQrUploadFail'))
      const u = data.data && typeof data.data.url === 'string' ? data.data.url.trim() : ''
      if (!u) throw new Error(t('adminSystem.contactQrUploadFail'))
      const row = contactQRs.value[rowIdx]
      if (row) row.image_url = u
      await loadBrandIcons()
      msg.value = t('adminSystem.contactQrUploaded')
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      uploadingContactRow.value = null
    }
  }

  function buildPublicContactPayload(): {
    title: string
    qr_items: { label: string; image_url: string }[]
    emails: string[]
  } {
    const emails = contactEmailsText.value
      .split(/[\n,;]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    const qr_items = contactQRs.value
      .map((r) => ({ label: r.label.trim(), image_url: r.image_url.trim() }))
      .filter((r) => r.label !== '' || r.image_url !== '')
    return { title: contactTitle.value.trim(), qr_items, emails }
  }

  async function refresh() {
    await load()
    await loadBrandIcons()
  }

  async function uploadBrandIcon(ev: Event) {
    const input = ev.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    contactPickQrIndex.value = null
    uploadingIcon.value = true
    err.value = ''
    msg.value = ''
    try {
      const fd = new FormData()
      fd.append('file', file)
      const { data } = await http.post<Envelope<{ url?: string }>>('/api/admin/site-upload/brand-icon', fd)
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.brandIconUploadFail'))
      const u = data.data && typeof data.data.url === 'string' ? data.data.url.trim() : ''
      if (!u) throw new Error(t('adminSystem.brandIconUploadFail'))
      form.brandIconUrl = u
      await refresh()
      await siteMeta.refreshFromServer()
      msg.value = t('adminSystem.brandIconUploaded')
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      uploadingIcon.value = false
    }
  }

  async function save() {
    saving.value = true
    err.value = ''
    msg.value = ''
    const contactPayload = buildPublicContactPayload()
    for (const r of contactPayload.qr_items) {
      if (r.label && !r.image_url) {
        err.value = t('adminSystem.contactQrLabelNeedsUrl')
        saving.value = false
        return
      }
    }
    const items = [
      { key: 'site.brand.name', value: form.brandName.trim(), description: t('adminSystem.descBrandName') },
      { key: 'site.brand.icon_url', value: form.brandIconUrl.trim(), description: t('adminSystem.descBrandIcon') },
      {
        key: 'site.login_captcha.enabled',
        value: form.loginCaptcha ? 'true' : 'false',
        description: t('adminSystem.descLoginCaptcha'),
      },
      {
        key: 'gateway.ip_blacklist',
        value: JSON.stringify(gatewayIpBlacklistRows.value.map((r) => r.value.trim()).filter(Boolean)),
        description: t('adminSystem.descGatewayIpBlacklist'),
      },
      {
        key: 'site.operator_display_name',
        value: form.operatorDisplayName.trim(),
        description: t('adminSystem.descOperator'),
      },
      {
        key: 'site.default_new_user_balance',
        value: String(Math.max(0, Math.trunc(Number(form.defaultNewUserBalance)) || 0)),
        description: t('adminSystem.descDefaultNewUserBalance'),
      },
      {
        key: 'site.register_promo_text',
        value: form.registerPromoText.trim(),
        description: t('adminSystem.descRegisterPromo'),
      },
      {
        key: 'site.no_bill_models',
        value: JSON.stringify(
          form.noBillModelsText
            .split('\n')
            .map((s) => s.trim())
            .filter(Boolean),
        ),
        description: t('adminSystem.descNoBillModels'),
      },
      {
        key: 'site.api.public_base_url',
        value: form.apiPublicBaseUrl.trim(),
        description: t('adminSystem.descApiPublicBaseUrl'),
      },
      {
        key: 'site.public_contact',
        value: JSON.stringify(contactPayload),
        description: t('adminSystem.descPublicContact'),
      },
      {
        key: 'upstream_model_probe.enabled',
        value: form.upstreamProbeEnabled ? 'true' : 'false',
        description: t('adminSystem.descUpstreamProbeEnabled'),
      },
      {
        key: 'upstream_model_probe.interval_minutes',
        value: String(Math.min(1440, Math.max(15, Math.trunc(Number(form.upstreamProbeInterval)) || 360))),
        description: t('adminSystem.descUpstreamProbeInterval'),
      },
      {
        key: 'upstream_capability.fail_stale_hours',
        value: String(Math.min(168, Math.max(0, Math.trunc(Number(form.upstreamCapabilityFailStaleHours)) || 6))),
        description: t('adminSystem.descUpstreamCapabilityFailStaleHours'),
      },
      {
        key: 'upstream_capability_probe.enabled',
        value: form.upstreamCapabilityProbeEnabled ? 'true' : 'false',
        description: t('adminSystem.descUpstreamCapabilityProbeEnabled'),
      },
      {
        key: 'gateway.routing.strategy_default',
        value: form.routingStrategy,
        description: t('adminSystem.descRoutingStrategy'),
      },
      {
        key: 'gateway.routing.failover_enabled_default',
        value: form.routingFailoverEnabled ? 'true' : 'false',
        description: t('adminSystem.descRoutingFailover'),
      },
      {
        key: 'gateway.routing.failover_max_attempts_default',
        value: String(Math.min(5, Math.max(1, Math.trunc(Number(form.routingFailoverMaxAttempts)) || 2))),
        description: t('adminSystem.descRoutingFailoverMax'),
      },
      {
        key: 'gateway.routing.sticky_enabled_default',
        value: form.routingStickyEnabled ? 'true' : 'false',
        description: t('adminSystem.descRoutingSticky'),
      },
      {
        key: 'gateway.routing.auto_health_from_probe',
        value: form.routingAutoHealthFromProbe ? 'true' : 'false',
        description: t('adminSystem.descRoutingAutoHealth'),
      },
      {
        key: 'gateway.routing.probe_fail_threshold',
        value: String(Math.min(20, Math.max(1, Math.trunc(Number(form.routingProbeFailThreshold)) || 3))),
        description: t('adminSystem.descRoutingProbeThreshold'),
      },
    ]
    try {
      const { data } = await http.put<Envelope<unknown>>('/api/admin/config', { items })
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.saveFail'))
      await refresh()
      await siteMeta.refreshFromServer()
      msg.value = t('adminSystem.saved')
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      saving.value = false
    }
  }

  async function resetDefaults() {
    if (!globalThis.confirm(t('adminSystem.resetDefaultsConfirm'))) return
    resettingDefaults.value = true
    err.value = ''
    msg.value = ''
    try {
      const { data } = await http.post<Envelope<{ deleted_rows?: number }>>(
        '/api/admin/site-settings/reset-defaults',
        {},
        { headers: { 'Content-Type': 'application/json' } },
      )
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.resetDefaultsFail'))
      await refresh()
      await siteMeta.refreshFromServer()
      brandPreviewNonce.value += 1
      msg.value = t('adminSystem.resetDefaultsDone')
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      resettingDefaults.value = false
    }
  }

  return reactive({
    t,
    siteMeta,
    form,
    contactTitle,
    contactEmailsText,
    contactQRs,
    contactPickQrIndex,
    uploadingContactRow,
    busy,
    busyUi,
    saving,
    uploadingIcon,
    msg,
    err,
    brandIcons,
    deletingBrandFilename,
    resettingDefaults,
    brandPreviewNonce,
    defaultNewUserBalanceYaml,
    defaultNewUserBalanceDbOverride,
    registerPromoTextYaml,
    registerPromoTextDbOverride,
    noBillModelsYaml,
    noBillModelsDbOverride,
    upstreamProbeEnabledDbOverride,
    upstreamProbeIntervalDbOverride,
    gatewayIpBlacklistDbOverride,
    gatewayIpBlacklistRows,
    addGatewayIpBlacklistRow,
    removeGatewayIpBlacklistRow,
    brandIconPreviewSrc,
    brandIconsForUi,
    brandIconsUploadedOnly,
    brandIconTitle,
    isBrandIconTileSelected,
    selectBrandIcon,
    confirmDeleteBrandIcon,
    addContactQrRow,
    removeContactQrRow,
    startPickContactLibrary,
    applyContactLibraryIcon,
    uploadContactQr,
    refresh,
    uploadBrandIcon,
    save,
    resetDefaults,
  })
}

export type AdminSiteSettingsContext = ReturnType<typeof createAdminSiteSettings>

export const adminSiteSettingsKey: InjectionKey<AdminSiteSettingsContext> = Symbol('adminSiteSettings')
