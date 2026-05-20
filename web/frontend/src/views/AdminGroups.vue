<script setup lang="ts">
import axios from 'axios'
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useI18n } from 'vue-i18n'
import { http, type Envelope } from '../api/http'
import {
  comboboxAnchorKey,
  resolveComboboxAnchor,
  type ComboboxAnchorVariant,
} from '../utils/comboboxAnchor'

const { t } = useI18n()

type GroupRow = {
  id: number
  name: string
  description?: string
  rate_multiplier: number
  rpm_limit: number
  subscription_type?: string
  default_model?: string
  enabled: boolean
  routing_strategy?: string
  routing_options?: Record<string, unknown>
  model_routing?: Record<string, number[]>
}

type ModelRoutingRow = { model: string; order: string }

type OverlapChannel = { id: number; name: string; healthy: boolean }

type ModelOverlapItem = {
  model: string
  channels: OverlapChannel[]
  pinned?: number[]
}

type ChBrief = { id: number; group_id: number; channel_name?: string; channel_type?: string }

type RateRow = {
  id: number
  model_name: string
  input_rate: number
  output_rate: number
  enabled: boolean
  billable?: boolean
}

const groups = ref<GroupRow[]>([])
const allChannels = ref<ChBrief[]>([])
const msg = ref('')
const msgIsError = ref(false)

/** 模型倍率保存结果：贴在表格下方、按钮上方，避免用户滚动在底部时看不到页面顶部的全局提示 */
const ratesFlashGid = ref<number | null>(null)
const ratesFlashMsg = ref('')
const ratesFlashIsError = ref(false)

function clearRatesFlash() {
  ratesFlashGid.value = null
  ratesFlashMsg.value = ''
  ratesFlashIsError.value = false
}

function flashRatesSuccess(gid: number, text: string) {
  ratesFlashGid.value = gid
  ratesFlashMsg.value = text
  ratesFlashIsError.value = false
}

function flashRatesError(gid: number, e: unknown) {
  ratesFlashGid.value = gid
  ratesFlashMsg.value = httpErr(e)
  ratesFlashIsError.value = true
}

function scrollRatesFlashIntoView(gid: number) {
  void nextTick(() => {
    document.getElementById(`rates-flash-${gid}`)?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  })
}

function clearFlash() {
  msg.value = ''
}

function flashSuccess(text: string) {
  msgIsError.value = false
  msg.value = text
}

function flashError(e: unknown) {
  msgIsError.value = true
  msg.value = httpErr(e)
}
const expandedId = ref<number | null>(null)
const ratesByGroup = ref<Record<number, RateRow[]>>({})
const ratesDraft = ref<RateRow[]>([])
const { busy: ratesLoading, visible: ratesLoadingUi, run: runRatesLoad } = useSmoothLoading()
/** 模型倍率表「模型 ID」建议（目录 + 分组渠道预设 + 当前表内已有 ID）；下拉用自定义列表以适配深浅色主题 */
const modelSuggestIds = ref<string[]>([])
const {
  busy: modelSuggestLoading,
  visible: modelSuggestLoadingUi,
  run: runModelSuggestLoad,
} = useSmoothLoading({ showDelay: 280, minVisible: 300 })
const { busy: pageLoading, visible: pageLoadingUi, run: runPageLoad } = useSmoothLoading()
let cachedCatalogModelIds: string[] | null = null

/** 当前展开哪一行的模型 ID 建议列表（替代 datalist，避免系统原生浅色弹层） */
const openModelSuggestRow = ref<number | null>(null)

const comboboxAnchorEls = new Map<string, HTMLElement>()

function setComboboxRef(rowIdx: number, el: unknown, variant: ComboboxAnchorVariant = 'desktop') {
  const key = comboboxAnchorKey(rowIdx, variant)
  if (el instanceof HTMLElement) comboboxAnchorEls.set(key, el)
  else comboboxAnchorEls.delete(key)
}

const modelSuggestPanelStyle = ref<Record<string, string>>({})

function updateModelSuggestPanelLayout() {
  const i = openModelSuggestRow.value
  if (i === null) {
    modelSuggestPanelStyle.value = {}
    return
  }
  const anchor = resolveComboboxAnchor(comboboxAnchorEls, i)
  if (!anchor) return
  const r = anchor.getBoundingClientRect()
  const margin = 6
  const edgePad = 10
  const vh = window.innerHeight
  const vw = window.innerWidth
  const maxDesired = 320
  const w = Math.min(Math.max(160, r.width), vw - 16)
  const left = Math.min(Math.max(8, r.left), vw - w - 8)
  const belowSpace = vh - r.bottom - margin - edgePad
  const aboveSpace = r.top - margin - edgePad
  /** 上方剩余空间更大则向上展开（上拉） */
  const openAbove = aboveSpace > belowSpace
  const maxH = openAbove
    ? Math.min(maxDesired, Math.max(56, aboveSpace))
    : Math.min(maxDesired, Math.max(56, belowSpace))
  const base: Record<string, string> = {
    position: 'fixed',
    left: `${left}px`,
    width: `${w}px`,
    zIndex: '9999',
    maxHeight: `${Math.floor(maxH)}px`,
    overflowY: 'auto',
    overscrollBehavior: 'contain',
  }
  if (openAbove) {
    modelSuggestPanelStyle.value = {
      ...base,
      top: 'auto',
      bottom: `${vh - r.top + margin}px`,
    }
  } else {
    modelSuggestPanelStyle.value = {
      ...base,
      top: `${r.bottom + margin}px`,
      bottom: 'auto',
    }
  }
}

