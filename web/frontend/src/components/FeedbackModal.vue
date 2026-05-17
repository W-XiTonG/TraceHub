<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const { t, tm } = useI18n()

type PresetRow = { key: string; title: string; template: string }

const presets = computed(() => {
  const raw = tm('feedbackPage.presets')
  return Array.isArray(raw) ? (raw as PresetRow[]) : []
})

const isCustom = ref(false)
const presetKey = ref('billing')
const title = ref('')
const detail = ref('')
const msg = ref('')
const err = ref('')
const submitting = ref(false)
const todayLeft = ref<number | null>(null)

type ListRow = {
  id: number
  preset_key: string
  title: string
  detail: string
  review_state: number
  reward_points: number
  created_at?: string
  reviewed_at?: string | null
}

const rows = ref<ListRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 8
const listErr = ref('')
const listLoading = ref(false)

function close() {
  emit('update:modelValue', false)
}

function onKeyDown(e: KeyboardEvent) {
  if (!props.modelValue) return
  if (e.key === 'Escape') {
    e.preventDefault()
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', onKeyDown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeyDown)
})

function presetLabel(key: string) {
  if (!key) return t('feedbackPage.typeCustom')
  const p = presets.value.find((x) => x.key === key)
  return p?.title || key
}

function stateLabel(s: number) {
  if (s === 1) return t('feedbackPage.state1')
  if (s === 2) return t('feedbackPage.state2')
  if (s === 3) return t('feedbackPage.state3')
  return t('feedbackPage.state0')
}

function applyPreset(k: string) {
  const p = presets.value.find((x) => x.key === k)
  if (p) {
    title.value = p.title
    detail.value = p.template
  }
}

function initDefaults() {
  const first = presets.value[0]?.key
  if (first) {
    presetKey.value = first
    isCustom.value = false
    applyPreset(first)
  }
  msg.value = ''
  err.value = ''
}

watch([presetKey, isCustom], ([k, custom]) => {
  if (!custom && k) applyPreset(k)
})

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

async function loadMeta() {
  try {
    const { data } = await http.get<Envelope<{ today_count: number; daily_limit: number }>>('/api/user/feedbacks/meta/throttle')
    if (data.code !== 0) return
    const lim = data.data?.daily_limit ?? 20
    const n = data.data?.today_count ?? 0
    todayLeft.value = Math.max(0, lim - n)
  } catch {
    todayLeft.value = null
  }
}

async function loadList() {
  listErr.value = ''
  listLoading.value = true
  try {
    const { data } = await http.get<Envelope<{ items: ListRow[]; total: number }>>('/api/user/feedbacks', {
      params: { page: page.value, page_size: pageSize },
    })
    if (data.code !== 0) throw new Error(data.message || t('feedbackPage.fail'))
    rows.value = data.data?.items ?? []
    total.value = data.data?.total ?? 0
  } catch (e: unknown) {
    listErr.value = e instanceof Error ? e.message : t('feedbackPage.fail')
    rows.value = []
    total.value = 0
  } finally {
    listLoading.value = false
  }
}

function prevListPage() {
  if (page.value > 1) {
    page.value--
    void loadList()
  }
}

function nextListPage() {
  if (page.value < totalPages()) {
    page.value++
    void loadList()
  }
}

function selectPreset(k: string) {
  isCustom.value = false
  presetKey.value = k
}

function selectCustom() {
  isCustom.value = true
}

