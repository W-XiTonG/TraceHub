<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type LogRow = {
  id: number
  request_id?: string
  model: string
  channel_id?: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: number
  request_time: string
  response_time_ms?: number
  status_code?: number
  error_message?: string
}

const items = ref<LogRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const err = ref('')
const flash = ref('')
const compact = ref(false)
const quickSearch = ref('')
const selectedIds = ref<number[]>([])

const filters = reactive({
  model: '',
  status_code: '',
  start_time: '',
  end_time: '',
})

const displayItems = computed(() => {
  const q = quickSearch.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((row) => {
    const blob = [
      row.model,
      row.request_id || '',
      String(row.status_code ?? ''),
      row.error_message || '',
    ]
      .join(' ')
      .toLowerCase()
    return blob.includes(q)
  })
})

const allDisplaySelected = computed(() => {
  const rows = displayItems.value
  if (!rows.length) return false
  return rows.every((r) => selectedIds.value.includes(r.id))
})

function pruneSelection() {
  const ids = new Set(displayItems.value.map((r) => r.id))
  selectedIds.value = selectedIds.value.filter((id) => ids.has(id))
}

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const r of displayItems.value) set.add(r.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(displayItems.value.map((r) => r.id))
    selectedIds.value = selectedIds.value.filter((id) => !drop.has(id))
  }
}

function toggleRow(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((x) => x !== id)
  } else {
    selectedIds.value = [...selectedIds.value, id]
  }
}

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: LogRow[]; total: number }>>('/api/user/usage-logs', {
      params: {
        page: page.value,
        page_size: pageSize,
        model: filters.model || undefined,
        status_code: filters.status_code || undefined,
        start_time: filters.start_time || undefined,
        end_time: filters.end_time || undefined,
      },
    })
    if (data.code !== 0) throw new Error(data.message || t('modelsPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
    pruneSelection()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('modelsPage.loadFail')
  }
}

function applyFilters() {
  page.value = 1
  void load()
}

function resetFilters() {
  filters.model = ''
  filters.status_code = ''
  filters.start_time = ''
  filters.end_time = ''
  quickSearch.value = ''
  page.value = 1
  void load()
}

function statusBadgeClass(code?: number): string {
  const base = 'th-badge-http'
  if (code == null) return `${base} th-badge-http--neutral`
  if (code >= 200 && code < 300) return `${base} th-badge-http--2xx`
  if (code >= 400 && code < 500) return `${base} th-badge-http--4xx`
  if (code >= 500) return `${base} th-badge-http--5xx`
  return `${base} th-badge-http--neutral`
}

function rowToTsv(row: LogRow) {
  const cells = [
    row.request_time,
    row.model,
    row.request_id || '',
    String(row.total_tokens),
    String(row.total_cost),
    String(row.status_code ?? ''),
    String(row.response_time_ms ?? ''),
    (row.error_message || '').replace(/\s+/g, ' ').trim(),
  ]
  return cells.join('\t')
}

const tsvHeader = computed(() =>
  ['time', 'model', 'request_id', 'tokens', 'cost', 'status', 'latency_ms', 'error'].join('\t'),
)

async function copyText(text: string) {
  if (!text.trim()) return
  try {
    await navigator.clipboard.writeText(text)
    flash.value = t('logsPage.copied')
    window.setTimeout(() => {
      if (flash.value === t('logsPage.copied')) flash.value = ''
    }, 2200)
  } catch {
    flash.value = t('logsPage.copyFail')
  }
}

function copySelectedTsv() {
  const set = new Set(selectedIds.value)
  const rows = displayItems.value.filter((r) => set.has(r.id))
  if (!rows.length) {
    err.value = t('logsPage.nothingSelected')
    return
  }
  err.value = ''
  const body = [tsvHeader.value, ...rows.map(rowToTsv)].join('\n')
  void copyText(body)
}

