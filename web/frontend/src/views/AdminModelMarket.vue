<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type Source = {
  id: number
  name: string
  provider_slug: string
  base_url: string
  models_path: string
  credential_mode: string
  has_static_secret: boolean
  channel_id: number
  enabled: boolean
  last_sync_at?: string
  last_sync_ok: boolean
  last_sync_message?: string
}

type Entry = {
  id: number
  model_id: string
  display_name: string
  provider_slug: string
  owned_by?: string
  description?: string
  source_id?: number
  visible: boolean
  enabled: boolean
  synced_at?: string
}

type Ch = { id: number; channel_name: string; channel_type: string }

const sources = ref<Source[]>([])
const channels = ref<Ch[]>([])
const entries = ref<Entry[]>([])
const entryTotal = ref(0)
const page = ref(1)
const pageSize = 30
const q = ref('')
const msg = ref('')
const err = ref('')

const sourceForm = reactive({
  name: '',
  provider_slug: 'openai',
  base_url: 'https://api.openai.com',
  models_path: '/v1/models',
  credential_mode: 'static',
  api_key: '',
  channel_id: 0,
  enabled: true,
})

const editingEntry = ref<Entry | null>(null)
const editForm = reactive({
  display_name: '',
  description: '',
  visible: true,
  enabled: true,
})

const internalHint = computed(
  () => '此页不面向终端用户开放，仅管理员可访问；请勿将管理后台地址公开到公网文档。',
)

async function loadSources() {
  const { data } = await http.get<Envelope<{ items: Source[] }>>('/api/admin/model-catalog/sources')
  if (data.code === 0) sources.value = data.data.items
}

async function loadChannels() {
  const { data } = await http.get<Envelope<{ items: Ch[] }>>('/api/admin/channels')
  if (data.code === 0) channels.value = data.data.items
}

async function loadEntries() {
  const { data } = await http.get<Envelope<{ items: Entry[]; total: number }>>(
    '/api/admin/model-catalog/entries',
    { params: { page: page.value, page_size: pageSize, q: q.value || undefined } },
  )
  if (data.code === 0) {
    entries.value = data.data.items
    entryTotal.value = data.data.total
  }
}

async function createSource() {
  err.value = ''
  msg.value = ''
  try {
    await http.post('/api/admin/model-catalog/sources', {
      name: sourceForm.name,
      provider_slug: sourceForm.provider_slug,
      base_url: sourceForm.base_url,
      models_path: sourceForm.models_path,
      credential_mode: sourceForm.credential_mode,
      api_key: sourceForm.credential_mode === 'static' ? sourceForm.api_key : undefined,
      channel_id: sourceForm.credential_mode === 'channel' ? sourceForm.channel_id : 0,
      enabled: sourceForm.enabled,
    })
    msg.value = '同步源已保存'
    sourceForm.api_key = ''
    await loadSources()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '保存失败'
  }
}

async function syncSource(id: number) {
  err.value = ''
  msg.value = ''
  try {
    const { data } = await http.post<Envelope<{ inserted: number; updated: number }>>(
      `/api/admin/model-catalog/sources/${id}/sync`,
    )
    if (data.code !== 0) throw new Error(data.message)
    msg.value = `同步完成：新增 ${data.data.inserted}，更新 ${data.data.updated}`
    await Promise.all([loadSources(), loadEntries()])
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '同步失败'
  }
}

async function removeSource(id: number) {
  if (!confirm('删除该同步源及其关联的目录记录？')) return
  await http.delete(`/api/admin/model-catalog/sources/${id}`)
  await Promise.all([loadSources(), loadEntries()])
}

function openEdit(e: Entry) {
  editingEntry.value = e
  editForm.display_name = e.display_name || e.model_id
  editForm.description = e.description || ''
  editForm.visible = e.visible
  editForm.enabled = e.enabled
}

async function saveEntry() {
  if (!editingEntry.value) return
  await http.put(`/api/admin/model-catalog/entries/${editingEntry.value.id}`, {
    display_name: editForm.display_name,
    description: editForm.description || undefined,
    visible: editForm.visible,
    enabled: editForm.enabled,
  })
  editingEntry.value = null
  await loadEntries()
}

