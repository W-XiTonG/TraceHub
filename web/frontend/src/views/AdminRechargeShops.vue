<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import { useSiteMetaStore } from '../stores/siteMeta'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()
const siteMeta = useSiteMetaStore()

type ShopRow = {
  label: string
  url: string
  description: string
  enabled: boolean
  sort: number
  show_qr: boolean
  qr_image_url: string
}

const title = ref('')
const hint = ref('')
const shops = ref<ShopRow[]>([])
const { busy: pageLoading, visible: pageLoadingUi, run: runPageLoad } = useSmoothLoading()
const saving = ref(false)
const uploadingQrRow = ref<number | null>(null)
const msg = ref('')
const msgTone = ref<'success' | 'error'>('success')

function addRow() {
  shops.value.push({
    label: '',
    url: '',
    description: '',
    enabled: true,
    sort: shops.value.length,
    show_qr: false,
    qr_image_url: '',
  })
}

function removeRow(idx: number) {
  shops.value.splice(idx, 1)
}

async function uploadQr(ev: Event, rowIdx: number) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  uploadingQrRow.value = rowIdx
  msg.value = ''
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await http.post<Envelope<{ url?: string }>>('/api/admin/site-upload/contact-qr', fd)
    if (data.code !== 0) throw new Error(data.message || t('adminPayment.rechargeShopsQrUploadFail'))
    const u = typeof data.data?.url === 'string' ? data.data.url.trim() : ''
    if (!u) throw new Error(t('adminPayment.rechargeShopsQrUploadFail'))
    const row = shops.value[rowIdx]
    if (row) {
      row.qr_image_url = u
      row.show_qr = true
    }
    msg.value = t('adminPayment.rechargeShopsQrUploaded')
  } catch (e: unknown) {
    msg.value = apiErrorMessage(e)
    msgTone.value = 'error'
  } finally {
    uploadingQrRow.value = null
  }
}

async function load() {
  await runPageLoad(async () => {
    const { data } = await http.get<
      Envelope<{ title?: string; hint?: string; shops?: ShopRow[] }>
    >('/api/admin/payment/recharge-shops')
    if (data.code === 0 && data.data) {
      title.value = typeof data.data.title === 'string' ? data.data.title : ''
      hint.value = typeof data.data.hint === 'string' ? data.data.hint : ''
      shops.value = Array.isArray(data.data.shops)
        ? data.data.shops.map((s) => ({
            label: typeof s.label === 'string' ? s.label : '',
            url: typeof s.url === 'string' ? s.url : '',
            description: typeof s.description === 'string' ? s.description : '',
            enabled: s.enabled !== false,
            sort: typeof s.sort === 'number' ? s.sort : 0,
            show_qr: s.show_qr === true,
            qr_image_url: typeof s.qr_image_url === 'string' ? s.qr_image_url : '',
          }))
        : []
    }
  })
}

