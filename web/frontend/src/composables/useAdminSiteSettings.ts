import axios from 'axios'
import type { InjectionKey } from 'vue'
import { computed, reactive, ref, watch } from 'vue'
import { useSmoothLoading } from './useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import {
  DEFAULT_SELECTIVE_FAILOVER_4XX_CODES,
  parseFailover4xxCodes,
} from '../constants/failover4xx'
import { useSiteMetaStore, type PublicContactPayload } from '../stores/siteMeta'
import defaultContactCardAsset from '../assets/xunji-contact-card.png'

/** 与后端 sitecfg.DefaultBrandIconURL 一致 */
export const defaultBrandIconUrl = '/tracehub-icon.svg'
export const defaultBrandFaviconUrl = '/tracehub-icon.svg'
/** 内置企微名片（仅预览；表单空时不写入库） */
export const defaultContactCardPreviewUrl = defaultContactCardAsset

export type SiteImageFieldId = 'icon' | 'favicon' | 'og' | 'contactCard'

export type BrandLibIcon = {
  url: string
  uploaded_at?: number
  filename?: string
  is_builtin_default?: boolean
  /** OG 等「无图」内置项 */
  is_builtin_no_image?: boolean
}

export type SiteSettingsPayload = {
  brand_name?: string
  brand_icon_url?: string
  brand_favicon_url?: string
  brand_og_image_url?: string
  brand_contact_card_url?: string
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
  beian_footer_text?: string
  beian_footer_text_file?: string
  beian_footer_text_db_set?: boolean
  beian_footer_text_db_value?: string
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
  routing_failover_first_attempt_timeout_sec?: number
  routing_failover_retry_attempt_timeout_sec?: number
  routing_failover_retry_after_sec?: number
  routing_failover_4xx_mode_default?: string
  routing_failover_4xx_codes_default?: number[] | string
  routing_sticky_enabled_default?: boolean
  routing_auto_health_from_probe?: boolean
  routing_probe_fail_threshold?: number
  upstream_timeout_standard_sec?: number
  upstream_timeout_stream_sec?: number
  upstream_timeout_image_sec?: number
  upstream_timeout_audio_sec?: number
  upstream_timeout_video_sec?: number
  gateway_ip_blacklist?: string[]
  gateway_ip_blacklist_db_set?: boolean
  https_force_redirect?: boolean
  https_hsts_enabled?: boolean
  https_hsts_max_age_seconds?: number
  tls_enabled?: boolean
  tls_cert_file?: string
  tls_key_file?: string
  tls_addr?: string
  tls_cert_present?: boolean
  tls_key_present?: boolean
  tls_cert_file_db_set?: boolean
  tls_key_file_db_set?: boolean
  tls_material_ready?: boolean
  tls_pair_ok?: boolean
  /** 以下三项来自运行配置 YAML，供全站计价换算等只读展示 */
  currency_quota_per_yuan?: number
  currency_quota_per_usd?: number
  pricing_cny_per_usd?: number
  gateway_admin_alert_enabled?: boolean
  gateway_admin_alert_enabled_db_set?: boolean
  gateway_admin_alert_config?: GatewayAdminAlertWireConfig | null
  gateway_admin_alert_config_db_set?: boolean
  gateway_admin_alert_secrets_configured?: Record<string, boolean>
}

export type GatewayAdminAlertWireConfig = {
  triggers?: {
    notify_5xx?: boolean
    notify_transport?: boolean
    notify_failover?: boolean
    notify_4xx?: boolean
    notify_402?: boolean
  }
  channels?: {
    email?: boolean
    dingtalk?: boolean
    feishu?: boolean
    wecom?: boolean
    generic_webhook?: boolean
    in_app?: boolean
  }
  emails?: string[]
  dingtalk_webhook?: string
  feishu_webhook?: string
  wecom_webhook?: string
  generic_webhook_url?: string
  cooldown_sec?: number
  min_interval_sec?: number
  include_user_email?: boolean
}

