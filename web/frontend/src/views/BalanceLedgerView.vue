<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import DateTimeLocalFilterInput from '../components/DateTimeLocalFilterInput.vue'
import { localDateTimeToRFC3339 } from '../utils/datetimeLocal'

const { t } = useI18n()

type LedgerDetail = Record<string, unknown>

type LedgerRow = {
  event_time: string
  entry_type: string
  delta: number
  direction: string
  amount_abs: number
  balance_after?: number | null
  detail: LedgerDetail
}

const items = ref<LedgerRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const err = ref('')

const filters = reactive({
  entry_type: '',
  direction: '' as '' | 'in' | 'out',
})

/** `datetime-local` 绑定值，请求时转为 RFC3339 */
const startLocal = ref('')
const endLocal = ref('')

function entryTypeLabel(kind: string) {
  switch (kind) {
    case 'api_usage':
      return t('ledgerPage.typeApiUsage')
    case 'redeem':
      return t('ledgerPage.typeRedeem')
    case 'admin_adjust':
      return t('ledgerPage.typeAdminAdjust')
    case 'feedback_reward':
      return t('ledgerPage.typeFeedbackReward')
    default:
      return kind
  }
}

function directionLabel(dir: string) {
  return dir === 'in' ? t('ledgerPage.directionIn') : t('ledgerPage.directionOut')
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
    return t('ledgerPage.detailApi', {
      model: String(d.model ?? '—'),
      inTok: numTok(d.prompt_tokens),
      outTok: numTok(d.completion_tokens),
      cost: numTok(d.total_cost),
    })
  }
  if (row.entry_type === 'redeem') {
    const hint = String(d.code_hint ?? '').trim() || '—'
    return t('ledgerPage.detailRedeem', { hint, amount: row.amount_abs })
  }
  if (row.entry_type === 'admin_adjust') {
    const signed = row.delta >= 0 ? `+${row.delta}` : String(row.delta)
    return t('ledgerPage.detailAdmin', { signed })
  }
  if (row.entry_type === 'feedback_reward') {
    const title = String(d.title ?? '').trim() || '—'
    const fid = numTok(d.feedback_id)
    return t('ledgerPage.detailFeedback', { title, id: fid, amount: row.amount_abs })
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

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: LedgerRow[]; total: number }>>('/api/user/balance-ledger', {
      params: {
        page: page.value,
        page_size: pageSize,
        entry_type: filters.entry_type || undefined,
        direction: filters.direction || undefined,
        start_time: localDateTimeToRFC3339(startLocal.value),
        end_time: localDateTimeToRFC3339(endLocal.value),
      },
    })
    if (data.code !== 0) throw new Error(data.message || t('ledgerPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
  } catch (e: unknown) {
    err.value = apiErrorMessage(e, t('ledgerPage.loadFail'))
  }
}

function applyFilters() {
  page.value = 1
  void load()
}

