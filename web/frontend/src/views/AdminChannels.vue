<script setup lang="ts">
import axios from 'axios'
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type Ch = {
  id: number
  group_id: number
  channel_type: string
  channel_name: string
  models: string[]
  enabled: boolean
  api_base_url?: string
  auth_type?: string
  health_status?: number
  priority?: number
  weight?: number
  concurrency?: number
  schedulable?: boolean
  has_api_secret?: boolean
}

type G = { id: number; name: string }

const channels = ref<Ch[]>([])
const groups = ref<G[]>([])
const form = reactive({
  group_id: 0,
  channel_type: 'openai',
  channel_name: '',
  api_key: '',
  api_base_url: '',
  modelsStr: 'gpt-4o,gpt-4o-mini',
})

const presetLoading = ref(false)
const fetchLoading = ref(false)
const presetHint = ref('')

/** 分组下拉用字符串 v-model，避免动态 option 与 v-model.number 在部分浏览器下不同步 */
const groupIdStr = ref('')

function normalizeGroupRows(raw: unknown): G[] {
  if (!Array.isArray(raw)) return []
  const out: G[] = []
  for (const x of raw) {
    if (!x || typeof x !== 'object') continue
    const o = x as Record<string, unknown>
    const id = Number(o.id)
    if (!Number.isFinite(id)) continue
    const name = typeof o.name === 'string' ? o.name : String(o.name ?? '')
    out.push({ id, name })
  }
  return out
}

function syncGroupPickToForm() {
  const gs = groups.value
  if (!gs.length) {
    groupIdStr.value = ''
    return
  }
  const ids = new Set(gs.map((x) => x.id))
  if (!ids.has(form.group_id)) {
    form.group_id = gs[0].id
  }
  groupIdStr.value = String(form.group_id)
}

watch(
  () => groups.value.map((x) => x.id).join(','),
  () => {
    syncGroupPickToForm()
  },
)

watch(groupIdStr, (s) => {
  const n = Number(s)
  if (!Number.isFinite(n) || s === '') return
  const ids = new Set(groups.value.map((x) => x.id))
  if (ids.has(n)) form.group_id = n
})

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