export function createAdminSiteSettings() {
  const { t } = useI18n()
  const siteMeta = useSiteMetaStore()

  const form = reactive({
    brandName: '',
    brandIconUrl: '',
    brandFaviconUrl: '',
    brandOgImageUrl: '',
    brandContactCardUrl: '',
    loginCaptcha: false,
    operatorDisplayName: '',
    defaultNewUserBalance: 0,
    registerPromoText: '',
    beianFooterText: '',
    noBillModelsText: '',
    apiPublicBaseUrl: '',
    upstreamProbeEnabled: false,
    upstreamProbeInterval: 360,
    upstreamCapabilityFailStaleHours: 6,
    upstreamCapabilityProbeEnabled: false,
    routingStrategy: 'weighted_random',
    routingFailoverEnabled: true,
    routingFailoverMaxAttempts: 2,
    routingFailoverFirstTimeout: 30,
    routingFailoverRetryTimeout: 0,
    routingFailoverRetryAfter: 15,
    routingFailover4xxMode: 'off',
    routingFailover4xxCodes: [] as number[],
    routingStickyEnabled: true,
    routingAutoHealthFromProbe: false,
    routingProbeFailThreshold: 3,
    upstreamTimeoutStandard: 120,
    upstreamTimeoutStream: 120,
    upstreamTimeoutImage: 600,
    upstreamTimeoutAudio: 600,
    upstreamTimeoutVideo: 1800,
    httpsForceRedirect: false,
    httpsHstsEnabled: true,
    httpsHstsMaxAgeSeconds: 31536000,
    tlsEnabled: false,
    gatewayAdminAlertEnabled: false,
    gwAlertNotify5xx: true,
    gwAlertNotifyTransport: true,
    gwAlertNotifyFailover: false,
    gwAlertNotify4xx: false,
    gwAlertNotify402: false,
    gwAlertChannelEmail: false,
    gwAlertChannelDingtalk: false,
    gwAlertChannelFeishu: false,
    gwAlertChannelWecom: false,
    gwAlertChannelGenericWebhook: false,
    gwAlertChannelInApp: true,
    gwAlertEmailsText: '',
    gwAlertDingtalkWebhook: '',
    gwAlertFeishuWebhook: '',
    gwAlertWecomWebhook: '',
    gwAlertGenericWebhookUrl: '',
    gwAlertCooldownSec: 300,
    gwAlertMinIntervalSec: 30,
  })

  watch(
    () => form.routingFailover4xxMode,
    (mode) => {
      if (mode === 'selective' && form.routingFailover4xxCodes.length === 0) {
        form.routingFailover4xxCodes = [...DEFAULT_SELECTIVE_FAILOVER_4XX_CODES]
      }
    },
  )

  const tlsCertFile = ref('')
  const tlsKeyFile = ref('')
  const tlsAddr = ref(':443')
  const tlsCertPresent = ref(false)
  const tlsKeyPresent = ref(false)
  const tlsCertDbOverride = ref(false)
  const tlsKeyDbOverride = ref(false)
  const uploadingTlsCert = ref(false)
  const uploadingTlsKey = ref(false)
  const tlsMaterialReady = ref(false)

  const contactTitle = ref('')
  const contactEmailsText = ref('')
  const contactQRs = ref<Array<{ label: string; image_url: string }>>([])
  const contactPickQrIndex = ref<number | null>(null)
  const uploadingContactRow = ref<number | null>(null)
  const { busy, visible: busyUi, run: runLoad } = useSmoothLoading()
  const saving = ref(false)
  const uploadingImageField = ref<SiteImageFieldId | null>(null)
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

  const gwAlertSecretDing = ref('')
  const gwAlertSecretFeishu = ref('')
  const gwAlertSecretWecom = ref('')
  const gwAlertSecretGeneric = ref('')
  const gwAlertClearDingSign = ref(false)
  const gwAlertClearFeishuSign = ref(false)
  const gwAlertClearWecomSign = ref(false)
  const gwAlertClearGenericSign = ref(false)
  const gwAlertSignConfigured = ref<Record<string, boolean>>({})

  function addGatewayIpBlacklistRow() {
    gatewayIpBlacklistRows.value.push({ value: '' })
  }

  function removeGatewayIpBlacklistRow(idx: number) {
    gatewayIpBlacklistRows.value.splice(idx, 1)
  }

  const brandIconPreviewSrc = computed(() => previewForUrl(form.brandIconUrl, defaultBrandIconUrl))
  const brandFaviconPreviewSrc = computed(() => previewForUrl(form.brandFaviconUrl, defaultBrandFaviconUrl))
  const brandOgPreviewSrc = computed(() => {
    const u = form.brandOgImageUrl.trim()
    return u || ''
  })
  const brandContactCardPreviewSrc = computed(() =>
    previewForUrl(form.brandContactCardUrl, defaultContactCardPreviewUrl),
  )

  function previewForUrl(stored: string, fallback: string): string {
    const u = stored.trim()
    return u || fallback
  }

  function iconsForUiWithBuiltin(
    storedUrl: string,
    builtin: BrandLibIcon,
    excludeUrl?: string,
  ): BrandLibIcon[] {
    const rest = brandIcons.value.filter((x) => {
      const u = x.url.trim()
      if (!u) return false
      if (excludeUrl && u === excludeUrl) return false
      return true
    })
    return [builtin, ...rest]
  }

  const brandIconsForUi = computed((): BrandLibIcon[] =>
    iconsForUiWithBuiltin(form.brandIconUrl, {
      url: defaultBrandIconUrl,
      uploaded_at: 0,
      filename: 'default',
      is_builtin_default: true,
    }, defaultBrandIconUrl),
  )

  const brandFaviconIconsForUi = computed((): BrandLibIcon[] =>
    iconsForUiWithBuiltin(form.brandFaviconUrl, {
      url: defaultBrandFaviconUrl,
      uploaded_at: 0,
      filename: 'default',
      is_builtin_default: true,
    }, defaultBrandFaviconUrl),
  )

  const brandOgIconsForUi = computed((): BrandLibIcon[] =>
    iconsForUiWithBuiltin(form.brandOgImageUrl, {
      url: '',
      uploaded_at: 0,
      filename: 'default',
      is_builtin_default: true,
      is_builtin_no_image: true,
    }),
  )

  const brandContactCardIconsForUi = computed((): BrandLibIcon[] =>
    iconsForUiWithBuiltin(form.brandContactCardUrl, {
      url: defaultContactCardPreviewUrl,
      uploaded_at: 0,
      filename: 'default',
      is_builtin_default: true,
    }),
  )

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
    form.brandFaviconUrl = typeof d.brand_favicon_url === 'string' ? d.brand_favicon_url : ''
    form.brandOgImageUrl = typeof d.brand_og_image_url === 'string' ? d.brand_og_image_url : ''
    form.brandContactCardUrl = typeof d.brand_contact_card_url === 'string' ? d.brand_contact_card_url : ''
    form.loginCaptcha = Boolean(d.login_captcha_enabled)
    form.operatorDisplayName = typeof d.operator_display_name === 'string' ? d.operator_display_name : ''
    form.defaultNewUserBalance = pickInt64ish(d.default_new_user_balance)
    defaultNewUserBalanceYaml.value = pickInt64ish(d.default_new_user_balance_file)
    defaultNewUserBalanceDbOverride.value = Boolean(d.default_new_user_balance_db_set)
    form.registerPromoText = typeof d.register_promo_text === 'string' ? d.register_promo_text : ''
    registerPromoTextYaml.value = typeof d.register_promo_text_file === 'string' ? d.register_promo_text_file : ''
    registerPromoTextDbOverride.value = Boolean(d.register_promo_text_db_set)
    form.beianFooterText = typeof d.beian_footer_text === 'string' ? d.beian_footer_text : ''
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
    form.routingFailoverFirstTimeout = pickInt64ish(d.routing_failover_first_attempt_timeout_sec) || 30
    form.routingFailoverRetryTimeout = pickInt64ish(d.routing_failover_retry_attempt_timeout_sec) ?? 0
    form.routingFailoverRetryAfter = pickInt64ish(d.routing_failover_retry_after_sec) || 15
    const m4 = String(d.routing_failover_4xx_mode_default || 'off').trim()
    form.routingFailover4xxMode =
      m4 === 'selective' || m4 === 'aggressive' || m4 === 'custom' ? m4 : 'off'
    form.routingFailover4xxCodes = parseFailover4xxCodes(d.routing_failover_4xx_codes_default)
    if (form.routingFailover4xxMode === 'selective' && form.routingFailover4xxCodes.length === 0) {
      form.routingFailover4xxCodes = [...DEFAULT_SELECTIVE_FAILOVER_4XX_CODES]
    }
    form.routingStickyEnabled = d.routing_sticky_enabled_default !== false
    form.routingAutoHealthFromProbe = Boolean(d.routing_auto_health_from_probe)
    form.routingProbeFailThreshold = pickInt64ish(d.routing_probe_fail_threshold) || 3
    const utStd = pickInt64ish(d.upstream_timeout_standard_sec)
    form.upstreamTimeoutStandard = utStd >= 10 && utStd <= 600 ? utStd : 120
    const utStream = pickInt64ish(d.upstream_timeout_stream_sec)
    form.upstreamTimeoutStream = utStream >= 10 && utStream <= 600 ? utStream : 120
    const utImg = pickInt64ish(d.upstream_timeout_image_sec)
    form.upstreamTimeoutImage = utImg >= 30 && utImg <= 3600 ? utImg : 600
    const utAud = pickInt64ish(d.upstream_timeout_audio_sec)
    form.upstreamTimeoutAudio = utAud >= 30 && utAud <= 3600 ? utAud : 600
    const utVid = pickInt64ish(d.upstream_timeout_video_sec)
    form.upstreamTimeoutVideo = utVid >= 60 && utVid <= 7200 ? utVid : 1800
    const gwList = Array.isArray(d.gateway_ip_blacklist)
      ? d.gateway_ip_blacklist.filter((x): x is string => typeof x === 'string' && x.trim())
      : []
    gatewayIpBlacklistRows.value = gwList.map((ip) => ({ value: ip }))
    gatewayIpBlacklistDbOverride.value = Boolean(d.gateway_ip_blacklist_db_set)
    form.httpsForceRedirect = Boolean(d.https_force_redirect)
    form.httpsHstsEnabled = d.https_hsts_enabled !== false
    const hstsAge = pickInt64ish(d.https_hsts_max_age_seconds)
    form.httpsHstsMaxAgeSeconds = hstsAge > 0 ? Math.min(63072000, hstsAge) : 31536000
    form.tlsEnabled = Boolean(d.tls_enabled)
    tlsCertFile.value = typeof d.tls_cert_file === 'string' ? d.tls_cert_file : ''
    tlsKeyFile.value = typeof d.tls_key_file === 'string' ? d.tls_key_file : ''
    tlsAddr.value = typeof d.tls_addr === 'string' && d.tls_addr.trim() ? d.tls_addr : ':443'
    tlsCertPresent.value = Boolean(d.tls_cert_present)
    tlsKeyPresent.value = Boolean(d.tls_key_present)
    tlsCertDbOverride.value = Boolean(d.tls_cert_file_db_set)
    tlsKeyDbOverride.value = Boolean(d.tls_key_file_db_set)
    tlsMaterialReady.value = Boolean(d.tls_material_ready)
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
    form.gatewayAdminAlertEnabled = Boolean(d.gateway_admin_alert_enabled)
    const gac = d.gateway_admin_alert_config
    if (gac && typeof gac === 'object') {
      form.gwAlertNotify5xx = gac.triggers?.notify_5xx !== false
      form.gwAlertNotifyTransport = gac.triggers?.notify_transport !== false
      form.gwAlertNotifyFailover = Boolean(gac.triggers?.notify_failover)
      form.gwAlertNotify4xx = Boolean(gac.triggers?.notify_4xx)
      form.gwAlertNotify402 = Boolean(gac.triggers?.notify_402)
      form.gwAlertChannelEmail = Boolean(gac.channels?.email)
      form.gwAlertChannelDingtalk = Boolean(gac.channels?.dingtalk)
      form.gwAlertChannelFeishu = Boolean(gac.channels?.feishu)
      form.gwAlertChannelWecom = Boolean(gac.channels?.wecom)
      form.gwAlertChannelGenericWebhook = Boolean(gac.channels?.generic_webhook)
      form.gwAlertChannelInApp = gac.channels?.in_app !== false
      form.gwAlertEmailsText = Array.isArray(gac.emails)
        ? gac.emails.filter((e): e is string => typeof e === 'string' && e.trim()).join('\n')
        : ''
      form.gwAlertDingtalkWebhook = typeof gac.dingtalk_webhook === 'string' ? gac.dingtalk_webhook : ''
      form.gwAlertFeishuWebhook = typeof gac.feishu_webhook === 'string' ? gac.feishu_webhook : ''
      form.gwAlertWecomWebhook = typeof gac.wecom_webhook === 'string' ? gac.wecom_webhook : ''
      form.gwAlertGenericWebhookUrl = typeof gac.generic_webhook_url === 'string' ? gac.generic_webhook_url : ''
      form.gwAlertCooldownSec = pickInt64ish(gac.cooldown_sec) || 300
      form.gwAlertMinIntervalSec = pickInt64ish(gac.min_interval_sec) || 30
    }
    gwAlertSignConfigured.value =
      d.gateway_admin_alert_secrets_configured && typeof d.gateway_admin_alert_secrets_configured === 'object'
        ? { ...d.gateway_admin_alert_secrets_configured }
        : {}
    gwAlertSecretDing.value = ''
    gwAlertSecretFeishu.value = ''
    gwAlertSecretWecom.value = ''
    gwAlertSecretGeneric.value = ''
    gwAlertClearDingSign.value = false
    gwAlertClearFeishuSign.value = false
    gwAlertClearWecomSign.value = false
    gwAlertClearGenericSign.value = false
  }

  function buildGatewayAdminAlertConfig(): GatewayAdminAlertWireConfig {
    return {
      triggers: {
        notify_5xx: form.gwAlertNotify5xx,
        notify_transport: form.gwAlertNotifyTransport,
        notify_failover: form.gwAlertNotifyFailover,
        notify_4xx: form.gwAlertNotify4xx,
        notify_402: form.gwAlertNotify402,
      },
      channels: {
        email: form.gwAlertChannelEmail,
        dingtalk: form.gwAlertChannelDingtalk,
        feishu: form.gwAlertChannelFeishu,
        wecom: form.gwAlertChannelWecom,
        generic_webhook: form.gwAlertChannelGenericWebhook,
        in_app: form.gwAlertChannelInApp,
      },
      emails: form.gwAlertEmailsText
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean),
      dingtalk_webhook: form.gwAlertDingtalkWebhook.trim(),
      feishu_webhook: form.gwAlertFeishuWebhook.trim(),
      wecom_webhook: form.gwAlertWecomWebhook.trim(),
      generic_webhook_url: form.gwAlertGenericWebhookUrl.trim(),
      cooldown_sec: Math.min(3600, Math.max(60, Math.trunc(Number(form.gwAlertCooldownSec)) || 300)),
      min_interval_sec: Math.min(600, Math.max(10, Math.trunc(Number(form.gwAlertMinIntervalSec)) || 30)),
    }
  }

  function buildGatewayAdminAlertSecretsPatch(): Record<string, string> | null {
    const sec: Record<string, string> = {}
    if (gwAlertClearDingSign.value) sec.dingtalk = ''
    else if (gwAlertSecretDing.value.trim()) sec.dingtalk = gwAlertSecretDing.value.trim()
    if (gwAlertClearFeishuSign.value) sec.feishu = ''
    else if (gwAlertSecretFeishu.value.trim()) sec.feishu = gwAlertSecretFeishu.value.trim()
    if (gwAlertClearWecomSign.value) sec.wecom = ''
    else if (gwAlertSecretWecom.value.trim()) sec.wecom = gwAlertSecretWecom.value.trim()
    if (gwAlertClearGenericSign.value) sec.generic_webhook = ''
    else if (gwAlertSecretGeneric.value.trim()) sec.generic_webhook = gwAlertSecretGeneric.value.trim()
    return Object.keys(sec).length ? sec : null
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

  function brandIconTitle(ico: BrandLibIcon, field: SiteImageFieldId = 'icon'): string {
    if (ico.is_builtin_default) {
      if (field === 'og') return t('adminSystem.brandOgDefault')
      if (field === 'contactCard') return t('adminSystem.brandContactCardDefault')
      if (field === 'favicon') return t('adminSystem.brandFaviconDefault')
      return t('adminSystem.brandIconDefault')
    }
    const name = typeof ico.filename === 'string' && ico.filename.trim() ? ico.filename.trim() : ico.url
    const ts = typeof ico.uploaded_at === 'number' && ico.uploaded_at > 0 ? ico.uploaded_at * 1000 : 0
    const when = ts ? new Date(ts).toLocaleString() : ''
    return when ? `${name}\n${when}` : name
  }

  function storedUrlForField(field: SiteImageFieldId): string {
    switch (field) {
      case 'icon':
        return form.brandIconUrl.trim()
      case 'favicon':
        return form.brandFaviconUrl.trim()
      case 'og':
        return form.brandOgImageUrl.trim()
      case 'contactCard':
        return form.brandContactCardUrl.trim()
    }
  }

  function setStoredUrlForField(field: SiteImageFieldId, url: string) {
    switch (field) {
      case 'icon':
        form.brandIconUrl = url
        break
      case 'favicon':
        form.brandFaviconUrl = url
        break
      case 'og':
        form.brandOgImageUrl = url
        break
      case 'contactCard':
        form.brandContactCardUrl = url
        break
    }
  }

  function isBuiltinSelected(field: SiteImageFieldId, stored: string): boolean {
    if (stored !== '') {
      if (field === 'icon' && stored === defaultBrandIconUrl) return true
      if (field === 'favicon' && stored === defaultBrandFaviconUrl) return true
      return false
    }
    return true
  }

  function isImageTileSelected(field: SiteImageFieldId, ico: BrandLibIcon): boolean {
    const stored = storedUrlForField(field)
    if (ico.is_builtin_default) return isBuiltinSelected(field, stored)
    return stored === ico.url.trim()
  }

  function isBrandIconTileSelected(ico: BrandLibIcon): boolean {
    return isImageTileSelected('icon', ico)
  }

  function selectLibraryIcon(field: SiteImageFieldId, ico: BrandLibIcon) {
    contactPickQrIndex.value = null
    if (ico.is_builtin_default) {
      setStoredUrlForField(field, '')
      return
    }
    setStoredUrlForField(field, ico.url.trim())
  }

  function selectBrandIcon(ico: BrandLibIcon) {
    selectLibraryIcon('icon', ico)
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
      for (const field of ['icon', 'favicon', 'og', 'contactCard'] as const) {
        if (storedUrlForField(field) === ico.url.trim()) {
          setStoredUrlForField(field, '')
        }
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

  async function uploadSiteImage(
    ev: Event,
    field: SiteImageFieldId,
    endpoint: string,
    failKey: string,
    successKey: string,
  ) {
    const input = ev.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    contactPickQrIndex.value = null
    uploadingImageField.value = field
    err.value = ''
    msg.value = ''
    try {
      const fd = new FormData()
      fd.append('file', file)
      const { data } = await http.post<Envelope<{ url?: string }>>(endpoint, fd)
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t(failKey))
      const u = data.data && typeof data.data.url === 'string' ? data.data.url.trim() : ''
      if (!u) throw new Error(t(failKey))
      setStoredUrlForField(field, u)
      await refresh()
      await siteMeta.refreshFromServer()
      brandPreviewNonce.value += 1
      msg.value = t(successKey)
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      uploadingImageField.value = null
    }
  }

  function uploadBrandIcon(ev: Event) {
    return uploadSiteImage(
      ev,
      'icon',
      '/api/admin/site-upload/brand-icon',
      'adminSystem.brandIconUploadFail',
      'adminSystem.brandIconUploaded',
    )
  }

  function uploadBrandFavicon(ev: Event) {
    return uploadSiteImage(
      ev,
      'favicon',
      '/api/admin/site-upload/brand-favicon',
      'adminSystem.brandFaviconUploadFail',
      'adminSystem.brandFaviconUploaded',
    )
  }

  function uploadBrandOgImage(ev: Event) {
    return uploadSiteImage(
      ev,
      'og',
      '/api/admin/site-upload/brand-og-image',
      'adminSystem.brandOgUploadFail',
      'adminSystem.brandOgUploaded',
    )
  }

  function uploadContactCard(ev: Event) {
    return uploadSiteImage(
      ev,
      'contactCard',
      '/api/admin/site-upload/contact-card',
      'adminSystem.brandContactCardUploadFail',
      'adminSystem.brandContactCardUploaded',
    )
  }

  async function uploadTlsFile(ev: Event, endpoint: string, failKey: string, successKey: string, busy: typeof uploadingTlsCert) {
    const input = ev.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = ''
    if (!file) return
    busy.value = true
    err.value = ''
    msg.value = ''
    try {
      const fd = new FormData()
      fd.append('file', file)
      const { data } = await http.post<Envelope<{ path?: string; pair_ok?: boolean }>>(endpoint, fd)
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t(failKey))
      await refresh()
      msg.value = typeof data.message === 'string' && data.message.trim() ? data.message : t(successKey)
    } catch (e: unknown) {
      err.value = httpErr(e)
    } finally {
      busy.value = false
    }
  }

  function uploadTlsCert(ev: Event) {
    return uploadTlsFile(
      ev,
      '/api/admin/site-upload/tls-cert',
      'adminSystem.tlsCertUploadFail',
      'adminSystem.tlsCertUploaded',
      uploadingTlsCert,
    )
  }

  function uploadTlsKey(ev: Event) {
    return uploadTlsFile(
      ev,
      '/api/admin/site-upload/tls-key',
      'adminSystem.tlsKeyUploadFail',
      'adminSystem.tlsKeyUploaded',
      uploadingTlsKey,
    )
  }

  function onTlsEnabledChange(ev: Event) {
    const el = ev.target as HTMLInputElement
    const want = el.checked
    if (want && !tlsMaterialReady.value) {
      el.checked = false
      err.value = t('adminSystem.errTLSNeedMaterial')
      msg.value = ''
      return
    }
    form.tlsEnabled = want
    err.value = ''
  }

  async function save() {
    saving.value = true
    err.value = ''
    msg.value = ''
    if (form.tlsEnabled && !tlsMaterialReady.value) {
      err.value = t('adminSystem.errTLSNeedMaterial')
      saving.value = false
      return
    }
    if (form.routingFailover4xxMode === 'custom' && form.routingFailover4xxCodes.length === 0) {
      err.value = t('adminGroups.errFailover4xxCodesRequired')
      saving.value = false
      return
    }
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
      { key: 'site.brand.favicon_url', value: form.brandFaviconUrl.trim(), description: t('adminSystem.descBrandFavicon') },
      { key: 'site.brand.og_image_url', value: form.brandOgImageUrl.trim(), description: t('adminSystem.descBrandOg') },
      {
        key: 'site.brand.contact_card_url',
        value: form.brandContactCardUrl.trim(),
        description: t('adminSystem.descBrandContactCard'),
      },
      {
        key: 'site.login_captcha.enabled',
        value: form.loginCaptcha ? 'true' : 'false',
        description: t('adminSystem.descLoginCaptcha'),
      },
      {
        key: 'site.https.force_redirect',
        value: form.httpsForceRedirect ? 'true' : 'false',
        description: t('adminSystem.chkHTTPSForceRedirect'),
      },
      {
        key: 'site.https.hsts_enabled',
        value: form.httpsHstsEnabled ? 'true' : 'false',
        description: t('adminSystem.chkHTTPSHSTS'),
      },
      {
        key: 'site.https.hsts_max_age_seconds',
        value: String(Math.min(63072000, Math.max(0, Math.trunc(Number(form.httpsHstsMaxAgeSeconds)) || 31536000))),
        description: t('adminSystem.lblHTTPSHSTSMaxAge'),
      },
      {
        key: 'server.tls.enabled',
        value: form.tlsEnabled ? 'true' : 'false',
        description: t('adminSystem.chkTLSEnabled'),
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
        key: 'site.beian_footer_text',
        value: form.beianFooterText.trim(),
        description: t('adminSystem.descBeianFooter'),
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
        key: 'gateway.routing.failover_first_attempt_timeout_sec',
        value: String(Math.min(120, Math.max(5, Math.trunc(Number(form.routingFailoverFirstTimeout)) || 30))),
        description: t('adminGroups.descFailoverFirstTimeout'),
      },
      {
        key: 'gateway.routing.failover_retry_attempt_timeout_sec',
        value: String(Math.min(600, Math.max(0, Math.trunc(Number(form.routingFailoverRetryTimeout)) || 0))),
        description: t('adminGroups.descFailoverRetryTimeout'),
      },
      {
        key: 'gateway.routing.failover_retry_after_sec',
        value: String(Math.min(300, Math.max(1, Math.trunc(Number(form.routingFailoverRetryAfter)) || 15))),
        description: t('adminGroups.descFailoverRetryAfter'),
      },
      {
        key: 'gateway.routing.failover_4xx_mode_default',
        value: form.routingFailover4xxMode,
        description: t('adminGroups.descFailover4xxMode'),
      },
      {
        key: 'gateway.routing.failover_4xx_codes_default',
        value:
          form.routingFailover4xxMode === 'selective' || form.routingFailover4xxMode === 'custom'
            ? form.routingFailover4xxCodes.join(',')
            : '',
        description: t('adminGroups.descFailover4xxCodes'),
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
      {
        key: 'gateway.upstream_timeout.standard_sec',
        value: String(Math.min(600, Math.max(10, Math.trunc(Number(form.upstreamTimeoutStandard)) || 120))),
        description: t('adminSystem.descUpstreamTimeoutStandard'),
      },
      {
        key: 'gateway.upstream_timeout.stream_sec',
        value: String(Math.min(600, Math.max(10, Math.trunc(Number(form.upstreamTimeoutStream)) || 120))),
        description: t('adminSystem.descUpstreamTimeoutStream'),
      },
      {
        key: 'gateway.upstream_timeout.image_sec',
        value: String(Math.min(3600, Math.max(30, Math.trunc(Number(form.upstreamTimeoutImage)) || 600))),
        description: t('adminSystem.descUpstreamTimeoutImage'),
      },
      {
        key: 'gateway.upstream_timeout.audio_sec',
        value: String(Math.min(3600, Math.max(30, Math.trunc(Number(form.upstreamTimeoutAudio)) || 600))),
        description: t('adminSystem.descUpstreamTimeoutAudio'),
      },
      {
        key: 'gateway.upstream_timeout.video_sec',
        value: String(Math.min(7200, Math.max(60, Math.trunc(Number(form.upstreamTimeoutVideo)) || 1800))),
        description: t('adminSystem.descUpstreamTimeoutVideo'),
      },
      {
        key: 'gateway.admin_alert.enabled',
        value: form.gatewayAdminAlertEnabled ? 'true' : 'false',
        description: t('adminSystem.descGatewayAdminAlertEnabled'),
      },
      {
        key: 'gateway.admin_alert.config',
        value: JSON.stringify(buildGatewayAdminAlertConfig()),
        description: t('adminSystem.descGatewayAdminAlertConfig'),
      },
    ]
    const secPatch = buildGatewayAdminAlertSecretsPatch()
    if (secPatch) {
      items.push({
        key: 'gateway.admin_alert.secrets',
        value: JSON.stringify(secPatch),
        description: t('adminSystem.descGatewayAdminAlertSecrets'),
      })
    }
    try {
      const { data } = await http.put<Envelope<unknown>>('/api/admin/config', { items })
      if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminSystem.saveFail'))
      await refresh()
      await siteMeta.refreshFromServer()
      msg.value = typeof data.message === 'string' && data.message.trim() ? data.message : t('adminSystem.saved')
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
    uploadingImageField,
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
    tlsCertFile,
    tlsKeyFile,
    tlsAddr,
    tlsCertPresent,
    tlsKeyPresent,
    tlsCertDbOverride,
    tlsKeyDbOverride,
    uploadingTlsCert,
    uploadingTlsKey,
    tlsMaterialReady,
    onTlsEnabledChange,
    uploadTlsCert,
    uploadTlsKey,
    gatewayIpBlacklistRows,
    addGatewayIpBlacklistRow,
    removeGatewayIpBlacklistRow,
    gwAlertSecretDing,
    gwAlertSecretFeishu,
    gwAlertSecretWecom,
    gwAlertSecretGeneric,
    gwAlertClearDingSign,
    gwAlertClearFeishuSign,
    gwAlertClearWecomSign,
    gwAlertClearGenericSign,
    gwAlertSignConfigured,
    brandIconPreviewSrc,
    brandFaviconPreviewSrc,
    brandOgPreviewSrc,
    brandContactCardPreviewSrc,
    brandIconsForUi,
    brandFaviconIconsForUi,
    brandOgIconsForUi,
    brandContactCardIconsForUi,
    brandIconsUploadedOnly,
    brandIconTitle,
    isBrandIconTileSelected,
    isImageTileSelected,
    selectBrandIcon,
    selectLibraryIcon,
    confirmDeleteBrandIcon,
    addContactQrRow,
    removeContactQrRow,
    startPickContactLibrary,
    applyContactLibraryIcon,
    uploadContactQr,
    refresh,
    uploadBrandIcon,
    uploadBrandFavicon,
    uploadBrandOgImage,
    uploadContactCard,
    save,
    resetDefaults,
  })
}

export type AdminSiteSettingsContext = ReturnType<typeof createAdminSiteSettings>

export const adminSiteSettingsKey: InjectionKey<AdminSiteSettingsContext> = Symbol('adminSiteSettings')