const suggestScrollOpts: AddEventListenerOptions = { passive: true }
let suggestScrollParents: HTMLElement[] = []

function scrollableAncestors(start: HTMLElement | null): HTMLElement[] {
  const out: HTMLElement[] = []
  let el = start?.parentElement ?? null
  while (el) {
    const st = window.getComputedStyle(el)
    const oy = st.overflowY
    if ((oy === 'auto' || oy === 'scroll') && el.scrollHeight > el.clientHeight + 2) {
      out.push(el)
    }
    el = el.parentElement
  }
  return out
}

function attachSuggestScrollReposition() {
  detachSuggestScrollReposition()
  const i = openModelSuggestRow.value
  if (i === null) return
  const anchor = resolveComboboxAnchor(comboboxAnchorEls, i)
  if (!anchor) return
  suggestScrollParents = scrollableAncestors(anchor)
  for (const p of suggestScrollParents) {
    p.addEventListener('scroll', updateModelSuggestPanelLayout, suggestScrollOpts)
  }
}

function detachSuggestScrollReposition() {
  for (const p of suggestScrollParents) {
    p.removeEventListener('scroll', updateModelSuggestPanelLayout, suggestScrollOpts)
  }
  suggestScrollParents = []
}

function filterModelSuggestIds(query: string): string[] {
  const all = modelSuggestIds.value
  const s = query.trim().toLowerCase()
  if (!s) return all.slice(0, 120)
  const out: string[] = []
  for (const id of all) {
    if (id.toLowerCase().includes(s)) {
      out.push(id)
      if (out.length >= 200) break
    }
  }
  return out
}

const activeModelSuggestQuery = computed(() => {
  const i = openModelSuggestRow.value
  if (i === null) return ''
  return ratesDraft.value[i]?.model_name ?? ''
})

const filteredOpenModelSuggestIds = computed(() => filterModelSuggestIds(activeModelSuggestQuery.value))

let suggestLayoutTimer: ReturnType<typeof setTimeout> | null = null
function scheduleSuggestLayout() {
  if (openModelSuggestRow.value === null) return
  if (suggestLayoutTimer) clearTimeout(suggestLayoutTimer)
  suggestLayoutTimer = setTimeout(() => {
    suggestLayoutTimer = null
    updateModelSuggestPanelLayout()
  }, 40)
}

function onModelSuggestDocMouseDown(e: MouseEvent) {
  if (openModelSuggestRow.value === null) return
  const t = e.target as HTMLElement | null
  if (t?.closest?.('.th-model-combobox')) return
  if (t?.closest?.('.th-model-combobox-floating')) return
  openModelSuggestRow.value = null
}

watch(openModelSuggestRow, (v) => {
  if (v !== null) {
    document.addEventListener('mousedown', onModelSuggestDocMouseDown, false)
    window.addEventListener('resize', updateModelSuggestPanelLayout)
    void nextTick(() => {
      updateModelSuggestPanelLayout()
      attachSuggestScrollReposition()
      requestAnimationFrame(() => updateModelSuggestPanelLayout())
    })
  } else {
    document.removeEventListener('mousedown', onModelSuggestDocMouseDown, false)
    window.removeEventListener('resize', updateModelSuggestPanelLayout)
    detachSuggestScrollReposition()
    modelSuggestPanelStyle.value = {}
  }
})

onUnmounted(() => {
  if (suggestLayoutTimer) clearTimeout(suggestLayoutTimer)
  document.removeEventListener('mousedown', onModelSuggestDocMouseDown, false)
  window.removeEventListener('resize', updateModelSuggestPanelLayout)
  detachSuggestScrollReposition()
})

function pickModelRateId(rowIdx: number, id: string) {
  const row = ratesDraft.value[rowIdx]
  if (row) row.model_name = id
  openModelSuggestRow.value = null
}

function pickOpenModelSuggestId(id: string) {
  const i = openModelSuggestRow.value
  if (i === null) return
  pickModelRateId(i, id)
}

function toggleModelSuggestRow(rowIdx: number) {
  if (openModelSuggestRow.value === rowIdx) {
    openModelSuggestRow.value = null
    return
  }
  openModelSuggestRow.value = rowIdx
  void nextTick(() => updateModelSuggestPanelLayout())
}

function focusModelSuggestRow(rowIdx: number) {
  openModelSuggestRow.value = rowIdx
  void nextTick(() => updateModelSuggestPanelLayout())
}

async function fetchCatalogModelIds(): Promise<string[]> {
  if (cachedCatalogModelIds !== null) return cachedCatalogModelIds
  const set = new Set<string>()
  try {
    for (let page = 1; page <= 25; page++) {
      const { data } = await http.get<Envelope<{ items?: Array<{ model_id?: string }> }>>('/api/admin/model-catalog/entries', {
        params: { page, page_size: 200 },
      })
      if (data.code !== 0) break
      const items = data.data.items || []
      for (const it of items) {
        const id = typeof it.model_id === 'string' ? it.model_id.trim() : ''
        if (id) set.add(id)
      }
      if (items.length < 200) break
    }
  } catch {
    /* 目录不可用时仍允许手输 */
  }
  cachedCatalogModelIds = Array.from(set).sort((a, b) => a.localeCompare(b))
  return cachedCatalogModelIds
}

