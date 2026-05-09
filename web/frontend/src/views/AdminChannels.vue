<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type Ch = {
  id: number
  group_id: number
  channel_type: string
  channel_name: string
  models: string[]
  enabled: boolean
}

type G = { id: number; name: string }

const channels = ref<Ch[]>([])
const groups = ref<G[]>([])
const form = reactive({
  group_id: 1,
  channel_type: 'openai',
  channel_name: '',
  api_key: '',
  api_base_url: '',
  modelsStr: 'gpt-4o,gpt-4o-mini',
})

async function load() {
  const [c, g] = await Promise.all([
    http.get<Envelope<{ items: Ch[] }>>('/api/admin/channels'),
    http.get<Envelope<{ items: G[] }>>('/api/admin/groups'),
  ])
  if (c.data.code === 0) channels.value = c.data.data.items
  if (g.data.code === 0) {
    groups.value = g.data.data.items as G[]
    if (groups.value.length && !form.group_id) form.group_id = groups.value[0].id
  }
}

async function createCh() {
  const models = form.modelsStr.split(',').map((s) => s.trim()).filter(Boolean)
  await http.post('/api/admin/channels', {
    group_id: form.group_id,
    channel_type: form.channel_type,
    channel_name: form.channel_name,
    auth_type: 'api_key',
    api_key: form.api_key,
    api_base_url: form.api_base_url,
    models,
    priority: 0,
    weight: 100,
    concurrency: 2,
    enabled: true,
  })
  form.api_key = ''
  await load()
}

async function remove(id: number) {
  if (!confirm('删除渠道？')) return
  await http.delete(`/api/admin/channels/${id}`)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="space-y-10">
    <div>
      <h1 class="mb-4 text-2xl font-bold">新增渠道</h1>
      <div class="grid max-w-xl gap-3 rounded-xl border border-slate-800 bg-slate-900 p-4 text-sm">
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">分组</span>
          <select v-model.number="form.group_id" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }} (#{{ g.id }})</option>
          </select>
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">类型</span>
          <select v-model="form.channel_type" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option value="openai">openai</option>
            <option value="deepseek">deepseek</option>
            <option value="claude">claude</option>
            <option value="gemini">gemini</option>
          </select>
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">名称</span>
          <input v-model="form.channel_name" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">API Key（加密存储）</span>
          <input v-model="form.api_key" type="password" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">Base URL（可空）</span>
          <input v-model="form.api_base_url" placeholder="默认官方地址" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">模型列表（逗号分隔）</span>
          <input v-model="form.modelsStr" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <button type="button" class="rounded-lg bg-amber-500 py-2 font-semibold text-slate-950 hover:bg-amber-400" @click="createCh">保存渠道</button>
      </div>
    </div>
    <div>
      <h2 class="mb-4 text-xl font-semibold">渠道列表</h2>
      <table class="w-full text-left text-sm">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">ID</th>
            <th class="pb-2">类型</th>
            <th class="pb-2">名称</th>
            <th class="pb-2">模型</th>
            <th></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800">
          <tr v-for="ch in channels" :key="ch.id">
            <td class="py-2">{{ ch.id }}</td>
            <td class="py-2">{{ ch.channel_type }}</td>
            <td class="py-2">{{ ch.channel_name }}</td>
            <td class="py-2 font-mono text-xs">{{ ch.models?.join(', ') }}</td>
            <td class="py-2 text-right">
              <button type="button" class="text-red-400 hover:underline" @click="remove(ch.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
