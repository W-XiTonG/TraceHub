<script setup lang="ts">
import axios from 'axios'
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { useSiteMetaStore } from '../stores/siteMeta'

const { t } = useI18n()
const siteMeta = useSiteMetaStore()

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
    await load()
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
    await load()
    await siteMeta.refreshFromServer()
  } catch (e: unknown) {
    err.value = httpErr(e)
  } finally {
    saving.value = false
  }
}

onMounted(() => void load())
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
            <p v-if="form.brandIconUrl.trim()" class="mt-2 flex items-center gap-2 text-[11px] text-slate-500">
              <span>{{ t('adminSystem.brandIconPreview') }}</span>
              <img :src="form.brandIconUrl.trim()" alt="" class="h-8 w-8 rounded border border-white/10 object-contain" />
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
        <button type="button" class="th-btn-secondary px-5 py-2.5 text-sm" :disabled="saving || busy" @click="load">
          {{ t('adminCommon.refresh') }}
        </button>
      </div>
    </div>
  </div>
</template>
