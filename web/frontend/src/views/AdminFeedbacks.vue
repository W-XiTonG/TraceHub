<script setup lang="ts">
import axios from 'axios'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type Row = {
  id: number
  user_id: number
  user_email: string
  preset_key: string
  title: string
  detail: string
  review_state: number
  reward_points: number
  balance_after_reward?: number | null
  admin_note?: string | null
  reviewed_by?: number | null
  reviewed_at?: string | null
  created_at?: string
}

const rows = ref<Row[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 30
const emailFilter = ref('')
const stateFilter = ref<string>('')
const err = ref('')
const msg = ref('')

const reviewOpen = ref(false)
const sel = ref<Row | null>(null)
const validBug = ref(true)
const rewardPointsStr = ref('0')
const adminNote = ref('')

const stateOptions = computed(() => [
  { value: '', label: t('adminFeedbacks.allStates') },
  { value: '0', label: t('adminFeedbacks.st0') },
  { value: '1', label: t('adminFeedbacks.st1') },
  { value: '2', label: t('adminFeedbacks.st2') },
  { value: '3', label: t('adminFeedbacks.st3') },
])

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function closedActionLabel(s: number): string {
  if (s === 1) return t('adminFeedbacks.doneInvalid')
  if (s === 2) return t('adminFeedbacks.doneNoReward')
  if (s === 3) return t('adminFeedbacks.doneReward')
  return t('adminFeedbacks.alreadyDone')
}

function stateLabel(s: number) {
  if (s === 1) return t('adminFeedbacks.st1')
  if (s === 2) return t('adminFeedbacks.st2')
  if (s === 3) return t('adminFeedbacks.st3')
  return t('adminFeedbacks.st0')
}

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function load() {
  err.value = ''
  try {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    const em = emailFilter.value.trim()
    if (em !== '') params.email = em
    const st = stateFilter.value.trim()
    if (st !== '') params.review_state = st
    const { data } = await http.get<Envelope<{ items: Row[]; total: number }>>('/api/admin/feedbacks', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    rows.value = data.data?.items ?? []
    total.value = data.data?.total ?? 0
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function applyFilter() {
  page.value = 1
  msg.value = ''
  void load()
}

function openReview(r: Row) {
  if (r.review_state !== 0) return
  err.value = ''
  sel.value = r
  validBug.value = true
  rewardPointsStr.value = '0'
  adminNote.value = ''
  reviewOpen.value = true
}

function closeReview() {
  reviewOpen.value = false
  sel.value = null
}

async function submitReview() {
  if (!sel.value) return
  const pts = parseInt(rewardPointsStr.value.trim(), 10) || 0
  if (pts > 0 && !validBug.value) {
    err.value = t('adminFeedbacks.needValidForReward')
    return
  }
  err.value = ''
  try {
    const { data } = await http.put<Envelope<{ new_balance?: number }>>(
      `/api/admin/feedbacks/${sel.value.id}/review`,
      {
        valid_bug: validBug.value,
        reward_points: pts,
        admin_note: adminNote.value,
      },
    )
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.requestFail'))
    msg.value = data.message?.trim() ? data.message : ''
    closeReview()
    await load()
  } catch (e: unknown) {
    err.value = httpErr(e)
  }
}

function clearFilters() {
  emailFilter.value = ''
  stateFilter.value = ''
  page.value = 1
  msg.value = ''
  void load()
}

function refreshOnly() {
  void load()
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

function onVisibility() {
  if (document.visibilityState !== 'visible') return
  void load()
}

onMounted(() => {
  void load()
  document.addEventListener('visibilitychange', onVisibility)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibility)
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold text-slate-100">{{ t('adminFeedbacks.title') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-500">{{ t('adminFeedbacks.intro') }}</p>
      <p class="mt-1 max-w-3xl text-xs text-slate-600">{{ t('adminFeedbacks.listSyncHint') }}</p>
    </div>

    <div class="flex flex-wrap items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminFeedbacks.lblEmail') }}</label>
        <input v-model="emailFilter" class="th-input w-56 text-sm" :placeholder="t('adminFeedbacks.phEmail')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminFeedbacks.filterState') }}</label>
        <select v-model="stateFilter" class="th-input w-44 text-sm">
          <option v-for="o in stateOptions" :key="o.value || 'all'" :value="o.value">{{ o.label }}</option>
        </select>
      </div>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="applyFilter">{{ t('adminFeedbacks.query') }}</button>
      <button
        type="button"
        class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]"
        @click="refreshOnly"
      >
        {{ t('adminFeedbacks.refresh') }}
      </button>
      <button
        type="button"
        class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]"
        @click="clearFilters"
      >
        {{ t('adminFeedbacks.clear') }}
      </button>
    </div>

    <p v-if="err" class="text-sm text-rose-400">{{ err }}</p>
    <p v-if="msg" class="text-sm text-emerald-400">{{ msg }}</p>

    <p class="text-xs text-slate-500">{{ t('adminFeedbacks.total', { n: total }) }}</p>

    <div class="overflow-x-auto rounded-lg border border-white/[0.06]">
      <table class="w-full min-w-[960px] text-left text-xs text-slate-300">
        <thead class="border-b border-white/[0.06] bg-white/[0.02] text-[10px] uppercase tracking-wide text-slate-500">
          <tr>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thId') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thUser') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thPreset') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thTitle') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thState') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thReward') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thCreated') }}</th>
            <th class="px-3 py-2">{{ t('adminFeedbacks.thActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rows" :key="r.id" class="border-b border-white/[0.04] align-top last:border-0">
            <td class="px-3 py-2 font-mono text-slate-400">{{ r.id }}</td>
            <td class="px-3 py-2">
              <div class="font-mono text-[11px] text-slate-400">#{{ r.user_id }}</div>
              <div class="max-w-[160px] truncate text-slate-400" :title="r.user_email">{{ r.user_email }}</div>
            </td>
            <td class="px-3 py-2 text-slate-500">{{ r.preset_key || '—' }}</td>
            <td class="max-w-[220px] px-3 py-2">
              <div class="truncate font-medium text-slate-200" :title="r.title">{{ r.title }}</div>
              <div class="mt-1 line-clamp-2 text-[11px] text-slate-500" :title="r.detail">{{ r.detail }}</div>
            </td>
            <td class="px-3 py-2">{{ stateLabel(r.review_state) }}</td>
            <td class="px-3 py-2 font-mono tabular-nums">{{ r.review_state === 3 ? r.reward_points : '—' }}</td>
            <td class="px-3 py-2 font-mono text-[11px] text-slate-500">{{ r.created_at?.slice(0, 19)?.replace('T', ' ') }}</td>
            <td class="px-3 py-2">
              <button
                v-if="r.review_state === 0"
                type="button"
                class="text-sky-400 hover:underline"
                @click="openReview(r)"
              >
                {{ t('adminFeedbacks.review') }}
              </button>
              <span v-else class="whitespace-nowrap text-slate-500" :title="t('adminFeedbacks.opDoneHint')">{{
                closedActionLabel(r.review_state)
              }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="flex items-center gap-2 text-xs text-slate-500">
      <button type="button" class="rounded border border-white/[0.08] px-2 py-1 disabled:opacity-40" :disabled="page <= 1" @click="prevPage">
        {{ t('adminFeedbacks.pagePrev') }}
      </button>
      <span>{{ page }} / {{ totalPages() }}</span>
      <button
        type="button"
        class="rounded border border-white/[0.08] px-2 py-1 disabled:opacity-40"
        :disabled="page >= totalPages()"
        @click="nextPage"
      >
        {{ t('adminFeedbacks.pageNext') }}
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="reviewOpen && sel"
        class="th-feedback-backdrop fixed inset-0 z-[80] flex items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        @click.self="closeReview"
      >
        <div class="th-feedback-modal thin-scrollbar max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl p-5 sm:p-6">
          <h2 class="th-feedback-modal__title text-lg font-semibold">{{ t('adminFeedbacks.dlgTitle', { id: sel.id }) }}</h2>
          <p class="th-feedback-modal__intro mt-2 text-xs">{{ sel.title }}</p>
          <pre class="th-dialog-code-block mt-3">{{ sel.detail }}</pre>

          <label class="th-modal-check-row mt-4 flex cursor-pointer items-center gap-2 text-sm">
            <input v-model="validBug" type="checkbox" class="rounded border-slate-600" />
            {{ t('adminFeedbacks.lblValid') }}
          </label>
          <p class="th-feedback-modal__muted mt-1 text-[11px]">{{ t('adminFeedbacks.validHint') }}</p>

          <label class="th-feedback-modal__label mt-4 block text-xs font-medium">{{ t('adminFeedbacks.lblReward') }}</label>
          <input v-model="rewardPointsStr" type="text" inputmode="numeric" class="th-input mt-1 w-full font-mono text-sm" />
          <p class="th-feedback-modal__muted mt-1 text-[11px]">{{ t('adminFeedbacks.rewardHint') }}</p>

          <label class="th-feedback-modal__label mt-4 block text-xs font-medium">{{ t('adminFeedbacks.lblNote') }}</label>
          <textarea v-model="adminNote" rows="3" class="th-input mt-1 w-full text-sm" :placeholder="t('adminFeedbacks.notePh')" />

          <div class="mt-5 flex flex-wrap gap-2">
            <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="submitReview">{{ t('adminFeedbacks.save') }}</button>
            <button type="button" class="th-modal-secondary-btn px-4 py-2 text-sm" @click="closeReview">
              {{ t('adminFeedbacks.close') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
