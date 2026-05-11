<script setup lang="ts">
import axios from 'axios'
import { onMounted, reactive, ref } from 'vue'
import { http, type Envelope } from '../api/http'

type GroupRow = {
  id: number
  name: string
  description?: string
  rate_multiplier: number
  rpm_limit: number
  subscription_type?: string
  default_model?: string
  enabled: boolean
}

type RateRow = {
  id: number
  model_name: string
  input_rate: number
  output_rate: number
  enabled: boolean
}

const groups = ref<GroupRow[]>([])
const msg = ref('')
const expandedId = ref<number | null>(null)
const ratesByGroup = ref<Record<number, RateRow[]>>({})
const ratesDraft = ref<RateRow[]>([])
const ratesLoading = ref(false)

const createForm = reactive({
  name: '',
  description: '',
  rate_multiplier: 1,
  rpm_limit: 0,
  default_model: '',
  enabled: true,
})

const editOpen = ref(false)
const editForm = reactive({
  id: 0,
  name: '',
  description: '',
  rate_multiplier: 1,
  rpm_limit: 0,
  default_model: '',
  enabled: true,
})

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : '请求失败'
}

async function load() {
  msg.value = ''
  try {
    const { data } = await http.get<Envelope<{ items: GroupRow[] }>>('/api/admin/groups')
    if (data.code !== 0) throw new Error(data.message || '加载失败')
    groups.value = data.data.items || []
  } catch (e: unknown) {
    msg.value = httpErr(e)
  }
}

async function createGroup() {
  msg.value = ''
  if (!createForm.name.trim()) {
    msg.value = '请输入分组名称'
    return
  }
  try {
    await http.post('/api/admin/groups', {
      name: createForm.name.trim(),
      description: createForm.description.trim() || undefined,
      rate_multiplier: createForm.rate_multiplier,
      rpm_limit: createForm.rpm_limit,
      default_model: createForm.default_model.trim() || undefined,
      enabled: createForm.enabled,
    })
    createForm.name = ''
    createForm.description = ''
    createForm.rate_multiplier = 1
    createForm.rpm_limit = 0
    createForm.default_model = ''
    createForm.enabled = true
    await load()
    msg.value = '分组已创建'
  } catch (e: unknown) {
    msg.value = httpErr(e)
  }
}

function openEdit(g: GroupRow) {
  editForm.id = g.id
  editForm.name = g.name
  editForm.description = g.description || ''
  editForm.rate_multiplier = g.rate_multiplier ?? 1
  editForm.rpm_limit = g.rpm_limit ?? 0
  editForm.default_model = g.default_model || ''
  editForm.enabled = g.enabled !== false
  editOpen.value = true
}

async function saveEdit() {
  msg.value = ''
  try {
    await http.put(`/api/admin/groups/${editForm.id}`, {
      name: editForm.name.trim(),
      description: editForm.description.trim(),
      rate_multiplier: editForm.rate_multiplier,
      rpm_limit: editForm.rpm_limit,
      default_model: editForm.default_model.trim() || '',
      enabled: editForm.enabled,
    })
    editOpen.value = false
    await load()
    msg.value = '已保存'
  } catch (e: unknown) {
    msg.value = httpErr(e)
  }
}

async function remove(g: GroupRow) {
  if (
    !confirm(
      `确定删除分组「${g.name}」？若仍有用户或渠道挂在该分组下，删除将失败。`,
    )
  )
    return
  msg.value = ''
  try {
    await http.delete(`/api/admin/groups/${g.id}`)
    await load()
    if (expandedId.value === g.id) expandedId.value = null
    msg.value = '已删除'
  } catch (e: unknown) {
    msg.value = httpErr(e)
  }
}

async function reloadRatesDraft(gid: number) {
  const { data } = await http.get<Envelope<{ items: RateRow[] }>>(`/api/admin/groups/${gid}/model-rates`)
  if (data.code !== 0) throw new Error(data.message || '加载倍率失败')
  const list = data.data.items || []
  ratesByGroup.value[gid] = list
  ratesDraft.value = list.map((r) => ({ ...r }))
}

async function toggleExpand(id: number) {
  if (expandedId.value === id) {
    expandedId.value = null
    return
  }
  expandedId.value = id
  ratesLoading.value = true
  ratesDraft.value = []
  try {
    await reloadRatesDraft(id)
  } catch (e: unknown) {
    msg.value = httpErr(e)
    expandedId.value = null
  } finally {
    ratesLoading.value = false
  }
}

