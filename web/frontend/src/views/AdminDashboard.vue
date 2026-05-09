<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

const s = ref<Record<string, unknown> | null>(null)

onMounted(async () => {
  const { data } = await http.get<Envelope<Record<string, unknown>>>('/api/admin/dashboard/summary')
  if (data.code === 0) s.value = data.data
})
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">运营概览</h1>
    <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">用户总数</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.total_users ?? '—' }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">启用渠道</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.enabled_channels ?? '—' }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">今日请求</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.today_requests ?? '—' }}</p>
      </div>
      <div class="rounded-xl border border-slate-800 bg-slate-900 p-4">
        <p class="text-xs text-slate-500">今日 Token</p>
        <p class="mt-1 text-2xl font-semibold">{{ s?.today_tokens ?? '—' }}</p>
      </div>
    </div>
  </div>
</template>
