<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

const balance = ref<Record<string, unknown> | null>(null)
const usage = ref<Record<string, unknown> | null>(null)
const err = ref('')

onMounted(async () => {
  try {
    const [b, u] = await Promise.all([
      http.get<Envelope<Record<string, unknown>>>('/api/user/balance'),
      http.get<Envelope<Record<string, unknown>>>('/api/user/usage?period=7d'),
    ])
    if (b.data.code === 0) balance.value = b.data.data
    if (u.data.code === 0) usage.value = u.data.data
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '加载失败'
  }
})
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">仪表盘</h1>
    <p v-if="err" class="text-red-400">{{ err }}</p>
    <div class="grid gap-4 md:grid-cols-2">
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-6">
        <h2 class="text-sm text-slate-500">余额（额度）</h2>
        <p class="mt-2 text-3xl font-semibold text-cyan-400">{{ balance?.balance ?? '—' }}</p>
        <p class="mt-1 text-xs text-slate-500">约合 ¥ {{ balance?.balance_yuan ?? '—' }} · 分组 {{ balance?.group_name ?? '—' }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-6">
        <h2 class="text-sm text-slate-500">近 7 日用量</h2>
        <p class="mt-2 text-lg">请求 {{ usage?.total_requests ?? '—' }}</p>
        <p class="text-lg">Token {{ usage?.total_tokens ?? '—' }}</p>
        <p class="text-lg text-amber-400/90">消费约 ¥ {{ usage?.total_cost ?? '—' }}</p>
      </div>
    </div>
    <p class="mt-8 text-sm text-slate-500">
      调用 AI 时请使用 <code class="rounded bg-slate-800 px-1">Authorization: Bearer &lt;API Key&gt;</code> 访问
      <code class="rounded bg-slate-800 px-1">/v1/chat/completions</code>
    </p>
  </div>
</template>
