<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

type LedgerDetail = Record<string, unknown>

type LedgerRow = {
  event_time: string
  entry_type: string
  delta: number
  direction: string
  amount_abs: number
  user_id?: number | null
  user_email?: string
  balance_after?: number | null
  detail: LedgerDetail
}

const items = ref<LedgerRow[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 30
const err = ref('')

const filterUserId = ref('')
const filterEmail = ref('')
const filterEntryType = ref('')

function entryTypeLabel(kind: string) {
  switch (kind) {
    case 'api_usage':
      return t('adminLedgerPage.typeApiUsage')
    case 'redeem':
      return t('adminLedgerPage.typeRedeem')
    case 'admin_adjust':
      return t('adminLedgerPage.typeAdminAdjust')
    case 'feedback_reward':
      return t('adminLedgerPage.typeFeedbackReward')
    default:
      return kind
  }
}

function directionLabel(dir: string) {
  return dir === 'in' ? t('adminLedgerPage.directionIn') : t('adminLedgerPage.directionOut')
}

function numTok(v: unknown): number {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v)
  if (typeof v === 'string' && v.trim() !== '') {
    const n = Number(v)
    return Number.isFinite(n) ? Math.trunc(n) : 0
  }
  return 0
}

function formatDetail(row: LedgerRow): string {
  const d = row.detail || {}
  if (row.entry_type === 'api_usage') {
    return t('adminLedgerPage.detailApi', {
      model: String(d.model ?? '—'),
      inTok: numTok(d.prompt_tokens),
      outTok: numTok(d.completion_tokens),
      cost: numTok(d.total_cost),
    })
  }
  if (row.entry_type === 'redeem') {
    const hint = String(d.code_hint ?? '').trim() || '—'
    return t('adminLedgerPage.detailRedeem', { hint, amount: row.amount_abs })
  }
  if (row.entry_type === 'admin_adjust') {
    const signed = row.delta >= 0 ? `+${row.delta}` : String(row.delta)
    return t('adminLedgerPage.detailAdmin', { signed })
  }
  if (row.entry_type === 'feedback_reward') {
    const title = String(d.title ?? '').trim() || '—'
    const fid = numTok(d.feedback_id)
    return t('adminLedgerPage.detailFeedback', { title, id: fid, amount: row.amount_abs })
  }
  return JSON.stringify(d)
}

function formatDelta(row: LedgerRow): string {
  if (row.delta > 0) return `+${row.delta}`
  return String(row.delta)
}

function fmtTime(iso: string) {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return iso
    return d.toLocaleString()
  } catch {
    return iso
  }
}

function userCell(row: LedgerRow): string {
  const id = row.user_id
  const em = (row.user_email ?? '').trim()
  if (id != null && id > 0) {
    return em ? `#${id} · ${em}` : `#${id}`
  }
  return em || '—'
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    const uid = filterUserId.value.trim()
    if (uid !== '') params.user_id = uid
    const em = filterEmail.value.trim()
    if (em !== '') params.email = em
    const et = filterEntryType.value.trim()
    if (et !== '') params.entry_type = et
    const { data } = await http.get<Envelope<{ items: LedgerRow[]; total: number }>>('/api/admin/balance-ledger', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminLedgerPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
  }).catch((e: unknown) => {
    err.value = e instanceof Error ? e.message : t('adminLedgerPage.loadFail')
  })
}

function applyFilters() {
  page.value = 1
  void load()
}

function clearFilters() {
  filterUserId.value = ''
  filterEmail.value = ''
  filterEntryType.value = ''
  page.value = 1
  void load()
}

function prevPage() {
  if (page.value <= 1) return
  page.value -= 1
  void load()
}

function nextPage() {
  if (page.value * pageSize >= total.value) return
  page.value += 1
  void load()
}

