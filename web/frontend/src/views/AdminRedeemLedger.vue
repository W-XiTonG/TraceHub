<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import ThPgStyleCombo from '../components/ThPgStyleCombo.vue'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'

const { t } = useI18n()
const route = useRoute()

type Row = {
  id: number
  user_id: number
  user_email: string
  redeem_code_id: number
  amount: number
  balance_after: number
  code_hint?: string | null
  created_at?: string
}

type RedeemCodeMatchMode = 'exact' | 'prefix' | 'contains'

const rows = ref<Row[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 50
const userIdFilter = ref('')
const redeemCodeFilter = ref('')
const redeemCodeMatch = ref<RedeemCodeMatchMode>('exact')
const redeemCodeIdFilter = ref('')
const msg = ref('')
const err = ref('')

const redeemMatchOptions = computed(() => [
  { value: 'exact' as const, label: t('adminRedeemLedger.matchExact') },
  { value: 'prefix' as const, label: t('adminRedeemLedger.matchPrefix') },
  { value: 'contains' as const, label: t('adminRedeemLedger.matchContains') },
])

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

async function load() {
  err.value = ''
  msg.value = ''
  await runLoad(async () => {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    const uid = userIdFilter.value.trim()
    if (uid !== '') params.user_id = uid
    const code = redeemCodeFilter.value.trim()
    if (code !== '') {
      params.redeem_code = code
      params.redeem_code_match = redeemCodeMatch.value
    }
    const rcid = redeemCodeIdFilter.value.trim()
    if (rcid !== '') params.redeem_code_id = rcid
    const { data } = await http.get<Envelope<{ items: Row[]; total: number }>>('/api/admin/redeem-ledger', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    rows.value = data.data.items || []
    total.value = data.data.total ?? 0
  }).catch((e: unknown) => {
    err.value = httpErr(e)
  })
}

function applyFilter() {
  page.value = 1
  void load()
}

function clearFilters() {
  userIdFilter.value = ''
  redeemCodeFilter.value = ''
  redeemCodeMatch.value = 'exact'
  redeemCodeIdFilter.value = ''
  page.value = 1
  void load()
}

function onMatchModeChange() {
  if (redeemCodeFilter.value.trim()) {
    page.value = 1
    void load()
  }
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    void load()
  }
}

function nextPage() {
  if (page.value < totalPages()) {
    page.value++
    void load()
  }
}

onMounted(() => {
  const q = route.query.user_id
  if (typeof q === 'string' && q.trim()) userIdFilter.value = q.trim()
  const rcid = route.query.redeem_code_id
  if (typeof rcid === 'string' && rcid.trim()) redeemCodeIdFilter.value = rcid.trim()
  const m = route.query.redeem_code_match
  if (m === 'exact' || m === 'prefix' || m === 'contains') redeemCodeMatch.value = m
  void load()
})
</script>

<template>
  <div class="space-y-8">
    <header>
      <h1 class="text-xl font-semibold text-slate-100">{{ t('adminRedeemLedger.title') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-400">{{ t('adminRedeemLedger.intro') }}</p>
    </header>

    <section class="th-console-muted-bar th-console-muted-bar--sm flex flex-col gap-3 px-4 py-3">
      <div class="grid grid-cols-1 items-end gap-3 md:grid-cols-12">
        <label class="flex flex-col gap-1 md:col-span-2">
          <span class="text-xs font-medium text-slate-500 dark:text-slate-400">{{ t('adminRedeemLedger.lblUserId') }}</span>
          <input
            v-model="userIdFilter"
            type="text"
            inputmode="numeric"
            class="th-input w-full min-w-0 text-sm"
            :placeholder="t('adminRedeemLedger.phUserId')"
            @keyup.enter="applyFilter"
          />
        </label>
        <div class="flex min-w-0 flex-col gap-1 md:col-span-5">
          <span class="text-xs font-medium text-slate-500 dark:text-slate-400">{{ t('adminRedeemLedger.lblRedeemCode') }}</span>
          <div class="flex min-w-0 gap-2">
            <input
              v-model="redeemCodeFilter"
              type="text"
              autocomplete="off"
              class="th-input min-w-0 flex-1 text-sm"
              :placeholder="t('adminRedeemLedger.phRedeemCode')"
              @keyup.enter="applyFilter"
            />
            <ThPgStyleCombo
              v-model="redeemCodeMatch"
              :options="redeemMatchOptions"
              :aria-label="t('adminRedeemLedger.matchModeHint')"
              @change="onMatchModeChange"
            />
          </div>
        </div>
        <label class="flex flex-col gap-1 md:col-span-2">
          <span class="text-xs font-medium text-slate-500 dark:text-slate-400">{{ t('adminRedeemLedger.lblRedeemCodeId') }}</span>
          <input
            v-model="redeemCodeIdFilter"
            type="text"
            inputmode="numeric"
            class="th-input w-full min-w-0 text-sm"
            :placeholder="t('adminRedeemLedger.phRedeemCodeId')"
            @keyup.enter="applyFilter"
          />
        </label>
        <div class="flex gap-2 md:col-span-3 md:justify-end">
          <button type="button" class="th-btn-primary-flat min-h-[2.5rem] flex-1 px-4 py-2 text-sm md:min-h-0 md:flex-none" @click="applyFilter">
            {{ t('adminCommon.query') }}
          </button>
          <button
            type="button"
            class="th-ghost-hover min-h-[2.5rem] flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm text-slate-700 dark:border-slate-600 dark:text-slate-200 md:min-h-0 md:flex-none"
            @click="clearFilters"
          >
            {{ t('adminRedeemLedger.clearFilters') }}
          </button>
        </div>
      </div>
    </section>

    <p v-if="msg" class="text-sm text-emerald-400">{{ msg }}</p>
    <p v-if="err" class="text-sm text-rose-400">{{ err }}</p>
    <p class="text-xs text-slate-500">{{ t('adminRedeemLedger.total', { n: total }) }}</p>

    <ThTableSkeleton v-if="loadingUi && rows.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />
    <div v-if="loadingUi && rows.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>
    <div class="th-table-wrap relative hidden overflow-x-auto rounded-lg border border-slate-800/80 lg:block">
      <ThLoadingState v-if="loadingUi && rows.length > 0" variant="overlay" />
      <table class="min-w-full text-left text-sm">
        <thead class="border-b border-slate-800 bg-slate-950/80 text-xs uppercase text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thTime') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thUser') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thEmail') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thHint') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thAmount') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thBalanceAfter') }}</th>
            <th class="px-3 py-2">{{ t('adminRedeemLedger.thCodeId') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rows" :key="r.id" class="border-b border-slate-800/60 hover:bg-white/[0.02]">
            <td class="px-3 py-2 font-mono text-xs text-slate-400">{{ r.created_at || '—' }}</td>
            <td class="px-3 py-2 font-mono text-slate-300">{{ r.user_id }}</td>
            <td class="px-3 py-2 text-slate-300">{{ r.user_email || '—' }}</td>
            <td class="px-3 py-2 font-mono text-xs text-slate-400">{{ r.code_hint || '—' }}</td>
            <td class="px-3 py-2 font-mono text-base font-semibold tabular-nums th-redeem-quota-fg">{{ r.amount }}</td>
            <td class="px-3 py-2 font-mono text-slate-300">{{ r.balance_after }}</td>
            <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ r.redeem_code_id }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard v-for="r in rows" :key="`rdl-m-${r.id}`">
        <template #title>
          <p class="font-mono text-xs text-slate-400">{{ r.created_at || '—' }}</p>
          <p class="mt-0.5 text-sm text-slate-200">{{ r.user_email || `#${r.user_id}` }}</p>
        </template>
        <MobileLabelValueRow :label="t('adminRedeemLedger.thHint')">{{ r.code_hint || '—' }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminRedeemLedger.thAmount')">
          <span class="font-mono font-semibold th-redeem-quota-fg">{{ r.amount }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminRedeemLedger.thBalanceAfter')">{{ r.balance_after }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminRedeemLedger.thCodeId')">{{ r.redeem_code_id }}</MobileLabelValueRow>
      </MobileLabelValueCard>
    </div>

    <div v-if="total > pageSize" class="flex items-center justify-between text-xs text-slate-500">
      <span>{{ t('adminRedeem.pageOf', { page, pages: totalPages() }) }}</span>
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
          :disabled="page >= totalPages()"
          class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
          @click="nextPage"
        >
          {{ t('adminCommon.nextPage') }}
        </button>
      </div>
    </div>
  </div>
</template>
