<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import { copyTextToClipboard } from '../utils/clipboard'
import { apiErrorMessage } from '../stores/auth'
import DateTimeLocalFilterInput from '../components/DateTimeLocalFilterInput.vue'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { localDateTimeToRFC3339 } from '../utils/datetimeLocal'
import {
  formatUsageLogUsageLine,
  resolveUsageBillingMode,
  usageBillingBadgeKey,
  type UsageLogBillingRow,
} from '../lib/usageLedgerDetail'
import {
  flattenGroupedUsageLogs,
  groupVideoUsageLogs,
  isVideoJobTraceMessage,
  type FlatUsageLogEntry,
  type UsageLogGroup,
} from '../lib/usageLogGrouping'

const { t } = useI18n()

type LogRow = UsageLogBillingRow & {
  id: number
  request_id?: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  total_cost: number
  request_time: string
  response_time_ms?: number
  status_code?: number
  error_message?: string
  ip_address?: string
}

const items = ref<LogRow[]>([])
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const total = ref(0)
const page = ref(1)
const pageSize = 20
const err = ref('')
const flash = ref('')
const compact = ref(false)
const quickSearch = ref('')
const selectedIds = ref<number[]>([])
const expandedGroups = ref<Set<string>>(new Set())

const filters = reactive({
  model: '',
  status_code: '',
  ip: '',
})

const startLocal = ref('')
const endLocal = ref('')

const displayItems = computed(() => {
  const q = quickSearch.value.trim().toLowerCase()
  if (!q) return items.value
  return items.value.filter((row) => {
    const blob = [
      row.model,
      row.request_id || '',
      row.ip_address || '',
      String(row.status_code ?? ''),
      row.error_message || '',
    ]
      .join(' ')
      .toLowerCase()
    return blob.includes(q)
  })
})

const groupedLogs = computed(() => groupVideoUsageLogs(displayItems.value))

const flatRows = computed(() =>
  flattenGroupedUsageLogs(groupedLogs.value.groups, groupedLogs.value.standalone, expandedGroups.value),
)

const selectableRows = computed(() =>
  flatRows.value
    .filter((e) => e.kind !== 'group-child')
    .map((e) => (e.kind === 'standalone' ? e.row : e.group.primary) as LogRow),
)

const collapsedSubCount = computed(() =>
  groupedLogs.value.groups.reduce(
    (n, g) => n + (expandedGroups.value.has(g.key) ? 0 : g.children.length),
    0,
  ),
)

const allDisplaySelected = computed(() => {
  const rows = selectableRows.value
  if (!rows.length) return false
  return rows.every((r) => selectedIds.value.includes(r.id))
})

const selectedCount = computed(() => selectedIds.value.length)

const toolbarSummary = computed(() => {
  const n = selectableRows.value.length
  const m = items.value.length
  const hidden = collapsedSubCount.value
  if (quickSearch.value.trim() && displayItems.value.length !== m) {
    const base = t('logsPage.toolbarFiltered', { n: displayItems.value.length, m })
    return hidden > 0 ? `${base} · ${t('logsPage.toolbarGroupedHidden', { k: hidden })}` : base
  }
  const base = t('logsPage.toolbarPageCount', { n, total: total.value })
  return hidden > 0 ? `${base} · ${t('logsPage.toolbarGroupedHidden', { k: hidden })}` : base
})

function pruneSelection() {
  const ids = new Set(selectableRows.value.map((r) => r.id))
  selectedIds.value = selectedIds.value.filter((id) => ids.has(id))
}

