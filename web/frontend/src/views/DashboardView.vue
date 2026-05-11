<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { http, type Envelope } from '../api/http'
import { useAuthStore } from '../stores/auth'

type Balance = {
  balance?: number
  balance_yuan?: number
  group_name?: string
  rate_multiplier?: number
  currency_unit?: number
}

type DayRow = { date: string; requests: number; cost: number }
type ModelRow = { model: string; requests: number; cost: number }

const balance = ref<Balance | null>(null)
const period = ref<'today' | '7d' | '30d' | 'all'>('7d')
const usage = ref<{
  period?: string
  total_requests?: number
  total_tokens?: number
  total_cost?: number
  daily_breakdown?: DayRow[]
  model_breakdown?: ModelRow[]
} | null>(null)
const err = ref('')
const exportBusy = ref(false)
const auth = useAuthStore()

const showAdminQuick = computed(() => ['admin', 'superadmin'].includes(auth.role))

const quickLinks = [
  { to: '/console/models', title: '可用模型', desc: '查看当前可用的 model 标识' },
  { to: '/console/keys', title: 'API Key', desc: '创建、命名与轮换密钥' },
  { to: '/console/logs', title: '调用日志', desc: '检索近期请求与状态' },
  { to: '/console/group', title: '我的分组', desc: '倍率与分组名称' },
  { to: '/console/billing', title: '充值中心', desc: '余额与充值入口' },
  { to: '/console/redeem', title: '兑换码', desc: '核销活动或批次码' },
  { to: '/console/settings', title: '账户设置', desc: '密码与基本资料' },
] as const

const dailyBars = computed(() => {
  const rows = usage.value?.daily_breakdown || []
  if (!rows.length) return []
  const maxCost = Math.max(...rows.map((r) => r.cost), 0.0001)
  return rows.map((r) => ({
    ...r,
    h: Math.round((r.cost / maxCost) * 100),
  }))
})

const modelRows = computed(() => usage.value?.model_breakdown || [])

async function loadBalance() {
  const { data } = await http.get<Envelope<Balance>>('/api/user/balance')
  if (data.code === 0) balance.value = data.data
}

async function loadUsage() {
  const { data } = await http.get<Envelope<typeof usage.value>>(`/api/user/usage?period=${period.value}`)
  if (data.code === 0) usage.value = data.data
}

