<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { http, type Envelope } from '../api/http'
import { useAuthStore } from '../stores/auth'
import ContactCardDisclosure from '../components/ContactCardDisclosure.vue'

const { t, tm } = useI18n()

type Balance = {
  balance?: number
  balance_yuan?: number
  group_name?: string
  rate_multiplier?: number
  currency_unit?: number
}

type DayRow = { date: string; requests: number; cost: number; tokens?: number }
type ModelRow = { model: string; requests: number; cost: number }
type HourRow = { at?: string; label?: string; requests: number; cost: number; tokens?: number }

type UsagePayload = {
  period?: string
  total_requests?: number
  total_tokens?: number
  total_cost?: number
  daily_breakdown?: DayRow[]
  model_breakdown?: ModelRow[]
  hourly_breakdown?: HourRow[]
  avg_rpm?: number
  avg_tpm?: number
  unique_models?: number
  lifetime?: { total_requests?: number; total_tokens?: number; total_cost?: number }
}

type AnnouncementItem = { id: number; title: string; content?: string; published_at?: string }

const balance = ref<Balance | null>(null)
const period = ref<'today' | '7d' | '30d' | 'all'>('7d')
const usage = ref<UsagePayload | null>(null)
const err = ref('')
const exportBusy = ref(false)
const auth = useAuthStore()
const announcements = ref<AnnouncementItem[]>([])
const readyOk = ref<boolean | null>(null)
const chartTab = ref<'cost_trend' | 'cost_dist' | 'req_dist' | 'req_rank'>('cost_trend')
const openFaq = ref<number | null>(0)

/** 消耗趋势图：悬停点索引与屏幕坐标（用于浮层提示） */
const trendHover = ref<number | null>(null)
const trendTip = ref({ x: 0, y: 0 })

/** 每日消费柱、分布条：Teleport 用视口坐标，避免被同 flex 内兄弟盖住 */
const dailyBarHover = ref<number | null>(null)
const dailyTip = ref({ x: 0, y: 0 })
const costDistHoverModel = ref<string | null>(null)
const costDistTip = ref({ x: 0, y: 0 })
const reqDistHoverModel = ref<string | null>(null)
const reqDistTip = ref({ x: 0, y: 0 })

const apiBase = computed(() => {
  const o = window.location.origin.replace(/\/$/, '')
  return `${o}/v1`
})

const greeting = computed(() => {
  const h = new Date().getHours()
  if (h < 12) return t('dashboard.greetMorning')
  if (h < 18) return t('dashboard.greetAfternoon')
  return t('dashboard.greetEvening')
})

const faqItems = computed(() => {
  const raw = tm('dashboard.faqItems') as { q: string; a: string }[] | undefined
  return Array.isArray(raw) ? raw : []
})

const trendSeries = computed(() => {
  if (period.value === 'today' && (usage.value?.hourly_breakdown?.length || 0) > 0) {
    return { kind: 'hour' as const, rows: usage.value!.hourly_breakdown! }
  }
  const days = usage.value?.daily_breakdown || []
  return { kind: 'day' as const, rows: days as { date: string; requests: number; cost: number; tokens?: number }[] }
})

const CHART_W = 320
const CHART_H = 120

/** 折线图几何：折线顶点（x 与命中检测一致） */
const trendGeometry = computed(() => {
  const rows = trendSeries.value.rows
  const n = rows.length
  if (!n) return { points: [] as { x: number; y: number; cost: number; row: (typeof rows)[0] }[], polyline: '' }
  const costs = rows.map((r) => (typeof r.cost === 'number' ? r.cost : 0))
  const max = Math.max(...costs, 1e-9)
  const min = Math.min(...costs, 0)
  const span = max - min || 1
  const step = n > 1 ? CHART_W / (n - 1) : 0
  const points = costs.map((v, i) => {
    const x = n > 1 ? i * step : CHART_W / 2
    const y = CHART_H - ((v - min) / span) * (CHART_H - 8) - 4
    return { x, y, cost: v, row: rows[i] }
  })
  const polyline = points.map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ')
  return { points, polyline }
})

function trendRowLabel(row: { date?: string; label?: string }) {
  if (row && typeof row === 'object' && 'label' in row && row.label) return String(row.label)
  if (row && typeof row === 'object' && 'date' in row && row.date) return String(row.date)
  return ''
}

function trendRowTime(row: DayRow & HourRow): Date | null {
  const r = row as Record<string, unknown>
  if (r.at != null) {
    const d = new Date(String(r.at))
    if (!Number.isNaN(d.getTime())) return d
  }
  if (typeof row.date === 'string' && row.date) {
    const d = new Date(row.date)
    if (!Number.isNaN(d.getTime())) return d
  }
  return null
}

