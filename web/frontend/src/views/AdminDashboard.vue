<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { http, type Envelope } from '../api/http'

type Summary = {
  total_users?: number
  enabled_channels?: number
  today_requests?: number
  today_tokens?: number
  today_cost_quota?: number
  today_cost_yuan?: number
  currency_quota_per_yuan?: number
  yesterday_requests?: number
  yesterday_tokens?: number
  yesterday_cost_quota?: number
  yesterday_cost_yuan?: number
  last_7d_requests?: number
  last_7d_tokens?: number
  last_7d_cost_quota?: number
  last_7d_cost_yuan?: number
  vs_yesterday_requests_pct?: number | null
  vs_yesterday_tokens_pct?: number | null
  vs_yesterday_cost_pct?: number | null
}

type InsightPoint = {
  date: string
  requests?: number
  tokens?: number
  cost_quota?: number
  cost_yuan?: number
}

type TopModelRow = { model: string; requests: number; tokens: number }

type ChannelUsageRow = { channel_id: number; channel_name: string; requests: number; pct: number }

type InsightPayload = {
  window_days?: number
  timeseries_granularity?: 'day' | 'hour'
  timeseries?: InsightPoint[]
  top_models_today?: { items?: TopModelRow[] }
  top_models_7d?: { items?: TopModelRow[] }
  channels?: {
    enabled?: number
    disabled?: number
    unhealthy?: number
    schedulable_off?: number
    usage_7d?: ChannelUsageRow[]
  }
}

type SnapshotUsers = {
  dau_today?: number
  new_users_today?: number
  new_users_7d?: number
  groups_count?: number
  negative_balance?: number
  zero_balance?: number
}

type SnapshotHttpWindow = {
  total_requests?: number
  n_4xx?: number
  n_5xx?: number
  fail_pct?: number
}

type SnapshotPayload = {
  users?: SnapshotUsers
  http?: { today?: SnapshotHttpWindow; last_7d?: SnapshotHttpWindow }
  recent_logs?: {
    id: number
    user_id?: number
    model: string
    status_code?: number
    request_time?: string
    total_tokens?: number
  }[]
  recent_channels?: {
    id: number
    channel_name: string
    channel_type?: string
    enabled?: boolean
    updated_at?: string
  }[]
  draft_announcements?: { id: number; title: string; updated_at?: string }[]
}

type OpsAuditRow = {
  id: number
  created_at?: string
  admin_id?: number
  admin_email?: string
  action?: string
  resource?: string
  resource_id?: string
  ip_address?: string
}

type OpsPayload = {
  recent_audit?: OpsAuditRow[]
  health?: {
    checks?: Record<string, string>
    app_version?: string
    go_version?: string
    uptime_sec?: number
  }
  config_outline?: Record<string, string | number | boolean>
  external_alerts?: { status?: string; scope?: string[] }
}

const { t } = useI18n()
const s = ref<Summary | null>(null)
const insight = ref<InsightPayload | null>(null)
const insightBusy = ref(false)
const insightErr = ref('')
const snapshot = ref<SnapshotPayload | null>(null)
const snapshotErr = ref('')
const ops = ref<OpsPayload | null>(null)
const opsErr = ref('')
const chartDays = ref<1 | 7 | 30>(7)
const chartMetric = ref<'requests' | 'tokens' | 'cost_yuan'>('requests')
const { busy, visible: busyUi, run: runPageLoad } = useSmoothLoading()
const err = ref('')
const autoRefresh = ref(false)
const updatedAt = ref<number | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

function fmtInt(n: number | undefined) {
  if (n === undefined || Number.isNaN(n)) return t('adminCommon.dash')
  return Math.trunc(n).toLocaleString()
}

function quotaPerYuan() {
  const p = s.value?.currency_quota_per_yuan
  if (typeof p === 'number' && p > 0) return p
  return 10000
}

const todayCostYuan = computed(() => {
  if (!s.value) return undefined
  if (typeof s.value.today_cost_yuan === 'number' && !Number.isNaN(s.value.today_cost_yuan)) {
    return s.value.today_cost_yuan
  }
  const q = Number(s.value.today_cost_quota ?? 0)
  return Math.round((q / quotaPerYuan()) * 100) / 100
})

const updatedLabel = computed(() => {
  if (!updatedAt.value) return ''
  try {
    return new Date(updatedAt.value).toLocaleString()
  } catch {
    return ''
  }
})

async function loadInsightOnly() {
  if (!s.value) return
  insightErr.value = ''
  insightBusy.value = true
  try {
    const { data } = await http.get<Envelope<InsightPayload>>(`/api/admin/dashboard/insight?days=${chartDays.value}`)
    if (data.code !== 0) {
      insight.value = null
      insightErr.value = (data as { message?: string }).message || t('adminDash.p2InsightLoadFail')
      return
    }
    insight.value = data.data ?? null
  } catch (e: unknown) {
    insight.value = null
    if (axios.isAxiosError(e)) {
      const body = e.response?.data as { message?: string } | undefined
      insightErr.value = body?.message || e.message || t('adminDash.p2InsightLoadFail')
    } else {
      insightErr.value = e instanceof Error ? e.message : t('adminDash.p2InsightLoadFail')
    }
  } finally {
    insightBusy.value = false
  }
}

async function load() {
  err.value = ''
  insightErr.value = ''
  snapshotErr.value = ''
  opsErr.value = ''
  await runPageLoad(async () => {
  try {
    const [sumR, insR, snapR, opsR] = await Promise.allSettled([
      http.get<Envelope<Summary>>('/api/admin/dashboard/summary'),
      http.get<Envelope<InsightPayload>>(`/api/admin/dashboard/insight?days=${chartDays.value}`),
      http.get<Envelope<SnapshotPayload>>('/api/admin/dashboard/snapshot'),
      http.get<Envelope<OpsPayload>>('/api/admin/dashboard/ops'),
    ])

    if (sumR.status !== 'fulfilled') {
      const reason = sumR.reason
      throw reason instanceof Error ? reason : new Error(t('adminCommon.requestFail'))
    }
    const d1 = sumR.value.data
    if (d1.code !== 0) {
      throw new Error((d1 as { message?: string }).message || t('adminCommon.loadFail'))
    }
    s.value = d1.data ?? {}

    if (insR.status === 'fulfilled') {
      const d2 = insR.value.data
      if (d2.code !== 0) {
        insight.value = null
        insightErr.value = (d2 as { message?: string }).message || t('adminDash.p2InsightLoadFail')
      } else {
        insight.value = d2.data ?? null
        insightErr.value = ''
      }
    } else {
      insight.value = null
      insightErr.value = t('adminDash.p2InsightLoadFail')
    }

    if (snapR.status === 'fulfilled') {
      const d3 = snapR.value.data
      if (d3.code !== 0) {
        snapshot.value = null
        snapshotErr.value = (d3 as { message?: string }).message || t('adminDash.p3SnapshotFail')
      } else {
        snapshot.value = d3.data ?? null
        snapshotErr.value = ''
      }
    } else {
      snapshot.value = null
      snapshotErr.value = t('adminDash.p3SnapshotFail')
    }

    if (opsR.status === 'fulfilled') {
      const d4 = opsR.value.data
      if (d4.code !== 0) {
        ops.value = null
        opsErr.value = (d4 as { message?: string }).message || t('adminDash.p4OpsFail')
      } else {
        ops.value = d4.data ?? null
        opsErr.value = ''
      }
    } else {
      ops.value = null
      opsErr.value = t('adminDash.p4OpsFail')
    }

    updatedAt.value = Date.now()
  } catch (e: unknown) {
    s.value = null
    insight.value = null
    snapshot.value = null
    ops.value = null
    if (axios.isAxiosError(e)) {
      if (e.code === 'ECONNABORTED') {
        err.value = t('adminDash.timeout')
      } else {
        const body = e.response?.data as { message?: string } | undefined
        err.value = body?.message || e.message || t('adminCommon.requestFail')
      }
    } else {
      err.value = e instanceof Error ? e.message : t('adminCommon.requestFail')
    }
  }
  })
}