async function loadModelRateSuggestions(gid: number) {
  await runModelSuggestLoad(async () => {
    const set = new Set<string>()
    for (const r of ratesDraft.value) {
      const m = r.model_name.trim()
      if (m) set.add(m)
    }
    for (const r of ratesByGroup.value[gid] || []) {
      const m = (r.model_name || '').trim()
      if (m) set.add(m)
    }
    const catalog = await fetchCatalogModelIds()
    for (const id of catalog) set.add(id)

    const types = new Set<string>()
    for (const ch of channelsForGroup(gid)) {
      const ct = (ch.channel_type || '').trim().toLowerCase()
      if (ct) types.add(ct)
    }
    await Promise.all(
      [...types].map(async (channelType) => {
        try {
          const { data } = await http.get<Envelope<{ merged?: string[] }>>('/api/admin/channel-model-presets', {
            params: { channel_type: channelType },
          })
          if (data.code !== 0) return
          for (const id of data.data.merged || []) {
            const m = typeof id === 'string' ? id.trim() : ''
            if (m) set.add(m)
          }
        } catch {
          /* 单个渠道类型失败可忽略 */
        }
      }),
    )
    modelSuggestIds.value = Array.from(set).sort((a, b) => a.localeCompare(b))
  })
}

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
  routingStrategy: '' as string,
  overrideRoutingOptions: false,
  failoverEnabled: false,
  failoverMaxAttempts: 2,
  stickyEnabled: true,
  modelRoutingRows: [] as ModelRoutingRow[],
})

const overlaps = ref<ModelOverlapItem[]>([])
const { busy: overlapsLoading, visible: overlapsLoadingUi, run: runOverlapsLoad } = useSmoothLoading()
const overlapPins = reactive<Record<string, number | ''>>({})
const showAdvancedModelRouting = ref(false)

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

async function load() {
  clearFlash()
  await runPageLoad(async () => {
    const [gRes, cRes] = await Promise.all([
      http.get<Envelope<{ items: GroupRow[] }>>('/api/admin/groups'),
      http.get<Envelope<{ items: ChBrief[] }>>('/api/admin/channels'),
    ])
    if (gRes.data.code !== 0) throw new Error(gRes.data.message || t('adminCommon.loadFail'))
    groups.value = gRes.data.data.items || []
    if (cRes.data.code === 0) allChannels.value = cRes.data.data.items || []
  }).catch((e: unknown) => {
    flashError(e)
  })
}

function channelsForGroup(gid: number): ChBrief[] {
  return allChannels.value.filter((c) => c.group_id === gid)
}

function modelRoutingFromRows(rows: ModelRoutingRow[]): Record<string, number[]> {
  const out: Record<string, number[]> = {}
  for (const r of rows) {
    const model = r.model.trim()
    if (!model) continue
    const ids = r.order
      .split(',')
      .map((s) => Number(s.trim()))
      .filter((n) => Number.isFinite(n) && n > 0)
    if (ids.length) out[model] = ids
  }
  return out
}

function modelRoutingToRows(mr?: Record<string, number[]>, excludeModels?: Set<string>): ModelRoutingRow[] {
  if (!mr) return []
  return Object.entries(mr)
    .filter(([model]) => !excludeModels?.has(model))
    .map(([model, ids]) => ({
      model,
      order: (ids || []).join(','),
    }))
}

function overlapModelSet(items: ModelOverlapItem[]): Set<string> {
  return new Set(items.map((it) => it.model))
}

async function loadOverlaps(groupId: number, mr?: Record<string, number[]>) {
  overlaps.value = []
  for (const k of Object.keys(overlapPins)) {
    delete overlapPins[k]
  }
  await runOverlapsLoad(async () => {
  try {
    const { data } = await http.get<Envelope<{ items: ModelOverlapItem[] }>>(
      `/api/admin/groups/${groupId}/model-overlaps`,
    )
    if (data.code !== 0) throw new Error(data.message || t('adminGroups.loadOverlapsFail'))
    overlaps.value = data.data.items || []
    const excl = overlapModelSet(overlaps.value)
    for (const it of overlaps.value) {
      const pin = it.pinned?.[0]
      overlapPins[it.model] = pin && pin > 0 ? pin : ''
    }
    editForm.modelRoutingRows = modelRoutingToRows(mr, excl)
  } catch (e: unknown) {
    overlaps.value = []
    editForm.modelRoutingRows = modelRoutingToRows(mr)
    flashError(e)
  }
  })
}

function buildModelRoutingForSave(): Record<string, number[]> {
  const out: Record<string, number[]> = {}
  for (const it of overlaps.value) {
    const pin = overlapPins[it.model]
    if (typeof pin === 'number' && pin > 0) {
      out[it.model] = [pin]
    }
  }
  const advanced = modelRoutingFromRows(editForm.modelRoutingRows)
  for (const [model, ids] of Object.entries(advanced)) {
    if (!ids.length) continue
    const existing = out[model]
    if (existing) {
      const seen = new Set(existing)
      for (const id of ids) {
        if (!seen.has(id)) {
          existing.push(id)
          seen.add(id)
        }
      }
      out[model] = existing
    } else {
      out[model] = ids
    }
  }
  return out
}

function overlapChannelLabel(ch: OverlapChannel): string {
  const name = (ch.name || '').trim()
  const health = ch.healthy ? '' : ` (${t('adminGroups.channelUnhealthy')})`
  return name ? `#${ch.id} ${name}${health}` : `#${ch.id}${health}`
}

