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

function fmtTime(iso: string) {
  try {
    const d = new Date(iso)
    if (Number.isNaN(d.getTime())) return iso
    return d.toLocaleString()
  } catch {
    return iso
  }
}

function userCell(l: Log): string {
  const id = l.user_id
  const em = (l.user_email ?? '').trim()
  if (id != null && id > 0) {
    return em ? `#${id} · ${em}` : `#${id}`
  }
  return em || '—'
}

function apiKeyCell(l: Log): string {
  const name = (l.api_key_name ?? '').trim()
  const pre = (l.api_key_prefix ?? '').trim()
  const id = l.api_key_id
  if (name) return pre ? `${name} (${pre}…)` : name
  if (pre) return `${pre}…`
  if (id != null && id > 0) return `#${id}`
  return '—'
}

function channelCell(l: Log): string {
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
      <table class="w-full min-w-[88rem] text-left text-xs text-slate-300">
        <thead class="sticky top-0 z-[1] border-b border-white/[0.06] bg-slate-950 text-[10px] uppercase tracking-wide text-slate-500 shadow-[0_1px_0_rgba(255,255,255,0.06)]">
          <tr>
            <th class="px-3 py-2">{{ t('adminLogs.thTime') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thUser') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thApiKey') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thChannel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thRequestId') }}</th>
            <th class="th-usage-logs-expand-col py-2" aria-hidden="true" />
            <th class="min-w-[12rem] px-3 py-2">{{ t('adminLogs.thModel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thUpstreamModel') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thBilling') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thUsage') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thCost') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thIp') }}</th>
            <th class="px-3 py-2">{{ t('adminLogs.thStatus') }}</th>
            <th class="max-w-[12rem] px-3 py-2">{{ t('adminLogs.thError') }}</th>
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
            class="border-b border-white/[0.04] align-top last:border-0"
            :class="entry.kind === 'group-child' ? 'th-usage-logs-row--child' : ''"
          >
            <td class="whitespace-nowrap px-3 py-2 font-mono text-[11px] text-slate-400">
              {{ fmtTime(rowFromEntry(entry).request_time) }}
            </td>
            <td class="max-w-[12rem] px-3 py-2 break-all text-[11px] text-slate-400">{{ userCell(rowFromEntry(entry)) }}</td>
            <td class="max-w-[10rem] px-3 py-2 break-all text-[11px] text-slate-500">{{ apiKeyCell(rowFromEntry(entry)) }}</td>
            <td class="max-w-[10rem] px-3 py-2 break-all text-[11px] text-slate-500">{{ channelCell(rowFromEntry(entry)) }}</td>
            <td
              class="max-w-[8rem] px-3 py-2 font-mono text-[10px] text-slate-500"
              :title="rowFromEntry(entry).request_id || ''"
            >
              {{ ridShort(rowFromEntry(entry).request_id) }}
            </td>
            <td class="th-usage-logs-expand-col align-top py-2">
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
            <td class="th-usage-logs-model-cell px-3 py-2">
              <p class="break-all font-mono text-[11px] leading-snug text-slate-300" :title="rowFromEntry(entry).model">
                <span v-if="parseModelCell(rowFromEntry(entry).model).route" class="text-slate-500">route:</span>
                <span :class="parseModelCell(rowFromEntry(entry).model).route ? 'text-slate-400' : ''">{{
                  parseModelCell(rowFromEntry(entry).model).path
                }}</span>
              </p>
            </td>
            <td
              class="max-w-[9rem] truncate px-3 py-2 font-mono text-[10px] text-slate-500"
              :title="(rowFromEntry(entry).model_upstream ?? '').trim()"
            >
              {{ (rowFromEntry(entry).model_upstream ?? '').trim() || '—' }}
            </td>
            <td class="whitespace-nowrap px-3 py-2">
              <span :class="billingBadgeClass(rowFromEntry(entry))">
                {{ t(usageBillingBadgeKey(resolveUsageBillingMode(rowFromEntry(entry)))) }}
              </span>
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono tabular-nums text-slate-300">
              {{ formatUsageLogUsageLine(t, rowFromEntry(entry)) }}
            </td>
            <td class="whitespace-nowrap px-3 py-2 font-mono tabular-nums text-slate-300">
              {{ rowFromEntry(entry).total_cost }}
            </td>
            <td
              class="max-w-[9rem] truncate px-3 py-2 font-mono text-[10px] text-slate-500"
              :title="(rowFromEntry(entry).ip_address ?? '').trim() || undefined"
            >
              {{ (rowFromEntry(entry).ip_address ?? '').trim() || t('adminCommon.dash') }}
            </td>
            <td class="whitespace-nowrap px-3 py-2">
              <span :class="statusBadgeClass(rowFromEntry(entry).status_code)">
                {{ rowFromEntry(entry).status_code ?? '—' }}
              </span>
            </td>
            <td
              class="max-w-[12rem] truncate px-3 py-2 text-[11px] text-amber-200/80"
              :title="rowFromEntry(entry).error_message || ''"
            >
              {{ displayError(rowFromEntry(entry).error_message) }}
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
          <p class="font-mono text-[11px] text-slate-400">{{ fmtTime(rowFromEntry(entry).request_time) }}</p>
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
        <MobileLabelValueRow :label="t('adminLogs.thUser')">{{ userCell(rowFromEntry(entry)) }}</MobileLabelValueRow>
        <MobileLabelValueRow :label="t('adminLogs.thChannel')">{{ channelCell(rowFromEntry(entry)) }}</MobileLabelValueRow>
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