function resetFilters() {
  filters.entry_type = ''
  filters.direction = ''
  startLocal.value = ''
  endLocal.value = ''
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
  <div class="mx-auto max-w-6xl px-4 py-8 text-slate-100 md:px-6">
    <p class="text-xs font-medium uppercase tracking-wider text-slate-500">{{ t('ledgerPage.pageKicker') }}</p>
    <h1 class="mt-1 text-2xl font-bold">{{ t('ledgerPage.title') }}</h1>
    <p class="mt-2 max-w-3xl text-sm text-slate-400">{{ t('ledgerPage.desc') }}</p>

    <div
      class="th-console-muted-bar mt-6 flex flex-col gap-3 px-4 py-3 md:flex-row md:flex-wrap md:items-end md:justify-between md:gap-x-4 md:gap-y-2"
    >
      <div
        class="grid w-full min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-[minmax(0,11rem)_minmax(0,11rem)_minmax(0,11.5rem)_minmax(0,11.5rem)_auto] lg:items-end"
      >
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('ledgerPage.filterEntryType') }}</span>
          <select v-model="filters.entry_type" class="th-input h-10 w-full text-sm">
            <option value="">{{ t('ledgerPage.optEntryAll') }}</option>
            <option value="api_usage">{{ t('ledgerPage.typeApiUsage') }}</option>
            <option value="redeem">{{ t('ledgerPage.typeRedeem') }}</option>
            <option value="admin_adjust">{{ t('ledgerPage.typeAdminAdjust') }}</option>
            <option value="feedback_reward">{{ t('ledgerPage.typeFeedbackReward') }}</option>
          </select>
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('ledgerPage.filterDirection') }}</span>
          <select v-model="filters.direction" class="th-input h-10 w-full text-sm">
            <option value="">{{ t('ledgerPage.optDirAll') }}</option>
            <option value="in">{{ t('ledgerPage.directionIn') }}</option>
            <option value="out">{{ t('ledgerPage.directionOut') }}</option>
          </select>
        </label>
        <div class="flex flex-col gap-1 text-xs lg:col-span-1">
          <label for="ledger-dt-start" class="block cursor-pointer text-slate-500">{{ t('ledgerPage.start') }}</label>
          <DateTimeLocalFilterInput
            v-model="startLocal"
            input-id="ledger-dt-start"
            :aria-label="t('ledgerPage.start')"
          />
        </div>
        <div class="flex flex-col gap-1 text-xs lg:col-span-1">
          <label for="ledger-dt-end" class="block cursor-pointer text-slate-500">{{ t('ledgerPage.end') }}</label>
          <DateTimeLocalFilterInput
            v-model="endLocal"
            input-id="ledger-dt-end"
            :aria-label="t('ledgerPage.end')"
          />
        </div>
        <div class="flex h-10 items-stretch gap-2 lg:col-span-1 lg:shrink-0">
          <button
            type="button"
            class="th-btn-primary-flat h-10 px-4 text-sm sm:px-4"
            @click="applyFilters"
          >
            {{ t('ledgerPage.filterBtn') }}
          </button>
          <button
            type="button"
            class="th-ghost-hover h-10 rounded-lg border border-slate-600 px-4 text-sm text-slate-300"
            @click="resetFilters"
          >
            {{ t('ledgerPage.resetBtn') }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="err" class="mt-4 text-sm text-red-400">{{ err }}</p>

    <div class="mt-6 overflow-x-auto rounded-xl border border-white/[0.08] bg-slate-950/40">
      <table class="w-full min-w-[52rem] border-collapse text-left text-sm">
        <thead>
          <tr class="border-b border-white/[0.08] text-xs uppercase tracking-wide text-slate-500">
            <th class="px-4 py-3 font-medium">{{ t('ledgerPage.thTime') }}</th>
            <th class="px-4 py-3 font-medium">{{ t('ledgerPage.thType') }}</th>
            <th class="px-4 py-3 font-medium">{{ t('ledgerPage.thDelta') }}</th>
            <th class="px-4 py-3 font-medium">{{ t('ledgerPage.thBalanceAfter') }}</th>
            <th class="px-4 py-3 font-medium">{{ t('ledgerPage.thDetail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!items.length && !err">
            <td colspan="5" class="px-4 py-10 text-center text-slate-500">{{ t('ledgerPage.empty') }}</td>
          </tr>
          <tr
            v-for="(row, idx) in items"
            :key="`${page}-${idx}`"
            class="border-b border-white/[0.05] last:border-0 hover:bg-white/[0.02]"
          >
            <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-300">{{ fmtTime(row.event_time) }}</td>
            <td class="px-4 py-3 align-top">
              <div class="flex max-w-[14rem] flex-col gap-1.5 sm:max-w-none sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-2 sm:gap-y-1">
                <span
                  class="inline-flex w-fit shrink-0 rounded-md border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 text-xs text-slate-300"
                  >{{ entryTypeLabel(row.entry_type) }}</span>
                <span class="text-[11px] text-slate-500 sm:text-slate-600">{{ directionLabel(row.direction) }}</span>
              </div>
            </td>
            <td
              class="whitespace-nowrap px-4 py-3 font-mono text-sm"
              :class="row.delta >= 0 ? 'text-emerald-400/95' : 'text-rose-400/95'"
            >
              {{ formatDelta(row) }}
            </td>
            <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-slate-400">
              {{ row.balance_after == null ? t('ledgerPage.dashLong') : row.balance_after }}
            </td>
            <td class="max-w-xl px-4 py-3 text-xs leading-relaxed text-slate-400">{{ formatDetail(row) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-slate-500">
      <span>{{ t('ledgerPage.pageInfo', { page, total }) }}</span>
      <div class="flex gap-2">
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-cyan-500/30 hover:text-cyan-200/90 disabled:opacity-40"
          :disabled="page <= 1"
          @click="prevPage"
        >
          {{ t('ledgerPage.prev') }}
        </button>
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-cyan-500/30 hover:text-cyan-200/90 disabled:opacity-40"
          :disabled="page * pageSize >= total"
          @click="nextPage"
        >
          {{ t('ledgerPage.next') }}
        </button>
      </div>
    </div>
  </div>
</template>