async function save() {
  saving.value = true
  msg.value = ''
  msgTone.value = 'success'
  try {
    for (const row of shops.value) {
      if (row.label.trim() && !row.url.trim()) {
        msg.value = t('adminPayment.rechargeShopsUrlRequired')
        msgTone.value = 'error'
        return
      }
      if (row.url.trim() && !row.label.trim()) {
        msg.value = t('adminPayment.rechargeShopsLabelRequired')
        msgTone.value = 'error'
        return
      }
    }
    const payload = {
      title: title.value.trim(),
      hint: hint.value.trim(),
      shops: shops.value
        .filter((r) => r.label.trim() || r.url.trim() || r.description.trim())
        .map((r) => ({
          label: r.label.trim(),
          url: r.url.trim(),
          description: r.description.trim(),
          enabled: r.enabled,
          sort: r.sort,
          show_qr: r.show_qr,
          qr_image_url: r.qr_image_url.trim(),
        })),
    }
    const { data } = await http.put<Envelope<unknown>>('/api/admin/payment/recharge-shops', payload)
    if (data.code === 0) {
      msg.value = t('adminPayment.saved')
      await siteMeta.refreshFromServer()
      await load()
    } else {
      msg.value = apiErrorMessage(data)
      msgTone.value = 'error'
    }
  } catch (e: unknown) {
    msg.value = apiErrorMessage(e)
    msgTone.value = 'error'
  } finally {
    saving.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <div class="max-w-3xl space-y-6">
    <h1 class="text-2xl font-bold">{{ t('adminPayment.rechargeShopsTitle') }}</h1>
    <p class="text-sm text-slate-500">{{ t('adminPayment.rechargeShopsHint') }}</p>
    <div class="rounded-xl border border-amber-500/30 bg-amber-950/20 p-4 text-xs text-amber-100/90">
      {{ t('adminPayment.rechargeShopsNotice') }}
    </div>
    <ThLoadingState v-if="pageLoadingUi" class="py-10" />

    <template v-else>
      <div class="space-y-4 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <div>
          <label class="mb-1 block text-xs text-slate-500">{{ t('adminPayment.rechargeShopsBlockTitle') }}</label>
          <input v-model="title" type="text" class="th-input w-full" :placeholder="t('adminPayment.rechargeShopsBlockTitlePh')" />
        </div>
        <div>
          <label class="mb-1 block text-xs text-slate-500">{{ t('adminPayment.rechargeShopsBlockHint') }}</label>
          <textarea v-model="hint" rows="2" class="th-input w-full" :placeholder="t('adminPayment.rechargeShopsBlockHintPh')" />
        </div>
      </div>

      <div class="space-y-4">
        <div class="flex items-center justify-between gap-3">
          <h2 class="text-sm font-medium text-slate-300">{{ t('adminPayment.rechargeShopsListTitle') }}</h2>
          <button type="button" class="th-btn-secondary text-xs" @click="addRow">{{ t('adminPayment.rechargeShopsAdd') }}</button>
        </div>
        <p v-if="!shops.length" class="text-xs text-slate-600">{{ t('adminPayment.rechargeShopsEmpty') }}</p>
        <div
          v-for="(row, idx) in shops"
          :key="idx"
          class="space-y-3 rounded-xl border border-slate-800 bg-slate-900/40 p-4"
        >
          <div class="flex items-center justify-between gap-2">
            <span class="text-xs font-medium uppercase tracking-wide text-slate-500">
              {{ t('adminPayment.rechargeShopsRow', { n: idx + 1 }) }}
            </span>
            <button type="button" class="text-xs text-red-400 hover:underline" @click="removeRow(idx)">
              {{ t('adminPayment.rechargeShopsRemove') }}
            </button>
          </div>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="row.enabled" type="checkbox" class="rounded" />
            {{ t('adminPayment.rechargeShopsEnabled') }}
          </label>
          <input v-model="row.label" type="text" class="th-input w-full" :placeholder="t('adminPayment.rechargeShopsLabelPh')" />
          <input v-model="row.url" type="url" class="th-input w-full" :placeholder="t('adminPayment.rechargeShopsUrlPh')" />
          <input
            v-model="row.description"
            type="text"
            class="th-input w-full"
            :placeholder="t('adminPayment.rechargeShopsDescPh')"
          />
          <label class="flex items-center gap-2 text-sm">
            <input v-model="row.show_qr" type="checkbox" class="rounded" />
            {{ t('adminPayment.rechargeShopsShowQr') }}
          </label>
          <div v-if="row.show_qr" class="space-y-2 rounded-lg border border-slate-800/80 bg-slate-950/40 p-3">
            <input
              v-model="row.qr_image_url"
              type="text"
              class="th-input w-full"
              :placeholder="t('adminPayment.rechargeShopsQrUrlPh')"
            />
            <div class="flex flex-wrap items-center gap-2">
              <label class="th-btn-secondary cursor-pointer text-xs">
                {{ uploadingQrRow === idx ? t('adminPayment.rechargeShopsQrUploading') : t('adminPayment.rechargeShopsQrUpload') }}
                <input type="file" accept="image/*" class="sr-only" @change="uploadQr($event, idx)" />
              </label>
              <img
                v-if="row.qr_image_url"
                :src="row.qr_image_url"
                alt=""
                class="h-16 w-16 rounded border border-slate-700 object-contain bg-white"
              />
            </div>
            <p class="text-[11px] text-slate-500">{{ t('adminPayment.rechargeShopsQrAutoHint') }}</p>
          </div>
          <div class="flex items-center gap-2">
            <label class="text-xs text-slate-500">{{ t('adminPayment.rechargeShopsSort') }}</label>
            <input v-model.number="row.sort" type="number" class="th-input w-24" min="0" step="1" />
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <button type="button" class="th-btn-primary disabled:opacity-50" :disabled="saving" @click="save">
          {{ saving ? t('adminPayment.rechargeShopsSaving') : t('adminPayment.save') }}
        </button>
        <p v-if="msg" class="text-sm" :class="msgTone === 'error' ? 'text-red-400' : 'text-emerald-400'">{{ msg }}</p>
      </div>
    </template>
  </div>
</template>