/** 趋势图横轴刻度索引：按索引步进取样 + 末段过近时合并，与 AdminDashboard 用量趋势一致，避免末两日等挤在同一视觉区域 */
const trendAxisTickIndices = computed(() => {
  const { rows, kind } = trendSeries.value
  const n = rows.length
  if (n <= 0) return new Set<number>()
  const maxLabels = kind === 'hour' ? 9 : 8
  let idxs: number[]
  if (n <= maxLabels) {
    idxs = Array.from({ length: n }, (_, i) => i)
  } else {
    const step = Math.max(1, Math.ceil((n - 1) / (maxLabels - 1)))
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
  return new Set(idxs)
})

function trendAxisShortLabel(row: DayRow & HourRow, kind: 'hour' | 'day'): string {
  if (kind === 'hour') {
    const d = trendRowTime(row)
    if (d) return String(d.getHours()).padStart(2, '0')
    const lab = trendRowLabel(row)
    const m = lab.match(/(\d{1,2}):(\d{2})/)
    if (m) return `${m[1].padStart(2, '0')}:${m[2]}`
    return lab.length > 8 ? lab.slice(-8) : lab
  }
  const d = trendRowTime(row)
  if (d) {
    return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  }
  const s = row.date || ''
  return s.length >= 10 ? s.slice(5, 10) : s
}

const trendSvgRef = ref<SVGSVGElement | null>(null)

function trendIndexFromEvent(e: MouseEvent, svg: SVGSVGElement): number {
  const { points } = trendGeometry.value
  if (!points.length) return 0
  const rect = svg.getBoundingClientRect()
  if (rect.width <= 0) return 0
  const svgX = ((e.clientX - rect.left) / rect.width) * CHART_W
  const x = Math.max(0, Math.min(CHART_W, svgX))
  let best = 0
  let bestD = Infinity
  for (let i = 0; i < points.length; i++) {
    const d = Math.abs(x - points[i].x)
    if (d < bestD) {
      bestD = d
      best = i
    }
  }
  return best
}

function clampClientTooltip(x: number, y: number, tw = 260, th = 120) {
  const pad = 10
  return {
    x: Math.max(pad, Math.min(x, window.innerWidth - tw - pad)),
    y: Math.max(pad, Math.min(y, window.innerHeight - th - pad)),
  }
}

function clampTrendTip(x: number, y: number) {
  return clampClientTooltip(x, y, 240, 120)
}

function onTrendSvgMove(e: MouseEvent) {
  const svg = trendSvgRef.value
  if (!svg) return
  const i = trendIndexFromEvent(e, svg)
  trendHover.value = i
  const c = clampTrendTip(e.clientX + 12, e.clientY + 8)
  trendTip.value = c
}

function onTrendSvgLeave() {
  trendHover.value = null
}

function dailyBarTip(b: DayRow) {
  return t('dashboard.dailyBarTip', {
    date: b.date,
    cost: fmtNum(b.cost, 2),
    req: String(b.requests ?? 0),
    tok: b.tokens != null ? String(b.tokens) : '—',
    reqWord: t('dashboard.req'),
    tokWord: t('dashboard.tok'),
  })
}

function modelDistTip(model: string, requests: number, maxReq: number) {
  const pct = maxReq > 0 ? Math.round((requests / maxReq) * 1000) / 10 : 0
  return t('dashboard.modelReqTip', { model, requests: String(requests), pct: String(pct) })
}

function modelCostTip(model: string, cost: number, pct: number) {
  return t('dashboard.modelCostTip', { model, cost: fmtNum(cost, 2), pct: String(pct) })
}

const sortedDaily = computed(() => {
  const rows = usage.value?.daily_breakdown || []
  return [...rows].sort((a, b) => a.date.localeCompare(b.date))
})

const maxDailyCost = computed(() => Math.max(...sortedDaily.value.map((d) => d.cost || 0), 0.0001))

const modelCostShare = computed(() => {
  const rows = usage.value?.model_breakdown || []
  const total = rows.reduce((a, m) => a + (m.cost || 0), 0) || 1
  return rows.map((m) => ({
    ...m,
    pct: Math.round(((m.cost || 0) / total) * 1000) / 10,
  }))
})

const modelByRequests = computed(() => {
  const rows = [...(usage.value?.model_breakdown || [])]
  return rows.sort((a, b) => (b.requests || 0) - (a.requests || 0))
})

const maxModelRequests = computed(() => Math.max(...modelByRequests.value.map((x) => x.requests || 0), 1))

function dailyBarTipText(i: number) {
  const b = sortedDaily.value[i]
  if (!b) return ''
  return dailyBarTip(b)
}

function onDailyBarEnter(i: number, e: MouseEvent) {
  dailyBarHover.value = i
  dailyTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 220, 110)
}

function onDailyBarMove(e: MouseEvent) {
  if (dailyBarHover.value === null) return
  dailyTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 220, 110)
}

function onDailyBarLeave() {
  dailyBarHover.value = null
}

type CostDistHoverRow = { model: string; cost: number; pct: number }

