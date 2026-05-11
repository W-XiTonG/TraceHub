<script setup lang="ts">
import axios from 'axios'
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

const presetLoading = ref(false)
const fetchLoading = ref(false)
const presetHint = ref('')

function currentIds(): string[] {
  return form.modelsStr
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

function setIds(ids: string[]) {
  const merged = [...new Set([...currentIds(), ...ids])].sort()
  form.modelsStr = merged.join(',')
}

/** 内置推荐 + 模型目录（与当前渠道类型匹配） */
async function applyPresets(mode: 'merged' | 'built_in' | 'catalog') {
  presetHint.value = ''
  presetLoading.value = true
  try {
    const { data } = await http.get<Envelope<{ built_in: string[]; catalog: string[]; merged: string[] }>>(
      '/api/admin/channel-model-presets',
      { params: { channel_type: form.channel_type } },
    )
    if (data.code !== 0) throw new Error(data.message || '加载失败')
    const d = data.data
    let pick: string[] = []
    if (mode === 'merged') pick = d.merged
    else if (mode === 'built_in') pick = d.built_in
    else pick = d.catalog
    if (pick.length === 0) {
      presetHint.value = '当前无推荐项，可尝试「从上游拉取」或手动填写'
      return
    }
    setIds(pick)
    presetHint.value = `已合并 ${pick.length} 个模型 ID（与已有列表去重）`
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
  } finally {
    presetLoading.value = false
  }
}

function httpErrMessage(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) {
      return d.message.trim()
    }
  }
  return e instanceof Error ? e.message : '请求失败'
}

/** 使用下方 API Key / Base URL 调用上游 GET /v1/models */
async function fetchFromUpstream() {
  presetHint.value = ''
  if (!form.api_key.trim()) {
    presetHint.value = '请先在上方填写 API Key，再拉取上游模型列表'
    return
  }
  fetchLoading.value = true
  try {
    const { data } = await http.post<Envelope<{ model_ids: string[]; count: number }>>(
      '/api/admin/channel-models/fetch-upstream',
      {
        api_key: form.api_key,
        api_base_url: form.api_base_url || undefined,
      },
    )
    if (data.code !== 0) throw new Error(data.message || '拉取失败')
    const ids = data.data.model_ids || []
    if (ids.length === 0) {
      presetHint.value = '上游返回的模型列表为空'
      return
    }
    setIds(ids)
    presetHint.value = `已从上游合并 ${ids.length} 个模型 ID`
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
  } finally {
    fetchLoading.value = false
  }
}

function clearModels() {
  form.modelsStr = ''
  presetHint.value = '已清空模型列表'
}

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
          <input
            v-model="form.api_base_url"
            placeholder="留空=官方；也可填 https://api.openai.com 仅域名"
            class="rounded border border-slate-700 bg-slate-950 px-2 py-1"
          />
          <span class="text-xs text-slate-600">「从上游拉取」会自动补全为 …/v1 并请求 …/v1/models</span>
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">模型列表（逗号分隔）</span>
          <textarea
            v-model="form.modelsStr"
            rows="3"
            class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono text-xs"
            placeholder="gpt-4o,gpt-4o-mini"
          />
        </label>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            :disabled="presetLoading"
            class="rounded border border-slate-600 px-2 py-1 text-xs hover:bg-slate-800 disabled:opacity-50"
            title="内置默认定价表 + 模型目录（可见且启用，provider 匹配或通用）"
            @click="applyPresets('merged')"
          >
            {{ presetLoading ? '…' : '合并推荐' }}
          </button>
          <button
            type="button"
            :disabled="presetLoading"
            class="rounded border border-slate-600 px-2 py-1 text-xs hover:bg-slate-800 disabled:opacity-50"
            @click="applyPresets('built_in')"
          >
            仅内置
          </button>
          <button
            type="button"
            :disabled="presetLoading"
            class="rounded border border-slate-600 px-2 py-1 text-xs hover:bg-slate-800 disabled:opacity-50"
            @click="applyPresets('catalog')"
          >
            仅目录
          </button>
          <button
            type="button"
            :disabled="fetchLoading"
            class="rounded border border-cyan-700 px-2 py-1 text-xs text-cyan-300 hover:bg-slate-800 disabled:opacity-50"
            title="调用上游 OpenAI 兼容 GET /v1/models"
            @click="fetchFromUpstream"
          >
            {{ fetchLoading ? '拉取中…' : '从上游拉取' }}
          </button>
          <button type="button" class="rounded border border-slate-600 px-2 py-1 text-xs text-slate-500 hover:bg-slate-800" @click="clearModels">清空</button>
        </div>
        <p v-if="presetHint" class="text-xs text-slate-400">{{ presetHint }}</p>
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