async function applyPresets(mode: 'merged' | 'built_in' | 'catalog') {
  presetHint.value = ''
  presetLoading.value = true
  try {
    const { data } = await http.get<Envelope<{ built_in: string[]; catalog: string[]; merged: string[] }>>(
      '/api/admin/channel-model-presets',
      { params: { channel_type: form.channel_type } },
    )
    if (!envelopeOk(data)) throw new Error(data.message || t('adminCommon.loadFail'))
    const d = pickEnvelopeData<{ merged?: unknown; built_in?: unknown; catalog?: unknown }>(data)
    if (!d) throw new Error(t('adminCommon.loadFail'))
    let pick: string[] = []
    if (mode === 'merged') pick = coerceStringList(d.merged)
    else if (mode === 'built_in') pick = coerceStringList(d.built_in)
    else pick = coerceStringList(d.catalog)
    if (pick.length === 0) {
      presetHint.value = t('adminChannels.presetEmpty')
      return
    }
    setIds(pick)
    presetHint.value = t('adminChannels.presetMerged', { n: pick.length })
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
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

/** 网关偶发把 code 打成字符串；兼容 Number(code)===0 */
function envelopeOk(body: unknown): boolean {
  if (!body || typeof body !== 'object') return false
  const c = Number((body as Record<string, unknown>).code)
  return c === 0
}

/** 优先 data.items，兼容少数代理把 items 顶在响应根上 */
function pickListItems(body: unknown): unknown[] {
  if (!body || typeof body !== 'object') return []
  const o = body as Record<string, unknown>
  const inner = o.data
  if (inner && typeof inner === 'object') {
    const items = (inner as Record<string, unknown>).items
    if (Array.isArray(items)) return items
  }
  if (Array.isArray(o.items)) return o.items
  return []
}

function pickEnvelopeData<T extends Record<string, unknown>>(body: unknown): T | null {
  if (!body || typeof body !== 'object') return null
  const d = (body as Record<string, unknown>).data
  if (d && typeof d === 'object') return d as T
  return null
}

function coerceStringList(v: unknown): string[] {
  if (!Array.isArray(v)) return []
  const out: string[] = []
  for (const x of v) {
    if (typeof x === 'string') {
      const s = x.trim()
      if (s) out.push(s)
    } else if (typeof x === 'number' && Number.isFinite(x)) {
      out.push(String(x))
    }
  }
  return out
}

async function fetchFromUpstream() {
  presetHint.value = ''
  if (!form.api_key.trim()) {
    presetHint.value = t('adminChannels.needApiKey')
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
    if (!envelopeOk(data)) throw new Error(data.message || t('adminCommon.loadFail'))
    const inner = pickEnvelopeData<{ model_ids?: unknown }>(data)
    const rawIds = inner?.model_ids ?? (data as Record<string, unknown>).model_ids
    const ids = coerceStringList(rawIds)
    if (ids.length === 0) {
      presetHint.value = t('adminChannels.upstreamEmpty')
      return
    }
    setIds(ids)
    presetHint.value = t('adminChannels.upstreamMerged', { n: ids.length })
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
  } finally {
    fetchLoading.value = false
  }
}

function clearModels() {
  form.modelsStr = ''
  presetHint.value = t('adminChannels.cleared')
}

async function load() {
  const [cRes, gRes] = await Promise.allSettled([
    http.get<Envelope<{ items: Ch[] }>>('/api/admin/channels'),
    http.get<Envelope<{ items: unknown[] }>>('/api/admin/groups'),
  ])

  if (cRes.status === 'fulfilled') {
    const body = cRes.value.data
    if (envelopeOk(body)) {
      const items = pickListItems(body) as Ch[]
      channels.value = Array.isArray(items) ? items : []
    }
  }

  if (gRes.status === 'fulfilled') {
    const body = gRes.value.data
    if (envelopeOk(body)) {
      groups.value = normalizeGroupRows(pickListItems(body))
      syncGroupPickToForm()
    }
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
  if (!confirm(t('adminChannels.confirmDelete'))) return
  await http.delete(`/api/admin/channels/${id}`)
  await load()
}

onMounted(load)

function groupLabel(gid: number): string {
  const g = groups.value.find((x) => x.id === gid)
  return g ? g.name : t('adminChannels.groupTag', { id: gid })
}

function typeBadgeClass(ty: string): string {
  const m: Record<string, string> = {
    openai: 'bg-emerald-500/15 text-emerald-400 ring-emerald-500/30',
    deepseek: 'bg-sky-500/15 text-sky-400 ring-sky-500/30',
    claude: 'bg-orange-500/15 text-orange-400 ring-orange-500/30',
    gemini: 'bg-blue-500/15 text-blue-400 ring-blue-500/30',
  }
  return m[ty?.toLowerCase()] ?? 'bg-slate-500/15 text-slate-400 ring-slate-500/30'
}

function healthLabel(status?: number): { text: string; cls: string } {
  switch (status) {
    case 1:
      return { text: t('adminChannels.healthOk'), cls: 'bg-emerald-500/10 text-emerald-400 ring-emerald-500/25' }
    case 2:
      return { text: t('adminChannels.healthBad'), cls: 'bg-red-500/10 text-red-400 ring-red-500/25' }
    case 3:
      return { text: t('adminChannels.healthPaused'), cls: 'bg-amber-500/10 text-amber-400 ring-amber-500/25' }
    default:
      return { text: t('adminChannels.healthUnknown'), cls: 'bg-slate-500/10 text-slate-400 ring-slate-500/20' }
  }
}

function truncateUrl(u: string, max = 42): string {
  if (!u || u.length <= max) return u || t('adminCommon.dash')
  return u.slice(0, max - 1) + t('adminCommon.ellipsis')
}
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-10 pb-10">
    <section>
      <div class="mb-6 flex flex-col gap-1 border-b border-slate-800/80 pb-4">
        <h1 class="text-2xl font-bold tracking-tight text-white">{{ t('adminChannels.pageTitle') }}</h1>
        <p class="text-sm text-slate-500">{{ t('adminChannels.pageDesc') }}</p>
      </div>
      <div class="rounded-2xl border border-slate-800/90 bg-gradient-to-b from-slate-900/90 to-slate-950 p-6 shadow-xl shadow-black/20 ring-1 ring-white/5">
        <h2 class="mb-4 text-sm font-semibold uppercase tracking-wider text-slate-500">{{ t('adminChannels.formNewTitle') }}</h2>
        <div class="grid max-w-2xl gap-4 text-sm">
          <div class="grid gap-4 sm:grid-cols-2">
            <div class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400" id="admin-ch-group-lbl">{{ t('adminChannels.lblGroup') }}</span>
              <select
                id="admin-ch-group"
                v-model="groupIdStr"
                aria-labelledby="admin-ch-group-lbl"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none ring-amber-500/0 transition focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
                :key="`grp-${groups.map((x) => x.id).join('-')}`"
              >
                <option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }} (#{{ g.id }})</option>
              </select>
            </div>
            <div class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400" id="admin-ch-type-lbl">{{ t('adminChannels.lblType') }}</span>
              <select
                id="admin-ch-type"
                v-model="form.channel_type"
                aria-labelledby="admin-ch-type-lbl"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              >
                <option value="openai">openai</option>
                <option value="deepseek">deepseek</option>
                <option value="claude">claude</option>
                <option value="gemini">gemini</option>
              </select>
            </div>
          </div>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.lblChannelName') }}</span>
            <input
              v-model="form.channel_name"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 placeholder:text-slate-600 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              :placeholder="t('adminChannels.channelNamePh')"
            />
          </label>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.lblApiKey') }}</span>
              <input
                v-model="form.api_key"
                type="password"
                autocomplete="off"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.lblBaseUrl') }}</span>
              <input
                v-model="form.api_base_url"
                :placeholder="t('adminChannels.baseUrlPh')"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              />
              <span class="text-xs text-slate-600">{{ t('adminChannels.baseUrlHint') }}</span>
            </label>
          </div>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.lblModels') }}</span>
            <textarea
              v-model="form.modelsStr"
              rows="4"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs leading-relaxed text-slate-200 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              :placeholder="t('adminChannels.modelsPh')"
            />
          </label>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              :disabled="presetLoading"
              class="rounded-lg border border-slate-600 bg-slate-800/50 px-3 py-1.5 text-xs font-medium text-slate-200 transition hover:bg-slate-800 disabled:opacity-40"
              :title="t('adminChannels.btnMergeRecTitle')"
              @click="applyPresets('merged')"
            >
              {{ presetLoading ? t('adminCommon.ellipsis') : t('adminChannels.btnMergeRec') }}
            </button>
            <button
              type="button"
              :disabled="presetLoading"
              class="rounded-lg border border-slate-600 bg-slate-800/50 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800 disabled:opacity-40"
              @click="applyPresets('built_in')"
            >
              {{ t('adminChannels.btnBuiltinOnly') }}
            </button>
            <button
              type="button"
              :disabled="presetLoading"
              class="rounded-lg border border-slate-600 bg-slate-800/50 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800 disabled:opacity-40"
              @click="applyPresets('catalog')"
            >
              {{ t('adminChannels.btnCatalogOnly') }}
            </button>
            <button
              type="button"
              :disabled="fetchLoading"
              class="th-admin-user-console-link inline-flex items-center justify-center px-3 py-1.5 text-xs font-medium disabled:opacity-40"
              @click="fetchFromUpstream"
            >
              {{ fetchLoading ? t('adminChannels.fetching') : t('adminChannels.btnFetchUpstream') }}
            </button>
            <button
              type="button"
              class="rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-500 hover:bg-slate-800/80"
              @click="clearModels"
            >
              {{ t('adminChannels.btnClear') }}
            </button>
          </div>
          <p v-if="presetHint" class="rounded-lg border border-slate-700/80 bg-slate-900/80 px-3 py-2 text-xs text-slate-400">{{ presetHint }}</p>
          <button
            type="button"
            class="th-btn-primary-flat w-full max-w-xs px-4 py-2.5 text-sm sm:w-auto"
            @click="createCh"
          >
            {{ t('adminChannels.saveChannel') }}
          </button>
        </div>
      </div>
    </section>

    <section>
      <div class="mb-6 flex flex-col justify-between gap-4 border-b border-slate-800/80 pb-4 sm:flex-row sm:items-end">
        <div>
          <h2 class="text-xl font-semibold text-white">{{ t('adminChannels.listTitle') }}</h2>
          <p class="mt-1 text-sm text-slate-500">{{ t('adminChannels.listCount', { n: channels.length }) }}</p>
        </div>
        <button
          type="button"
          class="self-start rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-400 hover:border-slate-600 hover:text-slate-300"
          @click="load"
        >
          {{ t('adminCommon.refresh') }}
        </button>
      </div>

      <div
        v-if="channels.length === 0"
        class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-slate-700 bg-slate-900/40 px-8 py-16 text-center"
      >
        <div class="mb-3 rounded-full bg-slate-800 p-3 ring-1 ring-slate-700">
          <svg class="h-8 w-8 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
          </svg>
        </div>
        <p class="text-sm font-medium text-slate-400">{{ t('adminChannels.emptyTitle') }}</p>
        <p class="mt-1 max-w-sm text-xs text-slate-600">{{ t('adminChannels.emptyDesc') }}</p>
      </div>

      <div v-else class="grid gap-4 lg:grid-cols-2">
        <article
          v-for="ch in channels"
          :key="ch.id"
          class="group flex flex-col overflow-hidden rounded-2xl border border-slate-800/90 bg-slate-900/60 shadow-lg shadow-black/25 ring-1 ring-white/[0.04] transition hover:border-slate-700 hover:ring-amber-500/10"
        >
          <div class="flex items-start justify-between gap-3 border-b border-slate-800/80 bg-slate-950/40 px-5 py-4">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="truncate text-lg font-semibold text-white">
                  {{ ch.channel_name || t('adminChannels.unnamed', { id: ch.id }) }}
                </h3>
                <span
                  class="inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ring-1 ring-inset"
                  :class="typeBadgeClass(ch.channel_type)"
                >
                  {{ ch.channel_type }}
                </span>
                <span
                  v-if="ch.enabled === false"
                  class="inline-flex shrink-0 rounded-full bg-slate-700 px-2 py-0.5 text-[10px] font-medium text-slate-400"
                >
                  {{ t('adminCommon.disabled') }}
                </span>
                <span
                  v-else
                  class="inline-flex shrink-0 rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] font-medium text-emerald-400 ring-1 ring-emerald-500/25"
                >
                  {{ t('adminCommon.enabled') }}
                </span>
              </div>
              <p class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-500">
                <span>ID <span class="font-mono text-slate-400">{{ ch.id }}</span></span>
                <span class="text-slate-700">·</span>
                <span>{{ groupLabel(ch.group_id) }}</span>
                <span v-if="ch.auth_type" class="text-slate-600">· {{ ch.auth_type }}</span>
              </p>
            </div>
            <button
              type="button"
              class="shrink-0 rounded-lg px-2.5 py-1.5 text-xs font-medium text-red-400 opacity-80 transition hover:bg-red-950/50 hover:opacity-100"
              @click="remove(ch.id)"
            >
              {{ t('adminCommon.delete') }}
            </button>
          </div>

          <div class="flex flex-1 flex-col gap-4 px-5 py-4">
            <div class="flex flex-wrap gap-3 text-xs">
              <div v-if="ch.priority != null" class="rounded-md bg-slate-950/80 px-2.5 py-1 ring-1 ring-slate-800">
                <span class="text-slate-500">{{ t('adminChannels.priority') }}</span>
                <span class="ml-1 font-mono text-slate-300">{{ ch.priority }}</span>
              </div>
              <div v-if="ch.weight != null" class="rounded-md bg-slate-950/80 px-2.5 py-1 ring-1 ring-slate-800">
                <span class="text-slate-500">{{ t('adminChannels.weight') }}</span>
                <span class="ml-1 font-mono text-slate-300">{{ ch.weight }}</span>
              </div>
              <div v-if="ch.concurrency != null" class="rounded-md bg-slate-950/80 px-2.5 py-1 ring-1 ring-slate-800">
                <span class="text-slate-500">{{ t('adminChannels.concurrency') }}</span>
                <span class="ml-1 font-mono text-slate-300">{{ ch.concurrency }}</span>
              </div>
              <div
                class="rounded-md px-2.5 py-1 ring-1 ring-inset"
                :class="healthLabel(ch.health_status).cls"
              >
                {{ healthLabel(ch.health_status).text }}
              </div>
              <div
                v-if="ch.has_api_secret === false"
                class="rounded-md bg-amber-950/40 px-2.5 py-1 text-amber-400 ring-1 ring-amber-900/40"
              >
                {{ t('adminChannels.noSecret') }}
              </div>
            </div>

            <div v-if="ch.api_base_url" class="text-xs">
              <span class="text-slate-500">{{ t('adminChannels.thBase') }}</span>
              <p class="mt-1 break-all font-mono text-[11px] leading-relaxed text-cyan-400/90" :title="ch.api_base_url">
                {{ truncateUrl(ch.api_base_url, 56) }}
              </p>
            </div>
            <div v-else class="text-xs text-slate-600">{{ t('adminChannels.baseDefault') }}</div>

            <div>
              <span class="text-xs font-medium text-slate-500">{{ t('adminChannels.modelsLabel', { n: ch.models?.length ?? 0 }) }}</span>
              <div
                class="mt-2 max-h-28 overflow-y-auto rounded-xl border border-slate-800/80 bg-slate-950/50 p-2"
              >
                <div v-if="!ch.models?.length" class="py-2 text-center text-xs text-slate-600">{{ t('adminChannels.noModelIds') }}</div>
                <div v-else class="flex flex-wrap gap-1.5">
                  <span
                    v-for="m in ch.models"
                    :key="m"
                    class="inline-flex max-w-full truncate rounded-md bg-slate-800/90 px-2 py-0.5 font-mono text-[11px] text-slate-300 ring-1 ring-slate-700/80"
                    :title="m"
                  >
                    {{ m }}
                  </span>
                </div>
              </div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
