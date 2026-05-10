<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type KeyRow = {
  id: number
  name?: string
  key_prefix?: string
  group_name?: string
  status: string
  last_used_at?: string
  usage_24h?: number
  daily_quota?: number
  ip_whitelist?: string[]
}

const items = ref<KeyRow[]>([])
const err = ref('')
const createdKey = ref('')
const loading = ref(false)

const createOpen = ref(false)
const createName = ref('')
const createErr = ref('')

const editOpen = ref(false)
const edit = reactive({
  id: 0,
  name: '',
  enabled: true,
  daily_quota: '' as string | number,
  ip_whitelist: '',
})

async function load() {
  const { data } = await http.get<Envelope<{ items: KeyRow[] }>>('/api/user/apikeys')
  if (data.code === 0) items.value = data.data.items
}

function openCreateModal() {
  createErr.value = ''
  createName.value = ''
  createOpen.value = true
}

function closeCreateModal() {
  createOpen.value = false
  createErr.value = ''
}

async function submitCreateKey() {
  const label = createName.value.trim()
  if (!label) {
    createErr.value = '请先填写 Key 名称（备注），便于在列表中识别。'
    return
  }
  createErr.value = ''
  createdKey.value = ''
  loading.value = true
  try {
    const { data } = await http.post<Envelope<{ key: string; id?: number }>>('/api/user/apikeys', { name: label })
    if (data.code !== 0) throw new Error(data.message || '创建失败')
    createdKey.value = data.data.key
    createOpen.value = false
    createName.value = ''
    await load()
  } catch (e: unknown) {
    createErr.value = e instanceof Error ? e.message : '创建失败'
  } finally {
    loading.value = false
  }
}

function openEdit(k: KeyRow) {
  edit.id = k.id
  edit.name = k.name || ''
  edit.enabled = k.status === 'enabled'
  edit.daily_quota = k.daily_quota != null && k.daily_quota >= 0 ? String(k.daily_quota) : k.daily_quota === -1 ? '-1' : ''
  edit.ip_whitelist = (k.ip_whitelist || []).join('\n')
  editOpen.value = true
}

async function saveEdit() {
  err.value = ''
  const lines = edit.ip_whitelist
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
  const body: Record<string, unknown> = {
    name: edit.name.trim() || undefined,
    enabled: edit.enabled,
    ip_whitelist: lines.length ? lines : [],
  }
  const dq = String(edit.daily_quota).trim()
  if (dq !== '') {
    const n = parseInt(dq, 10)
    if (!Number.isNaN(n)) body.daily_quota = n
  }
  try {
    const { data } = await http.put<Envelope<null>>(`/api/user/apikeys/${edit.id}`, body)
    if (data.code !== 0) throw new Error(data.message || '保存失败')
    editOpen.value = false
    await load()
  } catch (e: unknown) {
    err.value = e instanceof Error ? e.message : '保存失败'
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
    <h1 class="mb-2 text-2xl font-bold">API Key</h1>
    <p class="mb-6 text-sm text-slate-500">创建、重命名、启用/禁用与 IP 白名单；完整 Key 仅在创建时显示一次。</p>
    <div class="mb-6">
      <button
        type="button"
        class="rounded-lg bg-cyan-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-cyan-400 disabled:opacity-50"
        @click="openCreateModal"
      >
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
          <th class="pb-2">分组</th>
          <th class="pb-2">24h Token</th>
          <th class="pb-2">状态</th>
          <th class="pb-2"></th>
        </tr>
      </thead>
      <tbody class="divide-y divide-slate-800">
        <tr v-for="k in items" :key="k.id">
          <td class="py-2">{{ k.name || '—' }}</td>
          <td class="py-2 font-mono text-xs">{{ k.key_prefix }}</td>
          <td class="py-2 text-xs text-slate-400">{{ k.group_name || '—' }}</td>
          <td class="py-2 text-xs">{{ k.usage_24h ?? '—' }}</td>
          <td class="py-2">{{ k.status }}</td>
          <td class="space-x-2 py-2 text-right">
            <button type="button" class="text-cyan-400 hover:underline" @click="openEdit(k)">编辑</button>
            <button type="button" class="text-red-400 hover:underline" @click="remove(k.id)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <Transition name="th-modal">
      <div
        v-if="createOpen"
        class="fixed inset-0 z-40 flex items-center justify-center p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-key-title"
      >
        <div class="absolute inset-0 bg-black/60" aria-hidden="true" @click="closeCreateModal" />
        <div class="th-modal-surface relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-6 shadow-xl">
        <h3 id="create-key-title" class="mb-1 font-semibold">新建 API Key</h3>
        <p class="mb-4 text-sm text-slate-500">请先填写名称（备注），用于在列表中区分不同 Key；不会在请求中暴露。</p>
        <label class="mb-4 block text-sm">
          <span class="text-slate-400">Key 名称 / 备注</span>
          <input
            v-model="createName"
            type="text"
            required
            maxlength="128"
            autocomplete="off"
            placeholder="例如：生产环境 · 主业务"
            class="mt-1.5 w-full rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm"
            @keydown.enter.prevent="submitCreateKey"
          />
        </label>
        <p v-if="createErr" class="mb-3 rounded-lg border border-red-500/25 bg-red-950/35 px-3 py-2 text-sm text-red-300">{{ createErr }}</p>
        <div class="flex justify-end gap-2">
          <button type="button" class="rounded-lg border border-slate-600 px-3 py-2 text-sm" @click="closeCreateModal">取消</button>
          <button
            type="button"
            :disabled="loading"
            class="rounded-lg bg-cyan-600 px-3 py-2 text-sm font-medium text-white hover:bg-cyan-500 disabled:opacity-50"
            @click="submitCreateKey"
          >
            {{ loading ? '创建中…' : '生成 Key' }}
          </button>
        </div>
        </div>
      </div>
    </Transition>

    <Transition name="th-modal">
      <div v-if="editOpen" class="fixed inset-0 z-40 flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/60" aria-hidden="true" @click="editOpen = false" />
        <div class="th-modal-surface relative z-10 w-full max-w-md rounded-xl border border-slate-700 bg-slate-900 p-6 shadow-xl">
        <h3 class="mb-4 font-semibold">编辑 API Key</h3>
        <label class="mb-3 block text-sm">
          <span class="text-slate-500">名称</span>
          <input v-model="edit.name" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5" />
        </label>
        <label class="mb-3 flex items-center gap-2 text-sm">
          <input v-model="edit.enabled" type="checkbox" /> 启用
        </label>
        <label class="mb-3 block text-sm">
          <span class="text-slate-500">每日额度（空=不改；-1=不限）</span>
          <input v-model="edit.daily_quota" placeholder="留空表示本次不修改" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-xs" />
        </label>
        <label class="mb-4 block text-sm">
          <span class="text-slate-500">IP 白名单（每行或逗号分隔）</span>
          <textarea v-model="edit.ip_whitelist" rows="3" class="mt-1 w-full rounded border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-xs" />
        </label>
        <div class="flex justify-end gap-2">
          <button type="button" class="rounded border border-slate-600 px-3 py-1.5 text-sm" @click="editOpen = false">取消</button>
          <button type="button" class="rounded bg-cyan-600 px-3 py-1.5 text-sm font-medium text-white" @click="saveEdit">保存</button>
        </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
