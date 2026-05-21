<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AdminActionBanner from '../components/admin/AdminActionBanner.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { adminHttpErrMessage, useAdminActionFeedback } from '../composables/useAdminActionFeedback'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()
const { flashMsg, flashIsError, busy, clearFlash, flashInfo, flashError, flashSuccess, runAction, flashScrollId } =
  useAdminActionFeedback()

type Ch = {
  id: number
  group_id: number
  channel_type: string
  channel_name: string
  models: string[]
  model_alias?: Record<string, string>
  enabled: boolean
  api_base_url?: string
  auth_type?: string
  health_status?: number
  priority?: number
  weight?: number
  concurrency?: number
  schedulable?: boolean
  error_count?: number
  last_error_msg?: string
  has_api_secret?: boolean
  model_upstream_probe?: Record<string, unknown>
}

type G = { id: number; name: string }

type ChannelTypeMeta = {
  channel_type: string
  display_name?: string
  default_base_url?: string
  capabilities?: string[]
  video_protocol?: string
  admin_hints?: string[]
}

const channels = ref<Ch[]>([])
/** 首次列表请求结束后为 true，避免首屏先显示「共 0 条」与空状态 */
const channelsListReady = ref(false)
const channelTypesMeta = ref<ChannelTypeMeta[]>([])
const { busy: pageLoading, visible: pageLoadingUi, run: runPageLoad } = useSmoothLoading()
const groups = ref<G[]>([])
const form = reactive({
  group_id: 0,
  channel_type: 'openai',
  channel_name: '',
  api_key: '',
  api_base_url: '',
  modelsStr: 'gpt-4o,gpt-4o-mini',
  modelAliasStr: '',
  priority: 0,
  weight: 100,
})

const editOpen = ref(false)
const editForm = reactive({
  id: 0,
  channel_name: '',
  api_key: '',
  api_base_url: '',
  modelsStr: '',
  modelAliasStr: '',
  priority: 0,
  weight: 100,
  concurrency: 2,
  enabled: true,
  schedulable: true,
  health_status: 1,
  error_count: 0,
  last_error_msg: '',
  reset_error_count: false,
})

function parseModelAliasJson(raw: string): Record<string, string> | null {
  const t = raw.trim()
  if (!t) return {}
  try {
    const o = JSON.parse(t) as unknown
    if (!o || typeof o !== 'object' || Array.isArray(o)) return null
    const out: Record<string, string> = {}
    for (const [k, v] of Object.entries(o as Record<string, unknown>)) {
      const key = k.trim()
      const val = typeof v === 'string' ? v.trim() : ''
      if (key && val) out[key] = val
    }
    return out
  } catch {
    return null
  }
}

function formatModelAliasJson(m?: Record<string, string>): string {
  if (!m || !Object.keys(m).length) return ''
  return JSON.stringify(m, null, 2)
}

const presetLoading = ref(false)
const fetchLoading = ref(false)
const probingId = ref<number | null>(null)
const editSaving = ref(false)
const editModalMsg = ref('')
const editModalIsError = ref(false)
/** 表单内联提示（预设/拉取）；保存类操作用顶部 AdminActionBanner */
const presetHint = ref('')

const activeTypeMeta = computed(() => {
  const ct = form.channel_type.trim().toLowerCase()
  return channelTypesMeta.value.find((x) => x.channel_type.toLowerCase() === ct) ?? null
})

const baseUrlPlaceholder = computed(() => {
  const d = activeTypeMeta.value?.default_base_url?.trim()
  if (d) return t('adminChannels.baseUrlDefaultPh', { url: d })
  return t('adminChannels.baseUrlPh')
})

/** 分组下拉用字符串 v-model，避免动态 option 与 v-model.number 在部分浏览器下不同步 */
const groupIdStr = ref('')

function defaultBaseURLForType(channelType: string): string {
  const ct = channelType.trim().toLowerCase()
  const m = channelTypesMeta.value.find((x) => x.channel_type.toLowerCase() === ct)
  return m?.default_base_url?.trim() ?? ''
}

/** 拉取上游模型列表用的 Base URL：空则用类型默认；火山时若仍是其它厂商地址则改用方舟默认。 */
function baseURLForUpstreamFetch(): string {
  const typed = form.api_base_url.trim()
  const def = defaultBaseURLForType(form.channel_type)
  if (!typed) return def
  const ct = form.channel_type.trim().toLowerCase()
  if (ct === 'volcengine' && def && !typed.includes('volces.com')) {
    return def
  }
  return typed
}