async function removeEntry(id: number) {
  if (!confirm('从模型市场移除该条？')) return
  await http.delete(`/api/admin/model-catalog/entries/${id}`)
  await loadEntries()
}

onMounted(async () => {
  await Promise.all([loadSources(), loadChannels(), loadEntries()])
})
</script>

<template>
  <div class="space-y-10 text-sm">
    <div>
      <h1 class="mb-1 text-2xl font-bold text-amber-400">模型市场（内部）</h1>
      <p class="text-xs text-slate-500">{{ internalHint }}</p>
    </div>

    <p v-if="msg" class="rounded border border-emerald-800 bg-emerald-950/40 px-3 py-2 text-emerald-300">{{ msg }}</p>
    <p v-if="err" class="rounded border border-red-900 bg-red-950/30 px-3 py-2 text-red-300">{{ err }}</p>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
      <h2 class="mb-4 text-lg font-semibold text-slate-200">第三方 / 官方列表 API</h2>
      <p class="mb-4 text-xs text-slate-500">
        配置可访问的「模型列表」HTTP 接口（默认 OpenAI 兼容 <code class="text-amber-600">GET /v1/models</code>）。凭证支持：无认证、静态
        Key（加密存储）、或复用已有渠道的 Key 与 Base URL。
      </p>
      <div class="grid max-w-2xl gap-3">
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">名称</span>
          <input v-model="sourceForm.name" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" placeholder="如 OpenAI 官方模型列表" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">提供方标识</span>
          <input v-model="sourceForm.provider_slug" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">Base URL</span>
          <input v-model="sourceForm.base_url" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">模型列表路径</span>
          <input v-model="sourceForm.models_path" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">凭证模式</span>
          <select v-model="sourceForm.credential_mode" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option value="none">none（公开列表）</option>
            <option value="static">static（下方填写 API Key）</option>
            <option value="channel">channel（复用渠道）</option>
          </select>
        </label>
        <label v-if="sourceForm.credential_mode === 'static'" class="flex flex-col gap-1">
          <span class="text-slate-500">API Key（仅保存时上传，加密存储）</span>
          <input v-model="sourceForm.api_key" type="password" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label v-if="sourceForm.credential_mode === 'channel'" class="flex flex-col gap-1">
          <span class="text-slate-500">渠道</span>
          <select v-model.number="sourceForm.channel_id" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
            <option :value="0">请选择</option>
            <option v-for="ch in channels" :key="ch.id" :value="ch.id">
              #{{ ch.id }} {{ ch.channel_name || ch.channel_type }}
            </option>
          </select>
        </label>
        <label class="flex items-center gap-2 text-slate-400">
          <input v-model="sourceForm.enabled" type="checkbox" class="rounded border-slate-600" />
          启用
        </label>
        <button
          type="button"
          class="max-w-xs rounded-lg bg-amber-500 py-2 font-semibold text-slate-950 hover:bg-amber-400"
          @click="createSource"
        >
          保存同步源
        </button>
      </div>

      <h3 class="mb-2 mt-8 font-medium text-slate-300">已配置源</h3>
      <div class="overflow-x-auto">
        <table class="w-full text-left">
          <thead class="text-slate-500">
            <tr>
              <th class="pb-2 pr-3">ID</th>
              <th class="pb-2 pr-3">名称</th>
              <th class="pb-2 pr-3">Base</th>
              <th class="pb-2 pr-3">凭证</th>
              <th class="pb-2 pr-3">上次同步</th>
              <th class="pb-2"></th>
            </tr>
          </thead>
          <tbody class="divide-y divide-slate-800">
            <tr v-for="s in sources" :key="s.id">
              <td class="py-2 pr-3">{{ s.id }}</td>
              <td class="py-2 pr-3">{{ s.name }}</td>
              <td class="max-w-xs truncate py-2 pr-3 text-xs text-slate-400">{{ s.base_url }}{{ s.models_path }}</td>
              <td class="py-2 pr-3 text-xs">{{ s.credential_mode }}{{ s.channel_id ? ' #' + s.channel_id : '' }}</td>
              <td class="py-2 pr-3 text-xs">
                <span v-if="s.last_sync_at">{{ s.last_sync_ok ? '✓' : '✗' }} {{ s.last_sync_message }}</span>
                <span v-else class="text-slate-600">未同步</span>
              </td>
              <td class="space-x-2 py-2">
                <button type="button" class="text-amber-400 hover:underline" @click="syncSource(s.id)">拉取模型</button>
                <button type="button" class="text-red-400 hover:underline" @click="removeSource(s.id)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="rounded-xl border border-slate-800 bg-slate-900/60 p-5">
      <h2 class="mb-4 text-lg font-semibold text-slate-200">模型目录</h2>
      <div class="mb-4 flex flex-wrap items-end gap-3">
        <label class="flex flex-col gap-1">
          <span class="text-slate-500">搜索</span>
          <input v-model="q" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" placeholder="model id / 名称" />
        </label>
        <button
          type="button"
          class="rounded-lg border border-slate-600 px-3 py-1 hover:bg-slate-800"
          @click="
            page = 1;
            loadEntries()
          "
        >
          查询
        </button>
        <span class="text-slate-500">共 {{ entryTotal }} 条</span>
      </div>
      <table class="w-full text-left">
        <thead class="text-slate-500">
          <tr>
            <th class="pb-2">模型 ID</th>
            <th class="pb-2">显示名</th>
            <th class="pb-2">提供方</th>
            <th class="pb-2">展示</th>
            <th class="pb-2">启用</th>
            <th class="pb-2"></th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-800">
          <tr v-for="e in entries" :key="e.id">
            <td class="py-2 font-mono text-xs">{{ e.model_id }}</td>
            <td class="py-2">{{ e.display_name }}</td>
            <td class="py-2 text-xs text-slate-400">{{ e.provider_slug }}</td>
            <td class="py-2">{{ e.visible ? '是' : '否' }}</td>
            <td class="py-2">{{ e.enabled ? '是' : '否' }}</td>
            <td class="space-x-2 py-2">
              <button type="button" class="text-amber-400 hover:underline" @click="openEdit(e)">编辑</button>
              <button type="button" class="text-red-400 hover:underline" @click="removeEntry(e.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="entryTotal > pageSize" class="mt-4 flex gap-2">
        <button
          type="button"
          class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
          :disabled="page <= 1"
          @click="
            page--;
            loadEntries()
          "
        >
          上一页
        </button>
        <button
          type="button"
          class="rounded border border-slate-700 px-2 py-1 disabled:opacity-40"
          :disabled="page * pageSize >= entryTotal"
          @click="
            page++;
            loadEntries()
          "
        >
          下一页
        </button>
      </div>
    </section>

    <Transition name="th-modal">
      <div v-if="editingEntry" class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/60" aria-hidden="true" @click="editingEntry = null" />
        <div class="th-modal-surface relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-5 shadow-xl">
        <h3 class="mb-3 font-semibold">编辑 {{ editingEntry.model_id }}</h3>
        <label class="mb-3 flex flex-col gap-1">
          <span class="text-slate-500">显示名</span>
          <input v-model="editForm.display_name" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="mb-3 flex flex-col gap-1">
          <span class="text-slate-500">说明</span>
          <textarea v-model="editForm.description" rows="3" class="rounded border border-slate-700 bg-slate-950 px-2 py-1" />
        </label>
        <label class="mb-3 flex items-center gap-2">
          <input v-model="editForm.visible" type="checkbox" /> 在内部市场中展示
        </label>
        <label class="mb-4 flex items-center gap-2">
          <input v-model="editForm.enabled" type="checkbox" /> 启用
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="rounded border border-slate-600 px-3 py-1" @click="editingEntry = null">取消</button>
          <button type="button" class="rounded bg-amber-500 px-3 py-1 font-medium text-slate-950" @click="saveEntry">保存</button>
        </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