watch(chartDays, () => {
  void loadInsightOnly()
})

watch(chartMetric, () => {
  sparkHover.value = null
})

function clearPoll() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function setupPoll() {
  clearPoll()
  if (autoRefresh.value) {
    pollTimer = setInterval(() => {
      void load()
    }, 60_000)
  }
}

watch(autoRefresh, setupPoll)

onMounted(() => {
  void load()
  setupPoll()
})

onUnmounted(() => {
  clearPoll()
  sparkHover.value = null
  modelTip.value = null
  chTip.value = null
})

function fmtYuan(v: number | undefined) {
  if (v === undefined || Number.isNaN(v)) return t('adminCommon.dash')
  return `¥${v.toFixed(2)}`
}

function fmtVsPct(
  p: number | null | undefined,
  today: number | undefined,
  yest: number | undefined,
): string {
  if (typeof p === 'number' && Number.isFinite(p)) {
    return (p > 0 ? '+' : '') + p.toFixed(1) + '%'
  }
  if ((today ?? 0) > 0 && (yest ?? 0) === 0) return t('adminDash.pctFromZero')
  return t('adminCommon.dash')
}

function pctTone(p: number | null | undefined, today: number | undefined, yest: number | undefined): string {
  if (typeof p === 'number' && Number.isFinite(p)) {
    if (p > 0) return 'text-emerald-400'
    if (p < 0) return 'text-rose-400'
    return 'text-slate-400'
  }
  if (p == null && (today ?? 0) > 0 && (yest ?? 0) === 0) return 'text-emerald-400'
  return 'text-slate-400'
}

const SPARK_W = 320
const SPARK_H = 88

const sparkSvgRef = ref<SVGSVGElement | null>(null)
const sparkHover = ref<number | null>(null)
const sparkTip = ref({ x: 0, y: 0 })

const modelTip = ref<{ x: number; y: number; row: TopModelRow } | null>(null)
const chTip = ref<{ x: number; y: number; row: ChannelUsageRow } | null>(null)

function clampClientTooltip(x: number, y: number, tw = 260, th = 120) {
  const pad = 10
  return {
    x: Math.max(pad, Math.min(x, window.innerWidth - tw - pad)),
    y: Math.max(pad, Math.min(y, window.innerHeight - th - pad)),
  }
}

function sparkIndexFromEvent(e: MouseEvent, svg: SVGSVGElement): number {
  const pts = sparkLine.value.points
  if (!pts.length) return 0
  const rect = svg.getBoundingClientRect()
  if (rect.width <= 0) return 0
  const svgX = ((e.clientX - rect.left) / rect.width) * SPARK_W
  const x = Math.max(0, Math.min(SPARK_W, svgX))
  let best = 0
  let bestD = Infinity
  for (let i = 0; i < pts.length; i++) {
    const d = Math.abs(x - pts[i].x)
    if (d < bestD) {
      bestD = d
      best = i
    }
  }
  return best
}

function onSparkSvgMove(e: MouseEvent) {
  const svg = sparkSvgRef.value
  if (!svg) return
  const i = sparkIndexFromEvent(e, svg)
  sparkHover.value = i
  const c = clampClientTooltip(e.clientX + 12, e.clientY + 8, 260, 140)
  sparkTip.value = c
}

function onSparkSvgLeave() {
  sparkHover.value = null
}

function insightSeriesTickLabel(dateStr: string) {
  if (insight.value?.timeseries_granularity === 'hour') {
    try {
      const d = new Date(dateStr)
      if (!Number.isNaN(d.getTime())) {
        return `${String(d.getHours()).padStart(2, '0')}:00`
      }
    } catch {
      /* ignore */
    }
  }
  if (dateStr.length >= 10) return dateStr.slice(5, 10)
  return dateStr
}

function sparkHoverDateLabel(dateStr: string) {
  if (insight.value?.timeseries_granularity === 'hour') {
    try {
      const d = new Date(dateStr)
      if (!Number.isNaN(d.getTime())) {
        return d.toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
      }
    } catch {
      /* ignore */
    }
  }
  return dateStr
}

function sparkHoverText(i: number) {
  const row = sparkLine.value.points[i]?.row
  if (!row) return ''
  const d = sparkHoverDateLabel(row.date)
  const req = fmtInt(row.requests)
  const tok = fmtInt(row.tokens)
  const cy = typeof row.cost_yuan === 'number' && !Number.isNaN(row.cost_yuan) ? row.cost_yuan.toFixed(2) : t('adminCommon.dash')
  const cq = fmtInt(row.cost_quota)
  return t('adminDash.sparkTip', { date: d, req, tok, yuan: cy, quota: cq })
}

function onModelRowEnter(row: TopModelRow, e: MouseEvent) {
  modelTip.value = { row, ...clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 100) }
}

function onModelRowMove(e: MouseEvent) {
  if (!modelTip.value) return
  const c = clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 100)
  modelTip.value = { ...modelTip.value, ...c }
}

function onModelRowLeave() {
  modelTip.value = null
}

function modelTipText() {
  const r = modelTip.value?.row
  if (!r) return ''
  return t('adminDash.modelRowTip', {
    model: r.model,
    req: fmtInt(r.requests),
    tok: fmtInt(r.tokens),
  })
}

function onChRowEnter(row: ChannelUsageRow, e: MouseEvent) {
  chTip.value = { row, ...clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 100) }
}

function onChRowMove(e: MouseEvent) {
  if (!chTip.value) return
  const c = clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 100)
  chTip.value = { ...chTip.value, ...c }
}

function onChRowLeave() {
  chTip.value = null
}

function chTipText() {
  const r = chTip.value?.row
  if (!r) return ''
  return t('adminDash.chRowTip', {
    name: r.channel_name,
    req: fmtInt(r.requests),
    pct: String(r.pct),
  })
}

const sparkLine = computed(() => {
  const rows = insight.value?.timeseries ?? []
  const n = rows.length
  const key = chartMetric.value
  const vals = rows.map((r) => {
    if (key === 'requests') return Number(r.requests) || 0
    if (key === 'tokens') return Number(r.tokens) || 0
    return Number(r.cost_yuan) || 0
  })
  if (!n) {
    return {
      polyline: '',
      rows: [] as InsightPoint[],
      vals: [] as number[],
      minV: 0,
      maxV: 0,
      points: [] as { x: number; y: number; row: InsightPoint; v: number }[],
    }
  }
  const maxV = Math.max(...vals, 1e-9)
  const minV = Math.min(...vals, 0)
  const span = maxV - minV || 1
  const step = n > 1 ? SPARK_W / (n - 1) : 0
  const points = vals.map((v, i) => {
    const x = n > 1 ? i * step : SPARK_W / 2
    const y = SPARK_H - ((v - minV) / span) * (SPARK_H - 12) - 6
    return { x, y, row: rows[i], v }
  })
  const polyline = points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
  return { polyline, rows, vals, minV, maxV, points }
})

/** 横轴最多展示的日期刻度数（含首尾），按索引步进取样，避免「末两天」等与曲线对齐后仍挤在一起重叠 */
const AXIS_MAX_LABELS = 8

