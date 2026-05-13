<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

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
      params: { page: page.value, page_size: pageSize },
    })
    if (data.code !== 0) throw new Error(data.message || t('ledgerPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('ledgerPage.loadFail')
  }
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
            <td class="px-4 py-3">
              <span class="rounded-md border border-white/[0.08] bg-white/[0.04] px-2 py-0.5 text-xs text-slate-300">{{
                entryTypeLabel(row.entry_type)
              }}</span>
              <span class="ml-2 text-[11px] text-slate-600">{{ directionLabel(row.direction) }}</span>
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