function onCostDistEnter(m: CostDistHoverRow, e: MouseEvent) {
  costDistHoverModel.value = m.model
  costDistTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 110)
}

function onCostDistMove(e: MouseEvent) {
  if (!costDistHoverModel.value) return
  costDistTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 280, 110)
}

function onCostDistLeave() {
  costDistHoverModel.value = null
}

function costDistHoverText() {
  const model = costDistHoverModel.value
  if (!model) return ''
  const row = modelCostShare.value.find((x) => x.model === model)
  if (!row) return ''
  return modelCostTip(row.model, row.cost || 0, row.pct)
}

function onReqDistEnter(m: ModelRow, e: MouseEvent) {
  reqDistHoverModel.value = m.model
  reqDistTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 260, 110)
}

function onReqDistMove(e: MouseEvent) {
  if (!reqDistHoverModel.value) return
  reqDistTip.value = clampClientTooltip(e.clientX + 12, e.clientY + 8, 260, 110)
}

function onReqDistLeave() {
  reqDistHoverModel.value = null
}

function reqDistHoverText() {
  const model = reqDistHoverModel.value
  if (!model) return ''
  const row = modelByRequests.value.find((x) => x.model === model)
  if (!row) return ''
  return modelDistTip(row.model, row.requests || 0, maxModelRequests.value)
}

watch(chartTab, () => {
  dailyBarHover.value = null
  costDistHoverModel.value = null
  reqDistHoverModel.value = null
})

function trendHoverText(i: number) {
  const p = trendGeometry.value.points[i]
  if (!p) return ''
  const r = p.row as DayRow & HourRow
  const label = trendRowLabel(r)
  return t('dashboard.trendHoverTip', {
    label,
    cost: fmtNum(typeof r.cost === 'number' ? r.cost : 0, 2),
    req: String(r.requests ?? 0),
    tok: String(r.tokens ?? 0),
    reqWord: t('dashboard.req'),
    tokWord: t('dashboard.tok'),
  })
}

const quickLinks = computed(() => [
  { to: '/console/models', title: t('ql.models'), desc: t('ql.modelsD') },
  { to: '/console/playground', title: t('ql.playground'), desc: t('ql.playgroundD') },
  { to: '/console/keys', title: t('ql.keys'), desc: t('ql.keysD') },
  { to: '/console/logs', title: t('ql.logs'), desc: t('ql.logsD') },
  { to: '/console/billing', title: t('ql.billing'), desc: t('ql.billingD') },
])

const footerYear = new Date().getFullYear()

function fmtNum(n: number | undefined | null, d = 2) {
  if (n == null || Number.isNaN(n)) return '—'
  return Number(n).toFixed(d)
}

function shortDate(s?: string) {
  if (!s) return ''
  try {
    return new Date(s).toLocaleString(undefined, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  } catch {
    return s.slice(0, 16)
  }
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    /* ignore */
  }
}

async function loadBalance() {
  const { data } = await http.get<Envelope<Balance>>('/api/user/balance')
  if (data.code === 0) balance.value = data.data
}

async function loadUsage() {
  const { data } = await http.get<Envelope<UsagePayload>>(`/api/user/usage?period=${period.value}`)
  if (data.code === 0) usage.value = data.data
}

async function loadAnnouncements() {
  try {
    const { data } = await http.get<Envelope<{ items: AnnouncementItem[] }>>('/api/user/announcements?limit=6')
    if (data.code === 0 && data.data?.items) announcements.value = data.data.items
  } catch {
    announcements.value = []
  }
}

async function loadReady() {
  try {
    const r = await fetch('/ready', { credentials: 'same-origin' })
    readyOk.value = r.ok
  } catch {
    readyOk.value = false
  }
}

async function loadAll() {
  err.value = ''
  try {
    await Promise.all([loadBalance(), loadUsage(), loadAnnouncements(), loadReady()])
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : t('common.loadFailed')
  }
}

