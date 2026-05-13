<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { useSiteMetaStore } from '../stores/siteMeta'

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
}

const form = reactive({
  brandName: '',
  brandIconUrl: '',
  loginCaptcha: false,
  operatorDisplayName: '',
})

const busy = ref(true)
const saving = ref(false)
const uploadingIcon = ref(false)
const msg = ref('')
const err = ref('')
const brandIcons = ref<BrandLibIcon[]>([])
const deletingBrandFilename = ref<string | null>(null)

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

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function applyPayload(d: SiteSettings) {
  form.brandName = typeof d.brand_name === 'string' ? d.brand_name : ''
  form.brandIconUrl = typeof d.brand_icon_url === 'string' ? d.brand_icon_url : ''
  form.loginCaptcha = Boolean(d.login_captcha_enabled)
  form.operatorDisplayName = typeof d.operator_display_name === 'string' ? d.operator_display_name : ''
}

async function load() {
  busy.value = true
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.get<Envelope<SiteSettings>>('/api/admin/site-settings')
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    applyPayload(data.data ?? {})
  } catch (e: unknown) {
    err.value = httpErr(e)
  } finally {
    busy.value = false
  }
}

async function loadBrandIcons() {
  try {
    const { data } = await http.get<Envelope<{ icons?: BrandLibIcon[] }>>('/api/admin/site-brand/icons')
    if (data.code !== 0 || !Array.isArray(data.data?.icons)) {
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
    if (data.code !== 0) throw new Error(data.message || t('adminSystem.brandIconDeleteFail'))
    if (form.brandIconUrl.trim() === ico.url.trim()) {
      form.brandIconUrl = ''
    }
    msg.value = t('adminSystem.brandIconDeleted')
    await refresh()
    await siteMeta.refreshFromServer()
  } catch (e: unknown) {
    err.value = httpErr(e)
  } finally {
    deletingBrandFilename.value = null
  }
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
  uploadingIcon.value = true
  err.value = ''
  msg.value = ''
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await http.post<Envelope<{ url?: string }>>('/api/admin/site-upload/brand-icon', fd)
    if (data.code !== 0) throw new Error(data.message || t('adminSystem.brandIconUploadFail'))
    const u = data.data && typeof data.data.url === 'string' ? data.data.url.trim() : ''
    if (!u) throw new Error(t('adminSystem.brandIconUploadFail'))
    form.brandIconUrl = u
    msg.value = t('adminSystem.brandIconUploaded')
    await refresh()
    await siteMeta.refreshFromServer()
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
  ]
  try {
    const { data } = await http.put<Envelope<unknown>>('/api/admin/config', { items })
    if (data.code !== 0) throw new Error(data.message || t('adminSystem.saveFail'))
    msg.value = t('adminSystem.saved')
    await refresh()
    await siteMeta.refreshFromServer()
  } catch (e: unknown) {
    err.value = httpErr(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => void refresh())
</script>

<template>
  <div>
    <h1 class="text-xl font-semibold tracking-tight text-slate-100">{{ t('adminSystem.title') }}</h1>
    <p class="mt-1 text-sm text-slate-500">{{ t('adminSystem.intro') }}</p>

    <p v-if="err" class="mt-4 rounded-lg border border-red-500/20 bg-red-950/35 px-3 py-2 text-sm text-red-200">{{ err }}</p>
    <p v-if="msg" class="mt-4 rounded-lg border border-emerald-500/20 bg-emerald-950/25 px-3 py-2 text-sm text-emerald-100">{{ msg }}</p>

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
      </div>
    </div>
  </div>
</template>
