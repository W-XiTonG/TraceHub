<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type Log = {
  id: number
  model: string
  prompt_tokens: number
  completion_tokens: number
  total_cost: number
  status_code: number
  request_time: string
}

const items = ref<Log[]>([])

async function load() {
  const { data } = await http.get<Envelope<{ items: Log[] }>>('/api/admin/logs?page_size=30')
  if (data.code === 0) items.value = data.data.items as Log[]
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">调用日志</h1>
    <button type="button" class="mb-4 text-sm text-cyan-400 hover:underline" @click="load">刷新</button>
    <table class="w-full text-left text-xs">
      <thead class="text-slate-500">
        <tr>
          <th class="pb-2">时间</th>
          <th class="pb-2">模型</th>
          <th class="pb-2">Token</th>
          <th class="pb-2">费用</th>
          <th class="pb-2">状态</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-800">
        <tr v-for="l in items" :key="l.id">
          <td class="py-2 whitespace-nowrap">{{ l.request_time }}</td>
          <td class="py-2">{{ l.model }}</td>
          <td class="py-2">{{ l.prompt_tokens }} / {{ l.completion_tokens }}</td>
          <td class="py-2">{{ l.total_cost }}</td>
          <td class="py-2">{{ l.status_code }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
