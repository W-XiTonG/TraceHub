<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import MobileLabelValueCard from '../components/ui/MobileLabelValueCard.vue'
import MobileLabelValueRow from '../components/ui/MobileLabelValueRow.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import {
  formatUsageLogUsageLine,
  resolveUsageBillingMode,
  usageBillingBadgeKey,
  type UsageLogBillingRow,
} from '../lib/usageLedgerDetail'
import {
  flattenGroupedUsageLogs,
  groupVideoUsageLogs,
  isFailoverIntermediateRow,
  isFailoverOutcomeRow,
  isVideoJobTraceMessage,
  type FlatUsageLogEntry,
  type UsageLogGroup,
} from '../lib/usageLogGrouping'

const { t } = useI18n()

type Log = UsageLogBillingRow & {
  id: number
  user_id?: number | null
  user_email?: string
  api_key_id?: number | null
  api_key_name?: string
  api_key_prefix?: string
  channel_id?: number | null
  channel_name?: string
  channel_type?: string
  model_upstream?: string | null
  request_id?: string
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  status_code?: number | null
  error_message?: string
  request_time: string
  ip_address?: string
  billing_mode?: string
  upstream_attempt?: number
  is_client_outcome?: boolean
  is_failover_attempt?: boolean
}

const items = ref<Log[]>([])
const total = ref(0)
const { busy: loading, visible: loadingUi, run: runLoad } = useSmoothLoading()
const page = ref(1)
const pageSize = 30
const err = ref('')
const expandedGroups = ref<Set<string>>(new Set())

const filterUserId = ref('')
const filterEmail = ref('')
const filterModel = ref('')
const filterChannelId = ref('')
const filterIp = ref('')

const groupedLogs = computed(() => groupVideoUsageLogs(items.value))

const flatRows = computed(() =>
  flattenGroupedUsageLogs(groupedLogs.value.groups, groupedLogs.value.standalone, expandedGroups.value),
)

const collapsedSubCount = computed(() =>
  groupedLogs.value.groups.reduce(
    (n, g) => n + (expandedGroups.value.has(g.key) ? 0 : g.children.length),
    0,
  ),
)

