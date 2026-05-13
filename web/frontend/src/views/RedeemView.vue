<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import ContactCardDisclosure from '../components/ContactCardDisclosure.vue'

const { t } = useI18n()

const code = ref('')
const msg = ref('')
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
const ledgerLoading = ref(false)

const ledgerTotalPages = () => Math.max(1, Math.ceil(ledgerTotal.value / ledgerPageSize))

async function loadLedger() {
  ledgerErr.value = ''
  ledgerLoading.value = true
  try {
    const { data } = await http.get<Envelope<{ items: LedgerRow[]; total: number }>>('/api/user/redeem-ledger', {
      params: { page: ledgerPage.value, page_size: ledgerPageSize },
    })
    if (data.code !== 0) throw new Error(data.message || t('redeemPage.fail'))
    ledgerRows.value = data.data?.items ?? []
    ledgerTotal.value = data.data?.total ?? 0
  } catch (e: unknown) {
    ledgerErr.value = e instanceof Error ? e.message : t('redeemPage.fail')
    ledgerRows.value = []
    ledgerTotal.value = 0
  } finally {
    ledgerLoading.value = false
  }
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

async function redeem() {
  msg.value = ''
  result.value = null
  try {
    const { data } = await http.post<Envelope<{ amount_added?: number; new_balance?: number }>>('/api/user/redeem', {
      code: code.value,
    })
    if (data.code !== 0) throw new Error(data.message || t('redeemPage.fail'))
    msg.value = data.message || t('redeemPage.success')
    result.value = data.data
    ledgerPage.value = 1
    void loadLedger()
  } catch (e: unknown) {
    msg.value = e instanceof Error ? e.message : t('redeemPage.fail')
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
      <input v-model="code" :placeholder="t('redeemPage.ph')" class="th-input mb-4 w-full max-w-md" />
      <button type="button" class="th-btn-primary-flat max-w-md py-2.5 text-sm" @click="redeem">{{ t('redeemPage.submit') }}</button>
      <p class="mt-4 text-sm text-slate-400">{{ msg }}</p>
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
      <p v-if="ledgerErr" class="mb-2 text-sm text-rose-400">{{ ledgerErr }}</p>
      <p v-else-if="!ledgerLoading && ledgerRows.length === 0" class="text-sm text-slate-500">{{ t('redeemPage.ledgerEmpty') }}</p>
      <div v-else class="overflow-x-auto rounded-lg border border-slate-800/80">
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
              <td class="px-3 py-2 font-mono text-xs text-slate-400">{{ r.created_at || '—' }}</td>
              <td class="px-3 py-2 font-mono text-xs text-slate-300">{{ r.code_hint || '—' }}</td>
              <td class="px-3 py-2 font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ r.amount }}</td>
              <td class="px-3 py-2 font-mono text-slate-300">{{ r.balance_after }}</td>
              <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ r.redeem_code_id }}</td>
            </tr>
          </tbody>
        </table>
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
    </section>

    <div class="border-t border-white/[0.06] pt-6">
      <ContactCardDisclosure align="start" compact summary-key="home.contactHintRedeem" />
    </div>
  </div>
</template>