onMounted(() => void load())
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold text-slate-100">{{ t('adminLedgerPage.title') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-500">{{ t('adminLedgerPage.desc') }}</p>
    </div>

    <div class="flex flex-wrap items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLedgerPage.lblUserId') }}</label>
        <input v-model="filterUserId" type="text" inputmode="numeric" class="th-input w-28 font-mono text-sm" :placeholder="t('adminLedgerPage.phUserId')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLedgerPage.lblEmail') }}</label>
        <input v-model="filterEmail" class="th-input w-52 text-sm" :placeholder="t('adminLedgerPage.phEmail')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLedgerPage.lblEntryType') }}</label>
        <select v-model="filterEntryType" class="th-input w-44 text-sm">
          <option value="">{{ t('adminLedgerPage.entryAll') }}</option>
          <option value="api_usage">{{ t('adminLedgerPage.typeApiUsage') }}</option>
          <option value="redeem">{{ t('adminLedgerPage.typeRedeem') }}</option>
          <option value="admin_adjust">{{ t('adminLedgerPage.typeAdminAdjust') }}</option>
          <option value="feedback_reward">{{ t('adminLedgerPage.typeFeedbackReward') }}</option>
        </select>
      </div>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="applyFilters">{{ t('adminLedgerPage.query') }}</button>
      <button type="button" class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]" @click="clearFilters">
        {{ t('adminLedgerPage.clear') }}
      </button>
    </div>

    <p v-if="err" class="text-sm text-rose-400">{{ err }}</p>

    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />
    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div class="relative hidden overflow-x-auto rounded-lg border border-white/[0.06] lg:block">
      <ThLoadingState v-if="loadingUi && items.length > 0" variant="overlay" />
      <table class="w-full min-w-[56rem] text-left text-xs text-slate-300">
        <thead class="border-b border-white/[0.06] bg-white/[0.02] text-[10px] uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thTime') }}</th>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thUser') }}</th>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thType') }}</th>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thDelta') }}</th>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thBalanceAfter') }}</th>
            <th class="px-3 py-2">{{ t('adminLedgerPage.thDetail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!items.length && !err">
            <td colspan="6" class="px-3 py-10 text-center text-slate-500">{{ t('adminLedgerPage.empty') }}</td>
          </tr>
          <tr v-for="(row, idx) in items" :key="`${page}-${idx}`" class="border-b border-white/[0.04] align-top last:border-0">
            <td class="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-400">{{ fmtTime(row.event_time) }}</td>
            <td class="max-w-[14rem] px-3 py-2 text-[11px] text-slate-400">
              <span class="break-all">{{ userCell(row) }}</span>
            </td>
            <td class="px-3 py-2">
              <span class="rounded border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 text-[11px] text-slate-300">{{
                entryTypeLabel(row.entry_type)
              }}</span>
              <span class="ml-1 text-[10px] text-slate-600">{{ directionLabel(row.direction) }}</span>
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono text-sm" :class="row.delta >= 0 ? 'text-emerald-400/95' : 'text-rose-400/95'">
              {{ formatDelta(row) }}
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-500">
              {{ row.balance_after == null ? t('adminLedgerPage.dashLong') : row.balance_after }}
            </td>
            <td class="max-w-xl px-3 py-2 text-[11px] leading-relaxed text-slate-500">{{ formatDetail(row) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="(row, idx) in items" :key="`aled-m-${page}-${idx}`">
        <template #title>
          <p class="font-mono text-[11px] text-slate-400">{{ fmtTime(row.event_time) }}</p>
          <p class="mt-1 text-sm text-slate-200">{{ entryTypeLabel(row.entry_type) }}</p>
        </template>
        <MobileLabelValueRow :label="t('adminLedgerPage.thUser')">{{ userCell(row) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLedgerPage.thDelta')">
          <span class="font-mono" :class="row.delta >= 0 ? 'text-emerald-400/95' : 'text-rose-400/95'">{{ formatDelta(row) }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLedgerPage.thBalanceAfter')">{{ row.balance_after == null ? t('adminLedgerPage.dashLong') : row.balance_after }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLedgerPage.thDetail')"><span class="text-xs">{{ formatDetail(row) }}</span></MobileLabelValueRow>
      </MobileLabelValueCard>
      <p v-if="!items.length && !err" class="py-10 text-center text-slate-500">{{ t('adminLedgerPage.empty') }}</p>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
      <span>{{ t('adminLedgerPage.pageInfo', { page, total }) }}</span>
      <div class="flex gap-2">
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page <= 1"
          @click="prevPage"
        >
          {{ t('adminLedgerPage.prev') }}
        </button>
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page * pageSize >= total"
          @click="nextPage"
        >
          {{ t('adminLedgerPage.next') }}
        </button>
      </div>
    </div>
  </div>
</template>