async function downloadExport(fmt: 'csv' | 'json') {
  exportBusy.value = true
  try {
    const res = await http.get(`/api/user/usage/export?format=${fmt}`, { responseType: 'blob' })
    const blob = new Blob([res.data], { type: fmt === 'csv' ? 'text/csv' : 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = fmt === 'csv' ? 'usage-export.csv' : 'usage-export.json'
    a.click()
    URL.revokeObjectURL(url)
  } finally {
    exportBusy.value = false
  }
}

watch(period, () => {
  loadUsage().catch(() => {})
})

onMounted(loadAll)
</script>

<template>
  <div class="pb-8">
    <div class="flex flex-col gap-1 border-b border-white/[0.06] pb-6 md:flex-row md:items-end md:justify-between">
      <div>
        <p class="text-lg font-medium text-slate-200">{{ greeting }}，{{ auth.accountDisplay }}</p>
        <p class="mt-1 text-sm text-slate-500">{{ t('dashboard.welcomeSub') }}</p>
      </div>
      <div class="mt-4 flex flex-wrap items-center gap-2 md:mt-0">
        <span class="text-[10px] font-semibold uppercase tracking-widest text-slate-600">{{ t('dashboard.period') }}</span>
        <button
          v-for="p in ['today', '7d', '30d', 'all'] as const"
          :key="p"
          type="button"
          class="th-pill"
          :class="{ 'th-pill-active': period === p }"
          @click="period = p"
        >
          {{ p === 'today' ? t('dashboard.pToday') : p === '7d' ? t('dashboard.p7d') : p === '30d' ? t('dashboard.p30d') : t('dashboard.pAll') }}
        </button>
        <button type="button" :disabled="exportBusy" class="th-btn-secondary !py-1.5 !text-xs disabled:opacity-40" @click="downloadExport('csv')">{{ t('dashboard.exportCsv') }}</button>
        <button type="button" :disabled="exportBusy" class="th-btn-secondary !py-1.5 !text-xs disabled:opacity-40" @click="downloadExport('json')">{{ t('dashboard.exportJson') }}</button>
      </div>
    </div>
    <p v-if="err" class="mt-4 rounded-lg border border-red-500/20 bg-red-950/30 px-3 py-2 text-sm text-red-300">{{ err }}</p>

    <!-- 摘要指标卡 -->
    <div class="mt-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div class="th-glass relative overflow-hidden rounded-[var(--th-radius-lg)] p-5">
        <div
          class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-500/15 text-cyan-300/95 ring-1 ring-cyan-400/25"
          aria-hidden="true"
        >
          <!-- 银行卡轮廓：纯描边 + 磁条，避免无效 path 被整块填充成「豆腐块」 -->
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.85" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="5" width="20" height="14" rx="2" />
            <line x1="2" y1="10" x2="22" y2="10" />
            <rect x="5" y="14" width="7" height="3.5" rx="0.75" fill="currentColor" fill-opacity="0.35" stroke="none" />
          </svg>
        </div>
        <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.cardAccount') }}</p>
        <p class="mt-3 text-3xl font-semibold tracking-tight text-white md:text-4xl">{{ balance?.balance ?? '—' }}</p>
        <p class="mt-1 text-xs text-slate-500">{{ t('dashboard.cardYuanEquity', { yuan: fmtNum(balance?.balance_yuan, 2) }) }}</p>
        <div class="mt-4 flex flex-wrap items-center gap-2">
          <RouterLink to="/console/billing" class="th-btn-primary-flat !inline-flex !px-3 !py-1.5 !text-xs no-underline">{{ t('dashboard.recharge') }}</RouterLink>
          <span class="text-xs text-slate-500">{{ t('dashboard.lifetimeSpend') }} ¥{{ fmtNum(usage?.lifetime?.total_cost, 2) }}</span>
        </div>
      </div>
      <div class="th-glass relative overflow-hidden rounded-[var(--th-radius-lg)] p-5">
        <div
          class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500/15 text-emerald-300/95 ring-1 ring-emerald-400/25"
          aria-hidden="true"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
            <path d="M4 19h4v-8H4v8zm6 0h4V5h-4v14zm6 0h4v-5h-4v5z" />
          </svg>
        </div>
        <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.cardUsage') }}</p>
        <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.req') }}</p>
            <p class="text-2xl font-semibold text-white">{{ usage?.total_requests ?? '—' }}</p>
          </div>
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.uniqueModels') }}</p>
            <p class="text-2xl font-semibold text-white">{{ usage?.unique_models ?? '—' }}</p>
          </div>
        </div>
      </div>
      <div class="th-glass relative overflow-hidden rounded-[var(--th-radius-lg)] p-5">
        <div
          class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-xl bg-fuchsia-500/15 text-fuchsia-300/95 ring-1 ring-fuchsia-400/25"
          aria-hidden="true"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
            <path d="M13 2 3 14h8l-1 8 10-12h-8l1-8z" />
          </svg>
        </div>
        <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.cardResource') }}</p>
        <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.periodSpend') }}</p>
            <p class="text-2xl font-semibold text-amber-200/95">¥{{ fmtNum(usage?.total_cost, 2) }}</p>
          </div>
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.tok') }}</p>
            <p class="text-2xl font-semibold text-white">{{ usage?.total_tokens ?? '—' }}</p>
          </div>
        </div>
      </div>
      <div class="th-glass relative overflow-hidden rounded-[var(--th-radius-lg)] p-5">
        <div
          class="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-xl bg-violet-500/15 text-violet-300/95 ring-1 ring-violet-400/25"
          aria-hidden="true"
        >
          <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
          </svg>
        </div>
        <p class="text-[11px] font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.cardPerf') }}</p>
        <div class="mt-3 grid grid-cols-2 gap-3 text-sm">
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.avgRpm') }}</p>
            <p class="text-xl font-semibold text-white">{{ usage?.avg_rpm ?? '—' }}</p>
          </div>
          <div>
            <p class="text-xs text-slate-500">{{ t('dashboard.avgTpm') }}</p>
            <p class="text-xl font-semibold text-white">{{ usage?.avg_tpm ?? '—' }}</p>
          </div>
        </div>
        <p class="mt-2 text-[10px] leading-snug text-slate-600">{{ t('dashboard.perfHint') }}</p>
      </div>
    </div>

    <div class="mt-8 grid gap-6 xl:grid-cols-[1fr_320px] xl:items-stretch">
      <!-- 模型数据分析 -->
      <div class="th-glass flex min-h-0 min-w-0 flex-col rounded-[var(--th-radius-lg)] p-5 md:p-6 xl:h-full">
        <div class="flex flex-col gap-3 border-b border-white/[0.06] pb-4 sm:flex-row sm:items-center sm:justify-between">
          <h2 class="text-sm font-semibold text-slate-100">{{ t('dashboard.modelAnalysis') }}</h2>
          <div class="flex flex-wrap gap-1">
            <button v-for="tab in (['cost_trend', 'cost_dist', 'req_dist', 'req_rank'] as const)" :key="tab" type="button" class="rounded-lg px-2.5 py-1 text-xs transition" :class="chartTab === tab ? 'bg-cyan-500/20 text-cyan-100 ring-1 ring-cyan-500/30' : 'text-slate-500 hover:bg-white/[0.05]'" @click="chartTab = tab">
              {{
                tab === 'cost_trend'
                  ? t('dashboard.tabCostTrend')
                  : tab === 'cost_dist'
                    ? t('dashboard.tabCostDist')
                    : tab === 'req_dist'
                      ? t('dashboard.tabReqDist')
                      : t('dashboard.tabReqRank')
              }}
            </button>
          </div>
        </div>
        <div class="mt-5 min-h-0 flex-1">
          <div v-if="chartTab === 'cost_trend'" class="relative space-y-2">
            <p class="text-xs text-slate-500">{{ trendSeries.kind === 'hour' ? t('dashboard.trendHourly') : t('dashboard.trendDaily') }}</p>
            <div v-if="trendSeries.rows.length === 0" class="flex h-40 items-center justify-center text-sm text-slate-600">{{ t('dashboard.noChart') }}</div>
            <template v-else>
              <svg
                ref="trendSvgRef"
                class="w-full max-w-full cursor-crosshair"
                :viewBox="`0 0 ${CHART_W} ${CHART_H}`"
                preserveAspectRatio="none"
                @mousemove="onTrendSvgMove"
                @mouseleave="onTrendSvgLeave"
              >
                <!-- 命中层在下、视觉在上，避免透明 rect 盖住折线与圆点 -->
                <rect x="0" y="0" :width="CHART_W" :height="CHART_H" fill="transparent" pointer-events="all" />
                <polyline fill="none" stroke="rgb(34,211,238)" stroke-width="2" vector-effect="non-scaling-stroke" pointer-events="none" :points="trendGeometry.polyline" />
                <template v-if="trendHover !== null && trendGeometry.points[trendHover]">
                  <line
                    :x1="trendGeometry.points[trendHover].x"
                    y1="0"
                    :x2="trendGeometry.points[trendHover].x"
                    :y2="CHART_H"
                    stroke="rgba(148,163,184,0.45)"
                    stroke-width="1"
                    vector-effect="non-scaling-stroke"
                    stroke-dasharray="4 3"
                    pointer-events="none"
                  />
                  <circle :cx="trendGeometry.points[trendHover].x" :cy="trendGeometry.points[trendHover].y" r="5" fill="rgb(34,211,238)" stroke="rgb(15,23,42)" stroke-width="2" pointer-events="none" />
                </template>
              </svg>
              <Teleport to="body">
                <div
                  v-show="trendHover !== null"
                  class="pointer-events-none fixed z-[10000] max-w-[min(240px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[11px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-cyan-500/20"
                  :style="{ left: trendTip.x + 'px', top: trendTip.y + 'px' }"
                >
                  {{ trendHover !== null ? trendHoverText(trendHover) : '' }}
                </div>
              </Teleport>
            </template>
            <div v-if="trendSeries.rows.length" class="flex min-w-0 w-full text-[10px] tabular-nums leading-none text-slate-600">
              <span
                v-for="(r, i) in trendSeries.rows"
                :key="i"
                class="min-w-0 flex-1 truncate text-center"
                :title="trendAxisTickIndices.has(i) ? trendRowLabel(r) : undefined"
              >
                {{ trendAxisTickIndices.has(i) ? trendAxisShortLabel(r, trendSeries.kind) : '' }}
              </span>
            </div>
          </div>
          <div v-else-if="chartTab === 'cost_dist'" class="space-y-3">
            <p class="text-xs text-slate-500">{{ t('dashboard.distCostHint') }}</p>
            <div v-if="!modelCostShare.length" class="py-12 text-center text-sm text-slate-600">{{ t('dashboard.noModel') }}</div>
            <div v-else class="space-y-2">
              <div
                v-for="m in modelCostShare"
                :key="m.model"
                class="relative rounded-lg border border-white/[0.05] bg-slate-950/40 px-3 py-2"
                @mouseenter="onCostDistEnter(m, $event)"
                @mousemove="onCostDistMove($event)"
                @mouseleave="onCostDistLeave"
              >
                <div class="flex justify-between gap-2 text-xs">
                  <span class="truncate font-mono text-cyan-200/85">{{ m.model }}</span>
                  <span class="shrink-0 text-slate-400">{{ m.pct }}% · ¥{{ fmtNum(m.cost, 2) }}</span>
                </div>
                <div class="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/[0.06]">
                  <div class="h-full rounded-full bg-gradient-to-r from-cyan-600 to-teal-400 transition-all" :style="{ width: Math.min(100, m.pct) + '%' }" />
                </div>
              </div>
            </div>
          </div>
          <div v-else-if="chartTab === 'req_dist'" class="space-y-3">
            <p class="text-xs text-slate-500">{{ t('dashboard.distReqHint') }}</p>
            <div v-if="!modelByRequests.length" class="py-12 text-center text-sm text-slate-600">{{ t('dashboard.noModel') }}</div>
            <div v-else class="space-y-2">
              <div
                v-for="m in modelByRequests.slice(0, 12)"
                :key="'rq-' + m.model"
                class="relative flex cursor-default items-center gap-2 text-xs"
                @mouseenter="onReqDistEnter(m, $event)"
                @mousemove="onReqDistMove($event)"
                @mouseleave="onReqDistLeave"
              >
                <span class="w-28 truncate font-mono text-slate-400">{{ m.model }}</span>
                <div class="relative h-2 min-w-0 flex-1 overflow-visible rounded-full bg-white/[0.06]">
                  <div class="h-full rounded-full bg-emerald-500/70" :style="{ width: (100 * (m.requests || 0)) / maxModelRequests + '%' }" />
                </div>
                <span class="w-8 shrink-0 text-right text-slate-500">{{ m.requests }}</span>
              </div>
            </div>
          </div>
          <div v-else class="thin-scrollbar max-h-72 overflow-y-auto">
            <p class="mb-2 text-xs text-slate-500">{{ t('dashboard.rankReqHint') }}</p>
            <div v-if="!modelByRequests.length" class="py-12 text-center text-sm text-slate-600">{{ t('dashboard.noModel') }}</div>
            <div v-for="(m, i) in modelByRequests" :key="'rk-' + m.model" class="flex items-center justify-between gap-2 border-b border-white/[0.04] py-2 text-sm last:border-0" :title="t('dashboard.rankRowTip', { model: m.model, req: String(m.requests ?? 0), cost: fmtNum(m.cost, 2) })">
              <span class="text-slate-500">{{ i + 1 }}</span>
              <span class="min-w-0 flex-1 truncate font-mono text-xs text-cyan-100/90">{{ m.model }}</span>
              <span class="shrink-0 text-slate-400">{{ m.requests }} {{ t('dashboard.times') }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 侧栏：API 占满余高，服务可用性随内容（紧凑） -->
      <div class="flex min-h-0 min-w-0 flex-col gap-4 xl:h-full">
        <div class="th-glass flex min-h-0 min-w-0 flex-1 flex-col rounded-[var(--th-radius-lg)] p-5">
          <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.apiInfo') }}</h3>
          <p class="mt-1 text-xs text-slate-600">{{ t('dashboard.apiHint') }}</p>
          <div class="mt-4 flex min-h-0 flex-1 flex-col gap-3">
            <div class="th-dash-api-callout shrink-0 p-3">
              <p class="text-[10px] font-medium uppercase tracking-wider text-slate-500">{{ t('dashboard.apiBase') }}</p>
              <code class="th-dash-code-mono mt-1 block break-all text-xs">{{ apiBase }}</code>
              <div class="mt-2 flex gap-2">
                <button type="button" class="th-btn-secondary !px-2 !py-1 !text-[11px]" @click="copyText(apiBase)">{{ t('dashboard.copy') }}</button>
                <a :href="apiBase.replace(/\/v1$/, '') + '/docs'" class="th-btn-secondary !inline-flex !items-center !px-2 !py-1 !text-[11px] no-underline">{{ t('dashboard.openDocs') }}</a>
              </div>
            </div>
            <p class="mt-auto text-[11px] leading-relaxed text-slate-500">
              {{ t('dashboard.authHint') }}
              <code class="th-dash-code-inline px-1 py-0.5 text-[10px]">Authorization: Bearer &lt;API Key&gt;</code>
            </p>
          </div>
        </div>
        <div class="th-glass shrink-0 rounded-[var(--th-radius-lg)] px-5 pb-4 pt-4">
          <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.svcHealth') }}</h3>
          <div class="mt-2 flex items-center gap-2">
            <span class="h-2 w-2 rounded-full" :class="readyOk === true ? 'bg-emerald-400 shadow-[0_0_8px_rgba(52,211,153,0.6)]' : readyOk === false ? 'bg-rose-400' : 'bg-slate-500'" />
            <span class="text-sm text-slate-200">{{ readyOk === true ? t('dashboard.healthOk') : readyOk === false ? t('dashboard.healthBad') : t('dashboard.healthCheck') }}</span>
          </div>
          <p class="mt-1.5 text-xs leading-snug text-slate-600">{{ t('dashboard.healthDesc') }}</p>
        </div>
      </div>
    </div>

    <!-- 每日消费柱状（保留原有能力，布局优化） -->
    <div class="mt-6 grid gap-5 lg:grid-cols-2">
      <div class="th-glass p-5 md:p-6">
        <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.dailyTitle') }}</h3>
        <div v-if="sortedDaily.length > 0" class="relative mt-6 flex h-44 items-end gap-1 border-b border-white/[0.08] pb-2" @mouseleave="onDailyBarLeave">
          <div
            v-for="(b, i) in sortedDaily"
            :key="i"
            class="group/dcol relative flex min-w-0 flex-1 cursor-default flex-col items-center justify-end"
            @mouseenter="onDailyBarEnter(i, $event)"
            @mousemove="onDailyBarMove($event)"
          >
            <div
              class="w-full max-w-[14px] rounded-t-md bg-gradient-to-t from-cyan-600/90 to-cyan-400/70 shadow-sm shadow-cyan-900/40 transition group-hover/dcol:from-cyan-500 group-hover/dcol:to-teal-300/80"
              :style="{ height: Math.max(4, ((b.cost || 0) / maxDailyCost) * 100) + '%', minHeight: '4px' }"
            />
            <span class="mt-2 truncate text-[10px] text-slate-600">{{ b.date.slice(5) }}</span>
          </div>
        </div>
        <p v-else class="py-12 text-center text-sm text-slate-600">{{ t('dashboard.noDaily') }}</p>
      </div>
      <div class="th-glass p-5 md:p-6">
        <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.modelTop') }}</h3>
        <div class="thin-scrollbar mt-4 max-h-52 space-y-2 overflow-y-auto text-sm">
          <div
            v-for="(m, i) in (usage?.model_breakdown || [])"
            :key="m.model"
            class="flex items-center justify-between gap-2 rounded-lg border border-white/[0.05] bg-slate-950/50 px-3 py-2 transition hover:border-cyan-500/15"
            :title="t('dashboard.rankRowTip', { model: m.model, req: String(m.requests ?? 0), cost: fmtNum(m.cost, 2) })"
          >
            <span class="truncate font-mono text-xs text-cyan-200/90">{{ i + 1 }}. {{ m.model }}</span>
            <span class="shrink-0 text-xs text-slate-500">{{ m.requests }} {{ t('dashboard.times') }}</span>
            <span class="shrink-0 text-sm font-medium text-amber-300/90">¥{{ typeof m.cost === 'number' ? m.cost.toFixed(2) : m.cost }}</span>
          </div>
          <p v-if="!(usage?.model_breakdown?.length)" class="py-10 text-center text-sm text-slate-600">{{ t('dashboard.noModel') }}</p>
        </div>
      </div>
    </div>

    <!-- 公告 + FAQ + 快捷/API（末段同栅格收束，避免与底色融在一起「像没了」） -->
    <div class="mt-8 grid gap-5 lg:grid-cols-2">
      <div class="th-glass p-5 md:p-6">
        <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.announceTitle') }}</h3>
        <ul v-if="announcements.length" class="mt-4 space-y-3">
          <li v-for="a in announcements" :key="a.id" class="border-b border-white/[0.06] pb-3 last:border-0 last:pb-0">
            <p class="text-sm font-medium text-slate-100">{{ a.title }}</p>
            <p class="mt-0.5 text-[11px] text-slate-600">{{ shortDate(a.published_at) }}</p>
            <p v-if="a.content" class="mt-1 line-clamp-2 text-xs leading-relaxed text-slate-500">{{ a.content }}</p>
          </li>
        </ul>
        <p v-else class="mt-6 text-center text-sm text-slate-600">{{ t('dashboard.noAnnounce') }}</p>
      </div>
      <div class="th-glass p-5 md:p-6">
        <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.faqTitle') }}</h3>
        <div class="mt-4 space-y-1">
          <div v-for="(item, idx) in faqItems" :key="idx" class="rounded-lg border border-white/[0.05] bg-slate-950/30">
            <button type="button" class="flex w-full items-center justify-between gap-2 px-3 py-2.5 text-left text-sm text-slate-200 hover:bg-white/[0.04]" @click="openFaq = openFaq === idx ? null : idx">
              <span>{{ item.q }}</span>
              <span class="shrink-0 text-slate-500">{{ openFaq === idx ? '−' : '+' }}</span>
            </button>
            <p v-show="openFaq === idx" class="border-t border-white/[0.05] px-3 pb-3 pt-2 text-xs leading-relaxed text-slate-500">{{ item.a }}</p>
          </div>
        </div>
        <ContactCardDisclosure align="start" compact class="mt-5 border-t border-white/[0.06] pt-4" summary-key="home.contactHintDashboardFaq" />
      </div>

      <section class="th-glass shrink-0 p-5 md:p-6 lg:col-span-2">
        <div class="border-b border-white/[0.06] pb-5 md:pb-6">
          <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.tailQuickTitle') }}</h3>
          <div class="mt-3 flex flex-wrap gap-2">
            <RouterLink
              v-for="q in quickLinks"
              :key="q.to"
              :to="q.to"
              :title="q.desc"
              class="group inline-flex max-w-full items-center gap-1.5 rounded-lg border border-white/[0.08] bg-white/[0.04] px-3 py-2 text-xs font-medium text-slate-200 no-underline outline-none ring-cyan-500/0 transition hover:border-cyan-500/30 hover:bg-cyan-950/30 hover:text-cyan-100/95 focus-visible:ring-2"
            >
              <span class="truncate">{{ q.title }}</span>
              <span class="hidden max-w-[10rem] truncate text-[10px] font-normal text-slate-500 group-hover:text-slate-400 sm:inline">{{ q.desc }}</span>
            </RouterLink>
          </div>
        </div>
        <div class="pt-5 md:pt-6">
          <h3 class="text-xs font-semibold uppercase tracking-widest text-slate-500">{{ t('dashboard.tailApiTitle') }}</h3>
          <div class="th-dash-http-hint mt-3 px-3 py-3 text-xs leading-relaxed md:px-4 md:py-3.5 md:text-[13px]">
            {{ t('dashboard.footBefore') }}
            <code class="th-dash-code-inline mx-0.5 inline-block max-w-full break-all px-1.5 py-0.5 align-middle text-[11px] md:text-xs">Authorization: Bearer &lt;API Key&gt;</code>
            {{ t('dashboard.footMid') }}
            <code class="th-dash-code-inline mx-0.5 inline-block px-1.5 py-0.5 align-middle text-[11px] md:text-xs">/v1/chat/completions</code>
          </div>
        </div>
      </section>
    </div>

    <footer class="mt-8 flex shrink-0 flex-col items-center gap-4 border-t border-white/[0.06] pt-8 pb-1 text-[11px] text-slate-600 sm:flex-row sm:justify-between sm:gap-0 sm:pb-2">
      <p class="order-2 text-center leading-relaxed sm:order-1 sm:max-w-[60%] sm:text-left">
        {{ t('dashboard.footerNote', { year: footerYear }) }}
      </p>
      <nav class="order-1 flex flex-wrap justify-center gap-x-6 gap-y-2 sm:order-2 sm:justify-end" aria-label="Footer">
        <RouterLink to="/docs" class="text-slate-500 no-underline transition hover:text-cyan-300/90">{{ t('nav.docs') }}</RouterLink>
        <RouterLink to="/" class="text-slate-500 no-underline transition hover:text-cyan-300/90">{{ t('nav.siteHome') }}</RouterLink>
      </nav>
    </footer>

    <Teleport to="body">
      <div
        v-show="dailyBarHover !== null"
        class="pointer-events-none fixed z-[10000] max-w-[min(220px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[10px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-cyan-500/15"
        :style="{ left: dailyTip.x + 'px', top: dailyTip.y + 'px' }"
      >
        {{ dailyBarHover !== null ? dailyBarTipText(dailyBarHover) : '' }}
      </div>
    </Teleport>
    <Teleport to="body">
      <div
        v-show="costDistHoverModel !== null"
        class="pointer-events-none fixed z-[10000] max-w-[min(280px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[10px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-cyan-500/15"
        :style="{ left: costDistTip.x + 'px', top: costDistTip.y + 'px' }"
      >
        {{ costDistHoverText() }}
      </div>
    </Teleport>
    <Teleport to="body">
      <div
        v-show="reqDistHoverModel !== null"
        class="pointer-events-none fixed z-[10000] max-w-[min(260px,calc(100vw-24px))] whitespace-pre-line rounded-lg border border-white/15 bg-slate-950/98 px-2.5 py-2 text-[10px] leading-relaxed text-slate-100 shadow-2xl ring-1 ring-emerald-500/15"
        :style="{ left: reqDistTip.x + 'px', top: reqDistTip.y + 'px' }"
      >
        {{ reqDistHoverText() }}
      </div>
    </Teleport>
  </div>
</template>