function toggleSelectAll(checked: boolean) {
  if (checked) {
    const set = new Set(selectedIds.value)
    for (const r of selectableRows.value) set.add(r.id)
    selectedIds.value = [...set]
  } else {
    const drop = new Set(selectableRows.value.map((r) => r.id))
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
  await runLoad(async () => {
    const { data } = await http.get<Envelope<{ items: LogRow[]; total: number }>>('/api/user/usage-logs', {
      params: {
        page: page.value,
        page_size: pageSize,
        model: filters.model || undefined,
        status_code: filters.status_code || undefined,
        ip: filters.ip.trim() || undefined,
        start_time: localDateTimeToRFC3339(startLocal.value),
        end_time: localDateTimeToRFC3339(endLocal.value),
      },
    })
    if (data.code !== 0) throw new Error(data.message || t('modelsPage.loadFail'))
    items.value = data.data.items
    total.value = data.data.total
    pruneSelection()
  }).catch((e: unknown) => {
    err.value = apiErrorMessage(e, t('modelsPage.loadFail'))
  })
}

function applyFilters() {
  page.value = 1
  void load()
}

function resetFilters() {
  filters.model = ''
  filters.status_code = ''
  filters.ip = ''
  startLocal.value = ''
  endLocal.value = ''
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

/** 表格展示用；复制/导出仍用原始 ISO 时间 */
function fmtLogTime(iso: string): string {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return iso
    const p = (n: number) => String(n).padStart(2, '0')
    return `${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
  } catch {
    return iso
  }
}

function parseModelCell(model: string): { route: boolean; path: string } {
  const m = model.trim()
  if (m.startsWith('route:')) {
    return { route: true, path: m.slice(6) }
  }
  return { route: false, path: m }
}

function billingBadgeClass(row: LogRow): string {
  const base = 'th-badge-billing'
  return resolveUsageBillingMode(row) === 'video_duration'
    ? `${base} th-badge-billing--video`
    : `${base} th-badge-billing--token`
}

function costCellClass(cost: number): string {
  if (cost > 0) return 'font-medium tabular-nums text-amber-200/95'
  return 'tabular-nums text-slate-500'
}

function rowFromEntry(entry: FlatUsageLogEntry): LogRow {
  if (entry.kind === 'standalone') return entry.row as LogRow
  if (entry.kind === 'group-child') return entry.row as LogRow
  return entry.group.primary as LogRow
}

function isGroupExpanded(key: string): boolean {
  return expandedGroups.value.has(key)
}

function toggleGroup(key: string) {
  const next = new Set(expandedGroups.value)
  if (next.has(key)) next.delete(key)
  else next.add(key)
  expandedGroups.value = next
}

function displayError(msg?: string): string {
  if (!msg?.trim() || isVideoJobTraceMessage(msg)) return t('modelsPage.dash')
  return msg.trim()
}

function groupHasChildren(group: UsageLogGroup): boolean {
  return group.children.length > 0
}

function expandGroupTitle(group: UsageLogGroup, expanded: boolean): string {
  const hint = t('logsPage.videoGroupSubCount', { n: group.children.length })
  return expanded ? `${t('logsPage.videoGroupCollapse')} · ${hint}` : `${t('logsPage.videoGroupExpand')} · ${hint}`
}

function rowToTsv(row: LogRow) {
  const mode = resolveUsageBillingMode(row)
  const cells = [
    row.request_time,
    row.model,
    t(usageBillingBadgeKey(mode)),
    formatUsageLogUsageLine(t, row),
    row.request_id || '',
    row.ip_address || '',
    String(row.total_cost),
    String(row.status_code ?? ''),
    String(row.response_time_ms ?? ''),
    (row.error_message || '').replace(/\s+/g, ' ').trim(),
  ]
  return cells.join('\t')
}

const tsvHeader = computed(() =>
  ['time', 'model', 'billing', 'usage', 'request_id', 'client_ip', 'cost', 'status', 'latency_ms', 'error'].join('\t'),
)

async function copyText(text: string) {
  if (!text.trim()) return
  try {
    await copyTextToClipboard(text)
    flash.value = t('logsPage.copied')
    window.setTimeout(() => {
      if (flash.value === t('logsPage.copied')) flash.value = ''
    }, 2200)
  } catch {
    flash.value = t('logsPage.copyFail')
  }
}

function rowsForSelectedId(id: number): LogRow[] {
  for (const g of groupedLogs.value.groups) {
    if (g.primary.id === id) {
      return [g.primary as LogRow, ...(g.children as LogRow[])]
    }
  }
  const row = selectableRows.value.find((r) => r.id === id)
  return row ? [row] : []
}

function copySelectedTsv() {
  const set = new Set(selectedIds.value)
  const lines: string[] = []
  for (const id of selectedIds.value) {
    if (!set.has(id)) continue
    lines.push(...rowsForSelectedId(id).map(rowToTsv))
  }
  if (!lines.length) {
    err.value = t('logsPage.nothingSelected')
    return
  }
  err.value = ''
  const body = [tsvHeader.value, ...lines].join('\n')
  void copyText(body)
}

function copyOneEntry(entry: FlatUsageLogEntry) {
  if (entry.kind === 'group-primary' && entry.group.children.length) {
    const rows = [entry.group.primary, ...entry.group.children] as LogRow[]
    void copyText([tsvHeader.value, ...rows.map(rowToTsv)].join('\n'))
    return
  }
  void copyText(`${tsvHeader.value}\n${rowToTsv(rowFromEntry(entry))}`)
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
        class="th-ghost-hover shrink-0 self-start rounded-lg border border-slate-600/80 px-3 py-1.5 text-[13px] text-slate-300"
        @click="compact = !compact"
      >
        {{ compact ? t('logsPage.standardList') : t('logsPage.compactList') }}
      </button>
    </div>

    <div class="th-console-muted-bar mb-4 flex flex-col gap-3 px-4 py-3.5">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.model') }}</span>
          <input
            v-model="filters.model"
            :placeholder="t('logsPage.modelPh')"
            class="th-input h-10 text-sm"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.status') }}</span>
          <input
            v-model="filters.status_code"
            :placeholder="t('logsPage.statusPh')"
            class="th-input h-10 !w-20 shrink-0 text-sm"
          />
        </label>
        <label class="flex flex-col gap-1 text-xs">
          <span class="text-slate-500">{{ t('logsPage.lblFilterIp') }}</span>
          <input
            v-model="filters.ip"
            :placeholder="t('logsPage.phFilterIp')"
            class="th-input h-10 w-36 shrink-0 font-mono text-sm"
            autocomplete="off"
            spellcheck="false"
          />
        </label>
        <div class="flex flex-col gap-1 text-xs">
          <label for="usage-dt-start" class="block cursor-pointer text-slate-500">{{ t('logsPage.start') }}</label>
          <DateTimeLocalFilterInput
            v-model="startLocal"
            input-id="usage-dt-start"
            :aria-label="t('logsPage.start')"
          />
        </div>
        <div class="flex flex-col gap-1 text-xs">
          <label for="usage-dt-end" class="block cursor-pointer text-slate-500">{{ t('logsPage.end') }}</label>
          <DateTimeLocalFilterInput
            v-model="endLocal"
            input-id="usage-dt-end"
            :aria-label="t('logsPage.end')"
          />
        </div>
        <div class="flex flex-wrap items-end gap-2 sm:col-span-2 xl:col-span-2">
          <button type="button" class="th-btn-primary-flat h-10 flex-1 px-4 text-sm sm:flex-none" @click="applyFilters">
            {{ t('logsPage.filterBtn') }}
          </button>
          <button
            type="button"
            class="th-ghost-hover h-10 flex-1 rounded-lg border border-slate-600 px-4 text-sm text-slate-300 sm:flex-none"
            @click="resetFilters"
          >
            {{ t('logsPage.resetBtn') }}
          </button>
        </div>
      </div>
      <div class="flex flex-col gap-2 border-t border-white/[0.06] pt-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <div class="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center">
          <input
            v-model="quickSearch"
            type="search"
            :placeholder="t('logsPage.toolbarSearchPh')"
            class="th-input min-w-0 flex-1 sm:max-w-md"
          />
          <p class="shrink-0 text-[11px] tabular-nums text-slate-500">{{ toolbarSummary }}</p>
        </div>
        <button
          type="button"
          class="th-ghost-hover inline-flex items-center justify-center gap-2 rounded-lg border border-slate-600 px-3 py-2 text-sm text-slate-200 disabled:opacity-45"
          :disabled="selectedCount === 0"
          @click="copySelectedTsv"
        >
          {{ t('logsPage.copySelected') }}
          <span
            v-if="selectedCount > 0"
            class="rounded-full bg-cyan-500/15 px-1.5 py-0.5 text-[10px] font-medium tabular-nums text-cyan-300"
          >
            {{ t('logsPage.selectedCount', { n: selectedCount }) }}
          </span>
        </button>
      </div>
    </div>

    <p v-if="flash" class="mb-3 text-sm text-emerald-400">{{ flash }}</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="8" :columns="5" class="mb-4" />

    <div class="th-user-keys-table-wrap th-usage-logs-table-wrap relative hidden overflow-auto lg:block">
      <ThLoadingState v-if="loadingUi && items.length > 0" variant="overlay" :label="t('common.loading')" />
      <table class="w-full min-w-[56rem] text-left" :class="compact ? 'text-xs' : 'text-sm'">
        <thead class="text-[11px] uppercase tracking-wide text-slate-500">
          <tr>
            <th class="w-10 px-3 py-2.5" :class="compact ? 'py-1.5' : ''">
              <input
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="allDisplaySelected"
                @change="toggleSelectAll(($event.target as HTMLInputElement).checked)"
              />
            </th>
            <th class="w-[8.5rem] whitespace-nowrap px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">
              {{ t('logsPage.thTime') }}
            </th>
            <th class="th-usage-logs-expand-col py-2.5" :class="compact ? 'py-1.5' : ''" aria-hidden="true" />
            <th class="min-w-[12rem] px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.model') }}</th>
            <th v-if="!compact" class="hidden w-[7rem] px-3 py-2.5 font-medium xl:table-cell">{{ t('logsPage.thReqId') }}</th>
            <th class="w-[5.5rem] px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thBilling') }}</th>
            <th class="min-w-[8rem] px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thUsage') }}</th>
            <th class="w-[5rem] px-3 py-2.5 text-right font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thCost') }}</th>
            <th class="w-[4.5rem] px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">{{ t('logsPage.thStatus') }}</th>
            <th v-if="!compact" class="hidden min-w-[8rem] max-w-[14rem] px-3 py-2.5 font-medium xl:table-cell">
              {{ t('logsPage.thError') }}
            </th>
            <th class="w-[7.5rem] whitespace-nowrap px-3 py-2.5 font-medium" :class="compact ? 'py-1.5' : ''">
              {{ t('logsPage.thIp') }}
            </th>
            <th class="w-[4.5rem] px-3 py-2.5 text-right font-medium" :class="compact ? 'py-1.5' : ''">
              {{ t('logsPage.thLatency') }}
            </th>
            <th class="w-[5.5rem] px-3 py-2.5 text-right font-medium" :class="compact ? 'py-1.5' : ''">
              {{ t('logsPage.thActions') }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="entry in flatRows"
            :key="
              entry.kind === 'group-child'
                ? `c-${entry.row.id}`
                : entry.kind === 'standalone'
                  ? `s-${entry.row.id}`
                  : `p-${entry.group.primary.id}`
            "
            :class="entry.kind === 'group-child' ? 'th-usage-logs-row--child' : ''"
          >
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <input
                v-if="entry.kind !== 'group-child'"
                type="checkbox"
                class="h-4 w-4 rounded border-slate-600 bg-slate-950 text-cyan-500"
                :checked="selectedIds.includes(rowFromEntry(entry).id)"
                @change="toggleRow(rowFromEntry(entry).id)"
              />
            </td>
            <td
              class="whitespace-nowrap px-3 font-mono text-xs tabular-nums text-slate-400"
              :class="compact ? 'py-1.5' : 'py-2.5'"
              :title="rowFromEntry(entry).request_time"
            >
              {{ fmtLogTime(rowFromEntry(entry).request_time) }}
            </td>
            <td class="th-usage-logs-expand-col align-top" :class="compact ? 'py-1.5' : 'py-2.5'">
              <button
                v-if="entry.kind === 'group-primary' && groupHasChildren(entry.group)"
                type="button"
                class="th-usage-logs-expand-btn"
                :aria-expanded="isGroupExpanded(entry.group.key)"
                :aria-label="expandGroupTitle(entry.group, isGroupExpanded(entry.group.key))"
                :title="expandGroupTitle(entry.group, isGroupExpanded(entry.group.key))"
                @click.stop="toggleGroup(entry.group.key)"
              >
                <svg
                  class="h-3.5 w-3.5 transition-transform"
                  :class="isGroupExpanded(entry.group.key) ? 'rotate-90' : ''"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                  aria-hidden="true"
                >
                  <path
                    fill-rule="evenodd"
                    d="M7.21 14.77a.75.75 0 01.02-1.06L10.94 10 7.23 6.29a.75.75 0 111.04-1.08l4.25 4.25a.75.75 0 010 1.08l-4.25 4.25a.75.75 0 01-1.06-.02z"
                    clip-rule="evenodd"
                  />
                </svg>
                <span class="tabular-nums">{{ entry.group.children.length }}</span>
              </button>
              <span v-else-if="entry.kind === 'group-child'" class="th-usage-logs-sub-badge">
                {{ t('logsPage.videoGroupSubRow') }}
              </span>
            </td>
            <td class="th-usage-logs-model-cell px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <p class="break-all font-mono text-xs leading-snug text-slate-200" :title="rowFromEntry(entry).model">
                <span v-if="parseModelCell(rowFromEntry(entry).model).route" class="text-slate-500">route:</span>
                <span :class="parseModelCell(rowFromEntry(entry).model).route ? 'text-slate-300' : ''">{{
                  parseModelCell(rowFromEntry(entry).model).path
                }}</span>
              </p>
            </td>
            <td
              v-if="!compact"
              class="hidden max-w-[7rem] truncate px-3 font-mono text-[11px] text-slate-500 xl:table-cell"
              :class="compact ? '' : 'py-2.5'"
              :title="rowFromEntry(entry).request_id"
            >
              {{
                rowFromEntry(entry).request_id
                  ? rowFromEntry(entry).request_id!.slice(0, 8) + '…'
                  : t('modelsPage.dash')
              }}
            </td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span
                :class="billingBadgeClass(rowFromEntry(entry))"
                :title="t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry))))"
              >
                {{ t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry)))) }}
              </span>
            </td>
            <td
              class="max-w-[14rem] px-3 text-xs leading-relaxed text-slate-300"
              :class="compact ? 'py-1.5' : 'py-2.5'"
              :title="formatUsageLogUsageLine(t, rowFromEntry(entry))"
            >
              <span class="line-clamp-2">{{ formatUsageLogUsageLine(t, rowFromEntry(entry)) }}</span>
            </td>
            <td class="px-3 text-right text-xs" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span :class="costCellClass(rowFromEntry(entry).total_cost)">{{ rowFromEntry(entry).total_cost }}</span>
            </td>
            <td class="px-3" :class="compact ? 'py-1.5' : 'py-2.5'">
              <span :class="statusBadgeClass(rowFromEntry(entry).status_code)">
                {{ rowFromEntry(entry).status_code ?? t('modelsPage.dash') }}
              </span>
            </td>
            <td
              v-if="!compact"
              class="hidden max-w-[14rem] truncate px-3 text-[11px] text-amber-200/80 xl:table-cell"
              :class="compact ? '' : 'py-2.5'"
              :title="rowFromEntry(entry).error_message || ''"
            >
              {{ displayError(rowFromEntry(entry).error_message) }}
            </td>
            <td
              class="max-w-[9rem] truncate px-3 font-mono text-xs text-slate-400"
              :class="compact ? 'py-1.5' : 'py-2.5'"
              :title="rowFromEntry(entry).ip_address || ''"
            >
              {{ rowFromEntry(entry).ip_address?.trim() || t('modelsPage.dash') }}
            </td>
            <td class="px-3 text-right text-xs tabular-nums text-slate-400" :class="compact ? 'py-1.5' : 'py-2.5'">
              {{ rowFromEntry(entry).response_time_ms ?? t('modelsPage.dash') }}
            </td>
            <td class="px-3 text-right" :class="compact ? 'py-1.5' : 'py-2.5'">
              <button
                type="button"
                class="th-usage-logs-copy-btn"
                :title="t('logsPage.copyRow')"
                @click="copyOneEntry(entry)"
              >
                <svg class="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                  <path
                    d="M7 3.5A1.5 1.5 0 018.5 2h6A1.5 1.5 0 0116 3.5v6A1.5 1.5 0 0114.5 11h-1v1.5A1.5 1.5 0 0112 14H6a1.5 1.5 0 01-1.5-1.5v-6A1.5 1.5 0 016 5h1V3.5zm2 1v1H6v6h6v-1h1.5v-6h-4.5z"
                  />
                </svg>
                <span>{{ t('logsPage.copyRow') }}</span>
              </button>
            </td>
          </tr>
          <tr v-if="items.length && !displayItems.length">
            <td :colspan="compact ? 11 : 13" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.noClientMatch') }}</td>
          </tr>
          <tr v-if="!items.length">
            <td :colspan="compact ? 11 : 13" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.empty') }}</td>
          </tr>
        </tbody>
      </table>
      <p class="border-t border-white/[0.06] px-4 py-2.5 text-[11px] leading-relaxed text-slate-600">{{ t('logsPage.copyNote') }}</p>
    </div>

    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard
        v-for="entry in flatRows"
        :key="
          entry.kind === 'group-child'
            ? `log-m-c-${entry.row.id}`
            : entry.kind === 'standalone'
              ? `log-m-s-${entry.row.id}`
              : `log-m-p-${entry.group.primary.id}`
        "
        :class="entry.kind === 'group-child' ? 'th-usage-logs-card--child' : ''"
      >
        <template #title>
          <div class="flex items-start gap-2">
            <input
              v-if="entry.kind !== 'group-child'"
              type="checkbox"
              class="mt-0.5 h-4 w-4 shrink-0 rounded border-slate-600 bg-slate-950 text-cyan-500"
              :checked="selectedIds.includes(rowFromEntry(entry).id)"
              @change="toggleRow(rowFromEntry(entry).id)"
            />
            <div class="min-w-0 flex-1">
              <p class="font-mono text-xs tabular-nums text-slate-500" :title="rowFromEntry(entry).request_time">
                {{ fmtLogTime(rowFromEntry(entry).request_time) }}
              </p>
              <div class="flex flex-wrap items-center gap-2">
                <button
                  v-if="entry.kind === 'group-primary' && groupHasChildren(entry.group)"
                  type="button"
                  class="th-usage-logs-expand-btn"
                  @click="toggleGroup(entry.group.key)"
                >
                  {{
                    isGroupExpanded(entry.group.key)
                      ? t('logsPage.videoGroupCollapse')
                      : t('logsPage.videoGroupExpand')
                  }}
                  ({{ entry.group.children.length }})
                </button>
                <span
                  v-if="entry.kind === 'group-child'"
                  class="th-usage-logs-sub-badge"
                >
                  {{ t('logsPage.videoGroupSubRow') }}
                </span>
                <p class="break-all font-mono text-sm text-slate-200">
                  <span v-if="parseModelCell(rowFromEntry(entry).model).route" class="text-slate-500">route:</span
                  >{{ parseModelCell(rowFromEntry(entry).model).path }}
                </p>
              </div>
            </div>
          </div>
        </template>
        <MobileLabelValueRow :label="t('logsPage.thBilling')">
          <span :class="billingBadgeClass(rowFromEntry(entry))">{{
            t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry))))
          }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('logsPage.thUsage')">{{
          formatUsageLogUsageLine(t, rowFromEntry(entry))
        }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('logsPage.thCost')">
          <span :class="costCellClass(rowFromEntry(entry).total_cost)">{{ rowFromEntry(entry).total_cost }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('logsPage.thStatus')">
          <span :class="statusBadgeClass(rowFromEntry(entry).status_code)">{{
            rowFromEntry(entry).status_code ?? t('modelsPage.dash')
          }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow v-if="!compact && rowFromEntry(entry).request_id" :label="t('logsPage.thReqId')">
          <span class="break-all font-mono text-[11px]">{{ rowFromEntry(entry).request_id }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow
          v-if="!compact && rowFromEntry(entry).error_message?.trim() && !isVideoJobTraceMessage(rowFromEntry(entry).error_message)"
          :label="t('logsPage.thError')"
        >
          <span class="text-amber-200/80">{{ rowFromEntry(entry).error_message }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('logsPage.thIp')">{{
          rowFromEntry(entry).ip_address?.trim() || t('modelsPage.dash')
        }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('logsPage.thLatency')">{{
          rowFromEntry(entry).response_time_ms ?? t('modelsPage.dash')
        }}</MobileLabelValueRow>
        <div class="pt-1">
          <button type="button" class="th-usage-logs-copy-btn" @click="copyOneEntry(entry)">
            <svg class="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
              <path
                d="M7 3.5A1.5 1.5 0 018.5 2h6A1.5 1.5 0 0116 3.5v6A1.5 1.5 0 0114.5 11h-1v1.5A1.5 1.5 0 0112 14H6a1.5 1.5 0 01-1.5-1.5v-6A1.5 1.5 0 016 5h1V3.5zm2 1v1H6v6h6v-1h1.5v-6h-4.5z"
              />
            </svg>
            <span>{{ t('logsPage.copyRow') }}</span>
          </button>
        </div>
      </MobileLabelValueCard>
      <p v-if="items.length && !displayItems.length" class="py-8 text-center text-sm text-slate-500">{{ t('logsPage.noClientMatch') }}</p>
      <p v-if="!items.length" class="py-8 text-center text-sm text-slate-500">{{ t('logsPage.empty') }}</p>
    </div>

    <div
      v-if="total > pageSize"
      class="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-white/[0.06] pt-4 text-sm text-slate-400"
    >
      <span class="text-[11px] text-slate-600">{{ t('logsPage.copyNote') }}</span>
      <div class="flex items-center gap-3">
        <button
          type="button"
          class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-1.5 disabled:opacity-40"
          :disabled="page <= 1"
          @click="
            page--;
            load()
          "
        >
          {{ t('logsPage.prev') }}
        </button>
        <span class="tabular-nums">{{ t('logsPage.pageInfo', { page, total }) }}</span>
        <button
          type="button"
          class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-1.5 disabled:opacity-40"
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
  </div>
</template>