function copyOneRow(row: LogRow) {
  void copyText(`${tsvHeader.value}\n${rowToTsv(row)}`)
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="flex min-w-0 items-start gap-3">
        <span class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-violet-500/12 text-violet-300 ring-1 ring-violet-500/25">
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z" />
          </svg>
        </span>
        <div class="min-w-0">
          <p class="text-[11px] font-medium uppercase tracking-wider text-slate-500">{{ t('logsPage.pageKicker') }}</p>
          <h1 class="text-2xl font-bold tracking-tight text-slate-100">{{ t('logsPage.title') }}</h1>
          <p class="mt-1 max-w-2xl text-sm leading-relaxed text-slate-500">{{ t('logsPage.desc') }}</p>
        </div>
      </div>
      <button
        type="button"
        class="shrink-0 self-start text-[13px] text-cyan-400/90 underline-offset-4 hover:text-cyan-300 hover:underline"
        @click="compact = !compact"
      >
        {{ compact ? t('logsPage.standardList') : t('logsPage.compactList') }}
      </button>
    </div>

    <div class="th-console-muted-bar mb-4 flex flex-col gap-3 px-4 py-3">
      <div class="flex flex-wrap items-end gap-3">
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.model') }}</span>
          <input
            v-model="filters.model"
            :placeholder="t('logsPage.modelPh')"
            class="th-input text-sm"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.status') }}</span>
          <input
            v-model="filters.status_code"
            :placeholder="t('logsPage.statusPh')"
            class="th-input !w-20 shrink-0 text-sm"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.start') }}</span>
          <input v-model="filters.start_time" class="th-input !w-44 shrink-0 text-sm" />
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.end') }}</span>
          <input v-model="filters.end_time" class="th-input !w-44 shrink-0 text-sm" />
        </label>
        <button type="button" class="th-btn-primary-flat px-3 py-2 text-sm" @click="applyFilters">
          {{ t('logsPage.filterBtn') }}
        </button>
        <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-300" @click="resetFilters">
          {{ t('logsPage.resetBtn') }}
        </button>
        <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-400" @click="load">
          {{ t('logsPage.query') }}
        </button>
      </div>
      <div class="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
        <input
          v-model="quickSearch"
          type="search"
          :placeholder="t('logsPage.toolbarSearchPh')"
          class="th-input min-w-[12rem] flex-1 !max-w-md"
        />
        <button type="button" class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200" @click="copySelectedTsv">
          {{ t('logsPage.copySelected') }}
        </button>
      </div>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>
    <p class="mb-4 text-[11px] text-slate-600">{{ t('logsPage.copyNote') }}</p>

    <div class="th-user-keys-table-wrap overflow-x-auto">
      <table class="w-full min-w-[880px] text-left" :class="compact ? 'text-xs' : 'text-sm'">
        <thead class="text-slate-500">
          <tr>
            <th class="w-10 px-3 py-2.5" :class="compact ? 'py-1.5' : ''">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="allDisplaySelected"
                @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
              />
            </th>
            <th class="whitespace-nowrap px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thTime') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.model') }}</th>
            <th v-if="!compact" class="hidden max-w-[10rem] px-3 py-2.5 font-medium lg:table-cell">{{ t('logsPage.thReqId') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thTokCol') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thCost') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thStatus') }}</th>
            <th class="px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thLatency') }}</th>
            <th class="px-3 py-2.5 text-right font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thActions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in displayItems" :key="row.id">
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="selectedIds.includes(row.id)"
                @change="toggleRow(row.id)"
              />
            </td>
            <td class="whitespace-nowrap px-3 text-xs text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ row.request_time }}</td>
            <td class="px-3 font-mono text-xs text-slate-200" :class="compact ? 'py-1.5' : 'py-2.5'">{{ row.model }}</td>
            <td
              v-if="!compact"
              class="hidden max-w-[10rem] truncate px-3 font-mono text-[11px] text-slate-500 lg:table-cell"
              :class="compact ? '' : 'py-2.5'"
              :title="row.request_id"
            >
              {{ row.request_id || t('modelsPage.dash') }}
            </td>
            <td class="px-3 text-xs" :class="compact ? 'py-1.5' : 'py-2.5'">{{ row.total_tokens }}</td>
            <td class="px-3 text-xs" :class="compact ? 'py-1.5' : 'py-2.5'">{{ row.total_cost }}</td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span :class="statusBadgeClass(row.status_code)">
                {{ row.status_code ?? t('modelsPage.dash') }}
              </span>
            </td>
            <td class="px-3 text-xs text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">{{ row.response_time_ms ?? t('modelsPage.dash') }}</td>
            <td class="px-3 text-right" :class="compact ? 'py-1.5' : 'py-2.5'">
              <button
                type="button"
                class="text-[11px] text-cyan-400 hover:underline"
                :title="t('logsPage.copyRow')"
                @click="copyOneRow(row)"
              >
                {{ t('logsPage.copyRow') }}
              </button>
            </td>
          </tr>
          <tr v-if="items.length && !displayItems.length">
            <td :colspan="compact ? 8 : 9" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.noClientMatch') }}</td>
          </tr>
          <tr v-if="!items.length">
            <td :colspan="compact ? 8 : 9" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="total > pageSize" class="mt-4 flex items-center gap-3 text-sm text-slate-400">
      <button
        type="button"
        class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
        :disabled="page <= 1"
        @click="
          page--;
          load()
        "
      >
        {{ t('logsPage.prev') }}
      </button>
      <span>{{ t('logsPage.pageInfo', { page, total }) }}</span>
      <button
        type="button"
        class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
        :disabled="page * pageSize >= total"
        @click="
          page++;
          load()
        "
      >
        {{ t('logsPage.next') }}
      </button>
    </div>
  </div>
</template>