const pageSummary = computed(() => {
  const base = t('adminLogs.pageInfo', { page: page.value, total: total.value })
  const hidden = collapsedSubCount.value
  return hidden > 0 ? `${base} · ${t('logsPage.toolbarGroupedHidden', { k: hidden })}` : base
})

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e)) {
    if (e.response?.status === 401) return t('login.sessionExpired')
    const d = e.response?.data
    if (d && typeof d === 'object' && 'message' in d) {
      const m = (d as { message?: string }).message
      if (typeof m === 'string' && m.trim()) return m.trim()
    }
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

/** 表格展示用；完整内容放 title */
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

function userCellTitle(l: Log): string {
  const id = l.user_id
  const em = (l.user_email ?? '').trim()
  if (id != null && id > 0) {
    return em ? `#${id} · ${em}` : `#${id}`
  }
  return em || '—'
}

function userCellParts(l: Log): { primary: string; secondary: string } | null {
  const id = l.user_id
  const em = (l.user_email ?? '').trim()
  if (id != null && id > 0 && em) {
    return { primary: `#${id}`, secondary: em }
  }
  if (id != null && id > 0) {
    return { primary: `#${id}`, secondary: '' }
  }
  if (em) {
    return { primary: '', secondary: em }
  }
  return null
}

function apiKeyCellTitle(l: Log): string {
  const name = (l.api_key_name ?? '').trim()
  const pre = (l.api_key_prefix ?? '').trim()
  const id = l.api_key_id
  if (name && pre) return `${name} · ${pre}…`
  if (name) return name
  if (pre) return `${pre}…`
  if (id != null && id > 0) return `#${id}`
  return '—'
}

function apiKeyCellParts(l: Log): { primary: string; secondary: string } | null {
  const name = (l.api_key_name ?? '').trim()
  const pre = (l.api_key_prefix ?? '').trim()
  const id = l.api_key_id
  if (name && pre) {
    return { primary: name, secondary: `${pre}…` }
  }
  if (name) {
    return { primary: name, secondary: '' }
  }
  if (pre) {
    return { primary: `${pre}…`, secondary: '' }
  }
  if (id != null && id > 0) {
    return { primary: `#${id}`, secondary: '' }
  }
  return null
}

function channelCellTitle(l: Log): string {
  const id = l.channel_id
  const idPart = id != null && id > 0 ? `#${id}` : ''
  const n = (l.channel_name ?? '').trim()
  const ty = (l.channel_type ?? '').trim()
  const meta = [n, ty].filter(Boolean).join(' · ')
  if (idPart && meta) return `${idPart} · ${meta}`
  if (idPart) return idPart
  if (meta) return meta
  return '—'
}

function channelCellParts(l: Log): { primary: string; secondary: string } | null {
  const id = l.channel_id
  const n = (l.channel_name ?? '').trim()
  const ty = (l.channel_type ?? '').trim()
  const meta = [n, ty].filter(Boolean).join(' · ')
  if (id != null && id > 0 && meta) {
    return { primary: `#${id}`, secondary: meta }
  }
  if (id != null && id > 0) {
    return { primary: `#${id}`, secondary: '' }
  }
  if (meta) {
    return { primary: '', secondary: meta }
  }
  return null
}

function ridShort(rid: string | undefined): string {
  if (!rid) return '—'
  return rid.length > 18 ? `${rid.slice(0, 10)}…${rid.slice(-6)}` : rid
}

function rowFromEntry(entry: FlatUsageLogEntry): Log {
  if (entry.kind === 'standalone') return entry.row as Log
  if (entry.kind === 'group-child') return entry.row as Log
  return entry.group.primary as Log
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
  if (!msg?.trim() || isVideoJobTraceMessage(msg)) return t('adminCommon.dash')
  return msg.trim()
}

function groupHasChildren(group: UsageLogGroup): boolean {
  return group.children.length > 0
}

function expandGroupTitle(group: UsageLogGroup, expanded: boolean): string {
  const hint = t('logsPage.videoGroupSubCount', { n: group.children.length })
  return expanded ? `${t('logsPage.videoGroupCollapse')} · ${hint}` : `${t('logsPage.videoGroupExpand')} · ${hint}`
}

function billingBadgeClass(row: Log): string {
  const base = 'th-badge-billing'
  return resolveUsageBillingMode(row) === 'video_duration'
    ? `${base} th-badge-billing--video`
    : `${base} th-badge-billing--token`
}

function parseModelCell(model: string): { route: boolean; path: string } {
  const m = model.trim()
  if (m.startsWith('route:')) return { route: true, path: m.slice(6) }
  return { route: false, path: m }
}

function statusBadgeClass(code?: number | null): string {
  const base = 'th-badge-http'
  if (code == null) return `${base} th-badge-http--neutral`
  if (code >= 200 && code < 300) return `${base} th-badge-http--2xx`
  if (code >= 400 && code < 500) return `${base} th-badge-http--4xx`
  if (code >= 500) return `${base} th-badge-http--5xx`
  return `${base} th-badge-http--neutral`
}

async function load() {
  err.value = ''
  await runLoad(async () => {
    const params: Record<string, string | number> = { page: page.value, page_size: pageSize }
    const uid = filterUserId.value.trim()
    if (uid !== '') params.user_id = uid
    const em = filterEmail.value.trim()
    if (em !== '') params.email = em
    const m = filterModel.value.trim()
    if (m !== '') params.model = m
    const ch = filterChannelId.value.trim()
    if (ch !== '') params.channel_id = ch
    const ip = filterIp.value.trim()
    if (ip !== '') params.ip = ip
    const { data } = await http.get<Envelope<{ items: Log[]; total: number }>>('/api/admin/logs', { params })
    if (data.code !== 0) throw new Error(data.message || t('adminCommon.loadFail'))
    items.value = data.data?.items ?? []
    total.value = data.data?.total ?? 0
  }).catch((e: unknown) => {
    err.value = httpErr(e)
  })
}

function applyFilters() {
  page.value = 1
  void load()
}

function clearFilters() {
  filterUserId.value = ''
  filterEmail.value = ''
  filterModel.value = ''
  filterChannelId.value = ''
  filterIp.value = ''
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

const totalPages = () => Math.max(1, Math.ceil(total.value / pageSize))

onMounted(() => void load())
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold text-slate-100">{{ t('adminLogs.listHeading') }}</h1>
      <p class="mt-1 max-w-3xl text-sm text-slate-500">{{ t('adminLogs.desc') }}</p>
    </div>

    <div class="flex flex-wrap items-end gap-3">
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterUser') }}</label>
        <input v-model="filterUserId" type="text" inputmode="numeric" class="th-input w-28 font-mono text-sm" :placeholder="t('adminLogs.phFilterUser')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterEmail') }}</label>
        <input v-model="filterEmail" class="th-input w-52 text-sm" :placeholder="t('adminLogs.phFilterEmail')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterModel') }}</label>
        <input v-model="filterModel" class="th-input w-44 text-sm" :placeholder="t('adminLogs.phFilterModel')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterChannel') }}</label>
        <input v-model="filterChannelId" type="text" inputmode="numeric" class="th-input w-28 font-mono text-sm" :placeholder="t('adminLogs.phFilterChannel')" />
      </div>
      <div>
        <label class="mb-1 block text-xs text-slate-500">{{ t('adminLogs.lblFilterIp') }}</label>
        <input
          v-model="filterIp"
          class="th-input w-40 font-mono text-sm"
          :placeholder="t('adminLogs.phFilterIp')"
          autocomplete="off"
          spellcheck="false"
        />
      </div>
      <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm" @click="applyFilters">{{ t('adminLogs.apply') }}</button>
      <button type="button" class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]" @click="clearFilters">
        {{ t('adminLogs.clear') }}
      </button>
      <button type="button" class="rounded-lg border border-white/[0.08] px-4 py-2 text-sm text-slate-300 hover:bg-white/[0.05]" @click="load">
        {{ t('adminLogs.refresh') }}
      </button>
    </div>

    <p v-if="err" class="text-sm text-rose-400">{{ err }}</p>

    <ThTableSkeleton v-if="loadingUi && items.length === 0" :rows="8" :columns="6" toolbar class="mb-4" />

    <div class="th-admin-logs-table-scroll th-usage-logs-table-wrap thin-scrollbar relative hidden rounded-lg border border-white/[0.06] lg:block">
      <ThLoadingState v-if="loadingUi && items.length > 0" variant="overlay" />
      <table class="th-admin-logs-table w-full min-w-[80rem] text-left text-xs">
        <thead class="th-admin-logs-thead sticky top-0 z-[1] border-b text-[10px] uppercase tracking-wide">
          <tr>
            <th class="th-admin-logs-col-time px-2 py-2">{{ t('adminLogs.thTime') }}</th>
            <th class="th-admin-logs-col-user px-2 py-2">{{ t('adminLogs.thUser') }}</th>
            <th class="th-admin-logs-col-key px-2 py-2">{{ t('adminLogs.thApiKey') }}</th>
            <th class="th-admin-logs-col-channel px-2 py-2">{{ t('adminLogs.thChannel') }}</th>
            <th class="th-admin-logs-col-rid px-2 py-2">{{ t('adminLogs.thRequestId') }}</th>
            <th class="th-usage-logs-expand-col py-2" aria-hidden="true" />
            <th class="th-admin-logs-col-model px-2 py-2">{{ t('adminLogs.thModel') }}</th>
            <th class="th-admin-logs-col-upstream px-2 py-2">{{ t('adminLogs.thUpstreamModel') }}</th>
            <th class="th-admin-logs-col-billing px-2 py-2">{{ t('adminLogs.thBilling') }}</th>
            <th class="th-admin-logs-col-usage px-2 py-2">{{ t('adminLogs.thUsage') }}</th>
            <th class="th-admin-logs-col-cost px-2 py-2">{{ t('adminLogs.thCost') }}</th>
            <th class="th-admin-logs-col-ip px-2 py-2">{{ t('adminLogs.thIp') }}</th>
            <th class="th-admin-logs-col-status px-2 py-2">{{ t('adminLogs.thStatus') }}</th>
            <th class="th-admin-logs-col-error px-2 py-2">{{ t('adminLogs.thError') }}</th>
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
            class="border-b border-white/[0.04] align-middle last:border-0"
            :class="entry.kind === 'group-child' ? 'th-usage-logs-row--child' : ''"
          >
            <td
              class="th-admin-logs-col-time th-admin-logs-text-faint whitespace-nowrap px-2 py-1.5 font-mono text-[11px] tabular-nums"
              :title="rowFromEntry(entry).request_time"
            >
              {{ fmtLogTime(rowFromEntry(entry).request_time) }}
            </td>
            <td class="th-admin-logs-col-user px-2 py-1.5" :title="userCellTitle(rowFromEntry(entry))">
              <div v-if="userCellParts(rowFromEntry(entry))" class="th-admin-logs-compact th-admin-logs-text-muted">
                <span v-if="userCellParts(rowFromEntry(entry))!.primary" class="th-admin-logs-compact__tag">{{
                  userCellParts(rowFromEntry(entry))!.primary
                }}</span>
                <span v-if="userCellParts(rowFromEntry(entry))!.secondary" class="th-admin-logs-compact__main">{{
                  userCellParts(rowFromEntry(entry))!.secondary
                }}</span>
              </div>
              <span v-else class="text-slate-500">{{ t('adminCommon.dash') }}</span>
            </td>
            <td class="th-admin-logs-col-key px-2 py-1.5" :title="apiKeyCellTitle(rowFromEntry(entry))">
              <div v-if="apiKeyCellParts(rowFromEntry(entry))" class="th-admin-logs-compact th-admin-logs-text-faint">
                <span v-if="apiKeyCellParts(rowFromEntry(entry))!.primary" class="th-admin-logs-compact__main">{{
                  apiKeyCellParts(rowFromEntry(entry))!.primary
                }}</span>
                <span
                  v-if="apiKeyCellParts(rowFromEntry(entry))!.secondary"
                  class="th-admin-logs-compact__mono"
                  >{{ apiKeyCellParts(rowFromEntry(entry))!.secondary }}</span
                >
              </div>
              <span v-else class="text-slate-500">{{ t('adminCommon.dash') }}</span>
            </td>
            <td class="th-admin-logs-col-channel px-2 py-1.5" :title="channelCellTitle(rowFromEntry(entry))">
              <div v-if="channelCellParts(rowFromEntry(entry))" class="th-admin-logs-compact th-admin-logs-text-faint">
                <span v-if="channelCellParts(rowFromEntry(entry))!.primary" class="th-admin-logs-compact__tag">{{
                  channelCellParts(rowFromEntry(entry))!.primary
                }}</span>
                <span v-if="channelCellParts(rowFromEntry(entry))!.secondary" class="th-admin-logs-compact__main">{{
                  channelCellParts(rowFromEntry(entry))!.secondary
                }}</span>
              </div>
              <span v-else class="text-slate-500">{{ t('adminCommon.dash') }}</span>
            </td>
            <td class="th-admin-logs-col-rid px-2 py-1.5" :title="rowFromEntry(entry).request_id || ''">
              <span class="th-admin-logs-ellipsis th-admin-logs-text-faint font-mono text-[10px]">{{
                ridShort(rowFromEntry(entry).request_id)
              }}</span>
            </td>
            <td class="th-usage-logs-expand-col align-middle py-1.5">
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
            <td class="th-admin-logs-col-model px-2 py-1.5" :title="rowFromEntry(entry).model">
              <div class="th-admin-logs-compact th-admin-logs-compact--mono">
                <span v-if="parseModelCell(rowFromEntry(entry).model).route" class="th-admin-logs-compact__tag">route</span>
                <span class="th-admin-logs-compact__main">{{ parseModelCell(rowFromEntry(entry).model).path }}</span>
              </div>
              <span
                v-if="isFailoverIntermediateRow(rowFromEntry(entry))"
                class="mt-0.5 inline-block rounded bg-slate-800 px-1 py-px text-[9px] text-slate-400"
              >
                {{ t('logsPage.failoverAttempt', { n: rowFromEntry(entry).upstream_attempt ?? 1 }) }}
              </span>
              <span
                v-else-if="isFailoverOutcomeRow(rowFromEntry(entry))"
                class="mt-0.5 inline-block rounded bg-emerald-950/80 px-1 py-px text-[9px] text-emerald-400/90"
              >
                {{ t('logsPage.failoverResolved', { n: rowFromEntry(entry).upstream_attempt ?? 1 }) }}
              </span>
            </td>
            <td class="th-admin-logs-col-upstream px-2 py-1.5" :title="(rowFromEntry(entry).model_upstream ?? '').trim()">
              <span class="th-admin-logs-ellipsis th-admin-logs-text-muted font-mono text-[10px]">{{
                (rowFromEntry(entry).model_upstream ?? '').trim() || '—'
              }}</span>
            </td>
            <td class="th-admin-logs-col-billing px-2 py-1.5">
              <span
                :class="billingBadgeClass(rowFromEntry(entry))"
                :title="t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry))))"
              >
                {{ t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry)))) }}
              </span>
            </td>
            <td
              class="th-admin-logs-col-usage px-2 py-1.5"
              :title="formatUsageLogUsageLine(t, rowFromEntry(entry))"
            >
              <span class="th-admin-logs-ellipsis th-admin-logs-text-strong font-mono tabular-nums">{{
                formatUsageLogUsageLine(t, rowFromEntry(entry))
              }}</span>
            </td>
            <td class="th-admin-logs-col-cost px-2 py-1.5">
              <span class="th-admin-logs-ellipsis th-admin-logs-text-strong font-mono tabular-nums">{{
                rowFromEntry(entry).total_cost
              }}</span>
            </td>
            <td class="th-admin-logs-col-ip px-2 py-1.5" :title="(rowFromEntry(entry).ip_address ?? '').trim() || undefined">
              <span class="th-admin-logs-ellipsis th-admin-logs-text-faint font-mono text-[10px]">{{
                (rowFromEntry(entry).ip_address ?? '').trim() || t('adminCommon.dash')
              }}</span>
            </td>
            <td class="th-admin-logs-col-status px-2 py-1.5">
              <span :class="statusBadgeClass(rowFromEntry(entry).status_code)">
                {{ rowFromEntry(entry).status_code ?? '—' }}
              </span>
            </td>
            <td class="th-admin-logs-col-error px-2 py-1.5" :title="rowFromEntry(entry).error_message || ''">
              <span class="th-admin-logs-ellipsis text-[11px] text-amber-200/80">{{
                displayError(rowFromEntry(entry).error_message)
              }}</span>
            </td>
          </tr>
          <tr v-if="!loadingUi && items.length === 0">
            <td colspan="14" class="px-3 py-8 text-center text-slate-500">{{ t('logsPage.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="loadingUi && items.length === 0" class="py-8 lg:hidden">
      <ThLoadingState />
    </div>

    <div class="space-y-3 lg:hidden">
      <MobileLabelValueCard
        v-for="entry in flatRows"
        :key="
          entry.kind === 'group-child'
            ? `alog-m-c-${entry.row.id}`
            : entry.kind === 'standalone'
              ? `alog-m-s-${entry.row.id}`
              : `alog-m-p-${entry.group.primary.id}`
        "
        :class="entry.kind === 'group-child' ? 'th-usage-logs-card--child' : ''"
      >
        <template #title>
          <p class="font-mono text-[11px] tabular-nums text-slate-400" :title="rowFromEntry(entry).request_time">
            {{ fmtLogTime(rowFromEntry(entry).request_time) }}
          </p>
          <div class="mt-0.5 flex flex-wrap items-center gap-2">
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
            <span v-if="entry.kind === 'group-child'" class="th-usage-logs-sub-badge">
              {{ t('logsPage.videoGroupSubRow') }}
            </span>
            <p class="break-all font-mono text-sm text-slate-200">
              <span v-if="parseModelCell(rowFromEntry(entry).model).route" class="text-slate-500">route:</span
              >{{ parseModelCell(rowFromEntry(entry).model).path }}
            </p>
          </div>
        </template>
        <MobileLabelValueRow :label="t('adminLogs.thUser')">{{ userCellTitle(rowFromEntry(entry)) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thApiKey')">{{ apiKeyCellTitle(rowFromEntry(entry)) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thChannel')">{{ channelCellTitle(rowFromEntry(entry)) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thUpstreamModel')">
          {{ (rowFromEntry(entry).model_upstream ?? '').trim() || t('adminCommon.dash') }}
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thBilling')">
          <span :class="billingBadgeClass(rowFromEntry(entry))">{{
            t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry))))
          }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thUsage')">{{
          formatUsageLogUsageLine(t, rowFromEntry(entry))
        }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thCost')">{{ rowFromEntry(entry).total_cost }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thStatus')">
          <span :class="statusBadgeClass(rowFromEntry(entry).status_code)">{{
            rowFromEntry(entry).status_code ?? '—'
          }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow
          v-if="
            rowFromEntry(entry).error_message?.trim() &&
            !isVideoJobTraceMessage(rowFromEntry(entry).error_message)
          "
          :label="t('adminLogs.thError')"
        >
          <span class="text-amber-200/80">{{ rowFromEntry(entry).error_message }}</span>
        </MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thIp')">{{
          (rowFromEntry(entry).ip_address ?? '').trim() || t('adminCommon.dash')
        }}</MobileLabelValueRow>
      </MobileLabelValueCard>
      <p v-if="!loadingUi && items.length === 0" class="py-8 text-center text-sm text-slate-500">{{ t('logsPage.empty') }}</p>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
      <span>{{ pageSummary }}</span>
      <div class="flex gap-2">
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page <= 1"
          @click="prevPage"
        >
          {{ t('adminLogs.prev') }}
        </button>
        <span class="self-center tabular-nums">{{ page }} / {{ totalPages() }}</span>
        <button
          type="button"
          class="rounded-lg border border-white/[0.1] px-3 py-1.5 text-slate-300 transition hover:border-amber-500/30 hover:text-amber-200/90 disabled:opacity-40"
          :disabled="page >= totalPages()"
          @click="nextPage"
        >
          {{ t('adminLogs.next') }}
        </button>
      </div>
    </div>
  </div>
</template>
