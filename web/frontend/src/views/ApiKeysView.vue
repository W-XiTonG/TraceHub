<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type KeyRow = { id: number; name?: string; key_prefix?: string; status: string }

const items = ref<KeyRow[]>([])
const name = ref('')
const err = ref('')
const createdKey = ref('')
const loading = ref(false)

async function load() {
  const { data } = await http.get<Envelope<{ items: KeyRow[] }>>('/api/user/apikeys')
  if (data.code === 0) items.value = data.data.items
}

async function createKey() {
  err.value = ''
  createdKey.value = ''
  loading.value = true
  try {
    const { data } = await http.post<Envelope<{ key: string }>>('/api/user/apikeys', { name: name.value })
    if (data.code !== 0) throw new Error(data.message || '创建失败')
    createdKey.value = data.data.key
    name.value = ''
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '创建失败'
  } finally {
    loading.value = false
  }
}

async function remove(id: number) {
  if (!confirm('确定删除？')) return
  await http.delete(`/api/user/apikeys/${id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h1 class="mb-6 text-2xl font-bold">API Key</h1>
    <div class="mb-6 flex flex-wrap gap-2">
      <input v-model="name" placeholder="备注名称" class="rounded-lg border border-slate-700 bg-slate-900 px-3 py-2 text-sm" />
      <button type="button" :disabled="loading" class="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50" @click="createKey">
        新建 Key
      </button>
    </div>
    <p v-if="err" class="mb-4 text-sm text-red-400">{{ err }}</p>
    <div v-if="createdKey" class="mb-6 rounded-xl border border-amber-600/50 bg-amber-950/40 p-4 text-sm">
      <p class="font-medium text-amber-400">请立即保存，仅显示一次：</p>
      <code class="mt-2 block break-all text-cyan-300">{{ createdKey }}</code>
    </div>
    <table class="w-full text-left text-sm">
      <thead class="text-slate-500">
        <tr>
          <th class="pb-2">名称</th>
          <th class="pb-2">前缀</th>
          <th class="pb-2">状态</th>
          <th></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-800">
        <tr v-for="k in items" :key="k.id">
          <td class="py-2">{{ k.name || '—' }}</td>
          <td class="py-2 font-mono text-xs">{{ k.key_prefix }}</td>
          <td class="py-2">{{ k.status }}</td>
          <td class="py-2 text-right">
            <button type="button" class="text-red-400 hover:underline" @click="remove(k.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