function addRateDraft() {
  ratesDraft.value.push({
    id: 0,
    model_name: '',
    input_rate: 1,
    output_rate: 1,
    enabled: true,
  })
}

async function saveRates(gid: number) {
  msg.value = ''
  const rates = ratesDraft.value
    .filter((r) => r.model_name.trim())
    .map((r) => ({
      model_name: r.model_name.trim(),
      input_rate: r.input_rate,
      output_rate: r.output_rate,
      enabled: r.enabled,
    }))
  try {
    await http.put(`/api/admin/groups/${gid}/model-rates`, { rates })
    await reloadRatesDraft(gid)
    msg.value = '模型倍率已保存'
  } catch (e: unknown) {
    msg.value = httpErr(e)
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-10 pb-10">
    <header class="border-b border-slate-800/80 pb-6">
      <h1 class="text-2xl font-bold tracking-tight text-white">分组管理</h1>
      <p class="mt-1 text-sm text-slate-500">
        用户与渠道按分组隔离费率与路由池；新建渠道、注册用户时需指定分组。
      </p>
    </header>

    <p v-if="msg" class="rounded-lg border border-slate-700 bg-slate-900/80 px-4 py-2 text-sm text-slate-300">{{ msg }}</p>

    <!-- 新建 -->
    <section class="rounded-2xl border border-slate-800/90 bg-gradient-to-b from-slate-900/90 to-slate-950 p-6 shadow-xl ring-1 ring-white/5">
      <h2 class="mb-4 text-sm font-semibold uppercase tracking-wider text-slate-500">新建分组</h2>
      <div class="grid max-w-3xl gap-4 text-sm">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">名称 <span class="text-red-400">*</span></span>
            <input
              v-model="createForm.name"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              placeholder="例如：基础分组、VIP"
            />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">默认模型（可选）</span>
            <input
              v-model="createForm.default_model"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
            />
          </label>
        </div>
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">描述</span>
          <input
            v-model="createForm.description"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
          />
        </label>
        <div class="grid gap-4 sm:grid-cols-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">分组倍率</span>
            <input
              v-model.number="createForm.rate_multiplier"
              type="number"
              step="0.01"
              min="0"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono outline-none"
            />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">RPM 上限（0=不限制）</span>
            <input
              v-model.number="createForm.rpm_limit"
              type="number"
              min="0"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono outline-none"
            />
          </label>
          <label class="flex items-end gap-2 pb-2">
            <input id="cg-en" v-model="createForm.enabled" type="checkbox" class="rounded border-slate-600" />
            <label for="cg-en" class="text-xs text-slate-400">启用</label>
          </label>
        </div>
        <button
          type="button"
          class="w-full max-w-xs rounded-xl bg-gradient-to-r from-amber-500 to-amber-600 py-2.5 text-sm font-semibold text-slate-950 shadow-lg hover:from-amber-400 hover:to-amber-500 sm:w-auto"
          @click="createGroup"
        >
          创建分组
        </button>
      </div>
    </section>

    <!-- 列表 -->
    <section>
      <div class="mb-4 flex items-end justify-between border-b border-slate-800/80 pb-4">
        <div>
          <h2 class="text-xl font-semibold text-white">分组列表</h2>
          <p class="mt-1 text-sm text-slate-500">共 {{ groups.length }} 个</p>
        </div>
        <button
          type="button"
          class="rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-400 hover:border-slate-600 hover:text-slate-300"
          @click="load"
        >
          刷新
        </button>
      </div>

      <div v-if="groups.length === 0" class="rounded-2xl border border-dashed border-slate-700 py-16 text-center text-slate-500">
        暂无分组，请在上方创建。
      </div>

      <div v-else class="space-y-4">
        <article
          v-for="g in groups"
          :key="g.id"
          class="overflow-hidden rounded-2xl border border-slate-800/90 bg-slate-900/60 ring-1 ring-white/[0.04]"
        >
          <div class="flex flex-wrap items-start justify-between gap-4 border-b border-slate-800/80 px-5 py-4">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold text-white">{{ g.name }}</h3>
                <span
                  v-if="g.enabled === false"
                  class="rounded-full bg-slate-700 px-2 py-0.5 text-[10px] text-slate-400"
                >
                  已禁用
                </span>
                <span
                  v-else
                  class="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] font-medium text-emerald-400 ring-1 ring-emerald-500/25"
                >
                  启用
                </span>
              </div>
              <p v-if="g.description" class="mt-2 text-sm text-slate-400">{{ g.description }}</p>
              <dl class="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500">
                <div>ID <span class="font-mono text-slate-400">{{ g.id }}</span></div>
                <div>倍率 <span class="font-mono text-slate-300">{{ g.rate_multiplier }}×</span></div>
                <div>RPM <span class="font-mono text-slate-300">{{ g.rpm_limit || '不限' }}</span></div>
                <div v-if="g.default_model">
                  默认模型 <span class="font-mono text-slate-400">{{ g.default_model }}</span>
                </div>
              </dl>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
                @click="toggleExpand(g.id)"
              >
                {{ expandedId === g.id ? '收起倍率' : '模型倍率' }}
              </button>
              <button
                type="button"
                class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-amber-400 hover:bg-slate-800"
                @click="openEdit(g)"
              >
                编辑
              </button>
              <button
                type="button"
                class="rounded-lg px-3 py-1.5 text-xs text-red-400 hover:bg-red-950/40"
                @click="remove(g)"
              >
                删除
              </button>
            </div>
          </div>

          <div v-if="expandedId === g.id" class="border-t border-slate-800/80 bg-slate-950/40 px-5 py-4">
            <p v-if="ratesLoading" class="text-sm text-slate-500">加载中…</p>
            <template v-else>
              <p class="mb-3 text-xs text-slate-500">
                按模型覆盖输入/输出倍率；保存后与分组全局倍率叠加参与计费。
              </p>
              <div class="overflow-x-auto rounded-xl border border-slate-800">
                <table class="w-full text-left text-xs">
                  <thead class="bg-slate-900 text-slate-500">
                    <tr>
                      <th class="px-3 py-2">模型 ID</th>
                      <th class="px-3 py-2">输入倍率</th>
                      <th class="px-3 py-2">输出倍率</th>
                      <th class="px-3 py-2">启用</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800">
                    <tr v-for="(r, idx) in ratesDraft" :key="idx">
                      <td class="px-2 py-1">
                        <input
                          v-model="r.model_name"
                          class="w-full min-w-[8rem] rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono"
                        />
                      </td>
                      <td class="px-2 py-1">
                        <input
                          v-model.number="r.input_rate"
                          type="number"
                          step="0.01"
                          class="w-20 rounded border border-slate-700 bg-slate-950 px-2 py-1"
                        />
                      </td>
                      <td class="px-2 py-1">
                        <input
                          v-model.number="r.output_rate"
                          type="number"
                          step="0.01"
                          class="w-20 rounded border border-slate-700 bg-slate-950 px-2 py-1"
                        />
                      </td>
                      <td class="px-2 py-1">
                        <input v-model="r.enabled" type="checkbox" />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  type="button"
                  class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
                  @click="addRateDraft"
                >
                  添加一行
                </button>
                <button
                  type="button"
                  class="rounded-lg bg-amber-500 px-4 py-1.5 text-xs font-semibold text-slate-950 hover:bg-amber-400"
                  @click="saveRates(g.id)"
                >
                  保存倍率
                </button>
              </div>
            </template>
          </div>
        </article>
      </div>
    </section>

    <!-- 编辑弹层 -->
    <Teleport to="body">
      <div
        v-if="editOpen"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
        @click.self="editOpen = false"
      >
        <div
          class="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-slate-700 bg-slate-900 p-6 shadow-2xl"
          role="dialog"
          aria-modal="true"
        >
          <h3 class="mb-4 text-lg font-semibold text-white">编辑分组</h3>
          <div class="space-y-4 text-sm">
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">名称</span>
              <input v-model="editForm.name" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">描述</span>
              <input v-model="editForm.description" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">默认模型</span>
              <input v-model="editForm.default_model" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" />
            </label>
            <div class="grid grid-cols-2 gap-4">
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">分组倍率</span>
                <input v-model.number="editForm.rate_multiplier" type="number" step="0.01" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">RPM</span>
                <input v-model.number="editForm.rpm_limit" type="number" min="0" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
            </div>
            <label class="flex items-center gap-2">
              <input v-model="editForm.enabled" type="checkbox" />
              <span class="text-xs text-slate-400">启用</span>
            </label>
          </div>
          <div class="mt-6 flex justify-end gap-2">
            <button type="button" class="rounded-lg px-4 py-2 text-sm text-slate-400 hover:bg-slate-800" @click="editOpen = false">
              取消
            </button>
            <button
              type="button"
              class="rounded-lg bg-amber-500 px-4 py-2 text-sm font-semibold text-slate-950 hover:bg-amber-400"
              @click="saveEdit"
            >
              保存
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
