<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type GroupInfo = {
  id: number
  name: string
  description?: string
  rate_multiplier: number
  rpm_limit: number
  default_model?: string
  subscription_type?: string
}

type RateRow = {
  id: number
  model_name: string
  input_rate: number
  output_rate: number
  enabled: boolean
}

const group = ref<GroupInfo | null>(null)
const rates = ref<RateRow[]>([])
const err = ref('')

onMounted(async () => {
  try {
    const { data } = await http.get<Envelope<{ group: GroupInfo | null; model_rates: RateRow[] }>>('/api/user/group')
    if (data.code !== 0) throw new Error(data.message || '加载失败')
    group.value = data.data.group
    rates.value = data.data.model_rates || []
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '加载失败'
  }
})
</script>

<template>
  <div>
    <h1 class="mb-2 text-2xl font-bold">我的分组</h1>
    <p class="mb-6 text-sm text-slate-500">当前账号所在分组及模型倍率（由管理员配置）。</p>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>

    <div v-if="group" class="mb-8 rounded-xl border border-slate-800 bg-slate-900/60 p-6">
      <h2 class="text-lg font-semibold text-cyan-400">{{ group.name }}</h2>
      <p v-if="group.description" class="mt-2 text-sm text-slate-400">{{ group.description }}</p>
      <dl class="mt-4 grid gap-2 text-sm sm:grid-cols-2">
        <div><span class="text-slate-500">分组倍率</span> <span class="ml-2 font-mono">{{ group.rate_multiplier }}×</span></div>
        <div><span class="text-slate-500">RPM 限制</span> <span class="ml-2 font-mono">{{ group.rpm_limit || '—' }}</span></div>
        <div v-if="group.default_model"><span class="text-slate-500">默认模型</span> <span class="ml-2 font-mono text-xs">{{ group.default_model }}</span></div>
        <div><span class="text-slate-500">计费类型</span> <span class="ml-2">{{ group.subscription_type || '—' }}</span></div>
      </dl>
    </div>
    <div v-else-if="!err" class="rounded-xl border border-slate-800 bg-slate-900/40 p-6 text-slate-500">未分配分组，请联系管理员。</div>

    <h3 class="mb-3 text-sm font-medium uppercase tracking-wide text-slate-500">模型倍率</h3>
    <div class="overflow-x-auto rounded-xl border border-slate-800">
      <table class="w-full text-left text-sm">
        <thead class="bg-slate-900 text-slate-500">
          <tr>
            <th class="px-3 py-2">模型</th>
            <th class="px-3 py-2">输入倍率</th>
            <th class="px-3 py-2">输出倍率</th>
            <th class="px-3 py-2">启用</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800">
          <tr v-for="r in rates" :key="r.id">
            <td class="px-3 py-2 font-mono text-xs">{{ r.model_name }}</td>
            <td class="px-3 py-2">{{ r.input_rate }}</td>
            <td class="px-3 py-2">{{ r.output_rate }}</td>
            <td class="px-3 py-2">{{ r.enabled ? '是' : '否' }}</td>
          </tr>
          <tr v-if="!rates.length">
            <td colspan="4" class="px-3 py-6 text-center text-slate-500">暂无单独模型倍率（使用分组默认倍率）</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
