<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type ModelItem = {
  id: string
  object?: string
  created?: number
  owned_by?: string
  display_name?: string
  in_gateway?: boolean
  in_catalog?: boolean
}

type ModelsPayload = {
  items: ModelItem[]
  total: number
  group_id: number
  group_assigned: boolean
  hint?: string
}

const payload = ref<ModelsPayload | null>(null)
const err = ref('')
const q = ref('')

const filtered = computed(() => {
  const items = payload.value?.items ?? []
  const s = q.value.trim().toLowerCase()
  if (!s) return items
  return items.filter(
    (m) =>
      m.id.toLowerCase().includes(s) ||
      (m.owned_by ?? '').toLowerCase().includes(s) ||
      (m.display_name ?? '').toLowerCase().includes(s),
  )
})

async function load() {
  err.value = ''
  try {
    const { data } = await http.get<Envelope<ModelsPayload>>('/api/user/available-models')
    if (data.code !== 0) throw new Error(data.message || '加载失败')
    payload.value = data.data
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '加载失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <p class="th-kicker mb-2"><span class="th-kicker-accent">Models</span></p>
    <h1 class="text-3xl font-semibold tracking-tight text-slate-100">可用模型</h1>
    <p class="mt-2 max-w-2xl text-sm leading-relaxed text-slate-500">
      列表合并两类来源：<span class="text-slate-400">网关</span>（分组下启用且健康的渠道所配置的模型，与
      <code class="rounded-md border border-white/10 bg-black/30 px-1.5 py-0.5 font-mono text-xs text-sky-200/85">GET /v1/models</code>
      一致）与 <span class="text-slate-400">目录</span>（管理端模型市场中对用户可见的条目）。仅有「目录」时尚需在渠道中配置方可调用。
    </p>

    <p v-if="err" class="mt-4 rounded-lg border border-red-500/20 bg-red-950/30 px-3 py-2 text-sm text-red-300">{{ err }}</p>

    <div v-if="payload?.hint" class="mt-6 rounded-xl border border-amber-500/20 bg-amber-950/25 px-4 py-3 text-sm text-amber-100/95">
      {{ payload.hint }}
    </div>

    <template v-if="payload">
      <div class="mt-6 flex flex-wrap items-center gap-3">
        <input
          v-model="q"
          type="search"
          placeholder="按模型 ID、展示名或归属筛选…"
          class="th-input max-w-md flex-1 min-w-[200px]"
          aria-label="筛选模型"
        />
        <span class="text-xs text-slate-500">共 {{ payload.total }} 个 · 显示 {{ filtered.length }} 个</span>
      </div>

      <div class="th-glass mt-6 overflow-hidden">
        <div class="th-table-wrap overflow-x-auto">
          <table class="w-full text-left text-sm">
            <thead>
              <tr class="border-b border-white/[0.06] bg-slate-900/80 text-xs uppercase tracking-wider text-slate-500">
                <th class="px-4 py-3 font-medium">模型 ID</th>
                <th class="px-4 py-3 font-medium">展示名</th>
                <th class="px-4 py-3 font-medium">来源</th>
                <th class="px-4 py-3 font-medium">归属</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-white/[0.06]">
              <tr v-for="m in filtered" :key="m.id" class="transition hover:bg-white/[0.03]">
                <td class="px-4 py-3 font-mono text-sm text-cyan-100/95">{{ m.id }}</td>
                <td class="px-4 py-3 text-slate-300">{{ m.display_name || '—' }}</td>
                <td class="px-4 py-3">
                  <span v-if="m.in_gateway && m.in_catalog" class="text-xs text-emerald-400/90">网关 + 目录</span>
                  <span v-else-if="m.in_gateway" class="text-xs text-cyan-400/90">网关</span>
                  <span v-else class="text-xs text-slate-500">目录</span>
                </td>
                <td class="px-4 py-3 text-slate-400">{{ m.owned_by ?? '—' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="payload.total === 0" class="px-4 py-12 text-center text-sm text-slate-500">
          暂无数据：请确认已分配分组、渠道已启用且健康，或模型市场中存在对用户可见的条目。
        </p>
        <p v-else-if="filtered.length === 0" class="px-4 py-8 text-center text-sm text-slate-500">无匹配项，请调整筛选关键词。</p>
      </div>
    </template>
  </div>
</template>