async function createGroup() {
  clearFlash()
  if (!createForm.name.trim()) {
    msgIsError.value = true
    msg.value = t('adminGroups.nameRequired')
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
    flashSuccess(t('adminGroups.created'))
  } catch (e: unknown) {
    flashError(e)
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
  editForm.routingStrategy = g.routing_strategy || ''
  const opts = g.routing_options || {}
  editForm.overrideRoutingOptions = Object.keys(opts).length > 0
  editForm.failoverEnabled = Boolean(opts.failover_enabled)
  editForm.failoverMaxAttempts = Number(opts.failover_max_attempts) || 2
  editForm.stickyEnabled = opts.sticky_enabled !== false
  editForm.modelRoutingRows = []
  showAdvancedModelRouting.value = false
  editOpen.value = true
  void loadOverlaps(g.id, g.model_routing)
}

function addModelRoutingRow() {
  editForm.modelRoutingRows.push({ model: '', order: '' })
}

async function saveEdit() {
  clearFlash()
  try {
    const body: Record<string, unknown> = {
      name: editForm.name.trim(),
      description: editForm.description.trim(),
      rate_multiplier: editForm.rate_multiplier,
      rpm_limit: editForm.rpm_limit,
      default_model: editForm.default_model.trim() || '',
      enabled: editForm.enabled,
    }
    body.routing_strategy = editForm.routingStrategy.trim() || ''
    if (editForm.overrideRoutingOptions) {
      body.routing_options = {
        failover_enabled: editForm.failoverEnabled,
        failover_max_attempts: editForm.failoverMaxAttempts,
        sticky_enabled: editForm.stickyEnabled,
      }
    } else {
      body.routing_options = {}
    }
    body.model_routing = buildModelRoutingForSave()
    await http.put(`/api/admin/groups/${editForm.id}`, body)
    editOpen.value = false
    await load()
    flashSuccess(t('adminGroups.saved'))
  } catch (e: unknown) {
    flashError(e)
  }
}

async function remove(g: GroupRow) {
  if (!confirm(t('adminGroups.confirmDelete', { name: g.name }))) return
  clearFlash()
  try {
    await http.delete(`/api/admin/groups/${g.id}`)
    await load()
    if (expandedId.value === g.id) expandedId.value = null
    flashSuccess(t('adminGroups.deleted'))
  } catch (e: unknown) {
    flashError(e)
  }
}

async function reloadRatesDraft(gid: number) {
  const { data } = await http.get<Envelope<{ items: RateRow[] }>>(`/api/admin/groups/${gid}/model-rates`)
  if (data.code !== 0) throw new Error(data.message || t('adminGroups.loadRatesFail'))
  const list = data.data.items || []
  ratesByGroup.value[gid] = list
  ratesDraft.value = list.map((r) => ({ ...r, billable: r.billable !== false }))
}

async function toggleExpand(id: number) {
  if (expandedId.value === id) {
    expandedId.value = null
    modelSuggestIds.value = []
    openModelSuggestRow.value = null
    clearRatesFlash()
    return
  }
  clearRatesFlash()
  expandedId.value = id
  ratesDraft.value = []
  modelSuggestIds.value = []
  openModelSuggestRow.value = null
  await runRatesLoad(async () => {
    try {
      await reloadRatesDraft(id)
      void loadModelRateSuggestions(id)
    } catch (e: unknown) {
      flashError(e)
      expandedId.value = null
    }
  })
}

function addRateDraft() {
  ratesDraft.value.push({
    id: 0,
    model_name: '',
    input_rate: 1,
    output_rate: 1,
    enabled: true,
    billable: true,
  })
}

async function saveRates(gid: number) {
  clearFlash()
  clearRatesFlash()
  const rates = ratesDraft.value
    .filter((r) => r.model_name.trim())
    .map((r) => ({
      model_name: r.model_name.trim(),
      input_rate: r.input_rate,
      output_rate: r.output_rate,
      enabled: r.enabled,
      billable: r.billable !== false,
    }))
  try {
    await http.put(`/api/admin/groups/${gid}/model-rates`, { rates })
    await reloadRatesDraft(gid)
    void loadModelRateSuggestions(gid)
    flashRatesSuccess(gid, t('adminGroups.ratesSaved'))
    scrollRatesFlashIntoView(gid)
  } catch (e: unknown) {
    flashRatesError(gid, e)
    scrollRatesFlashIntoView(gid)
  }
}

onMounted(load)
</script>

<template>
  <div class="mx-auto max-w-6xl min-w-0 space-y-10 pb-10">
    <header class="border-b border-slate-800/80 pb-6">
      <h1 class="text-2xl font-bold tracking-tight text-white">{{ t('adminGroups.title') }}</h1>
      <p class="mt-1 text-sm text-slate-500">{{ t('adminGroups.intro') }}</p>
    </header>

    <p
      v-if="msg"
      class="rounded-lg border px-4 py-2 text-sm"
      :class="msgIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
    >
      {{ msg }}
    </p>

    <section class="rounded-2xl border border-slate-800/90 bg-gradient-to-b from-slate-900/90 to-slate-950 p-6 shadow-xl ring-1 ring-white/5">
      <h2 class="mb-4 text-sm font-semibold uppercase tracking-wider text-slate-500">{{ t('adminGroups.createSection') }}</h2>
      <div class="grid max-w-3xl gap-4 text-sm">
        <div class="grid gap-4 sm:grid-cols-2">
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminGroups.lblName') }} <span class="text-red-400">{{ t('adminGroups.nameRequiredMark') }}</span></span>
            <input
              v-model="createForm.name"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
              :placeholder="t('adminGroups.namePh')"
            />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminGroups.defModelOpt') }}</span>
            <input
              v-model="createForm.default_model"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
            />
          </label>
        </div>
        <label class="flex flex-col gap-1.5">
          <span class="text-xs font-medium text-slate-400">{{ t('adminGroups.lblDesc') }}</span>
          <input
            v-model="createForm.description"
            class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none focus:border-amber-500/50 focus:ring-2 focus:ring-amber-500/20"
          />
        </label>
        <div class="grid gap-4 sm:grid-cols-3">
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminGroups.lblRateMul') }}</span>
            <input
              v-model.number="createForm.rate_multiplier"
              type="number"
              step="0.01"
              min="0"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono outline-none"
            />
          </label>
          <label class="flex flex-col gap-1.5">
            <span class="text-xs font-medium text-slate-400">{{ t('adminGroups.lblRpm') }}</span>
            <input
              v-model.number="createForm.rpm_limit"
              type="number"
              min="0"
              class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono outline-none"
            />
          </label>
          <label class="flex items-end gap-2 pb-2">
            <input id="cg-en" v-model="createForm.enabled" type="checkbox" class="rounded border-slate-600" />
            <label for="cg-en" class="text-xs text-slate-400">{{ t('adminCommon.enabled') }}</label>
          </label>
        </div>
        <button
          type="button"
          class="th-btn-primary-flat w-full max-w-xs px-4 py-2.5 text-sm sm:w-auto"
          @click="createGroup"
        >
          {{ t('adminGroups.createBtn') }}
        </button>
      </div>
    </section>

    <section>
      <div class="mb-4 flex items-end justify-between border-b border-slate-800/80 pb-4">
        <div>
          <h2 class="text-xl font-semibold text-white">{{ t('adminGroups.listTitle') }}</h2>
          <p class="mt-1 text-sm text-slate-500">{{ t('adminGroups.listCount', { n: groups.length }) }}</p>
        </div>
        <button
          type="button"
          class="rounded-lg border border-slate-700 px-3 py-1.5 text-xs text-slate-400 hover:border-slate-600 hover:text-slate-300"
          @click="load"
        >
          {{ t('adminCommon.refresh') }}
        </button>
      </div>

      <div v-if="groups.length === 0" class="rounded-2xl border border-dashed border-slate-700 py-16 text-center text-slate-500">
        {{ t('adminGroups.empty') }}
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
                  class="th-badge-apikey th-badge-apikey--disabled"
                >
                  {{ t('adminCommon.disabled') }}
                </span>
                <span
                  v-else
                  class="th-badge-apikey th-badge-apikey--enabled"
                >
                  {{ t('adminCommon.enabled') }}
                </span>
              </div>
              <p v-if="g.description" class="mt-2 text-sm text-slate-400">{{ g.description }}</p>
              <dl class="mt-3 flex flex-wrap gap-x-6 gap-y-1 text-xs text-slate-500">
                <div>ID <span class="font-mono text-slate-400">{{ g.id }}</span></div>
                <div>{{ t('adminGroups.cardMul') }} <span class="font-mono text-slate-300">{{ g.rate_multiplier }}×</span></div>
                <div>{{ t('adminGroups.cardRpm') }} <span class="font-mono text-slate-300">{{ g.rpm_limit || t('adminGroups.rpmUnlimited') }}</span></div>
                <div v-if="g.default_model">
                  {{ t('adminGroups.cardDef') }} <span class="font-mono text-slate-400">{{ g.default_model }}</span>
                </div>
              </dl>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-200"
                @click="toggleExpand(g.id)"
              >
                {{ expandedId === g.id ? t('adminGroups.collapseRates') : t('adminGroups.expandRates') }}
              </button>
              <button
                type="button"
                class="th-ghost-hover rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-200"
                @click="openEdit(g)"
              >
                {{ t('adminGroups.edit') }}
              </button>
              <button
                type="button"
                class="th-link-danger-plain rounded-lg px-3 py-1.5 text-xs hover:bg-red-500/10"
                @click="remove(g)"
              >
                {{ t('adminCommon.delete') }}
              </button>
            </div>
          </div>

          <div v-if="expandedId === g.id" class="border-t border-slate-800/80 bg-slate-950/40 px-5 py-4">
            <ThLoadingState v-if="ratesLoadingUi" variant="inline" />
            <template v-else>
              <p class="mb-1 text-xs text-slate-500">{{ t('adminGroups.ratesHint') }}</p>
              <p class="mb-3 text-[11px] leading-relaxed text-slate-500">
                <span v-if="modelSuggestLoadingUi">{{ t('adminGroups.modelSuggestLoading') }}</span>
                <span v-else>{{ t('adminGroups.modelSuggestHint') }}</span>
              </p>
              <div class="hidden overflow-x-auto rounded-xl border border-slate-800 lg:block">
                <table class="w-full border-collapse text-left text-xs">
                  <thead class="bg-slate-900 text-slate-500">
                    <tr>
                      <th class="align-middle px-3 py-2">{{ t('adminGroups.thModelId') }}</th>
                      <th class="align-middle px-3 py-2">{{ t('adminGroups.thIn') }}</th>
                      <th class="align-middle px-3 py-2">{{ t('adminGroups.thOut') }}</th>
                      <th class="align-middle px-3 py-2 text-center">{{ t('adminGroups.thEn') }}</th>
                      <th class="align-middle px-3 py-2 text-center">{{ t('adminGroups.thNoBill') }}</th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-slate-800">
                    <tr v-for="(r, idx) in ratesDraft" :key="idx">
                      <td class="align-middle px-2 py-2 max-w-[min(100%,22rem)] overflow-hidden">
                        <div
                          class="th-model-combobox relative max-w-full min-w-0"
                          :ref="(el) => setComboboxRef(idx, el, 'desktop')"
                        >
                          <div class="flex max-w-full min-w-0 gap-1">
                            <input
                              v-model="r.model_name"
                              class="th-input h-9 min-w-0 w-0 flex-1 py-1 font-mono text-xs leading-normal"
                              autocomplete="off"
                              spellcheck="false"
                              :placeholder="t('adminGroups.modelIdPh')"
                              @focus="focusModelSuggestRow(idx)"
                              @input="scheduleSuggestLayout"
                            />
                            <button
                              type="button"
                              class="th-input flex h-9 w-9 min-w-9 shrink-0 items-center justify-center !w-9 !max-w-9 !px-0"
                              :title="t('adminGroups.modelSuggestToggle')"
                              :aria-expanded="openModelSuggestRow === idx"
                              @mousedown.prevent
                              @click="toggleModelSuggestRow(idx)"
                            >
                              <svg class="h-4 w-4 opacity-70" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                <path
                                  fill-rule="evenodd"
                                  d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.21 8.27a.75.75 0 01.02-1.06z"
                                  clip-rule="evenodd"
                                />
                              </svg>
                            </button>
                          </div>
                        </div>
                      </td>
                      <td class="align-middle px-2 py-2">
                        <input
                          v-model.number="r.input_rate"
                          type="number"
                          step="0.01"
                          class="th-input h-9 w-20 py-1 font-mono text-xs leading-normal"
                        />
                      </td>
                      <td class="align-middle px-2 py-2">
                        <input
                          v-model.number="r.output_rate"
                          type="number"
                          step="0.01"
                          class="th-input h-9 w-20 py-1 font-mono text-xs leading-normal"
                        />
                      </td>
                      <td class="align-middle px-2 py-2 text-center">
                        <input
                          v-model="r.enabled"
                          type="checkbox"
                          class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                        />
                      </td>
                      <td class="align-middle px-2 py-2 text-center">
                        <input
                          type="checkbox"
                          class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                          :checked="r.billable === false"
                          :title="t('adminGroups.thNoBill')"
                          @change="r.billable = !($event.target as HTMLInputElement).checked"
                        />
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div class="space-y-3 lg:hidden">
                <article
                  v-for="(r, idx) in ratesDraft"
                  :key="`rate-m-${g.id}-${idx}`"
                  class="rounded-xl border border-slate-800 bg-slate-950/50 p-3 text-xs"
                >
                  <p class="mb-2 text-[10px] font-semibold uppercase tracking-wide text-slate-500">{{ t('adminGroups.thModelId') }}</p>
                  <div class="th-model-combobox relative max-w-full min-w-0" :ref="(el) => setComboboxRef(idx, el, 'mobile')">
                    <div class="flex max-w-full min-w-0 gap-1">
                      <input
                        v-model="r.model_name"
                        class="th-input h-9 min-w-0 w-0 flex-1 py-1 font-mono text-xs leading-normal"
                        autocomplete="off"
                        spellcheck="false"
                        :placeholder="t('adminGroups.modelIdPh')"
                        @focus="focusModelSuggestRow(idx)"
                        @input="scheduleSuggestLayout"
                      />
                      <button
                        type="button"
                        class="th-input flex h-9 w-9 min-w-9 shrink-0 items-center justify-center !w-9 !max-w-9 !px-0"
                        :title="t('adminGroups.modelSuggestToggle')"
                        :aria-expanded="openModelSuggestRow === idx"
                        @mousedown.prevent
                        @click="toggleModelSuggestRow(idx)"
                      >
                        <svg class="h-4 w-4 opacity-70" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                          <path
                            fill-rule="evenodd"
                            d="M5.23 7.21a.75.75 0 011.06.02L10 11.17l3.71-3.94a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0L5.21 8.27a.75.75 0 01.02-1.06z"
                            clip-rule="evenodd"
                          />
                        </svg>
                      </button>
                    </div>
                  </div>
                  <div class="mt-3 grid grid-cols-2 gap-3">
                    <label class="flex flex-col gap-1">
                      <span class="text-[10px] text-slate-500">{{ t('adminGroups.thIn') }}</span>
                      <input v-model.number="r.input_rate" type="number" step="0.01" class="th-input h-9 w-full py-1 font-mono text-xs" />
                    </label>
                    <label class="flex flex-col gap-1">
                      <span class="text-[10px] text-slate-500">{{ t('adminGroups.thOut') }}</span>
                      <input v-model.number="r.output_rate" type="number" step="0.01" class="th-input h-9 w-full py-1 font-mono text-xs" />
                    </label>
                  </div>
                  <div class="mt-3 flex flex-wrap gap-4">
                    <label class="flex items-center gap-2 text-slate-400">
                      <input v-model="r.enabled" type="checkbox" class="h-4 w-4 accent-cyan-600" />
                      {{ t('adminGroups.thEn') }}
                    </label>
                    <label class="flex items-center gap-2 text-slate-400">
                      <input
                        type="checkbox"
                        class="h-4 w-4 accent-cyan-600"
                        :checked="r.billable === false"
                        :title="t('adminGroups.thNoBill')"
                        @change="r.billable = !($event.target as HTMLInputElement).checked"
                      />
                      {{ t('adminGroups.thNoBill') }}
                    </label>
                  </div>
                </article>
              </div>
              <p
                v-if="ratesFlashGid === g.id && ratesFlashMsg"
                :id="`rates-flash-${g.id}`"
                class="mt-3 rounded-lg border px-3 py-2 text-xs"
                :class="ratesFlashIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
                role="status"
                aria-live="polite"
              >
                {{ ratesFlashMsg }}
              </p>
              <div class="mt-3 flex flex-wrap gap-2">
                <button
                  type="button"
                  class="rounded-lg border border-slate-600 px-3 py-1.5 text-xs text-slate-300 hover:bg-slate-800"
                  @click="addRateDraft"
                >
                  {{ t('adminGroups.addRow') }}
                </button>
                <button
                  type="button"
                  class="th-btn-primary-flat px-4 py-1.5 text-xs"
                  @click="saveRates(g.id)"
                >
                  {{ t('adminGroups.saveRatesBtn') }}
                </button>
              </div>
            </template>
          </div>
        </article>
      </div>
    </section>

    <Teleport to="body">
      <div
        v-if="editOpen"
        class="fixed inset-0 z-50 flex cursor-pointer items-center justify-center bg-black/60 p-4 backdrop-blur-sm"
        @click.self="editOpen = false"
      >
        <div
          class="max-h-[90vh] w-full max-w-2xl cursor-default overflow-y-auto rounded-2xl border border-slate-700 bg-slate-900 p-6 shadow-2xl"
          role="dialog"
          aria-modal="true"
        >
          <h3 class="mb-4 text-lg font-semibold text-white">{{ t('adminGroups.editModalTitle') }}</h3>
          <div class="space-y-4 text-sm">
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminGroups.lblName') }}</span>
              <input v-model="editForm.name" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminGroups.lblDesc') }}</span>
              <input v-model="editForm.description" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2" />
            </label>
            <label class="flex flex-col gap-1">
              <span class="text-xs text-slate-400">{{ t('adminGroups.lblDefModel') }}</span>
              <input v-model="editForm.default_model" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono text-xs" />
            </label>
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminGroups.lblRateMul') }}</span>
                <input v-model.number="editForm.rate_multiplier" type="number" step="0.01" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
              <label class="flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminGroups.lblRpmShort') }}</span>
                <input v-model.number="editForm.rpm_limit" type="number" min="0" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 font-mono" />
              </label>
            </div>
            <label class="flex items-center gap-2">
              <input v-model="editForm.enabled" type="checkbox" />
              <span class="text-xs text-slate-400">{{ t('adminCommon.enabled') }}</span>
            </label>
            <div class="border-t border-slate-800 pt-4">
              <p class="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{{ t('adminGroups.secRouting') }}</p>
              <label class="mb-3 flex flex-col gap-1">
                <span class="text-xs text-slate-400">{{ t('adminGroups.lblRoutingStrategy') }}</span>
                <select v-model="editForm.routingStrategy" class="rounded-lg border border-slate-700 bg-slate-950 px-3 py-2 text-sm">
                  <option value="">{{ t('adminGroups.routingInherit') }}</option>
                  <option value="weighted_random">{{ t('adminGroups.routingWeighted') }}</option>
                  <option value="priority">{{ t('adminGroups.routingPriority') }}</option>
                  <option value="fixed_order">{{ t('adminGroups.routingFixedOrder') }}</option>
                </select>
              </label>
              <label class="mb-2 flex items-center gap-2">
                <input v-model="editForm.overrideRoutingOptions" type="checkbox" />
                <span class="text-xs text-slate-400">{{ t('adminGroups.overrideRoutingOptions') }}</span>
              </label>
              <div v-if="editForm.overrideRoutingOptions" class="mb-3 space-y-2 rounded-lg border border-slate-800 bg-slate-950/60 p-3">
                <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminGroups.hintFailover') }}</p>
                <label class="flex items-center gap-2 text-xs text-slate-300">
                  <input v-model="editForm.failoverEnabled" type="checkbox" />
                  {{ t('adminGroups.lblFailover') }}
                </label>
                <label v-if="editForm.failoverEnabled" class="flex flex-col gap-1 text-xs text-slate-400">
                  {{ t('adminGroups.lblFailoverMax') }}
                  <input v-model.number="editForm.failoverMaxAttempts" type="number" min="1" max="5" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono" />
                </label>
                <label class="flex items-center gap-2 text-xs text-slate-300">
                  <input v-model="editForm.stickyEnabled" type="checkbox" />
                  {{ t('adminGroups.lblSticky') }}
                </label>
              </div>
              <details class="th-admin-disclosure mt-4 border-t border-slate-800 pt-4">
                <summary>
                  <span class="text-xs font-semibold uppercase tracking-wide text-slate-500">{{ t('adminGroups.secModelPin') }}</span>
                  <span class="flex items-center gap-2 text-[11px] text-slate-500">
                    <ThLoadingState
                      v-if="overlapsLoadingUi"
                      variant="inline"
                      size="sm"
                      :label="t('adminGroups.loadingOverlaps')"
                    />
                    <span
                      v-else-if="overlaps.length > 0"
                      class="rounded-full bg-slate-800/90 px-2 py-0.5 font-mono tabular-nums text-slate-400"
                    >{{ t('adminGroups.modelPinCount', { n: overlaps.length }) }}</span>
                    <span v-else class="text-slate-600">{{ t('adminGroups.noOverlaps') }}</span>
                    <svg class="th-admin-disclosure__chevron h-4 w-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                      <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.24 4.5a.75.75 0 01-1.08 0l-4.24-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
                    </svg>
                  </span>
                </summary>
                <div class="th-admin-disclosure__body space-y-2">
                  <p class="text-xs text-slate-500">{{ t('adminGroups.modelPinHint') }}</p>
                <ThLoadingState v-if="overlapsLoadingUi" variant="inline" size="sm" :label="t('adminGroups.loadingOverlaps')" />
                <p v-else-if="overlaps.length === 0" class="text-xs text-slate-600">{{ t('adminGroups.noOverlaps') }}</p>
                <template v-else>
                    <div class="th-admin-disclosure__scroll thin-scrollbar hidden overflow-x-auto rounded-lg border border-slate-800 lg:block">
                      <table class="w-full text-left text-xs">
                        <thead class="sticky top-0 z-[1] bg-slate-950 text-slate-500">
                      <tr>
                        <th class="px-2 py-2">{{ t('adminGroups.thOverlapModel') }}</th>
                        <th class="px-2 py-2">{{ t('adminGroups.thOverlapChannels') }}</th>
                        <th class="px-2 py-2">{{ t('adminGroups.thOverlapPin') }}</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-slate-800">
                      <tr v-for="it in overlaps" :key="it.model">
                        <td class="px-2 py-2 font-mono text-slate-300">{{ it.model }}</td>
                        <td class="px-2 py-2 text-slate-400">
                          <span v-for="(ch, ci) in it.channels" :key="ch.id">
                            <span v-if="ci > 0"> · </span>
                            <span :class="ch.healthy ? '' : 'text-amber-500/90'">{{ overlapChannelLabel(ch) }}</span>
                          </span>
                        </td>
                        <td class="px-2 py-2">
                          <select
                            v-model="overlapPins[it.model]"
                            class="w-full min-w-[10rem] rounded border border-slate-700 bg-slate-950 px-2 py-1"
                          >
                            <option value="">{{ t('adminGroups.pinInherit') }}</option>
                            <option v-for="ch in it.channels" :key="ch.id" :value="ch.id">
                              {{ overlapChannelLabel(ch) }}
                            </option>
                          </select>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                    <ul class="th-admin-disclosure__scroll thin-scrollbar space-y-2 lg:hidden">
                  <li
                    v-for="it in overlaps"
                    :key="`ov-m-${it.model}`"
                    class="rounded-lg border border-slate-800 bg-slate-950/50 p-3 text-xs"
                  >
                    <p class="font-mono text-slate-300">{{ it.model }}</p>
                    <p class="mt-2 text-[10px] uppercase tracking-wide text-slate-500">{{ t('adminGroups.thOverlapChannels') }}</p>
                    <p class="mt-0.5 text-slate-400">
                      <span v-for="(ch, ci) in it.channels" :key="ch.id">
                        <span v-if="ci > 0"> · </span>
                        <span :class="ch.healthy ? '' : 'text-amber-500/90'">{{ overlapChannelLabel(ch) }}</span>
                      </span>
                    </p>
                    <label class="mt-2 block">
                      <span class="text-[10px] uppercase tracking-wide text-slate-500">{{ t('adminGroups.thOverlapPin') }}</span>
                      <select
                        v-model="overlapPins[it.model]"
                        class="th-input mt-1 w-full text-xs"
                      >
                        <option value="">{{ t('adminGroups.pinInherit') }}</option>
                        <option v-for="ch in it.channels" :key="ch.id" :value="ch.id">
                          {{ overlapChannelLabel(ch) }}
                        </option>
                      </select>
                    </label>
                  </li>
                    </ul>
                  </template>
                </div>
              </details>
              <div class="mt-3 space-y-2">
                <button
                  type="button"
                  class="text-xs text-amber-400 hover:underline"
                  @click="showAdvancedModelRouting = !showAdvancedModelRouting"
                >
                  {{ showAdvancedModelRouting ? t('adminGroups.hideAdvancedRouting') : t('adminGroups.showAdvancedRouting') }}
                </button>
                <div v-if="showAdvancedModelRouting" class="space-y-2">
                  <p class="text-xs text-slate-500">{{ t('adminGroups.modelRoutingHint') }}</p>
                  <div v-for="(row, idx) in editForm.modelRoutingRows" :key="idx" class="grid gap-2 sm:grid-cols-2">
                    <input v-model="row.model" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono text-xs" :placeholder="t('adminGroups.phModelId')" />
                    <input v-model="row.order" class="rounded border border-slate-700 bg-slate-950 px-2 py-1 font-mono text-xs" :placeholder="t('adminGroups.phChannelOrder')" />
                  </div>
                  <button type="button" class="text-xs text-amber-400 hover:underline" @click="addModelRoutingRow">{{ t('adminGroups.addModelRoutingRow') }}</button>
                  <p class="text-[11px] text-slate-600">{{ t('adminGroups.channelIdsHint', { ids: channelsForGroup(editForm.id).map((c) => c.id).join(', ') || '—' }) }}</p>
                </div>
              </div>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-2">
            <button type="button" class="rounded-lg px-4 py-2 text-sm text-slate-400 hover:bg-slate-800" @click="editOpen = false">
              {{ t('adminCommon.cancel') }}
            </button>
            <button
              type="button"
              class="th-btn-primary-flat px-4 py-2 text-sm"
              @click="saveEdit"
            >
              {{ t('adminCommon.save') }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>
    <Teleport to="body">
      <ul
        v-if="openModelSuggestRow !== null && !modelSuggestLoading"
        class="th-model-combobox-list th-model-combobox-floating thin-scrollbar"
        :style="modelSuggestPanelStyle"
        role="listbox"
        tabindex="-1"
        @wheel.stop
      >
        <template v-if="filteredOpenModelSuggestIds.length">
          <li
            v-for="opt in filteredOpenModelSuggestIds"
            :key="opt"
            class="th-model-combobox-item"
            role="option"
            @mousedown.prevent="pickOpenModelSuggestId(opt)"
          >
            {{ opt }}
          </li>
        </template>
        <li v-else class="th-model-combobox-item th-model-combobox-item--muted cursor-default">
          {{ t('adminGroups.modelSuggestEmpty') }}
        </li>
      </ul>
    </Teleport>
  </div>
</template>