async function loadAll() {
  err.value = ''
  try {
    await Promise.all([loadBalance(), loadUsage()])
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '加载失败'
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
  <div>
    <p class="th-kicker mb-2"><span class="th-kicker-accent">Overview</span></p>
    <h1 class="text-3xl font-semibold tracking-tight text-slate-100">仪表盘</h1>
    <p class="mt-2 max-w-2xl text-sm leading-relaxed text-slate-500">余额与用量概览；切换统计周期查看趋势与模型分布。</p>
    <p v-if="err" class="mt-4 rounded-lg border border-red-500/20 bg-red-950/30 px-3 py-2 text-sm text-red-300">{{ err }}</p>

    <div class="mt-8 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <RouterLink
        v-for="q in quickLinks"
        :key="q.to"
        :to="q.to"
        class="group th-glass flex flex-col rounded-[var(--th-radius-lg)] p-4 no-underline outline-none ring-cyan-500/30 transition hover:border-cyan-500/20 hover:bg-white/[0.03] focus-visible:ring-2"
      >
        <span class="text-sm font-semibold tracking-tight text-slate-200 group-hover:text-white">{{ q.title }}</span>
        <span class="mt-1 text-xs leading-relaxed text-slate-500">{{ q.desc }}</span>
      </RouterLink>
      <RouterLink
        v-if="showAdminQuick"
        to="/admin"
        class="group th-glass flex flex-col rounded-[var(--th-radius-lg)] border border-fuchsia-500/15 bg-fuchsia-950/10 p-4 no-underline outline-none ring-fuchsia-500/25 transition hover:border-fuchsia-500/30 hover:bg-fuchsia-950/20 focus-visible:ring-2"
      >
        <span class="text-sm font-semibold tracking-tight text-fuchsia-100/95 group-hover:text-white">管理后台</span>
        <span class="mt-1 text-xs leading-relaxed text-fuchsia-200/50">用户、渠道、分组与审计</span>
      </RouterLink>
    </div>

    <div class="mt-6 flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
      <div class="flex flex-wrap items-center gap-2">
        <span class="mr-1 text-xs font-medium uppercase tracking-widest text-slate-600">周期</span>
        <button
          v-for="p in ['today', '7d', '30d', 'all'] as const"
          :key="p"
          type="button"
          class="th-pill"
          :class="{ 'th-pill-active': period === p }"
          @click="period = p"
        >
          {{ p === 'today' ? '今日' : p === '7d' ? '近 7 天' : p === '30d' ? '近 30 天' : '全部' }}
        </button>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" :disabled="exportBusy" class="th-btn-secondary !py-2 text-sm disabled:opacity-40" @click="downloadExport('csv')">导出 CSV</button>
        <button type="button" :disabled="exportBusy" class="th-btn-secondary !py-2 text-sm disabled:opacity-40" @click="downloadExport('json')">导出 JSON</button>
      </div>
    </div>

    <div class="mt-8 grid gap-5 lg:grid-cols-2">
      <div class="th-glass p-6 md:p-8">
        <h2 class="text-xs font-medium uppercase tracking-widest text-slate-500">余额（额度）</h2>
        <p class="mt-3 bg-gradient-to-r from-cyan-300 to-teal-400 bg-clip-text text-4xl font-semibold tracking-tight text-transparent md:text-5xl">
          {{ balance?.balance ?? '—' }}
        </p>
        <p class="mt-3 text-xs leading-relaxed text-slate-500">
          约合 ¥ {{ balance?.balance_yuan?.toFixed?.(2) ?? balance?.balance_yuan ?? '—' }} · 分组
          <span class="text-slate-400">{{ balance?.group_name ?? '—' }}</span> · 倍率
          <span class="font-mono text-cyan-500/90">{{ balance?.rate_multiplier ?? '—' }}×</span>
        </p>
      </div>
      <div class="th-glass p-6 md:p-8">
        <h2 class="text-xs font-medium uppercase tracking-widest text-slate-500">用量摘要（{{ usage?.period ?? period }}）</h2>
        <div class="mt-4 space-y-2">
          <p class="text-base text-slate-400">请求 <span class="font-semibold text-white">{{ usage?.total_requests ?? '—' }}</span></p>
          <p class="text-base text-slate-400">Token <span class="font-semibold text-white">{{ usage?.total_tokens ?? '—' }}</span></p>
          <p class="text-lg font-medium text-amber-300/95">
            消费约 ¥
            {{
              usage?.total_cost == null
                ? '—'
                : typeof usage.total_cost === 'number'
                  ? usage.total_cost.toFixed(2)
                  : usage.total_cost
            }}
          </p>
        </div>
      </div>
    </div>

    <div class="mt-8 grid gap-5 lg:grid-cols-2">
      <div class="th-glass p-6">
        <h3 class="text-xs font-medium uppercase tracking-widest text-slate-500">每日消费（元）</h3>
        <div v-if="dailyBars.length" class="mt-6 flex h-44 items-end gap-1 border-b border-white/[0.08] pb-2">
          <div v-for="(b, i) in dailyBars" :key="i" class="group relative flex min-w-0 flex-1 flex-col items-center justify-end">
            <div
              class="w-full max-w-[16px] rounded-t-md bg-gradient-to-t from-cyan-600/90 to-cyan-400/70 shadow-sm shadow-cyan-900/40 transition group-hover:from-cyan-500 group-hover:to-teal-300/80"
              :style="{ height: b.h + '%', minHeight: '4px' }"
              :title="`${b.date} ¥${b.cost.toFixed(2)}`"
            />
            <span class="mt-2 truncate text-[10px] text-slate-600">{{ b.date.slice(5) }}</span>
          </div>
        </div>
        <p v-else class="py-12 text-center text-sm text-slate-600">该周期暂无按日数据</p>
      </div>
      <div class="th-glass p-6">
        <h3 class="text-xs font-medium uppercase tracking-widest text-slate-500">模型消费 TOP</h3>
        <div class="mt-4 max-h-52 space-y-2 overflow-y-auto text-sm">
          <div
            v-for="(m, i) in modelRows"
            :key="m.model"
            class="flex items-center justify-between gap-2 rounded-lg border border-white/[0.05] bg-slate-950/50 px-3 py-2 transition hover:border-cyan-500/15"
          >
            <span class="truncate font-mono text-xs text-cyan-200/90">{{ i + 1 }}. {{ m.model }}</span>
            <span class="shrink-0 text-xs text-slate-500">{{ m.requests }} 次</span>
            <span class="shrink-0 text-sm font-medium text-amber-300/90">¥{{ typeof m.cost === 'number' ? m.cost.toFixed(2) : m.cost }}</span>
          </div>
          <p v-if="!modelRows.length" class="py-10 text-center text-sm text-slate-600">暂无模型维度数据</p>
        </div>
      </div>
    </div>

    <div class="th-glass--subtle mt-10 p-4 text-sm text-slate-500">
      调用请携带
      <code class="mx-1 rounded-md border border-white/10 bg-black/30 px-2 py-0.5 font-mono text-xs text-cyan-200/90">Authorization: Bearer &lt;API Key&gt;</code>
      请求
      <code class="mx-1 rounded-md border border-white/10 bg-black/30 px-2 py-0.5 font-mono text-xs text-cyan-200/90">/v1/chat/completions</code>
    </div>
  </div>
</template>