function fillDefaultBaseURL() {
  const d = defaultBaseURLForType(form.channel_type)
  if (d) form.api_base_url = d
}

async function loadChannelTypes() {
  try {
    const { data } = await http.get<Envelope<{ items: ChannelTypeMeta[] }>>('/api/admin/channel-types')
    if (!envelopeOk(data)) return
    const inner = pickEnvelopeData<{ items?: unknown }>(data)
    const raw = inner?.items ?? (data as Record<string, unknown>).items
    if (!Array.isArray(raw)) return
    const out: ChannelTypeMeta[] = []
    for (const x of raw) {
      if (!x || typeof x !== 'object') continue
      const o = x as Record<string, unknown>
      const channel_type = typeof o.channel_type === 'string' ? o.channel_type.trim() : ''
      if (!channel_type) continue
      out.push({
        channel_type,
        display_name: typeof o.display_name === 'string' ? o.display_name : undefined,
        default_base_url: typeof o.default_base_url === 'string' ? o.default_base_url : undefined,
        capabilities: coerceStringList(o.capabilities),
        video_protocol: typeof o.video_protocol === 'string' ? o.video_protocol : undefined,
        admin_hints: coerceStringList(o.admin_hints),
      })
    }
    channelTypesMeta.value = out
  } catch {
    /* 元数据加载失败时仍可用硬编码下拉与空默认 URL */
  }
}

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
    const hint = t('adminChannels.presetMerged', { n: pick.length })
    presetHint.value = hint
    flashInfo(hint)
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
    flashError(e)
  } finally {
    presetLoading.value = false
  }
}

