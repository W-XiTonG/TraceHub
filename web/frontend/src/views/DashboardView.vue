<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { http, type Envelope } from '../api/http'

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
    <h1 class="mb-2 text-2xl font-bold">仪表盘</h1>
    <p class="mb-6 text-sm text-slate-500">余额与用量概览；周期与图表数据来自用量统计接口。</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div class="mb-6 flex flex-wrap gap-2">
      <RouterLink to="/console/keys" class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800">API Key</RouterLink>
      <RouterLink to="/console/logs" class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800">调用日志</RouterLink>
      <RouterLink to="/console/group" class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800">我的分组</RouterLink>
      <RouterLink to="/console/billing" class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800">充值</RouterLink>
      <button
        type="button"
        :disabled="exportBusy"
        class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800 disabled:opacity-50"
        @click="downloadExport('csv')"
      >
        导出 CSV
      </button>
      <button
        type="button"
        :disabled="exportBusy"
        class="rounded-lg border border-slate-700 px-3 py-1.5 text-sm hover:bg-slate-800 disabled:opacity-50"
        @click="downloadExport('json')"
      >
        导出 JSON
      </button>
    </div>

    <div class="mb-6 flex flex-wrap gap-2 text-sm">
      <span class="self-center text-slate-500">统计周期</span>
      <button
        v-for="p in ['today', '7d', '30d', 'all'] as const"
        :key="p"
        type="button"
        class="rounded-lg px-3 py-1.5 capitalize"
        :class="period === p ? 'bg-cyan-600 text-white' : 'border border-slate-700 text-slate-300 hover:bg-slate-800'"
        @click="period = p"
      >
        {{ p === 'today' ? '今日' : p === '7d' ? '近 7 天' : p === '30d' ? '近 30 天' : '全部' }}
      </button>
    </div>

    <div class="grid gap-4 lg:grid-cols-2">
      <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <h2 class="text-sm text-slate-500">余额（额度）</h2>
        <p class="mt-2 text-3xl font-semibold text-cyan-400">{{ balance?.balance ?? '—' }}</p>
        <p class="mt-1 text-xs text-slate-500">
          约合 ¥ {{ balance?.balance_yuan?.toFixed?.(2) ?? balance?.balance_yuan ?? '—' }} · 分组 {{ balance?.group_name ?? '—' }} · 分组倍率
          {{ balance?.rate_multiplier ?? '—' }}×
        </p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900/60 p-6">
        <h2 class="text-sm text-slate-500">用量摘要（{{ usage?.period ?? period }}）</h2>
        <p class="mt-2 text-lg">请求 <span class="font-semibold text-slate-200">{{ usage?.total_requests ?? '—' }}</span></p>
        <p class="text-lg">Token <span class="font-semibold text-slate-200">{{ usage?.total_tokens ?? '—' }}</span></p>
        <p class="text-lg text-amber-400/90">
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

    <div class="mt-8 grid gap-8 lg:grid-cols-2">
      <div class="rounded-xl border border-slate-800 bg-slate-900/40 p-5">
        <h3 class="mb-4 text-sm font-medium text-slate-400">每日消费（元）</h3>
        <div v-if="dailyBars.length" class="flex h-40 items-end gap-1 border-b border-slate-700 pb-1">
          <div
            v-for="(b, i) in dailyBars"
            :key="i"
            class="group relative flex min-w-0 flex-1 flex-col items-center justify-end"
          >
            <div
              class="w-full max-w-[14px] rounded-t bg-cyan-600/80 transition group-hover:bg-cyan-500"
              :style="{ height: b.h + '%', minHeight: '4px' }"
              :title="`${b.date} ¥${b.cost.toFixed(2)}`"
            />
            <span class="mt-1 truncate text-[10px] text-slate-600">{{ b.date.slice(5) }}</span>
          </div>
        </div>
        <p v-else class="py-8 text-center text-sm text-slate-600">该周期暂无按日数据</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900/40 p-5">
        <h3 class="mb-4 text-sm font-medium text-slate-400">模型消费 TOP</h3>
        <div class="max-h-48 space-y-2 overflow-y-auto text-sm">
          <div v-for="(m, i) in modelRows" :key="m.model" class="flex justify-between gap-2 rounded bg-slate-950/60 px-2 py-1.5">
            <span class="truncate font-mono text-xs text-cyan-200/90">{{ i + 1 }}. {{ m.model }}</span>
            <span class="shrink-0 text-slate-400">{{ m.requests }} 次</span>
            <span class="shrink-0 text-amber-400/90">¥{{ typeof m.cost === 'number' ? m.cost.toFixed(2) : m.cost }}</span>
          </div>
          <p v-if="!modelRows.length" class="py-6 text-center text-slate-600">暂无模型维度数据</p>
        </div>
      </div>
    </div>

    <p class="mt-10 text-sm text-slate-500">
      调用 AI 请使用 <code class="rounded bg-slate-800 px-1">Authorization: Bearer &lt;API Key&gt;</code> 访问
      <code class="rounded bg-slate-800 px-1">/v1/chat/completions</code>
    </p>
  </div>
</template>