async function submit() {
  err.value = ''
  msg.value = ''
  submitting.value = true
  try {
    const pk = isCustom.value ? '' : presetKey.value
    const { data } = await http.post<Envelope<{ id: number }>>('/api/user/feedbacks', {
      preset_key: pk,
      title: title.value.trim(),
      detail: detail.value.trim(),
    })
    if (data.code !== 0) throw new Error(data.message || t('feedbackPage.fail'))
    msg.value = t('feedbackPage.success')
    page.value = 1
    void loadList()
    void loadMeta()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('feedbackPage.fail')
  } finally {
    submitting.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      initDefaults()
      void loadMeta()
      void loadList()
    }
  },
)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="modelValue"
      class="th-feedback-backdrop fixed inset-0 z-[90] flex items-end justify-center p-0 sm:items-center sm:p-4 sm:p-6"
      role="dialog"
      aria-modal="true"
      :aria-label="t('feedbackPage.title')"
      @click.self="close"
    >
      <div
        class="th-feedback-modal flex max-h-[min(100dvh,900px)] w-full max-w-3xl flex-col overflow-hidden rounded-t-2xl sm:max-h-[min(92vh,900px)] sm:rounded-xl"
        @click.stop
      >
        <header class="th-feedback-modal__head flex shrink-0 items-start justify-between gap-4 px-5 py-4 sm:px-6">
          <div class="min-w-0 pr-2">
            <h2 class="th-feedback-modal__title text-lg font-semibold leading-snug sm:text-xl">{{ t('feedbackPage.title') }}</h2>
            <p class="th-feedback-modal__intro mt-1 text-xs leading-relaxed sm:text-[13px]">{{ t('feedbackPage.intro') }}</p>
          </div>
          <button type="button" class="th-feedback-modal__close shrink-0 rounded-lg px-3 py-2 text-sm transition" @click="close">
            {{ t('common.close') }}
          </button>
        </header>

        <div class="thin-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-4 sm:px-6 sm:py-5">
          <p class="th-feedback-hint mb-3">{{ t('feedbackPage.rewardHint') }}</p>
          <p v-if="todayLeft != null" class="th-feedback-modal__muted mb-4 text-xs">{{ t('feedbackPage.todayLimit', { n: todayLeft }) }}</p>

          <p class="th-feedback-modal__label mb-2 text-[10px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.presetLabel') }}</p>
          <div class="mb-4 flex max-h-[min(30vh,11rem)] flex-wrap gap-2 overflow-y-auto thin-scrollbar">
            <button
              v-for="p in presets"
              :key="p.key"
              type="button"
              class="th-feedback-chip"
              :class="{ 'th-feedback-chip--on': !isCustom && presetKey === p.key }"
              @click="selectPreset(p.key)"
            >
              {{ p.title }}
            </button>
            <button type="button" class="th-feedback-chip" :class="{ 'th-feedback-chip--on': isCustom }" @click="selectCustom">
              {{ t('feedbackPage.customTitle') }}
            </button>
          </div>

          <label class="th-feedback-modal__label mb-1.5 block text-xs font-medium">{{ t('feedbackPage.titleLabel') }}</label>
          <input v-model="title" class="th-input mb-4 w-full text-sm" :placeholder="t('feedbackPage.titlePh')" />

          <label class="th-feedback-modal__label mb-1.5 block text-xs font-medium">{{ t('feedbackPage.detailLabel') }}</label>
          <p class="th-feedback-modal__muted mb-2 text-[11px] leading-relaxed">{{ t('feedbackPage.detailHint') }}</p>
          <textarea v-model="detail" rows="5" class="th-input w-full font-mono text-sm leading-relaxed sm:min-h-[9rem]" />

          <details class="mt-6 border-t pt-4" style="border-color: var(--th-modal-divider)">
            <summary class="th-feedback-modal__summary cursor-pointer text-sm font-medium outline-none">
              {{ t('feedbackPage.listTitle') }}
            </summary>
            <div class="mt-3">
              <div class="mb-2 flex justify-end">
                <button type="button" class="th-feedback-refresh" :disabled="listLoading" @click="loadList">
                  {{ t('feedbackPage.refresh') }}
                </button>
              </div>
              <p v-if="listErr" class="mb-2 text-xs text-rose-500">{{ listErr }}</p>
              <p v-if="!listLoading && rows.length === 0" class="th-feedback-modal__muted text-xs">{{ t('feedbackPage.listEmpty') }}</p>
              <div v-else class="th-feedback-table-wrap overflow-x-auto">
                <table class="w-full min-w-[520px] text-left text-[11px]">
                  <thead>
                    <tr>
                      <th class="px-3 py-2 text-[9px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.thTime') }}</th>
                      <th class="px-3 py-2 text-[9px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.thType') }}</th>
                      <th class="px-3 py-2 text-[9px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.thTitle') }}</th>
                      <th class="px-3 py-2 text-[9px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.thState') }}</th>
                      <th class="px-3 py-2 text-[9px] font-semibold uppercase tracking-wide">{{ t('feedbackPage.thReward') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="r in rows" :key="r.id">
                      <td class="px-3 py-2 font-mono text-[11px] opacity-80">{{ r.created_at?.slice(0, 16)?.replace('T', ' ') }}</td>
                      <td class="px-3 py-2 opacity-80">{{ presetLabel(r.preset_key) }}</td>
                      <td class="max-w-[160px] truncate px-3 py-2" :title="r.title">{{ r.title }}</td>
                      <td class="px-3 py-2">{{ stateLabel(r.review_state) }}</td>
                      <td class="px-3 py-2 font-mono tabular-nums">{{ r.review_state === 3 ? r.reward_points : '—' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div v-if="total > pageSize" class="th-feedback-modal__muted mt-2 flex items-center gap-2 text-[11px]">
                <button
                  type="button"
                  class="th-feedback-refresh px-2 py-1"
                  :disabled="page <= 1 || listLoading"
                  @click="prevListPage"
                >
                  ‹
                </button>
                <span>{{ page }} / {{ totalPages() }}</span>
                <button
                  type="button"
                  class="th-feedback-refresh px-2 py-1"
                  :disabled="page >= totalPages() || listLoading"
                  @click="nextListPage"
                >
                  ›
                </button>
              </div>
            </div>
          </details>
        </div>

        <footer
          class="th-feedback-modal__footer shrink-0 border-t px-5 py-4 pb-[max(1rem,env(safe-area-inset-bottom))] sm:px-6"
          style="border-color: var(--th-modal-divider); background: var(--th-modal-footer-bg); box-shadow: var(--th-modal-footer-shadow)"
        >
          <div class="flex flex-wrap items-center gap-3">
            <button type="button" class="th-btn-primary-flat text-sm" :disabled="submitting" @click="submit">
              {{ submitting ? t('feedbackPage.submitting') : t('feedbackPage.submit') }}
            </button>
          </div>
          <p v-if="err" class="mt-3 text-sm text-rose-500">{{ err }}</p>
          <p v-if="msg" class="mt-3 text-sm text-emerald-600">{{ msg }}</p>
        </footer>
      </div>
    </div>
  </Teleport>
</template>