const axisTicks = computed(() => {
  const rows = insight.value?.timeseries ?? []
  const n = rows.length
  if (!n) return [] as { key: string; label: string; xPct: number; index: number; n: number }[]

  let idxs: number[]
  if (n <= AXIS_MAX_LABELS) {
    idxs = Array.from({ length: n }, (_, i) => i)
  } else {
    const step = Math.max(1, Math.ceil((n - 1) / (AXIS_MAX_LABELS - 1)))
    idxs = []
    for (let i = 0; i < n; i += step) {
      idxs.push(i)
    }
    const last = n - 1
    if (idxs[idxs.length - 1] !== last) {
      const prev = idxs[idxs.length - 1]
      if (last - prev < step) {
        idxs[idxs.length - 1] = last
      } else {
        idxs.push(last)
      }
    }
  }

  return idxs.map((i) => {
    const d = rows[i].date
    const xPct = n <= 1 ? 50 : (i / (n - 1)) * 100
    return {
      key: `${d}-${i}`,
      label: insightSeriesTickLabel(d),
      xPct,
      index: i,
      n,
    }
  })
})

function axisTickStyle(tk: { xPct: number; index: number; n: number }) {
  if (tk.n <= 1) return { left: '50%', transform: 'translateX(-50%)' }
  if (tk.index === 0) return { left: '0%', transform: 'translateX(0)' }
  if (tk.index === tk.n - 1) return { left: '100%', transform: 'translateX(-100%)' }
  return { left: `${tk.xPct}%`, transform: 'translateX(-50%)' }
}

function maxBarRequests(items: TopModelRow[]): number {
  if (!items.length) return 1
  return Math.max(...items.map((x) => Number(x.requests) || 0), 1)
}

