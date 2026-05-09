<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type Row = { id: number; email: string; balance: number; status: number; group_name?: string }

const items = ref<Row[]>([])
const deltaByUser = ref<Record<number, string>>({})

async function load() {
  const { data } = await http.get<Envelope<{ items: Row[] }>>('/api/admin/users')
  if (data.code === 0) items.value = data.data.items
}

async function adjust(id: number) {
  const raw = deltaByUser.value[id]
  const delta = parseInt(raw || '0', 10)
  if (!delta) return
  await http.post(`/api/admin/users/${id}/balance`, { delta })
  deltaByUser.value[id] = ''
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">用户管理</h1>
    <table class="w-full text-left text-sm">
      <thead class="text-slate-500">
        <tr>
          <th class="pb-2">ID</th>
          <th class="pb-2">邮箱</th>
          <th class="pb-2">分组</th>
          <th class="pb-2">余额</th>
          <th class="pb-2">调整额度</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-800">
        <tr v-for="u in items" :key="u.id">
          <td class="py-2">{{ u.id }}</td>
          <td class="py-2">{{ u.email }}</td>
          <td class="py-2">{{ u.group_name }}</td>
          <td class="py-2">{{ u.balance }}</td>
          <td class="py-2">
            <input v-model="deltaByUser[u.id]" type="number" placeholder="±额度" class="mr-2 w-28 rounded border border-slate-700 bg-slate-900 px-2 py-1 text-xs" />
            <button type="button" class="text-cyan-400 hover:underline" @click="adjust(u.id)">应用</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
