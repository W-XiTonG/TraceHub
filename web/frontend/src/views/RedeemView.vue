<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { apiErrorMessage } from '../stores/auth'
import ContactCardDisclosure from '../components/ContactCardDisclosure.vue'
import DateTimeLocalFilterInput from '../components/DateTimeLocalFilterInput.vue'
import { localDateTimeToRFC3339 } from '../utils/datetimeLocal'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()

const code = ref('')
const msg = ref('')
const msgTone = ref<'success' | 'error'>('success')
const result = ref<{ amount_added?: number; new_balance?: number } | null>(null)

type LedgerRow = {
  id: number
  redeem_code_id: number
  amount: number
  balance_after: number
  code_hint?: string | null
  created_at?: string
}

const ledgerRows = ref<LedgerRow[]>([])
const ledgerTotal = ref(0)
const ledgerPage = ref(1)
const ledgerPageSize = 20
const ledgerErr = ref('')
const { busy: ledgerLoading, visible: ledgerLoadingUi, run: runLedgerLoad } = useSmoothLoading()

const ledgerStartLocal = ref('')
const ledgerEndLocal = ref('')
const ledgerCodeFilter = ref('')

const ledgerTotalPages = () => Math.max(1, Math.ceil(ledgerTotal.value / ledgerPageSize))

/** 列表时间与日期筛选均为「本机时区」语义；勿与表格内原始 UTC 字符串逐字对照。 */
function fmtLedgerTime(iso: string | undefined | null) {
  if (iso == null || String(iso).trim() === '') return '—'
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return String(iso)
    return d.toLocaleString()
  } catch {
    return String(iso)
  }
}

async function loadLedger() {
  ledgerErr.value = ''
  await runLedgerLoad(async () => {
    const codeQ = ledgerCodeFilter.value.trim()
    const { data } = await http.get<Envelope<{ items: LedgerRow[]; total: number }>>('/api/user/redeem-ledger', {
      params: {
        page: ledgerPage.value,
        page_size: ledgerPageSize,
        start_time: localDateTimeToRFC3339(ledgerStartLocal.value),
        end_time: localDateTimeToRFC3339(ledgerEndLocal.value),
        redeem_code: codeQ || undefined,
      },
    })
    if (data.code !== 0) throw new Error(data.message || t('redeemPage.fail'))
    ledgerRows.value = data.data?.items ?? []
    ledgerTotal.value = data.data?.total ?? 0
  }).catch((e: unknown) => {
    ledgerErr.value = apiErrorMessage(e, t('redeemPage.fail'))
    ledgerRows.value = []
    ledgerTotal.value = 0
  })
}

function ledgerPrev() {
  if (ledgerPage.value > 1) {
    ledgerPage.value--
    void loadLedger()
  }
}

function ledgerNext() {
  if (ledgerPage.value < ledgerTotalPages()) {
    ledgerPage.value++
    void loadLedger()
  }
}

function applyLedgerFilters() {
  ledgerPage.value = 1
  void loadLedger()
}

function resetLedgerFilters() {
  ledgerStartLocal.value = ''
  ledgerEndLocal.value = ''
  ledgerCodeFilter.value = ''
  ledgerPage.value = 1
  void loadLedger()
}

async function redeem() {
  msg.value = ''
  result.value = null
  try {
    const { data } = await http.post<Envelope<{ amount_added?: number; new_balance?: number }>>('/api/user/redeem', {
      code: code.value,
    })
    if (data.code !== 0) throw new Error(data.message || t('redeemPage.fail'))
    msgTone.value = 'success'
    msg.value = data.message || t('redeemPage.success')
    result.value = data.data
    ledgerPage.value = 1
    void loadLedger()
  } catch (e: unknown) {
    msgTone.value = 'error'
    msg.value = apiErrorMessage(e, t('redeemPage.fail'))
  }
}

onMounted(() => {
  void loadLedger()
})
</script>

