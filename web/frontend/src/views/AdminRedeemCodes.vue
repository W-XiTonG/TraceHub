<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { copyTextToClipboard } from '../utils/clipboard'

const { t } = useI18n()

type Row = {
  id: number
  code_hint: string
  code_type: string
  amount: number
  status: number
  max_uses: number
  used_count: number
  valid_until?: string | null
  created_at?: string
}

const rows = ref<Row[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 20
const statusFilter = ref('')
const msg = ref('')
const err = ref('')
const lastBatchCodes = ref('')

const batchForm = reactive({
  count: 10,
  amount: 10000,
  max_uses: 1,
  valid_days: 365,
  prefix: 'TH-',
})

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function statusLabel(st: number): string {
  if (st === 1) return t('adminRedeem.stActive')
  if (st === 2) return t('adminRedeem.stExhausted')
  if (st === 0) return t('adminRedeem.stRevoked')
  return String(st)
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    if (statusFilter.value !== '') params.status = statusFilter.value
    const { data } = await http.get<Envelope<{ items: Row[]; total: number }>>('/api/admin/redeem-codes', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    rows.value = data.data.items || []
    total.value = data.data.total ?? 0
  }).catch((e: unknown) => {
    err.value = httpErr(e)
  })
}

async function revoke(id: number) {
  if (!confirm(t('adminRedeem.confirmRevoke'))) return
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.delete<Envelope<unknown>>(`/api/admin/redeem-codes/${id}`)
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
    msg.value = t('adminRedeem.revoked')
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function generateBatch() {
  err.value = ''
  msg.value = ''
  lastBatchCodes.value = ''
  try {
    const { data } = await http.post<Envelope<{ codes: string[] }>>('/api/admin/redeem-codes', {
      count: batchForm.count,
      amount: batchForm.amount,
      max_uses: batchForm.max_uses,
      valid_days: batchForm.valid_days,
      prefix: batchForm.prefix.trim() || 'TH-',
      code_type: 'fixed',
    })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
    const codes = data.data?.codes ?? []
    lastBatchCodes.value = codes.join('\n')
    msg.value = t('adminRedeem.batchDone', { n: codes.length })
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

async function copyBatch() {
  if (!lastBatchCodes.value) return
  err.value = ''
  try {
    await copyTextToClipboard(lastBatchCodes.value)
    msg.value = t('adminRedeem.copied')
  } catch {
    err.value = t('adminUsers.copyFail')
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    void load()
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++
    void load()
  }
}

function onStatusFilterChange() {
  page.value = 1
  void load()
}

function refreshList() {
  page.value = 1
  void load()
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="space-y-10">
    <header>
      <h1 class="text-xl font-semibold text-slate-100">{{ t('adminRedeem.title') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-400">{{ t('adminRedeem.intro') }}</p>
    </header>

    <section class="rounded-xl border border-amber-500/15 bg-amber-950/10 p-5">
      <h2 class="text-sm font-medium text-amber-200/90">{{ t('adminRedeem.secBatch') }}</h2>
      <div class="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">{{ t('adminRedeem.lblCount') }}</span>
          <input
            v-model.number="batchForm.count"
            type="number"
            min="1"
            max="500"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm outline-none focus:border-amber-500/40"
          />
        </label>
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">{{ t('adminRedeem.lblAmount') }}</span>
          <input
            v-model.number="batchForm.amount"
            type="number"
            min="1"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm outline-none focus:border-amber-500/40"
          />
        </label>
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">{{ t('adminRedeem.lblMaxUses') }}</span>
          <input
            v-model.number="batchForm.max_uses"
            type="number"
            min="1"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm outline-none focus:border-amber-500/40"
          />
        </label>
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">{{ t('adminRedeem.lblValidDays') }}</span>
          <input
            v-model.number="batchForm.valid_days"
            type="number"
            min="0"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm outline-none focus:border-amber-500/40"
          />
        </label>
        <label class="flex flex-col gap-1.5 sm:col-span-2">
          <span class="text-xs font-medium text-slate-400">{{ t('adminRedeem.lblPrefix') }}</span>
          <input
            v-model="batchForm.prefix"
            maxlength="64"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm outline-none focus:border-amber-500/40"
          />
        </label>
      </div>
      <p class="mt-3 text-xs text-slate-500">{{ t('adminRedeem.batchHint') }}</p>
      <div class="mt-4 flex flex-wrap gap-2">
        <button
          type="button"
          class="th-btn-primary-flat px-4 py-2 text-sm"
          @click="generateBatch"
        >
          {{ t('adminRedeem.genBtn') }}
        </button>
        <button
          v-if="lastBatchCodes"
          type="button"
          class="rounded-lg border border-slate-600 px-4 py-2 text-sm text-slate-200 transition hover:bg-white/[0.06]"
          @click="copyBatch"
        >
          {{ t('adminRedeem.copyBtn') }}
        </button>
      </div>
      <textarea
        v-if="lastBatchCodes"
        v-model="lastBatchCodes"
        readonly
        rows="8"
        class="mt-4 w-full resize-y rounded-lg border border-slate-700 bg-slate-950 p-3 font-mono text-xs text-slate-200"
      />
    </section>

    <section>
      <div class="mb-4 flex flex-wrap items-end justify-between gap-3 border-b border-slate-800/80 pb-4">
        <h2 class="text-sm font-medium text-slate-200">{{ t('adminRedeem.secList') }}</h2>
        <div class="flex flex-wrap items-center gap-2">
          <label class="flex items-center gap-2 text-xs text-slate-400">
            <span>{{ t('adminRedeem.filterStatus') }}</span>
            <select
              v-model="statusFilter"
              class="rounded-lg border border-slate-700 bg-slate-950 px-2 py-1.5 text-slate-100"
              @change="onStatusFilterChange"
            >
              <option value="">{{ t('adminRedeem.allStatus') }}</option>
              <option value="1">1 — {{ t('adminRedeem.stActive') }}</option>
              <option value="2">2 — {{ t('adminRedeem.stExhausted') }}</option>
              <option value="0">0 — {{ t('adminRedeem.stRevoked') }}</option>
            </select>
          </label>
          <button
            type="button"
            class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-white/[0.06]"
            @click="refreshList"
          >
            {{ t('adminCommon.refresh') }}
          </button>
        </div>
      </div>
      <p v-if="err" class="mb-3 text-sm text-rose-400">{{ err }}</p>
      <p v-if="msg" class="mb-3 text-sm text-emerald-400/90">{{ msg }}</p>
      <p class="mb-3 text-xs text-slate-500">{{ t('adminRedeem.total', { n: total }) }}</p>
      <ThTableSkeleton v-if="loadingUi && rows.length === 0" :rows="8" :columns="8" class="mb-4" />
      <div v-if="loadingUi && rows.length === 0" class="py-8 lg:hidden">
        <ThLoadingState />
      </div>
      <div class="relative hidden overflow-x-auto rounded-lg border border-slate-800/80 lg:block">
        <ThLoadingState v-if="loadingUi && rows.length > 0" variant="overlay" />
        <table class="min-w-full text-left text-sm">
          <thead class="border-b border-slate-800 bg-slate-950/80 text-xs uppercase text-slate-500">
            <tr>
              <th class="px-3 py-2">{{ t('adminRedeem.thId') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thHint') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thAmount') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thUses') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thStatus') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thValidUntil') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thCreated') }}</th>
              <th class="px-3 py-2">{{ t('adminRedeem.thActions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rows" :key="r.id" class="border-b border-slate-800/60 hover:bg-white/[0.02]">
              <td class="px-3 py-2 font-mono text-slate-300">{{ r.id }}</td>
              <td class="px-3 py-2 font-mono text-xs text-slate-400">{{ r.code_hint }}</td>
              <td class="px-3 py-2 font-mono text-slate-300">{{ r.amount }}</td>
              <td class="px-3 py-2 font-mono text-slate-400">{{ r.used_count }} / {{ r.max_uses }}</td>
              <td class="px-3 py-2 text-slate-300">{{ statusLabel(r.status) }}</td>
              <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ r.valid_until || '—' }}</td>
              <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ r.created_at || '—' }}</td>
              <td class="px-3 py-2">
                <button
                  v-if="r.status === 1"
                  type="button"
                  class="text-xs text-rose-400/90 hover:underline"
                  @click="revoke(r.id)"
                >
                  {{ t('adminRedeem.revoke') }}
                </button>
                <span v-else class="text-xs text-slate-600">—</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="mt-3 space-y-3 lg:hidden">
        <MobileLabelValueCard v-for="r in rows" :key="`rc-m-${r.id}`" :title="r.code_hint">
          <MobileLabelValueRow :label="t('adminRedeem.thId')">{{ r.id }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminRedeem.thAmount')">{{ r.amount }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminRedeem.thUses')">{{ r.used_count }} / {{ r.max_uses }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminRedeem.thStatus')">{{ statusLabel(r.status) }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminRedeem.thValidUntil')">{{ r.valid_until || '—' }}</MobileLabelValueRow>
          <MobileLabelValueRow :label="t('adminRedeem.thCreated')">{{ r.created_at || '—' }}</MobileLabelValueRow>
          <div v-if="r.status === 1" class="pt-1">
            <button type="button" class="text-xs text-rose-400/90 hover:underline" @click="revoke(r.id)">{{ t('adminRedeem.revoke') }}</button>
          </div>
        </MobileLabelValueCard>
      </div>
      <div v-if="total > pageSize" class="mt-4 flex items-center justify-between text-xs text-slate-500">
        <span>{{ t('adminRedeem.pageOf', { page: page, pages: totalPages }) }}</span>
        <div class="flex gap-2">
          <button
            type="button"
            :disabled="page <= 1"
            class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            @click="prevPage"
          >
            {{ t('adminCommon.prevPage') }}
          </button>
          <button
            type="button"
            :disabled="page >= totalPages"
            class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            @click="nextPage"
          >
            {{ t('adminCommon.nextPage') }}
          </button>
        </div>
      </div>
    </section>
  </div>
</template>
