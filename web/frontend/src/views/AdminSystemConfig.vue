<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { useSiteMetaStore, type PublicContactPayload } from '../stores/siteMeta'

const { t } = useI18n()
const siteMeta = useSiteMetaStore()

/** 与后端 sitecfg.DefaultBrandIconURL 一致 */
const defaultBrandIconUrl = '/tracehub-icon.svg'

type BrandLibIcon = {
  url: string
  uploaded_at?: number
  filename?: string
  /** 首项内置默认图标：点击后清空 URL，保存即恢复默认 */
  is_builtin_default?: boolean
}

type SiteSettings = {
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
  routing_strategy_default?: string
  routing_failover_enabled_default?: boolean
  routing_failover_max_attempts_default?: number
  routing_sticky_enabled_default?: boolean
  routing_auto_health_from_probe?: boolean
  routing_probe_fail_threshold?: number
}

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
const busy = ref(true)
const saving = ref(false)
const uploadingIcon = ref(false)
const msg = ref('')
const err = ref('')
const brandIcons = ref<BrandLibIcon[]>([])
const deletingBrandFilename = ref<string | null>(null)
const resettingDefaults = ref(false)
/** 恢复默认后递增，配合预览图 :key 避免浏览器强缓存 */
const brandPreviewNonce = ref(0)
/** 配置文件 site.default_new_user_balance（仅展示说明） */
const defaultNewUserBalanceYaml = ref(0)
/** 数据库是否已有该键的覆盖 */
const defaultNewUserBalanceDbOverride = ref(false)
/** 配置文件 site.register_promo_text（仅展示说明） */
const registerPromoTextYaml = ref('')
/** 数据库是否已有注册推广文案的覆盖 */
const registerPromoTextDbOverride = ref(false)
/** 配置文件 site.no_bill_models（仅展示） */
const noBillModelsYaml = ref('')
const noBillModelsDbOverride = ref(false)

/** 上游探测：数据库是否覆盖（仅展示） */
const upstreamProbeEnabledDbOverride = ref(false)
const upstreamProbeIntervalDbOverride = ref(false)

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