function httpErrMessage(e: unknown): string {
  return adminHttpErrMessage(e, t('adminCommon.requestFail'))
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
    const base = baseURLForUpstreamFetch()
    const { data } = await http.post<Envelope<{ model_ids: string[]; count: number }>>(
      '/api/admin/channel-models/fetch-upstream',
      {
        api_key: form.api_key,
        api_base_url: base || undefined,
        channel_type: form.channel_type,
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
    const hint = t('adminChannels.upstreamMerged', { n: ids.length })
    presetHint.value = hint
    flashInfo(hint)
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
    flashError(e)
  } finally {
    fetchLoading.value = false
  }
}

function clearModels() {
  form.modelsStr = ''
  presetHint.value = t('adminChannels.cleared')
}

async function load() {
  try {
    await runPageLoad(async () => {
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
    })
  } finally {
    channelsListReady.value = true
  }
}

async function createCh() {
  const models = form.modelsStr.split(',').map((s) => s.trim()).filter(Boolean)
  await runAction({
    validate: () => {
      if (!form.channel_name.trim()) return t('adminChannels.needName')
      if (!form.api_key.trim()) return t('adminChannels.needApiKeyCreate')
      if (!models.length) return t('adminChannels.needModels')
      if (!form.group_id) return t('adminChannels.needGroup')
      if (parseModelAliasJson(form.modelAliasStr) === null) return t('adminChannels.modelAliasInvalid')
      return null
    },
    successMessage: t('adminChannels.channelCreated'),
    action: async () => {
      const model_alias = parseModelAliasJson(form.modelAliasStr) ?? {}
      const { data } = await http.post<Envelope<unknown>>('/api/admin/channels', {
        group_id: form.group_id,
        channel_type: form.channel_type,
        channel_name: form.channel_name.trim(),
        auth_type: 'api_key',
        api_key: form.api_key,
        api_base_url: form.api_base_url,
        models,
        model_alias,
        priority: form.priority,
        weight: form.weight,
        concurrency: 2,
        enabled: true,
      })
      if (!envelopeOk(data)) throw new Error(data.message || t('adminCommon.requestFail'))
      form.api_key = ''
    },
    onSuccess: () => load(),
  })
}

function openEdit(ch: Ch) {
  editForm.id = ch.id
  editForm.channel_name = ch.channel_name || ''
  editForm.api_key = ''
  editForm.api_base_url = ch.api_base_url || ''
  editForm.modelsStr = (ch.models || []).join(',')
  editForm.modelAliasStr = formatModelAliasJson(ch.model_alias)
  editForm.priority = ch.priority ?? 0
  editForm.weight = ch.weight ?? 100
  editForm.concurrency = ch.concurrency ?? 2
  editForm.enabled = ch.enabled !== false
  editForm.schedulable = ch.schedulable !== false
  editForm.health_status = ch.health_status ?? 1
  editForm.error_count = ch.error_count ?? 0
  editForm.last_error_msg = ch.last_error_msg || ''
  editForm.reset_error_count = false
  editModalMsg.value = ''
  editModalIsError.value = false
  editOpen.value = true
}

async function saveEdit() {
  const models = editForm.modelsStr.split(',').map((s) => s.trim()).filter(Boolean)
  editSaving.value = true
  clearFlash()
  editModalMsg.value = ''
  const model_alias = parseModelAliasJson(editForm.modelAliasStr)
  if (model_alias === null) {
    editModalMsg.value = t('adminChannels.modelAliasInvalid')
    editModalIsError.value = true
    editSaving.value = false
    return
  }
  try {
    const body: Record<string, unknown> = {
      channel_name: editForm.channel_name.trim(),
      api_base_url: editForm.api_base_url,
      models,
      model_alias,
      priority: editForm.priority,
      weight: editForm.weight,
      concurrency: editForm.concurrency,
      enabled: editForm.enabled,
      schedulable: editForm.schedulable,
      health_status: editForm.health_status,
      reset_error_count: editForm.reset_error_count,
    }
    if (editForm.api_key.trim()) body.api_key = editForm.api_key.trim()
    const { data } = await http.put<Envelope<unknown>>(`/api/admin/channels/${editForm.id}`, body)
    if (!envelopeOk(data)) throw new Error(data.message || t('adminCommon.requestFail'))
    editOpen.value = false
    flashSuccess(t('adminChannels.channelUpdated'))
    await load()
  } catch (e: unknown) {
    editModalIsError.value = true
    editModalMsg.value = httpErrMessage(e)
    flashError(e)
  } finally {
    editSaving.value = false
  }
}

async function remove(id: number) {
  if (!confirm(t('adminChannels.confirmDelete'))) return
  await runAction({
    successMessage: t('adminChannels.channelDeleted'),
    action: async () => {
      const { data } = await http.delete<Envelope<unknown>>(`/api/admin/channels/${id}`)
      if (data && typeof data === 'object' && 'code' in data && !envelopeOk(data)) {
        throw new Error((data as Envelope<unknown>).message || t('adminCommon.requestFail'))
      }
    },
    onSuccess: () => load(),
  })
}

onMounted(async () => {
  await loadChannelTypes()
  await load()
})

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
    volcengine: 'bg-violet-500/15 text-violet-300 ring-violet-500/30',
    kling: 'bg-rose-500/15 text-rose-300 ring-rose-500/30',
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

type ProbeRow = { in_upstream_list?: boolean; checked_at?: string; error?: string }

function probeMap(ch: Ch): Record<string, unknown> | undefined {
  const m = ch.model_upstream_probe
  return m && typeof m === 'object' ? m : undefined
}

function channelProbeErrMsg(ch: Ch): string {
  const raw = probeMap(ch)?.['_channel']
  if (!raw || typeof raw !== 'object') return ''
  const err = (raw as Record<string, unknown>).error
  return typeof err === 'string' ? err.trim() : ''
}

function modelProbeBadge(ch: Ch, modelId: string): { text: string; title: string; cls: string } {
  const pm = probeMap(ch)
  const raw = pm?.[modelId]
  const row = raw && typeof raw === 'object' ? (raw as ProbeRow) : null
  const chErr = channelProbeErrMsg(ch)
  if (!row && chErr) {
    return {
      text: t('adminChannels.probeUnknown'),
      title: chErr,
      cls: 'th-admin-probe--warn',
    }
  }
  if (!row) {
    return { text: t('adminChannels.probeUnknown'), title: '', cls: 'th-admin-probe--muted' }
  }
  if (typeof row.error === 'string' && row.error.trim()) {
    return { text: t('adminChannels.probeErr'), title: row.error, cls: 'th-admin-probe--warn' }
  }
  const inList = Boolean(row.in_upstream_list)
  const checked = typeof row.checked_at === 'string' ? row.checked_at.trim() : ''
  const titleBase = checked ? t('adminChannels.probeTitleChecked', { t: checked }) : ''
  const disclaimer = t('adminSystem.hintUpstreamProbeListDisclaimer')
  if (inList) {
    return { text: t('adminChannels.probeInList'), title: titleBase, cls: 'th-admin-probe--ok' }
  }
  return {
    text: t('adminChannels.probeNotInList'),
    title: titleBase ? `${titleBase}\n${disclaimer}` : disclaimer,
    cls: 'th-admin-probe--fail',
  }
}

async function probeChannelNow(ch: Ch) {
  probingId.value = ch.id
  presetHint.value = ''
  clearFlash()
  try {
    const { data } = await http.post<Envelope<{ model_upstream_probe?: Record<string, unknown> }>>(
      `/api/admin/channels/${ch.id}/probe-upstream-models`,
      {},
    )
    if (!envelopeOk(data)) throw new Error((data as { message?: string }).message || t('adminCommon.requestFail'))
    flashMsg.value = t('adminChannels.probeDone')
    flashIsError.value = false
    await load()
  } catch (e: unknown) {
    presetHint.value = httpErrMessage(e)
    flashError(e)
  } finally {
    probingId.value = null
  }
}
</script>

<template>
  <div class="mx-auto max-w-6xl space-y-10 pb-10">
    <section>
      <div class="mb-6 flex flex-col gap-1 border-b border-slate-800/80 pb-4">
        <h1 class="text-2xl font-bold tracking-tight text-white">{{ t('adminChannels.pageTitle') }}</h1>
        <p class="text-sm text-slate-500">{{ t('adminChannels.pageDesc') }}</p>
      </div>

      <AdminActionBanner
        :message="flashMsg"
        :is-error="flashIsError"
        :loading="busy"
        :scroll-id="flashScrollId"
        class="mb-4"
      />

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
                <option
                  v-for="tm in channelTypesMeta.length ? channelTypesMeta : [
                    { channel_type: 'openai' },
                    { channel_type: 'deepseek' },
                    { channel_type: 'claude' },
                    { channel_type: 'gemini' },
                    { channel_type: 'volcengine' },
                    { channel_type: 'kling' },
                  ]"
                  :key="tm.channel_type"
                  :value="tm.channel_type"
                >
                  {{ tm.display_name ? `${tm.display_name} (${tm.channel_type})` : tm.channel_type }}
                </option>
              </select>
            </div>
          </div>
          <div
            v-if="activeTypeMeta && (activeTypeMeta.admin_hints?.length || activeTypeMeta.capabilities?.length)"
            class="rounded-lg border border-slate-800/80 bg-slate-950/60 px-3 py-2 text-xs text-slate-400"
          >
            <p v-if="activeTypeMeta.capabilities?.length" class="mb-1 text-slate-500">
              {{ t('adminChannels.channelCapabilities', { caps: activeTypeMeta.capabilities.join(', ') }) }}
            </p>
            <p class="mb-1 font-medium text-slate-500">{{ t('adminChannels.channelTypeHints') }}</p>
            <ul class="list-inside list-disc space-y-0.5 leading-relaxed text-slate-500">
              <li v-for="(hint, hi) in activeTypeMeta.admin_hints" :key="hi">{{ hint }}</li>
            </ul>
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
                :placeholder="baseUrlPlaceholder"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              />
              <div class="flex flex-wrap items-center gap-2">
                <span class="text-xs text-slate-600">{{ t('adminChannels.baseUrlHint') }}</span>
                <button
                  v-if="defaultBaseURLForType(form.channel_type)"
                  type="button"
                  class="text-xs text-amber-400/90 hover:text-amber-300"
                  @click="fillDefaultBaseURL"
                >
                  {{ t('adminChannels.btnFillDefaultUrl') }}
                </button>
              </div>
            </label>
          </div>
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.priority') }}</span>
              <input v-model.number="form.priority" type="number" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm" />
            </label>
            <label class="flex flex-col gap-1.5">
              <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.weight') }}</span>
              <input v-model.number="form.weight" type="number" min="1" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-sm" />
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
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminChannels.lblModelAlias') }}</span>
            <textarea
              v-model="form.modelAliasStr"
              rows="4"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs leading-relaxed text-slate-200 outline-none focus:border-violet-500/40 focus:ring-2 focus:ring-violet-500/15"
              :placeholder="t('adminChannels.modelAliasPh')"
            />
            <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminChannels.modelAliasHint') }}</p>
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
            class="th-btn-primary-flat w-full max-w-xs px-4 py-2.5 text-sm sm:w-auto disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="busy"
            @click="createCh"
          >
            {{ busy ? t('adminChannels.savingChannel') : t('adminChannels.saveChannel') }}
          </button>
        </div>
      </div>
    </section>

    <section>
      <div class="mb-6 flex flex-col justify-between gap-4 border-b border-slate-800/80 pb-4 sm:flex-row sm:items-end">
        <div>
          <h2 class="text-xl font-semibold text-white">{{ t('adminChannels.listTitle') }}</h2>
          <p v-if="!channelsListReady" class="mt-1 text-sm text-slate-500">{{ t('adminChannels.listLoading') }}</p>
          <p v-else class="mt-1 text-sm text-slate-500">{{ t('adminChannels.listCount', { n: channels.length }) }}</p>
        </div>
        <button
          type="button"
          class="self-start rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-400 hover:border-slate-600 hover:text-slate-300 disabled:cursor-not-allowed disabled:opacity-50"
          :disabled="pageLoading"
          @click="load"
        >
          {{ pageLoading ? t('adminChannels.listRefreshing') : t('adminCommon.refresh') }}
        </button>
      </div>

      <ThTableSkeleton v-if="!channelsListReady" :rows="4" :columns="3" class="mb-4" />

      <div
        v-else-if="channels.length === 0"
        class="flex flex-col items-center justify-center rounded-2xl border border-dashed border-slate-700 bg-slate-900/40 px-8 py-16 text-center"
      >
        <ThLoadingState v-if="pageLoading" class="py-4" />
        <template v-else>
          <div class="mb-3 rounded-full bg-slate-800 p-3 ring-1 ring-slate-700">
            <svg class="h-8 w-8 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
            </svg>
          </div>
          <p class="text-sm font-medium text-slate-400">{{ t('adminChannels.emptyTitle') }}</p>
          <p class="mt-1 max-w-sm text-xs text-slate-600">{{ t('adminChannels.emptyDesc') }}</p>
        </template>
      </div>

      <div v-else class="relative grid gap-4 lg:grid-cols-2">
        <ThLoadingState v-if="pageLoadingUi" variant="overlay" class="!rounded-2xl" />
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
            <div class="flex shrink-0 gap-1">
              <button
                type="button"
                class="rounded-lg px-2.5 py-1.5 text-xs font-medium text-slate-300 hover:bg-slate-800"
                @click="openEdit(ch)"
              >
                {{ t('adminChannels.edit') }}
              </button>
              <button
                type="button"
                class="rounded-lg px-2.5 py-1.5 text-xs font-medium text-red-400 opacity-80 transition hover:bg-red-950/50 hover:opacity-100"
                @click="remove(ch.id)"
              >
                {{ t('adminCommon.delete') }}
              </button>
            </div>
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
              <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
                <span class="text-xs font-medium text-slate-500">{{ t('adminChannels.modelsLabel', { n: ch.models?.length ?? 0 }) }}</span>
                <button
                  type="button"
                  class="shrink-0 rounded-md border border-slate-600/90 bg-slate-800/80 px-2 py-1 text-[10px] font-medium text-slate-300 hover:border-cyan-500/30 hover:text-cyan-200/90 disabled:opacity-40"
                  :disabled="probingId === ch.id"
                  @click="probeChannelNow(ch)"
                >
                  {{ probingId === ch.id ? t('adminChannels.probeProbing') : t('adminChannels.probeNow') }}
                </button>
              </div>
              <p v-if="channelProbeErrMsg(ch)" class="mb-2 text-[11px] leading-snug text-amber-300/90">
                {{ t('adminChannels.probeChannelErr', { msg: channelProbeErrMsg(ch) }) }}
              </p>
              <div
                class="mt-2 max-h-28 overflow-y-auto rounded-xl border border-slate-800/80 bg-slate-950/50 p-2"
              >
                <div v-if="!ch.models?.length" class="py-2 text-center text-xs text-slate-600">{{ t('adminChannels.noModelIds') }}</div>
                <div v-else class="flex flex-wrap gap-1.5">
                  <span
                    v-for="m in ch.models"
                    :key="m"
                    class="inline-flex max-w-[11rem] flex-col gap-0.5 rounded-md bg-slate-800/90 px-2 py-0.5 ring-1 ring-slate-700/80"
                  >
                    <span class="truncate font-mono text-[11px] text-slate-300" :title="m">{{ m }}</span>
                    <span
                      class="inline-flex max-w-full shrink-0 self-start truncate rounded px-1 py-px text-[9px] font-semibold"
                      :class="modelProbeBadge(ch, m).cls"
                      :title="modelProbeBadge(ch, m).title"
                    >
                      {{ modelProbeBadge(ch, m).text }}
                    </span>
                  </span>
                </div>
              </div>
            </div>
          </div>
        </article>
      </div>
    </section>

    <Teleport to="body">
      <div v-if="editOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4" @click.self="editOpen = false">
        <div class="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl border border-slate-700 bg-slate-900 p-6">
          <h3 class="mb-4 text-lg font-semibold text-white">{{ t('adminChannels.editModalTitle') }}</h3>
          <div class="space-y-3 text-sm">
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminChannels.lblChannelName') }}</span>
              <input v-model="editForm.channel_name" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminChannels.lblApiKey') }}</span>
              <input v-model="editForm.api_key" type="password" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" :placeholder="t('adminChannels.editKeyPh')" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminChannels.lblBaseUrl') }}</span>
              <input v-model="editForm.api_base_url" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminChannels.lblModels') }}</span>
              <textarea v-model="editForm.modelsStr" rows="3" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminChannels.lblModelAlias') }}</span>
              <textarea
                v-model="editForm.modelAliasStr"
                rows="4"
                class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs leading-relaxed"
                :placeholder="t('adminChannels.modelAliasPh')"
              />
              <p class="text-[10px] leading-relaxed text-slate-500">{{ t('adminChannels.modelAliasHint') }}</p>
            </label>
            <div class="grid grid-cols-2 gap-3">
              <label class="flex flex-col gap-1 text-xs text-slate-400">
                {{ t('adminChannels.priority') }}
                <input v-model.number="editForm.priority" type="number" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono" />
              </label>
              <label class="flex flex-col gap-1 text-xs text-slate-400">
                {{ t('adminChannels.weight') }}
                <input v-model.number="editForm.weight" type="number" min="1" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono" />
              </label>
            </div>
            <label class="flex flex-col gap-1 text-xs text-slate-400">
              {{ t('adminChannels.concurrency') }}
              <input v-model.number="editForm.concurrency" type="number" min="1" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono" />
            </label>
            <label class="flex flex-col gap-1 text-xs text-slate-400">
              {{ t('adminChannels.lblHealth') }}
              <select v-model.number="editForm.health_status" class="rounded border border-slate-700 bg-slate-950 px-2 py-1">
                <option :value="1">{{ t('adminChannels.healthOk') }}</option>
                <option :value="2">{{ t('adminChannels.healthBad') }}</option>
                <option :value="3">{{ t('adminChannels.healthPaused') }}</option>
              </select>
            </label>
            <label class="flex items-center gap-2 text-xs text-slate-300">
              <input v-model="editForm.enabled" type="checkbox" /> {{ t('adminCommon.enabled') }}
            </label>
            <label class="flex items-center gap-2 text-xs text-slate-300">
              <input v-model="editForm.schedulable" type="checkbox" /> {{ t('adminChannels.schedulable') }}
            </label>
            <p v-if="editForm.error_count" class="text-xs text-slate-500">
              {{ t('adminChannels.errorCount', { n: editForm.error_count }) }}
              <span v-if="editForm.last_error_msg" class="block text-red-400/80">{{ editForm.last_error_msg }}</span>
            </p>
            <label class="flex items-center gap-2 text-xs text-amber-300">
              <input v-model="editForm.reset_error_count" type="checkbox" /> {{ t('adminChannels.resetErrorCount') }}
            </label>
          </div>
          <p
            v-if="editModalMsg"
            class="mt-4 rounded-lg border px-3 py-2 text-xs"
            :class="editModalIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
          >
            {{ editModalMsg }}
          </p>
          <div class="mt-6 flex justify-end gap-2">
            <button type="button" class="rounded-lg px-4 py-2 text-sm text-slate-400" :disabled="editSaving" @click="editOpen = false">{{ t('adminCommon.cancel') }}</button>
            <button type="button" class="th-btn-primary-flat px-4 py-2 text-sm disabled:cursor-not-allowed disabled:opacity-50" :disabled="editSaving" @click="saveEdit">
              {{ editSaving ? t('adminChannels.savingChannel') : t('adminCommon.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