<template>
  <div class="max-w-3xl space-y-10">
    <section>
      <h1 class="mb-6 text-2xl font-bold">{{ t('redeemPage.title') }}</h1>
      <div class="flex w-full max-w-md min-w-0 flex-col gap-3 sm:flex-row sm:items-stretch">
        <input
          v-model="code"
          :placeholder="t('redeemPage.ph')"
          class="th-input min-w-0 flex-1"
          autocomplete="one-time-code"
          @keydown.enter.prevent="redeem"
        />
        <button type="button" class="th-btn-primary-flat shrink-0 py-2.5 text-sm sm:px-6" @click="redeem">{{ t('redeemPage.submit') }}</button>
      </div>
      <p
        v-if="msg"
        class="mt-3 text-sm"
        :class="msgTone === 'error' ? 'text-rose-400' : 'text-emerald-300/95'"
      >
        {{ msg }}
      </p>
      <div v-if="result" class="th-redeem-result mt-4 max-w-md space-y-2 rounded-lg p-4 text-sm">
        <p v-if="result.amount_added != null">
          <span class="th-redeem-result__label">{{ t('redeemPage.amountAdded') }}</span>
          <span class="ml-2 font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ result.amount_added }}</span>
        </p>
        <p v-if="result.new_balance != null">
          <span class="th-redeem-result__label">{{ t('redeemPage.newBalance') }}</span>
          <span class="ml-2 font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ result.new_balance }}</span>
        </p>
      </div>
    </section>

    <section class="border-t border-white/[0.06] pt-8">
      <div class="mb-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 class="text-lg font-semibold text-slate-100">{{ t('redeemPage.ledgerTitle') }}</h2>
          <p class="mt-1 text-xs text-slate-500">{{ t('redeemPage.ledgerIntro') }}</p>
        </div>
        <button
          type="button"
          class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-white/[0.06]"
          :disabled="ledgerLoading"
          @click="loadLedger"
        >
          {{ t('redeemPage.ledgerRefresh') }}
        </button>
      </div>

      <div
        class="th-console-muted-bar mb-4 flex flex-col gap-3 px-4 py-3 sm:flex-row sm:flex-wrap sm:items-end sm:gap-x-3 sm:gap-y-2"
      >
        <div class="flex flex-col gap-1 text-xs">
          <label for="redeem-ledger-dt-start" class="block cursor-pointer text-slate-500">{{ t('redeemPage.ledgerFilterStart') }}</label>
          <DateTimeLocalFilterInput
            v-model="ledgerStartLocal"
            input-id="redeem-ledger-dt-start"
            :aria-label="t('redeemPage.ledgerFilterStart')"
          />
        </div>
        <div class="flex flex-col gap-1 text-xs">
          <label for="redeem-ledger-dt-end" class="block cursor-pointer text-slate-500">{{ t('redeemPage.ledgerFilterEnd') }}</label>
          <DateTimeLocalFilterInput
            v-model="ledgerEndLocal"
            input-id="redeem-ledger-dt-end"
            :aria-label="t('redeemPage.ledgerFilterEnd')"
          />
        </div>
        <label class="flex min-w-[10rem] flex-1 flex-col gap-1 text-xs sm:max-w-xs">
          <span class="text-slate-500">{{ t('redeemPage.ledgerFilterCode') }}</span>
          <input
            v-model="ledgerCodeFilter"
            type="search"
            autocomplete="off"
            class="th-input h-10 text-sm"
            :placeholder="t('redeemPage.ledgerFilterCodePh')"
            @keydown.enter.prevent="applyLedgerFilters"
          />
        </label>
        <div class="flex h-10 shrink-0 items-stretch gap-2">
          <button type="button" class="th-btn-primary-flat h-10 px-4 text-sm" @click="applyLedgerFilters">
            {{ t('redeemPage.ledgerFilterBtn') }}
          </button>
          <button
            type="button"
            class="th-ghost-hover h-10 rounded-lg border border-slate-600 px-4 text-sm text-slate-300"
            @click="resetLedgerFilters"
          >
            {{ t('redeemPage.ledgerFilterReset') }}
          </button>
        </div>
      </div>
      <p class="mb-2 text-[11px] leading-relaxed text-slate-500">{{ t('redeemPage.ledgerFilterTimeHint') }}</p>
      <p v-if="ledgerErr" class="mb-2 text-sm text-rose-400">{{ ledgerErr }}</p>
      <ThTableSkeleton v-if="ledgerLoadingUi && ledgerRows.length === 0" :rows="6" :columns="5" class="mb-4" />
      <div v-if="ledgerLoadingUi && ledgerRows.length === 0" class="py-8 lg:hidden">
        <ThLoadingState />
      </div>
      <p v-else-if="!ledgerLoading && ledgerRows.length === 0" class="text-sm text-slate-500">{{ t('redeemPage.ledgerEmpty') }}</p>
      <template v-else-if="ledgerRows.length > 0 || ledgerLoading">
        <div class="relative hidden overflow-x-auto rounded-lg border border-slate-800/80 lg:block">
          <ThLoadingState v-if="ledgerLoadingUi" variant="overlay" />
          <table class="min-w-full text-left text-sm">
            <thead class="border-b border-slate-800 bg-slate-950/80 text-xs uppercase text-slate-500">
              <tr>
                <th class="px-3 py-2">{{ t('redeemPage.thTime') }}</th>
                <th class="px-3 py-2">{{ t('redeemPage.thHint') }}</th>
                <th class="px-3 py-2">{{ t('redeemPage.thAmount') }}</th>
                <th class="px-3 py-2">{{ t('redeemPage.thBalanceAfter') }}</th>
                <th class="px-3 py-2">{{ t('redeemPage.thCodeId') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in ledgerRows" :key="r.id" class="border-b border-slate-800/60 hover:bg-white/[0.02]">
                <td class="px-3 py-2 font-mono text-xs text-slate-400">{{ fmtLedgerTime(r.created_at) }}</td>
                <td class="px-3 py-2 font-mono text-xs text-slate-300">{{ r.code_hint || '—' }}</td>
                <td class="px-3 py-2 font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ r.amount }}</td>
                <td class="px-3 py-2 font-mono text-slate-300">{{ r.balance_after }}</td>
                <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ r.redeem_code_id }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="space-y-3 lg:hidden">
          <MobileLabelValueCard v-for="r in ledgerRows" :key="`rd-m-${r.id}`">
            <template #title>
              <p class="font-mono text-xs text-slate-400">{{ fmtLedgerTime(r.created_at) }}</p>
              <p class="mt-0.5 font-mono text-sm text-slate-300">{{ r.code_hint || '—' }}</p>
            </template>
            <MobileLabelValueRow :label="t('redeemPage.thAmount')">
              <span class="font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ r.amount }}</span>
            </MobileLabelValueRow>
            <MobileLabelValueRow :label="t('redeemPage.thBalanceAfter')">{{ r.balance_after }}</MobileLabelValueRow>
            <MobileLabelValueRow :label="t('redeemPage.thCodeId')">{{ r.redeem_code_id }}</MobileLabelValueRow>
          </MobileLabelValueCard>
        </div>
        <div v-if="ledgerTotal > ledgerPageSize" class="mt-3 flex items-center justify-between text-xs text-slate-500">
        <span>{{ ledgerPage }} / {{ ledgerTotalPages() }}</span>
        <div class="flex gap-2">
          <button
            type="button"
            class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            :disabled="ledgerPage <= 1 || ledgerLoading"
            @click="ledgerPrev"
          >
            {{ t('adminCommon.prevPage') }}
          </button>
          <button
            type="button"
            class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
            :disabled="ledgerPage >= ledgerTotalPages() || ledgerLoading"
            @click="ledgerNext"
          >
            {{ t('adminCommon.nextPage') }}
          </button>
        </div>
        </div>
      </template>
    </section>

    <div class="border-t border-white/[0.06] pt-6">
      <ContactCardDisclosure align="start" compact summary-key="home.contactHintRedeem" />
    </div>
  </div>
</template>