function applyPayload(d: SiteSettings) {
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
  form.routingStrategy =
    typeof d.routing_strategy_default === 'string' && d.routing_strategy_default.trim()
      ? d.routing_strategy_default
      : 'weighted_random'
  form.routingFailoverEnabled = Boolean(d.routing_failover_enabled_default)
  form.routingFailoverMaxAttempts = pickInt64ish(d.routing_failover_max_attempts_default) || 2
  form.routingStickyEnabled = d.routing_sticky_enabled_default !== false
  form.routingAutoHealthFromProbe = Boolean(d.routing_auto_health_from_probe)
  form.routingProbeFailThreshold = pickInt64ish(d.routing_probe_fail_threshold) || 3
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
  busy.value = true
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.get<Envelope<SiteSettings>>('/api/admin/site-settings', {
      params: { _: Date.now() },
    })
    if (!isEnvelopeSuccess(data)) throw new Error(data.message || t('adminCommon.loadFail'))
    applyPayload(data.data ?? {})
  } catch (e: unknown) {
    err.value = httpErr(e)
  } finally {
    busy.value = false
  }
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
    {
      key: 'site.brand.name',
      value: form.brandName.trim(),
      description: t('adminSystem.descBrandName'),
    },
    {
      key: 'site.brand.icon_url',
      value: form.brandIconUrl.trim(),
      description: t('adminSystem.descBrandIcon'),
    },
    {
      key: 'site.login_captcha.enabled',
      value: form.loginCaptcha ? 'true' : 'false',
      description: t('adminSystem.descLoginCaptcha'),
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

onMounted(() => void refresh())
</script>

<template>
  <div>
    <h1 class="text-xl font-semibold tracking-tight text-slate-100">{{ t('adminSystem.title') }}</h1>
    <p class="mt-1 text-sm text-slate-500">{{ t('adminSystem.intro') }}</p>

    <p v-if="err" class="th-admin-flash--error mt-4 rounded-lg border px-3 py-2 text-sm">{{ err }}</p>
    <p v-if="msg" class="th-admin-flash--success mt-4 rounded-lg border px-3 py-2 text-sm">{{ msg }}</p>

    <div v-if="busy" class="mt-8 text-sm text-slate-500">{{ t('common.loading') }}</div>

    <div v-else class="mt-8 space-y-8">
      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBrand') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBrandHint') }}</p>
        <div class="mt-5 space-y-4">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblBrandName') }}</label>
            <input v-model="form.brandName" type="text" class="th-input max-w-xl" autocomplete="off" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblBrandIcon') }}</label>
            <div class="flex max-w-xl flex-wrap items-center gap-2">
              <input v-model="form.brandIconUrl" type="text" class="th-input min-w-[12rem] flex-1 font-mono text-sm" autocomplete="off" />
              <label class="th-btn-secondary inline-flex cursor-pointer items-center px-3 py-2 text-xs font-medium select-none">
                <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" class="sr-only" :disabled="uploadingIcon || saving" @change="uploadBrandIcon" />
                {{ uploadingIcon ? t('adminSystem.brandIconUploading') : t('adminSystem.brandIconUpload') }}
              </label>
            </div>
            <div class="mt-3">
              <p class="text-[11px] font-medium text-slate-400">{{ t('adminSystem.brandIconLibrary') }}</p>
              <p class="mt-0.5 text-[11px] text-slate-500">{{ t('adminSystem.brandIconLibraryHint') }}</p>
              <div class="mt-2 flex max-w-xl flex-wrap gap-2">
                <div
                  v-for="ico in brandIconsForUi"
                  :key="ico.is_builtin_default ? '__builtin_default__' : ico.url"
                  class="relative h-11 w-11 shrink-0"
                >
                  <button
                    type="button"
                    :title="brandIconTitle(ico)"
                    class="h-11 w-11 overflow-hidden rounded-md border bg-black/25 p-0 transition-colors hover:border-white/25"
                    :class="
                      isBrandIconTileSelected(ico) ? 'border-cyan-500 ring-2 ring-cyan-500/30' : 'border-white/10'
                    "
                    @click="selectBrandIcon(ico)"
                  >
                    <img :src="ico.url.trim()" alt="" class="h-full w-full object-contain" />
                  </button>
                  <button
                    v-if="!ico.is_builtin_default && ico.filename"
                    type="button"
                    :title="t('adminSystem.brandIconDelete')"
                    :disabled="deletingBrandFilename !== null || saving || uploadingIcon"
                    class="absolute -right-1 -top-1 z-10 flex h-[18px] w-[18px] items-center justify-center rounded-full border border-red-500/40 bg-red-950/90 text-[11px] leading-none font-semibold text-red-100 shadow-sm hover:bg-red-900/95 disabled:cursor-not-allowed disabled:opacity-40"
                    @click.stop="void confirmDeleteBrandIcon(ico)"
                  >
                    ×
                  </button>
                </div>
              </div>
              <p v-if="brandIcons.length === 0" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.brandIconLibraryEmpty') }}</p>
            </div>
            <p class="mt-2 flex items-center gap-2 text-[11px] text-slate-500">
              <span>{{ t('adminSystem.brandIconPreview') }}</span>
              <img
                :key="`${brandIconPreviewSrc}|${brandPreviewNonce}`"
                :src="brandIconPreviewSrc"
                alt=""
                class="h-8 w-8 rounded border border-white/10 bg-black/20 object-contain"
              />
            </p>
            <p class="mt-1 text-[11px] text-slate-500">{{ t('adminSystem.hintBrandIcon') }}</p>
          </div>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secSecurity') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secSecurityHint') }}</p>
        <label class="mt-4 flex cursor-pointer items-center gap-2.5 text-sm text-slate-200 select-none">
          <input v-model="form.loginCaptcha" type="checkbox" class="h-4 w-4 shrink-0 rounded border-slate-600 text-cyan-500" />
          <span>{{ t('adminSystem.chkLoginCaptcha') }}</span>
        </label>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secNewUser') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secNewUserHint') }}</p>
        <div class="mt-4 max-w-md">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblDefaultNewUserBalance')
          }}</label>
          <input
            v-model.number="form.defaultNewUserBalance"
            type="number"
            min="0"
            step="1"
            class="th-input w-full font-mono text-sm"
            autocomplete="off"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">
            {{ t('adminSystem.hintDefaultNewUserYaml', { n: defaultNewUserBalanceYaml }) }}
          </p>
          <p v-if="defaultNewUserBalanceDbOverride" class="mt-1 text-[11px] leading-relaxed text-amber-100/80">
            {{ t('adminSystem.hintDefaultNewUserDb') }}
          </p>
        </div>
        <div class="mt-6 max-w-2xl border-t border-white/[0.06] pt-6">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblRegisterPromo')
          }}</label>
          <textarea
            v-model="form.registerPromoText"
            rows="4"
            class="th-input w-full resize-y font-sans text-sm leading-relaxed"
            :placeholder="t('adminSystem.phRegisterPromo')"
            autocomplete="off"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintRegisterPromoYaml') }}</p>
          <dl v-if="registerPromoTextYaml.trim()" class="mt-1 th-admin-compare-tile">
            <dd class="m-0 whitespace-pre-wrap text-[11px] leading-relaxed">{{ registerPromoTextYaml }}</dd>
          </dl>
          <p v-if="registerPromoTextDbOverride" class="mt-2 text-[11px] leading-relaxed text-amber-100/80">
            {{ t('adminSystem.hintRegisterPromoDb') }}
          </p>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secBilling') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secBillingHint') }}</p>
        <div class="mt-4 max-w-2xl">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblNoBillModels')
          }}</label>
          <textarea
            v-model="form.noBillModelsText"
            rows="5"
            class="th-input w-full resize-y font-mono text-xs leading-relaxed"
            :placeholder="t('adminSystem.phNoBillModels')"
            autocomplete="off"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintNoBillModels') }}</p>
          <dl v-if="noBillModelsYaml.trim()" class="mt-2 th-admin-compare-tile">
            <dt class="text-[10px] uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblNoBillModelsYaml') }}</dt>
            <dd class="m-0 mt-1 whitespace-pre-wrap font-mono text-[11px] leading-relaxed">{{ noBillModelsYaml }}</dd>
          </dl>
          <p v-if="noBillModelsDbOverride" class="mt-2 text-[11px] leading-relaxed text-amber-100/80">
            {{ t('adminSystem.hintNoBillModelsDb') }}
          </p>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secApiPublic') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secApiPublicHint') }}</p>
        <div class="mt-4 max-w-xl">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
            t('adminSystem.lblApiPublicBaseUrl')
          }}</label>
          <input
            v-model="form.apiPublicBaseUrl"
            type="url"
            class="th-input w-full font-mono text-sm"
            autocomplete="off"
            placeholder="https://api.example.com"
          />
          <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintApiPublicBaseUrl') }}</p>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secGatewayRouting') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secGatewayRoutingHint') }}</p>
        <div class="mt-5 max-w-xl space-y-4">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              t('adminSystem.lblRoutingStrategy')
            }}</label>
            <select v-model="form.routingStrategy" class="th-input max-w-md text-sm">
              <option value="weighted_random">{{ t('adminSystem.routingWeighted') }}</option>
              <option value="priority">{{ t('adminSystem.routingPriority') }}</option>
              <option value="fixed_order">{{ t('adminSystem.routingFixedOrder') }}</option>
            </select>
          </div>
          <label class="flex cursor-pointer items-start gap-3">
            <input v-model="form.routingFailoverEnabled" type="checkbox" class="mt-1" />
            <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingFailover') }}</span>
          </label>
          <div v-if="form.routingFailoverEnabled">
            <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminSystem.lblRoutingFailoverMax') }}</label>
            <input v-model.number="form.routingFailoverMaxAttempts" type="number" min="1" max="5" class="th-input max-w-xs font-mono text-sm" />
          </div>
          <label class="flex cursor-pointer items-start gap-3">
            <input v-model="form.routingStickyEnabled" type="checkbox" class="mt-1" />
            <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingSticky') }}</span>
          </label>
          <label class="flex cursor-pointer items-start gap-3">
            <input v-model="form.routingAutoHealthFromProbe" type="checkbox" class="mt-1" />
            <span class="text-xs text-slate-300">{{ t('adminSystem.lblRoutingAutoHealth') }}</span>
          </label>
          <div v-if="form.routingAutoHealthFromProbe">
            <label class="mb-1.5 block text-xs font-medium text-slate-500">{{ t('adminSystem.lblRoutingProbeThreshold') }}</label>
            <input v-model.number="form.routingProbeFailThreshold" type="number" min="1" max="20" class="th-input max-w-xs font-mono text-sm" />
          </div>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secUpstreamProbe') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secUpstreamProbeHint') }}</p>
        <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamProbeListDisclaimer') }}</p>
        <div class="mt-5 max-w-xl space-y-4">
          <label class="flex cursor-pointer items-start gap-3">
            <input v-model="form.upstreamProbeEnabled" type="checkbox" class="mt-1" />
            <span class="block text-xs font-medium text-slate-300">{{ t('adminSystem.lblUpstreamProbeEnabled') }}</span>
          </label>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              t('adminSystem.lblUpstreamProbeInterval')
            }}</label>
            <input
              v-model.number="form.upstreamProbeInterval"
              type="number"
              min="15"
              max="1440"
              step="1"
              class="th-input max-w-xs font-mono text-sm"
              autocomplete="off"
            />
            <p class="mt-2 text-[11px] leading-relaxed text-slate-500">{{ t('adminSystem.hintUpstreamProbeInterval') }}</p>
          </div>
          <p
            v-if="upstreamProbeEnabledDbOverride || upstreamProbeIntervalDbOverride"
            class="text-[11px] leading-relaxed text-amber-100/80"
          >
            {{ t('adminSystem.hintUpstreamProbeDb') }}
          </p>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secContact') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secContactHint') }}</p>
        <div class="mt-4 max-w-xl space-y-3">
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              t('adminSystem.lblContactTitle')
            }}</label>
            <input v-model="contactTitle" type="text" class="th-input w-full" autocomplete="off" :placeholder="t('adminSystem.phContactTitle')" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{
              t('adminSystem.lblContactEmails')
            }}</label>
            <textarea
              v-model="contactEmailsText"
              rows="3"
              class="th-input w-full resize-y font-mono text-sm"
              :placeholder="t('adminSystem.phContactEmails')"
              autocomplete="off"
            />
          </div>
        </div>
        <div class="mt-6 border-t border-white/[0.06] pt-6">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <p class="text-xs font-medium text-slate-400">{{ t('adminSystem.lblContactQrRows') }}</p>
            <button type="button" class="text-xs font-medium text-cyan-400/90 hover:underline" @click="addContactQrRow">
              {{ t('adminSystem.contactQrAddRow') }}
            </button>
          </div>
          <p v-if="contactPickQrIndex !== null" class="mt-2 text-[11px] text-amber-100/90">{{ t('adminSystem.contactPickActive') }}</p>
          <div v-if="!contactQRs.length" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.contactQrEmpty') }}</div>
          <div v-for="(row, idx) in contactQRs" :key="idx" class="mt-4 th-admin-compare-tile">
            <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
              <span class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.contactQrRow', { n: idx + 1 }) }}</span>
              <button type="button" class="text-[11px] text-rose-300/90 hover:underline" @click="removeContactQrRow(idx)">
                {{ t('adminSystem.contactQrRemove') }}
              </button>
            </div>
            <label class="mb-2 block text-xs text-slate-400">
              <span class="mb-1 block text-slate-500">{{ t('adminSystem.lblContactQrLabel') }}</span>
              <input v-model="row.label" type="text" class="th-input w-full text-sm" autocomplete="off" />
            </label>
            <label class="mb-2 block text-xs text-slate-400">
              <span class="mb-1 block text-slate-500">{{ t('adminSystem.lblContactQrUrl') }}</span>
              <input v-model="row.image_url" type="text" class="th-input w-full font-mono text-xs" autocomplete="off" />
            </label>
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <label class="th-btn-secondary inline-flex cursor-pointer items-center px-3 py-1.5 text-[11px] font-medium select-none">
                <input
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  class="sr-only"
                  :disabled="uploadingContactRow !== null || saving"
                  @change="uploadContactQr($event, idx)"
                />
                {{ uploadingContactRow === idx ? t('adminSystem.contactQrUploading') : t('adminSystem.contactQrUpload') }}
              </label>
              <button
                type="button"
                class="rounded border border-white/10 px-3 py-1.5 text-[11px] text-slate-300 hover:bg-white/[0.06]"
                :class="contactPickQrIndex === idx ? 'border-cyan-500/50 ring-1 ring-cyan-500/30' : ''"
                @click="startPickContactLibrary(idx)"
              >
                {{ t('adminSystem.contactQrPickLibrary') }}
              </button>
            </div>
            <div v-if="row.image_url.trim()" class="mt-2">
              <img :src="row.image_url.trim()" alt="" class="h-16 w-16 rounded border border-white/10 object-contain" />
            </div>
          </div>
        </div>
        <div class="mt-6">
          <p class="text-[11px] font-medium text-slate-400">{{ t('adminSystem.contactLibraryForQr') }}</p>
          <p class="mt-0.5 text-[11px] text-slate-500">{{ t('adminSystem.contactLibraryForQrHint') }}</p>
          <div class="mt-2 flex max-w-xl flex-wrap gap-2">
            <button
              v-for="ico in brandIconsUploadedOnly"
              :key="ico.url"
              type="button"
              :title="ico.filename || ico.url"
              class="h-11 w-11 overflow-hidden rounded-md th-admin-compare-tile--row !p-0 transition-colors hover:border-cyan-500/40"
              @click="applyContactLibraryIcon(ico)"
            >
              <img :src="ico.url.trim()" alt="" class="h-full w-full object-contain" />
            </button>
          </div>
          <p v-if="!brandIconsUploadedOnly.length" class="mt-2 text-[11px] text-slate-600">{{ t('adminSystem.contactLibraryEmpty') }}</p>
        </div>
      </section>

      <section class="th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold text-slate-100">{{ t('adminSystem.secOps') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminSystem.secOpsHint') }}</p>
        <div class="mt-4">
          <label class="mb-1.5 block text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('adminSystem.lblOperator') }}</label>
          <input v-model="form.operatorDisplayName" type="text" class="th-input max-w-xl" autocomplete="off" />
        </div>
      </section>

      <section class="rounded-lg border border-dashed border-white/[0.1] bg-black/[0.06] p-4">
        <p class="text-xs text-slate-500">{{ t('adminSystem.moreHint') }}</p>
      </section>

      <div class="flex flex-wrap gap-3">
        <button type="button" class="th-btn-primary px-5 py-2.5 text-sm" :disabled="saving" @click="save">
          {{ saving ? t('common.submitting') : t('adminCommon.save') }}
        </button>
        <button type="button" class="th-btn-secondary px-5 py-2.5 text-sm" :disabled="saving || busy" @click="refresh">
          {{ t('adminCommon.refresh') }}
        </button>
        <button
          type="button"
          class="th-btn-secondary border-amber-500/25 px-5 py-2.5 text-sm text-amber-100/90 hover:border-amber-500/40"
          :disabled="
            saving ||
            busy ||
            resettingDefaults ||
            uploadingIcon ||
            deletingBrandFilename !== null ||
            contactPickQrIndex !== null
          "
          @click="void resetDefaults()"
        >
          {{ resettingDefaults ? t('common.loading') : t('adminSystem.resetDefaults') }}
        </button>
      </div>
    </div>
  </div>
</template>