function fmtLogTime(iso?: string) {
  if (!iso) return t('adminCommon.dash')
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

const showSkeleton = computed(() => !s.value && busyUi.value)
</script>

<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h1 class="text-2xl font-bold tracking-tight text-slate-100">{{ t('adminDash.title') }}</h1>
        <p v-if="updatedLabel" class="mt-1 text-xs text-slate-500">{{ t('adminDash.updatedAt', { time: updatedLabel }) }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <label class="flex cursor-pointer items-center gap-2 text-sm text-slate-400 select-none">
          <input v-model="autoRefresh" type="checkbox" class="h-4 w-4 rounded border-slate-600 bg-slate-950/80 text-amber-500" />
          {{ t('adminDash.autoRefresh') }}
        </label>
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-950/25 px-3 py-2 text-sm font-medium text-amber-100/95 transition hover:border-amber-400/45 hover:bg-amber-950/40 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="busy"
          @click="load"
        >
          <svg
            class="h-4 w-4 shrink-0"
            :class="{ 'animate-spin': busy }"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            aria-hidden="true"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('adminCommon.refresh') }}
        </button>
      </div>
    </div>

    <p v-if="err" class="mb-4 rounded-lg border border-red-500/25 bg-red-950/25 px-3 py-2 text-sm text-red-300">
      {{ err }}
      <button type="button" class="ml-2 underline underline-offset-2 hover:text-red-200" @click="load">{{ t('adminDash.retry') }}</button>
    </p>

    <div v-if="!showSkeleton && s" class="mb-6 flex flex-wrap gap-2">
      <span class="mr-1 w-full text-xs font-medium uppercase tracking-wider text-slate-500 sm:w-auto sm:self-center">{{ t('adminDash.p3QuickTitle') }}</span>
      <RouterLink
        to="/admin/users"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipUsers')"
      >{{ t('layout.adminNavUsers') }}</RouterLink>
      <RouterLink
        to="/admin/channels"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipChannels')"
      >{{ t('layout.adminNavChannels') }}</RouterLink>
      <RouterLink
        :to="{ name: 'admin-groups' }"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipGroups')"
      >{{ t('layout.adminNavGroups') }}</RouterLink>
      <RouterLink
        to="/admin/logs"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipLogs')"
      >{{ t('layout.adminNavLogs') }}</RouterLink>
      <RouterLink
        to="/admin/model-market"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipMarket')"
      >{{ t('layout.adminNavMarket') }}</RouterLink>
      <RouterLink
        to="/admin/announcements"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipAnnounce')"
      >{{ t('layout.adminNavAnnouncements') }}</RouterLink>
      <RouterLink
        :to="{ name: 'admin-settings-brand' }"
        class="th-admin-quick-pill"
        :title="t('adminSystem.title')"
      >{{ t('layout.adminNavSystem') }}</RouterLink>
      <RouterLink
        to="/docs"
        class="th-admin-quick-pill"
        :title="t('adminDash.quickTipDocs')"
      >{{ t('adminDash.p3NavDocs') }}</RouterLink>
    </div>

    <div v-if="showSkeleton" class="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
      <div v-for="i in 5" :key="'sk-' + i" class="th-glass animate-pulse rounded-[var(--th-radius-lg)] p-5">
        <div class="h-3 w-24 rounded bg-slate-600/40" />
        <div class="mt-4 h-8 w-20 rounded bg-slate-600/30" />
        <div class="mt-2 h-3 w-32 rounded bg-slate-600/25" />
      </div>
    </div>

    <div v-if="showSkeleton" class="mt-6 th-glass animate-pulse rounded-[var(--th-radius-lg)] p-6">
      <div class="h-4 w-48 rounded bg-slate-600/35" />
      <div class="mt-5 grid gap-3 sm:grid-cols-2">
        <div class="h-28 rounded-lg bg-slate-600/20" />
        <div class="h-28 rounded-lg bg-slate-600/20" />
      </div>
    </div>

    <div v-if="showSkeleton" class="mt-6 th-glass animate-pulse rounded-[var(--th-radius-lg)] p-6">
      <div class="h-4 w-56 rounded bg-slate-600/35" />
      <div class="mt-6 h-36 max-w-xl rounded-lg bg-slate-600/20" />
      <div class="mt-6 grid gap-4 md:grid-cols-2">
        <div class="h-44 rounded-lg bg-slate-600/15" />
        <div class="h-44 rounded-lg bg-slate-600/15" />
      </div>
      <div class="mt-6 h-32 rounded-lg bg-slate-600/18" />
    </div>

    <div v-if="showSkeleton" class="mt-6 th-glass animate-pulse rounded-[var(--th-radius-lg)] p-6">
      <div class="h-4 w-64 rounded bg-slate-600/35" />
      <div class="mt-5 grid gap-4 lg:grid-cols-3">
        <div class="h-36 rounded-lg bg-slate-600/18" />
        <div class="h-36 rounded-lg bg-slate-600/18" />
        <div class="h-36 rounded-lg bg-slate-600/18" />
      </div>
      <div class="mt-5 grid gap-4 md:grid-cols-2">
        <div class="h-40 rounded-lg bg-slate-600/15" />
        <div class="h-40 rounded-lg bg-slate-600/15" />
      </div>
    </div>

    <div v-if="showSkeleton" class="mt-6 th-glass animate-pulse rounded-[var(--th-radius-lg)] p-6">
      <div class="h-4 w-72 rounded bg-slate-600/35" />
      <div class="mt-5 grid gap-4 lg:grid-cols-3">
        <div class="h-44 rounded-lg bg-slate-600/16" />
        <div class="h-44 rounded-lg bg-slate-600/16" />
        <div class="h-44 rounded-lg bg-slate-600/16" />
      </div>
    </div>

    <div v-else>
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-5">
        <div class="th-glass relative overflow-visible rounded-[var(--th-radius-lg)] p-5 pr-14">
          <div class="group/ic pointer-events-auto absolute right-3 top-3 z-20">
            <button
              type="button"
              class="flex h-9 w-9 cursor-help items-center justify-center rounded-xl bg-cyan-500/12 text-cyan-300/90 ring-1 ring-cyan-400/20 outline-none transition hover:bg-cyan-500/18 focus-visible:ring-2 focus-visible:ring-amber-400/40"
              :aria-label="t('adminDash.statIconAria')"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.85" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
                <circle cx="9" cy="7" r="4" />
                <path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
              </svg>
            </button>
            <div
              class="pointer-events-none absolute right-0 top-11 z-30 hidden w-[min(17.5rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-left text-[11px] leading-relaxed text-slate-100 shadow-xl ring-1 ring-cyan-500/15 group-hover/ic:block group-focus-within/ic:block"
            >
              <p class="font-semibold text-slate-100">{{ t('adminDash.tipUsersTitle') }}</p>
              <p class="mt-1 whitespace-pre-line text-slate-300">
                {{
                  t('adminDash.tipUsersBody', {
                    total: fmtInt(s?.total_users),
                    dau: snapshot?.users?.dau_today != null ? fmtInt(snapshot.users.dau_today) : t('adminCommon.dash'),
                    newToday: snapshot?.users?.new_users_today != null ? fmtInt(snapshot.users.new_users_today) : t('adminCommon.dash'),
                  })
                }}
              </p>
            </div>
          </div>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.users') }}</p>
          <p class="mt-2 text-2xl font-semibold tracking-tight text-slate-100">{{ fmtInt(s?.total_users) }}</p>
        </div>
        <div class="th-glass relative overflow-visible rounded-[var(--th-radius-lg)] p-5 pr-14">
          <div class="group/ic pointer-events-auto absolute right-3 top-3 z-20">
            <button
              type="button"
              class="flex h-9 w-9 cursor-help items-center justify-center rounded-xl bg-emerald-500/12 text-emerald-300/90 ring-1 ring-emerald-400/20 outline-none transition hover:bg-emerald-500/18 focus-visible:ring-2 focus-visible:ring-amber-400/40"
              :aria-label="t('adminDash.statIconAria')"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.85" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M10 13a5 5 0 0 0 7.54.54L21 10" />
                <path d="M14 11a5 5 0 0 0-7.54-.54L3 14" />
                <path d="M3 3v6h6M21 21v-6h-6" />
              </svg>
            </button>
            <div
              class="pointer-events-none absolute right-0 top-11 z-30 hidden w-[min(17.5rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-left text-[11px] leading-relaxed text-slate-100 shadow-xl ring-1 ring-emerald-500/15 group-hover/ic:block group-focus-within/ic:block"
            >
              <p class="font-semibold text-slate-100">{{ t('adminDash.tipChannelsTitle') }}</p>
              <p class="mt-1 whitespace-pre-line text-slate-300">
                {{
                  t('adminDash.tipChannelsBody', {
                    enabledSum: fmtInt(s?.enabled_channels),
                    enabledInsight: insight?.channels?.enabled != null ? fmtInt(insight.channels.enabled) : t('adminCommon.dash'),
                    disabled: insight?.channels?.disabled != null ? fmtInt(insight.channels.disabled) : t('adminCommon.dash'),
                    unhealthy: insight?.channels?.unhealthy != null ? fmtInt(insight.channels.unhealthy) : t('adminCommon.dash'),
                  })
                }}
              </p>
            </div>
          </div>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.channels') }}</p>
          <p class="mt-2 text-2xl font-semibold tracking-tight text-slate-100">{{ fmtInt(s?.enabled_channels) }}</p>
        </div>
        <div class="th-glass relative overflow-visible rounded-[var(--th-radius-lg)] p-5 pr-14">
          <div class="group/ic pointer-events-auto absolute right-3 top-3 z-20">
            <button
              type="button"
              class="flex h-9 w-9 cursor-help items-center justify-center rounded-xl bg-violet-500/12 text-violet-300/90 ring-1 ring-violet-400/20 outline-none transition hover:bg-violet-500/18 focus-visible:ring-2 focus-visible:ring-amber-400/40"
              :aria-label="t('adminDash.statIconAria')"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M4 19h4v-8H4v8zm6 0h4V5h-4v14zm6 0h4v-5h-4v5z" />
              </svg>
            </button>
            <div
              class="pointer-events-none absolute right-0 top-11 z-30 hidden w-[min(17.5rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-left text-[11px] leading-relaxed text-slate-100 shadow-xl ring-1 ring-violet-500/15 group-hover/ic:block group-focus-within/ic:block"
            >
              <p class="font-semibold text-slate-100">{{ t('adminDash.tipReqTitle') }}</p>
              <p class="mt-1 whitespace-pre-line text-slate-300">
                {{
                  t('adminDash.tipReqBody', {
                    today: fmtInt(s?.today_requests),
                    yest: fmtInt(s?.yesterday_requests),
                    d7: fmtInt(s?.last_7d_requests),
                    vs: fmtVsPct(s?.vs_yesterday_requests_pct, s?.today_requests, s?.yesterday_requests),
                  })
                }}
              </p>
            </div>
          </div>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.req') }}</p>
          <p class="mt-2 text-2xl font-semibold tracking-tight text-slate-100">{{ fmtInt(s?.today_requests) }}</p>
        </div>
        <div class="th-glass relative overflow-visible rounded-[var(--th-radius-lg)] p-5 pr-14">
          <div class="group/ic pointer-events-auto absolute right-3 top-3 z-20">
            <button
              type="button"
              class="flex h-9 w-9 cursor-help items-center justify-center rounded-xl bg-fuchsia-500/12 text-fuchsia-300/90 ring-1 ring-fuchsia-400/20 outline-none transition hover:bg-fuchsia-500/18 focus-visible:ring-2 focus-visible:ring-amber-400/40"
              :aria-label="t('adminDash.statIconAria')"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M13 2 3 14h8l-1 8 10-12h-8l1-8z" />
              </svg>
            </button>
            <div
              class="pointer-events-none absolute right-0 top-11 z-30 hidden w-[min(17.5rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-left text-[11px] leading-relaxed text-slate-100 shadow-xl ring-1 ring-fuchsia-500/15 group-hover/ic:block group-focus-within/ic:block"
            >
              <p class="font-semibold text-slate-100">{{ t('adminDash.tipTokTitle') }}</p>
              <p class="mt-1 whitespace-pre-line text-slate-300">
                {{
                  t('adminDash.tipTokBody', {
                    today: fmtInt(s?.today_tokens),
                    yest: fmtInt(s?.yesterday_tokens),
                    d7: fmtInt(s?.last_7d_tokens),
                    vs: fmtVsPct(s?.vs_yesterday_tokens_pct, s?.today_tokens, s?.yesterday_tokens),
                  })
                }}
              </p>
            </div>
          </div>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.tokens') }}</p>
          <p class="mt-2 text-2xl font-semibold tracking-tight text-slate-100">{{ fmtInt(s?.today_tokens) }}</p>
        </div>
        <div class="th-glass relative overflow-visible rounded-[var(--th-radius-lg)] p-5 pr-14">
          <div class="group/ic pointer-events-auto absolute right-3 top-3 z-20">
            <button
              type="button"
              class="flex h-9 w-9 cursor-help items-center justify-center rounded-xl bg-amber-500/12 text-amber-200/95 ring-1 ring-amber-400/25 outline-none transition hover:bg-amber-500/18 focus-visible:ring-2 focus-visible:ring-amber-400/40"
              :aria-label="t('adminDash.statIconAria')"
            >
              <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.85" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <rect x="2" y="5" width="20" height="14" rx="2" />
                <line x1="2" y1="10" x2="22" y2="10" />
                <rect x="5" y="14" width="7" height="3.5" rx="0.75" fill="currentColor" fill-opacity="0.35" stroke="none" />
              </svg>
            </button>
            <div
              class="pointer-events-none absolute right-0 top-11 z-30 hidden w-[min(17.5rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-left text-[11px] leading-relaxed text-slate-100 shadow-xl ring-1 ring-amber-500/20 group-hover/ic:block group-focus-within/ic:block"
            >
              <p class="font-semibold text-slate-100">{{ t('adminDash.tipCostTitle') }}</p>
              <p class="mt-1 whitespace-pre-line text-slate-300">
                {{
                  t('adminDash.tipCostBody', {
                    todayQ: fmtInt(s?.today_cost_quota),
                    yestQ: fmtInt(s?.yesterday_cost_quota),
                    d7Q: fmtInt(s?.last_7d_cost_quota),
                    todayY: fmtYuan(s?.today_cost_yuan ?? todayCostYuan),
                    vs: fmtVsPct(s?.vs_yesterday_cost_pct, s?.today_cost_quota, s?.yesterday_cost_quota),
                    per: String(quotaPerYuan()),
                  })
                }}
              </p>
            </div>
          </div>
          <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.todayCost') }}</p>
          <p class="mt-2 text-2xl font-semibold tracking-tight text-slate-100">{{ fmtInt(s?.today_cost_quota) }}</p>
          <p class="mt-1 text-xs leading-relaxed text-slate-500">
            {{ t('adminDash.costQuotaHint') }}
          </p>
          <p v-if="todayCostYuan !== undefined" class="mt-1 text-xs text-amber-200/90">
            {{ t('adminDash.costEquiv', { yuan: todayCostYuan.toFixed(2), per: String(quotaPerYuan()) }) }}
          </p>
        </div>
      </div>

      <div v-if="s" class="mt-6 th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.compareSection') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminDash.last7dHint') }}</p>

        <div class="th-admin-divider-t mt-4 pt-4">
          <div class="flex flex-wrap items-center gap-2">
            <p class="text-xs font-medium uppercase tracking-wider text-slate-500">{{ t('adminDash.vsYesterdayLine') }}</p>
            <div class="group/vsh relative">
              <button
                type="button"
                class="flex h-6 w-6 cursor-help items-center justify-center rounded-md border border-white/[0.08] bg-black/[0.15] text-[11px] font-semibold text-slate-500 outline-none transition hover:border-amber-400/25 hover:text-amber-200/90 focus-visible:ring-2 focus-visible:ring-amber-400/40"
                :aria-label="t('adminDash.vsPctHintTitle')"
              >?</button>
              <div
                class="pointer-events-none absolute left-0 top-8 z-30 hidden w-[min(18rem,calc(100vw-2rem))] rounded-lg border border-white/15 bg-slate-950/95 px-2.5 py-2 text-[11px] leading-relaxed text-slate-200 shadow-xl ring-1 ring-amber-500/15 group-hover/vsh:block group-focus-within/vsh:block"
              >
                {{ t('adminDash.vsPctHintBody') }}
              </div>
            </div>
          </div>
          <div class="mt-2 flex flex-wrap gap-x-6 gap-y-2 text-sm">
            <div class="flex items-baseline gap-2">
              <span class="text-slate-500">{{ t('adminDash.vsYesterdayReq') }}</span>
              <span
                :class="pctTone(s.vs_yesterday_requests_pct, s.today_requests, s.yesterday_requests)"
                class="font-mono font-medium tabular-nums"
              >{{ fmtVsPct(s.vs_yesterday_requests_pct, s.today_requests, s.yesterday_requests) }}</span>
            </div>
            <div class="flex items-baseline gap-2">
              <span class="text-slate-500">{{ t('adminDash.vsYesterdayTok') }}</span>
              <span
                :class="pctTone(s.vs_yesterday_tokens_pct, s.today_tokens, s.yesterday_tokens)"
                class="font-mono font-medium tabular-nums"
              >{{ fmtVsPct(s.vs_yesterday_tokens_pct, s.today_tokens, s.yesterday_tokens) }}</span>
            </div>
            <div class="flex items-baseline gap-2">
              <span class="text-slate-500">{{ t('adminDash.vsYesterdayCost') }}</span>
              <span
                :class="pctTone(s.vs_yesterday_cost_pct, s.today_cost_quota, s.yesterday_cost_quota)"
                class="font-mono font-medium tabular-nums"
              >{{ fmtVsPct(s.vs_yesterday_cost_pct, s.today_cost_quota, s.yesterday_cost_quota) }}</span>
            </div>
          </div>
        </div>

        <div class="mt-6 grid gap-4 sm:grid-cols-2">
          <div class="th-admin-compare-tile">
            <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.yesterday') }}</h3>
            <dl class="mt-3 space-y-2 text-sm">
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblRequests') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.yesterday_requests) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblTokens') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.yesterday_tokens) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblCostQuota') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.yesterday_cost_quota) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblCostYuan') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtYuan(s.yesterday_cost_yuan) }}</dd>
              </div>
            </dl>
          </div>
          <div class="th-admin-compare-tile">
            <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.last7d') }}</h3>
            <dl class="mt-3 space-y-2 text-sm">
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblRequests') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.last_7d_requests) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblTokens') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.last_7d_tokens) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblCostQuota') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtInt(s.last_7d_cost_quota) }}</dd>
              </div>
              <div class="flex justify-between gap-3">
                <dt class="text-slate-500">{{ t('adminDash.lblCostYuan') }}</dt>
                <dd class="font-mono tabular-nums">{{ fmtYuan(s.last_7d_cost_yuan) }}</dd>
              </div>
            </dl>
          </div>
        </div>
      </div>

      <div v-if="s" class="relative mt-6 th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6" :class="{ 'opacity-70': insightBusy }">
        <p v-if="insightErr" class="mb-3 text-xs text-amber-300/95">{{ insightErr }}</p>
        <template v-if="insight">
          <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.p2TrendTitle') }}</h2>
              <p v-if="insight.timeseries_granularity === 'hour'" class="mt-1 text-[11px] leading-relaxed text-slate-500">
                {{ t('adminDash.p2TrendHourlyHint') }}
              </p>
            </div>
            <div class="flex flex-wrap items-center gap-2">
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-xs font-medium transition"
                :class="chartDays === 1 ? 'bg-amber-500/25 text-amber-100 ring-1 ring-amber-400/35' : 'text-slate-400 hover:bg-white/[0.05]'"
                @click="chartDays = 1"
              >
                {{ t('adminDash.p2TrendToday') }}
              </button>
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-xs font-medium transition"
                :class="chartDays === 7 ? 'bg-amber-500/25 text-amber-100 ring-1 ring-amber-400/35' : 'text-slate-400 hover:bg-white/[0.05]'"
                @click="chartDays = 7"
              >
                {{ t('adminDash.p2Trend7') }}
              </button>
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-xs font-medium transition"
                :class="chartDays === 30 ? 'bg-amber-500/25 text-amber-100 ring-1 ring-amber-400/35' : 'text-slate-400 hover:bg-white/[0.05]'"
                @click="chartDays = 30"
              >
                {{ t('adminDash.p2Trend30') }}
              </button>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-2">
            <button
              type="button"
              class="rounded-lg px-3 py-1.5 text-xs transition"
              :class="chartMetric === 'requests' ? 'bg-cyan-500/20 text-cyan-100 ring-1 ring-cyan-400/30' : 'text-slate-400 hover:bg-white/[0.05]'"
              @click="chartMetric = 'requests'"
            >
              {{ t('adminDash.p2MetricReq') }}
            </button>
            <button
              type="button"
              class="rounded-lg px-3 py-1.5 text-xs transition"
              :class="chartMetric === 'tokens' ? 'bg-cyan-500/20 text-cyan-100 ring-1 ring-cyan-400/30' : 'text-slate-400 hover:bg-white/[0.05]'"
              @click="chartMetric = 'tokens'"
            >
              {{ t('adminDash.p2MetricTok') }}
            </button>
            <button
              type="button"
              class="rounded-lg px-3 py-1.5 text-xs transition"
              :class="chartMetric === 'cost_yuan' ? 'bg-cyan-500/20 text-cyan-100 ring-1 ring-cyan-400/30' : 'text-slate-400 hover:bg-white/[0.05]'"
              @click="chartMetric = 'cost_yuan'"
            >
              {{ t('adminDash.p2MetricCost') }}
            </button>
          </div>
          <div class="mt-4 w-full max-w-xl overflow-x-auto">
            <div v-if="sparkLine.points.length === 0" class="flex h-24 items-center justify-center text-xs text-slate-500">{{ t('adminCommon.dash') }}</div>
            <template v-else>
              <svg
                ref="sparkSvgRef"
                class="block h-24 w-full min-w-[280px] max-w-xl cursor-crosshair text-cyan-400/90"
                :viewBox="`0 0 ${SPARK_W} ${SPARK_H}`"
                preserveAspectRatio="none"
                role="img"
                :aria-label="t('adminDash.p2TrendTitle')"
                @mousemove="onSparkSvgMove"
                @mouseleave="onSparkSvgLeave"
              >
                <rect x="0" y="0" :width="SPARK_W" :height="SPARK_H" fill="transparent" pointer-events="all" />
                <polyline
                  v-if="sparkLine.polyline"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  vector-effect="non-scaling-stroke"
                  pointer-events="none"
                  :points="sparkLine.polyline"
                />
                <template v-if="sparkHover !== null && sparkLine.points[sparkHover]">
                  <line
                    :x1="sparkLine.points[sparkHover].x"
                    y1="0"
                    :x2="sparkLine.points[sparkHover].x"
                    :y2="SPARK_H"
                    stroke="rgba(148,163,184,0.45)"
                    stroke-width="1"
                    vector-effect="non-scaling-stroke"
                    stroke-dasharray="4 3"
                    pointer-events="none"
                  />
                  <circle
                    :cx="sparkLine.points[sparkHover].x"
                    :cy="sparkLine.points[sparkHover].y"
                    r="5"
                    fill="rgb(34,211,238)"
                    stroke="rgb(15,23,42)"
                    stroke-width="2"
                    pointer-events="none"
                  />
                </template>
              </svg>
              <Teleport to="body">
                <div
                  v-show="sparkHover !== null && sparkLine.points.length > 0"
                  class="pointer-events-none fixed z-[10000] max-w-[min(260px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[11px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-cyan-500/20"
                  :style="{ left: sparkTip.x + 'px', top: sparkTip.y + 'px' }"
                >
                  {{ sparkHover !== null ? sparkHoverText(sparkHover) : '' }}
                </div>
              </Teleport>
            </template>
            <div
              class="relative mx-auto mt-1 w-full min-w-[280px] max-w-xl text-[10px] leading-none text-slate-500"
              style="min-height: 14px"
            >
              <span v-for="tk in axisTicks" :key="tk.key" class="absolute top-0 whitespace-nowrap" :style="axisTickStyle(tk)">{{ tk.label }}</span>
            </div>
          </div>

          <div class="mt-8 border-t border-white/[0.06] pt-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.p2TopModels') }}</h2>
              <RouterLink to="/admin/model-market" class="text-xs text-amber-400/90 underline-offset-2 hover:text-amber-300 hover:underline">
                {{ t('adminDash.p2LinkMarket') }} →
              </RouterLink>
            </div>
            <div class="mt-4 grid gap-6 md:grid-cols-2">
              <div>
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p2TopToday') }}</h3>
                <ul class="mt-3 space-y-2">
                  <li
                    v-for="m in insight.top_models_today?.items ?? []"
                    :key="'td-' + m.model"
                    class="cursor-default text-sm"
                    @mouseenter="onModelRowEnter(m, $event)"
                    @mousemove="onModelRowMove($event)"
                    @mouseleave="onModelRowLeave"
                  >
                    <div class="flex justify-between gap-2 text-xs text-slate-400">
                      <span class="min-w-0 truncate font-mono text-slate-200" :title="m.model">{{ m.model }}</span>
                      <span class="shrink-0 tabular-nums">{{ fmtInt(m.requests) }}</span>
                    </div>
                    <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                      <div
                        class="h-full rounded-full bg-cyan-500/40"
                        :style="{ width: (100 * (Number(m.requests) || 0)) / maxBarRequests(insight.top_models_today?.items ?? []) + '%' }"
                      />
                    </div>
                  </li>
                  <li v-if="!(insight.top_models_today?.items ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
                </ul>
              </div>
              <div>
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p2Top7d') }}</h3>
                <ul class="mt-3 space-y-2">
                  <li
                    v-for="m in insight.top_models_7d?.items ?? []"
                    :key="'7d-' + m.model"
                    class="cursor-default text-sm"
                    @mouseenter="onModelRowEnter(m, $event)"
                    @mousemove="onModelRowMove($event)"
                    @mouseleave="onModelRowLeave"
                  >
                    <div class="flex justify-between gap-2 text-xs text-slate-400">
                      <span class="min-w-0 truncate font-mono text-slate-200" :title="m.model">{{ m.model }}</span>
                      <span class="shrink-0 tabular-nums">{{ fmtInt(m.requests) }}</span>
                    </div>
                    <div class="mt-1 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                      <div
                        class="h-full rounded-full bg-violet-500/35"
                        :style="{ width: (100 * (Number(m.requests) || 0)) / maxBarRequests(insight.top_models_7d?.items ?? []) + '%' }"
                      />
                    </div>
                  </li>
                  <li v-if="!(insight.top_models_7d?.items ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
                </ul>
              </div>
            </div>
          </div>

          <div class="mt-8 border-t border-white/[0.06] pt-6">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.p2ChannelTitle') }}</h2>
              <RouterLink to="/admin/channels" class="text-xs text-amber-400/90 underline-offset-2 hover:text-amber-300 hover:underline">
                {{ t('adminDash.p2LinkChannels') }} →
              </RouterLink>
            </div>
            <div class="mt-3 flex flex-wrap gap-3 text-xs text-slate-400">
              <span>{{ t('adminDash.p2ChannelEnabled') }}: <strong class="text-slate-200">{{ fmtInt(insight.channels?.enabled) }}</strong></span>
              <span>{{ t('adminDash.p2ChannelDisabled') }}: <strong class="text-slate-200">{{ fmtInt(insight.channels?.disabled) }}</strong></span>
              <span>{{ t('adminDash.p2ChannelUnhealthy') }}: <strong class="text-rose-300/90">{{ fmtInt(insight.channels?.unhealthy) }}</strong></span>
              <span>{{ t('adminDash.p2ChannelSchedOff') }}: <strong class="text-amber-200/90">{{ fmtInt(insight.channels?.schedulable_off) }}</strong></span>
            </div>
            <p class="mt-4 text-xs font-medium uppercase tracking-wider text-slate-500">{{ t('adminDash.p2ChannelUsageTitle') }}</p>
            <div v-if="(insight.channels?.usage_7d ?? []).length" class="mt-3 max-w-xl space-y-2">
              <div
                v-for="row in insight.channels?.usage_7d ?? []"
                :key="'ch-' + row.channel_id"
                class="cursor-default"
                @mouseenter="onChRowEnter(row, $event)"
                @mousemove="onChRowMove($event)"
                @mouseleave="onChRowLeave"
              >
                <div class="flex justify-between gap-2 text-xs text-slate-400">
                  <span class="min-w-0 truncate text-slate-200" :title="row.channel_name">{{ row.channel_name }}</span>
                  <span class="shrink-0 font-mono tabular-nums">{{ row.pct }}% · {{ fmtInt(row.requests) }}</span>
                </div>
                <div class="mt-1 h-2 overflow-hidden rounded-full bg-white/[0.06]">
                  <div class="h-full rounded-full bg-amber-500/30" :style="{ width: Math.min(100, row.pct) + '%' }" />
                </div>
              </div>
            </div>
            <p v-else class="mt-2 text-xs text-slate-500">{{ t('adminDash.p2NoChannelUsage') }}</p>
          </div>
        </template>
        <p v-else-if="!insightBusy" class="text-sm text-slate-500">{{ t('adminDash.p2InsightLoadFail') }}</p>
      </div>

      <div v-if="s" class="mt-6 th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.p3SectionTitle') }}</h2>
        <p v-if="snapshotErr" class="mt-2 text-xs text-amber-300/95">{{ snapshotErr }}</p>
        <template v-if="snapshot">
          <div class="mt-5 grid gap-6 lg:grid-cols-3">
            <div class="th-admin-compare-tile">
              <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p3UsersTitle') }}</h3>
              <dl class="mt-3 space-y-2 text-sm">
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3DauToday') }}</dt>
                  <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.users?.dau_today) }}</dd>
                </div>
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3NewToday') }}</dt>
                  <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.users?.new_users_today) }}</dd>
                </div>
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3New7d') }}</dt>
                  <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.users?.new_users_7d) }}</dd>
                </div>
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3Groups') }}</dt>
                  <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.users?.groups_count) }}</dd>
                </div>
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3NegBal') }}</dt>
                  <dd class="font-mono tabular-nums text-rose-300/90">{{ fmtInt(snapshot.users?.negative_balance) }}</dd>
                </div>
                <div class="flex justify-between gap-2">
                  <dt class="text-slate-500">{{ t('adminDash.p3ZeroBal') }}</dt>
                  <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.users?.zero_balance) }}</dd>
                </div>
              </dl>
            </div>
            <div class="th-admin-compare-tile">
              <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p3HttpTitle') }}</h3>
              <div class="mt-3 space-y-4 text-sm">
                <div>
                  <p class="text-xs text-slate-500">{{ t('adminDash.p3HttpToday') }}</p>
                  <dl class="mt-2 space-y-1">
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3HttpTotal') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.today?.total_requests) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3Http4xx') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.today?.n_4xx) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3Http5xx') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.today?.n_5xx) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3HttpFailPct') }}</dt>
                      <dd class="font-mono tabular-nums">{{ (snapshot.http?.today?.fail_pct ?? 0).toFixed(1) }}%</dd>
                    </div>
                  </dl>
                </div>
                <div>
                  <p class="text-xs text-slate-500">{{ t('adminDash.p3Http7d') }}</p>
                  <dl class="mt-2 space-y-1">
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3HttpTotal') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.last_7d?.total_requests) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3Http4xx') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.last_7d?.n_4xx) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3Http5xx') }}</dt>
                      <dd class="font-mono tabular-nums">{{ fmtInt(snapshot.http?.last_7d?.n_5xx) }}</dd>
                    </div>
                    <div class="flex justify-between gap-2">
                      <dt class="text-slate-500">{{ t('adminDash.p3HttpFailPct') }}</dt>
                      <dd class="font-mono tabular-nums">{{ (snapshot.http?.last_7d?.fail_pct ?? 0).toFixed(1) }}%</dd>
                    </div>
                  </dl>
                </div>
              </div>
            </div>
            <div class="th-admin-compare-tile">
              <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p3DraftAnnouncements') }}</h3>
              <ul class="mt-3 space-y-2 text-sm">
                <li v-for="a in snapshot.draft_announcements ?? []" :key="'ann-' + a.id" class="min-w-0">
                  <RouterLink
                    to="/admin/announcements"
                    class="block truncate text-cyan-400/90 no-underline hover:text-cyan-300 hover:underline"
                    :title="a.title"
                  >#{{ a.id }} {{ a.title }}</RouterLink>
                </li>
                <li v-if="!(snapshot.draft_announcements ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
              </ul>
            </div>
          </div>

          <div class="mt-8 grid gap-6 lg:grid-cols-2">
            <div>
              <div class="flex items-center justify-between gap-2">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p3RecentLogs') }}</h3>
                <RouterLink to="/admin/logs" class="text-xs text-amber-400/90 no-underline hover:underline">{{ t('layout.adminNavLogs') }} →</RouterLink>
              </div>
              <div class="th-admin-table-shell mt-3 hidden overflow-x-auto lg:block">
                <table class="w-full min-w-[520px] text-left text-xs">
                  <thead class="th-admin-table-head">
                    <tr>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p3ThTime') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p3ThModel') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p3ThUser') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p3ThStatus') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p3ThTokens') }}</th>
                    </tr>
                  </thead>
                  <tbody class="th-admin-table-body">
                    <tr v-for="row in snapshot.recent_logs ?? []" :key="'log-' + row.id">
                      <td class="whitespace-nowrap px-2 py-1.5 font-mono text-slate-400">{{ fmtLogTime(row.request_time) }}</td>
                      <td class="max-w-[10rem] truncate px-2 py-1.5 font-mono" :title="row.model">{{ row.model }}</td>
                      <td class="px-2 py-1.5 font-mono text-slate-400">{{ fmtInt(row.user_id) }}</td>
                      <td class="px-2 py-1.5 font-mono">{{ fmtInt(row.status_code) }}</td>
                      <td class="px-2 py-1.5 font-mono">{{ fmtInt(row.total_tokens) }}</td>
                    </tr>
                    <tr v-if="!(snapshot.recent_logs ?? []).length">
                      <td colspan="5" class="px-2 py-4 text-center text-slate-500">{{ t('adminCommon.dash') }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <ul class="mt-3 space-y-2 lg:hidden">
                <li
                  v-for="row in snapshot.recent_logs ?? []"
                  :key="'log-m-' + row.id"
                  class="th-admin-compare-tile--row rounded-lg border border-white/[0.06] px-3 py-2 text-xs"
                >
                  <p class="font-mono text-slate-400">{{ fmtLogTime(row.request_time) }}</p>
                  <p class="mt-1 truncate font-mono text-slate-200" :title="row.model">{{ row.model }}</p>
                  <p class="mt-1 text-slate-500">{{ t('adminDash.p3ThUser') }} {{ fmtInt(row.user_id) }} · {{ t('adminDash.p3ThStatus') }} {{ fmtInt(row.status_code) }} · {{ fmtInt(row.total_tokens) }} tok</p>
                </li>
                <li v-if="!(snapshot.recent_logs ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
              </ul>
            </div>
            <div>
              <div class="flex items-center justify-between gap-2">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p3RecentChannels') }}</h3>
                <RouterLink to="/admin/channels" class="text-xs text-amber-400/90 no-underline hover:underline">{{ t('layout.adminNavChannels') }} →</RouterLink>
              </div>
              <ul class="mt-3 space-y-2 text-sm">
                <li
                  v-for="ch in snapshot.recent_channels ?? []"
                  :key="'rch-' + ch.id"
                  class="th-admin-compare-tile--row flex flex-wrap items-center justify-between gap-2"
                >
                  <span class="min-w-0 truncate font-medium" :title="ch.channel_name">{{ ch.channel_name }}</span>
                  <span class="shrink-0 text-xs text-slate-500">{{ fmtLogTime(ch.updated_at) }}</span>
                  <span class="w-full text-[11px] text-slate-500 sm:w-auto">{{ t('adminDash.p3ThEnabled') }}: {{ ch.enabled ? t('adminCommon.yes') : t('adminCommon.no') }}</span>
                </li>
                <li v-if="!(snapshot.recent_channels ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
              </ul>
            </div>
          </div>
        </template>
      </div>

      <div v-if="s" class="mt-6 th-glass rounded-[var(--th-radius-lg)] p-5 md:p-6">
        <h2 class="text-sm font-semibold tracking-tight text-slate-100">{{ t('adminDash.p4SectionTitle') }}</h2>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminDash.p4SectionDesc') }}</p>
        <p v-if="opsErr" class="mt-2 text-xs text-amber-300/95">{{ opsErr }}</p>
        <template v-if="ops">
          <p class="mt-3 text-xs leading-relaxed text-slate-500">{{ t('adminDash.p4AuditPermHint') }}</p>
          <div class="mt-6 grid gap-6 lg:grid-cols-3">
            <div class="th-admin-compare-tile lg:col-span-2">
              <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p4AuditTitle') }}</h3>
              <div class="th-admin-table-shell mt-3 hidden overflow-x-auto lg:block">
                <table class="w-full min-w-[640px] text-left text-xs">
                  <thead class="th-admin-table-head">
                    <tr>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p4ThTime') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p4ThActor') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p4ThAction') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p4ThResource') }}</th>
                      <th class="px-2 py-2 font-medium text-slate-500">{{ t('adminDash.p4ThIp') }}</th>
                    </tr>
                  </thead>
                  <tbody class="th-admin-table-body">
                    <tr v-for="row in ops.recent_audit ?? []" :key="'aud-' + row.id">
                      <td class="whitespace-nowrap px-2 py-1.5 font-mono text-slate-400">{{ fmtLogTime(row.created_at) }}</td>
                      <td class="max-w-[10rem] truncate px-2 py-1.5" :title="row.admin_email || ''">
                        <span class="font-mono text-slate-400">#{{ row.admin_id }}</span>
                        <span v-if="row.admin_email" class="ml-1 text-slate-300">{{ row.admin_email }}</span>
                      </td>
                      <td class="max-w-[12rem] truncate px-2 py-1.5 font-mono text-xs" :title="row.action">{{ row.action }}</td>
                      <td class="max-w-[10rem] truncate px-2 py-1.5 font-mono text-slate-400" :title="(row.resource || '') + (row.resource_id ? ' / ' + row.resource_id : '')">
                        {{ row.resource || t('adminCommon.dash') }}<span v-if="row.resource_id"> / {{ row.resource_id }}</span>
                      </td>
                      <td class="whitespace-nowrap px-2 py-1.5 font-mono text-slate-500">{{ row.ip_address || t('adminCommon.dash') }}</td>
                    </tr>
                    <tr v-if="!(ops.recent_audit ?? []).length">
                      <td colspan="5" class="px-2 py-4 text-center text-slate-500">{{ t('adminCommon.dash') }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <ul class="mt-3 space-y-2 lg:hidden">
                <li
                  v-for="row in ops.recent_audit ?? []"
                  :key="'aud-m-' + row.id"
                  class="th-admin-compare-tile--row rounded-lg border border-white/[0.06] px-3 py-2 text-xs"
                >
                  <p class="font-mono text-slate-400">{{ fmtLogTime(row.created_at) }}</p>
                  <p class="mt-1 text-slate-300">
                    <span class="font-mono text-slate-400">#{{ row.admin_id }}</span>
                    <span v-if="row.admin_email" class="ml-1">{{ row.admin_email }}</span>
                  </p>
                  <p class="mt-1 truncate font-mono text-slate-400" :title="row.action">{{ row.action }}</p>
                  <p class="mt-0.5 truncate font-mono text-slate-500" :title="(row.resource || '') + (row.resource_id ? ' / ' + row.resource_id : '')">
                    {{ row.resource || t('adminCommon.dash') }}<span v-if="row.resource_id"> / {{ row.resource_id }}</span>
                  </p>
                  <p class="mt-0.5 text-slate-500">{{ row.ip_address || t('adminCommon.dash') }}</p>
                </li>
                <li v-if="!(ops.recent_audit ?? []).length" class="text-xs text-slate-500">{{ t('adminCommon.dash') }}</li>
              </ul>
            </div>
            <div class="space-y-4">
              <div class="th-admin-compare-tile">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p4HealthTitle') }}</h3>
                <dl class="mt-3 space-y-2 text-sm">
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4Db') }}</dt>
                    <dd class="max-w-[55%] truncate font-mono text-xs" :title="ops.health?.checks?.database">{{ ops.health?.checks?.database }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4Redis') }}</dt>
                    <dd class="max-w-[55%] truncate font-mono text-xs" :title="ops.health?.checks?.redis">{{ ops.health?.checks?.redis }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4AppVer') }}</dt>
                    <dd class="font-mono text-xs">{{ ops.health?.app_version }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4GoVer') }}</dt>
                    <dd class="max-w-[60%] truncate font-mono text-xs" :title="ops.health?.go_version">{{ ops.health?.go_version }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4Uptime') }}</dt>
                    <dd class="font-mono text-xs">{{ fmtInt(ops.health?.uptime_sec) }} s</dd>
                  </div>
                </dl>
                <p class="mt-3 text-[11px] leading-relaxed text-slate-500">{{ t('adminDash.p4HealthPathsHint') }}</p>
              </div>
              <div class="th-admin-compare-tile">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p4ConfigTitle') }}</h3>
                <dl class="mt-3 space-y-2 text-sm">
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgMode') }}</dt>
                    <dd class="font-mono text-xs">{{ String(ops.config_outline?.server_mode ?? '') }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgQuotaPerYuan') }}</dt>
                    <dd class="font-mono tabular-nums">{{ fmtInt(Number(ops.config_outline?.currency_quota_per_yuan)) }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgCors') }}</dt>
                    <dd class="font-mono tabular-nums">{{ fmtInt(Number(ops.config_outline?.cors_origins_count)) }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgRealtime') }}</dt>
                    <dd>{{ ops.config_outline?.realtime_call_mapping ? t('adminCommon.yes') : t('adminCommon.no') }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgBrand') }}</dt>
                    <dd class="max-w-[55%] truncate font-mono text-xs" :title="String(ops.config_outline?.brand_name ?? '')">
                      {{ String(ops.config_outline?.brand_name ?? '') }}
                    </dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgEmail') }}</dt>
                    <dd>{{ ops.config_outline?.email_login_enabled ? t('adminCommon.yes') : t('adminCommon.no') }}</dd>
                  </div>
                  <div class="flex justify-between gap-2">
                    <dt class="text-slate-500">{{ t('adminDash.p4CfgLoginCaptcha') }}</dt>
                    <dd>{{ ops.config_outline?.login_captcha_enabled ? t('adminCommon.yes') : t('adminCommon.no') }}</dd>
                  </div>
                </dl>
                <RouterLink
                  :to="{ name: 'admin-settings-brand' }"
                  class="mt-3 inline-flex text-xs font-medium text-cyan-400/90 transition hover:text-cyan-300"
                >
                  {{ t('adminDash.p4SysCfgLink') }} →
                </RouterLink>
              </div>
              <div class="th-admin-note-dashed">
                <h3 class="text-xs font-semibold uppercase tracking-wider text-slate-500">{{ t('adminDash.p4AlertsTitle') }}</h3>
                <p class="mt-2 text-xs leading-relaxed text-slate-400">{{ t('adminDash.p4AlertsBody') }}</p>
                <p v-if="(ops.external_alerts?.scope ?? []).length" class="mt-2 font-mono text-[11px] text-slate-500">
                  {{ (ops.external_alerts?.scope ?? []).join(', ') }}
                </p>
              </div>
            </div>
          </div>
        </template>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-show="modelTip !== null"
        class="pointer-events-none fixed z-[10000] max-w-[min(280px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[10px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-cyan-500/15"
        :style="modelTip ? { left: modelTip.x + 'px', top: modelTip.y + 'px' } : {}"
      >
        {{ modelTipText() }}
      </div>
    </Teleport>
    <Teleport to="body">
      <div
        v-show="chTip !== null"
        class="pointer-events-none fixed z-[10000] max-w-[min(280px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[10px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-amber-500/15"
        :style="chTip ? { left: chTip.x + 'px', top: chTip.y + 'px' } : {}"
      >
        {{ chTipText() }}
      </div>
    </Teleport>
  </div>
</template>
