<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AdminModelChannelPickModal from '../components/admin/AdminModelChannelPickModal.vue'
import ThLoadingState from '../components/ui/ThLoadingState.vue'
import ThTableSkeleton from '../components/ui/ThTableSkeleton.vue'
import { useAdminModelIdCombobox } from '../composables/useAdminModelIdCombobox'
import { useSmoothLoading } from '../composables/useSmoothLoading'
import { useQuotaPer1kUpstreamField } from '../composables/useQuotaPer1kUpstreamField'
import { http, type Envelope } from '../api/http'
import {
  billingQuotaWithMarkup,
  formatUpstreamPrice,
  upstreamPriceFromQuotaPer1K,
} from '../utils/upstreamQuotaDerive'

const { t } = useI18n()

type DraftRow = {
  id?: number
  /** 多渠道 CSV 导入预览：行所属渠道 */
  channel_id?: number
  model_name: string
  /** 用户「添加一行」写入的模型，不参与服务端分页，渲染在表格底部 */
  isCustom?: boolean
  /** 合并视图行：模型名只读，不可从本页删除（无库记录时） */
  locked?: boolean
  code_in_ref?: number
  code_out_ref?: number
  sources?: string[]
  enabled: boolean
  quota_in_per_1k: string
  quota_out_per_1k: string
  /** 行内成本加成 %；空=全站默认 */
  markup_percent: string
  billing_mode: string
  quota_per_request: string
  input_rate: number
  output_rate: number
}

type ApiRow = {
  id: number
  model_name: string
  enabled: boolean
  quota_in_per_1k?: number | null
  quota_out_per_1k?: number | null
  input_rate: number
  output_rate: number
  markup_percent?: number | null
  billing_mode?: string | null
  quota_per_request?: number | null
}

type WorkspaceRow = {
  model_name: string
  code_in_per_1k: number
  code_out_per_1k: number
  sources: string[]
  global?: ApiRow | null
  channel?: ApiRow | null
}

type ChannelListItem = {
  id: number
  channel_name?: string
  channel_type: string
  enabled?: boolean
}

/** 空字符串 = 全站默认；正整数 = 按渠道覆盖 */
const selectedChannelId = ref<number | ''>('')
const channelOptions = ref<ChannelListItem[]>([])
const channelsLoading = ref(false)

const activeChannelId = computed(() => {
  const id = selectedChannelId.value
  if (id === '' || id == null) return 0
  const n = typeof id === 'number' ? id : Number(id)
  return Number.isFinite(n) && n > 0 ? Math.trunc(n) : 0
})

const isChannelScope = computed(() => activeChannelId.value > 0)

const pricingWorkspaceScope = computed<'global' | number>(() =>
  isChannelScope.value ? activeChannelId.value : 'global',
)

const selectedChannelLabel = computed(() => {
  if (!isChannelScope.value) return ''
  const id = activeChannelId.value
  const ch = channelOptions.value.find((c) => c.id === id)
  if (!ch) return `#${id}`
  const name = (ch.channel_name || '').trim()
  return name ? `${name} (#${id})` : `#${id} · ${ch.channel_type}`
})

const customDrafts = ref<DraftRow[]>([])
const workspacePageDrafts = ref<DraftRow[]>([])
/** 当前页工作区行加载时的快照，用于撤销未入库行的本地编辑 */
const workspaceSnapshotByModel = ref<Map<string, DraftRow>>(new Map())
const rows = ref<DraftRow[]>([])
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const searchDraft = ref('')
const searchApplied = ref('')
const { busy, visible: listUi, run: runListLoad } = useSmoothLoading()
const pageSaving = ref(false)
const batchSaving = ref(false)
const exporting = ref(false)
const exportingTemplate = ref(false)
const dataMenuOpen = ref(false)
const dataMenuRef = ref<HTMLElement | null>(null)
const templateMenuOpen = ref(false)
const templateMenuRef = ref<HTMLElement | null>(null)
const importing = ref(false)
const importPreviewOpen = ref(false)
const importPreviewRows = ref<DraftRow[]>([])
const importPreviewSubmitting = ref(false)
const csvFileInput = ref<HTMLInputElement | null>(null)
type DeriveUnitPreset = 'quota' | 'usd_1m' | 'usd_1k' | 'cny_1m' | 'cny_1k'
const deriveUnitPreset = ref<DeriveUnitPreset>('usd_1m')
/** 已生效的毛利 %（驱动表格反算与失焦换算） */
const deriveMargin = ref('0')
/** 高级面板内编辑中的毛利 %，点「应用」后写入 deriveMargin */
const deriveMarginDraft = ref('0')
const deriveAdvancedRef = ref<HTMLDetailsElement | null>(null)

const codeRefShowQuotaOnly = computed(() => deriveUnitPreset.value === 'quota')

const deriveCurrency = computed<'cny' | 'usd'>(() =>
  deriveUnitPreset.value.startsWith('cny') ? 'cny' : 'usd',
)
const deriveDenom = computed(() => (deriveUnitPreset.value.endsWith('_1k') ? 1000 : 1_000_000))
const sitePerYuan = ref<number | null>(null)
const sitePerUsd = ref<number | null>(null)
const siteCnyPerUsd = ref<number | null>(null)
const flashMsg = ref('')
const flashIsError = ref(false)
/** 成功/提示自动关闭（毫秒）；错误稍长便于阅读 */
const FLASH_DISMISS_MS = 3200
const FLASH_ERROR_DISMISS_MS = 5500
let flashDismissTimer: ReturnType<typeof setTimeout> | undefined

/** 勾选行索引（与 `rows` 对齐）；`load` 后会清空 */
const selectedRows = ref<number[]>([])

function isRowSelected(idx: number): boolean {
  return selectedRows.value.includes(idx)
}

const rowSavingIdx = ref<number | null>(null)
const savingFull = ref(false)
const busyResetPage = ref(false)
const busyResetSelected = ref(false)
const busyResetFull = ref(false)

const {
  modelSuggestLoading,
  modelSuggestLoadingUi,
  modelSuggestTotal,
  openModelSuggestRow,
  modelSuggestPanelStyle,
  setComboboxRef,
  filteredOpenModelSuggestIds,
  suggestListTruncated,
  channelCountForModel,
  scheduleSuggestLayout,
  toggleModelSuggestRow,
  focusModelSuggestRow,
  pickOpenModelSuggestId,
  loadModelSuggestions,
  channelPickOpen,
  channelPickModelId,
  channelPickChannels,
  channelPickSelectedId,
  confirmChannelPick,
  cancelChannelPick,
} = useAdminModelIdCombobox(rows, {
  workspaceScope: pricingWorkspaceScope,
  enableChannelPick: true,
})

const totalPages = computed(() => {
  if (total.value <= 0) return 1
  return Math.max(1, Math.ceil(total.value / pageSize.value))
})

function rebuildRows() {
  rows.value = [...workspacePageDrafts.value, ...customDrafts.value]
}

function cloneDraftRow(d: DraftRow): DraftRow {
  return {
    ...d,
    sources: d.sources ? [...d.sources] : [],
  }
}

function syncWorkspaceSnapshotsFromDrafts() {
  const m = new Map<string, DraftRow>()
  for (const d of workspacePageDrafts.value) {
    const key = d.model_name.trim()
    if (key) m.set(key, cloneDraftRow(d))
  }
  workspaceSnapshotByModel.value = m
}

function rowDiffersFromSnapshot(r: DraftRow): boolean {
  const key = r.model_name.trim()
  if (!key) return false
  const snap = workspaceSnapshotByModel.value.get(key)
  if (!snap) return rowHasNonDefaultOverrides(r)
  return (
    r.enabled !== snap.enabled ||
    r.quota_in_per_1k !== snap.quota_in_per_1k ||
    r.quota_out_per_1k !== snap.quota_out_per_1k ||
    r.markup_percent !== snap.markup_percent ||
    r.billing_mode !== snap.billing_mode ||
    r.quota_per_request !== snap.quota_per_request ||
    r.input_rate !== snap.input_rate ||
    r.output_rate !== snap.output_rate
  )
}

function clearFlashDismissTimer() {
  if (flashDismissTimer !== undefined) {
    clearTimeout(flashDismissTimer)
    flashDismissTimer = undefined
  }
}

function clearFlash() {
  clearFlashDismissTimer()
  flashMsg.value = ''
  flashIsError.value = false
}

function showFlash(text: string, isError: boolean, dismissMs?: number) {
  clearFlashDismissTimer()
  flashIsError.value = isError
  flashMsg.value = text
  const ms = dismissMs ?? (isError ? FLASH_ERROR_DISMISS_MS : FLASH_DISMISS_MS)
  const shown = text
  flashDismissTimer = setTimeout(() => {
    flashDismissTimer = undefined
    if (flashMsg.value === shown) clearFlash()
  }, ms)
}

function flashSuccess(text: string) {
  showFlash(text, false)
}

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function flashError(e: unknown) {
  showFlash(httpErr(e), true)
}

function flashWarn(text: string) {
  showFlash(text, true)
}

function workspaceItemToDraft(w: WorkspaceRow): DraftRow {
  const g = isChannelScope.value ? w.channel : w.global
  return {
    id: g?.id,
    model_name: w.model_name,
    isCustom: false,
    locked: true,
    code_in_ref: w.code_in_per_1k,
    code_out_ref: w.code_out_per_1k,
    sources: Array.isArray(w.sources) ? w.sources : [],
    enabled: g?.enabled ?? true,
    quota_in_per_1k: g?.quota_in_per_1k != null ? String(g.quota_in_per_1k) : '',
    quota_out_per_1k: g?.quota_out_per_1k != null ? String(g.quota_out_per_1k) : '',
    markup_percent:
      g?.markup_percent != null && g.markup_percent !== undefined ? String(g.markup_percent) : '',
    billing_mode: (g?.billing_mode || 'token').trim() || 'token',
    quota_per_request:
      g?.quota_per_request != null && g.quota_per_request !== undefined
        ? String(g.quota_per_request)
        : '',
    input_rate: g?.input_rate ?? 1,
    output_rate: g?.output_rate ?? 1,
  }
}

/** 空字符串表示不覆盖代码基准；非空须为非负整数 */
function parseQuota(raw: string): { ok: boolean; value?: number } {
  const s = raw.trim()
  if (s === '') return { ok: true }
  const n = Number(s)
  if (!Number.isFinite(n) || !Number.isInteger(n) || n < 0) return { ok: false }
  return { ok: true, value: n }
}

function rateDiffersDefault(r: DraftRow): boolean {
  return Math.abs(r.input_rate - 1) > 1e-9 || Math.abs(r.output_rate - 1) > 1e-9
}

function rowHasNonDefaultOverrides(r: DraftRow): boolean {
  if (!r.enabled) return true
  const qIn = parseQuota(r.quota_in_per_1k)
  const qOut = parseQuota(r.quota_out_per_1k)
  if (qIn.value !== undefined || qOut.value !== undefined) return true
  return rateDiffersDefault(r)
}

function shouldPersistRow(r: DraftRow): boolean {
  if (r.isCustom || r.locked === false || r.locked === undefined) {
    return r.model_name.trim().length > 0
  }
  if (r.id != null) return true
  return rowHasNonDefaultOverrides(r)
}

type ApiPricingItem = {
  channel_id?: number
  model_name: string
  enabled: boolean
  quota_in_per_1k?: number | null
  quota_out_per_1k?: number | null
  input_rate: number
  output_rate: number
  markup_percent?: number | null
  billing_mode?: string
  quota_per_request?: number | null
}

/** 导入预览接口返回的单行（与 PUT 项字段一致） */
type ApiImportPreviewRow = {
  channel_id?: number
  model_name: string
  enabled: boolean
  quota_in_per_1k?: number | null
  quota_out_per_1k?: number | null
  input_rate: number
  output_rate: number
  markup_percent?: number | null
  billing_mode?: string
  quota_per_request?: number | null
}

type CsvImportMode = 'global' | 'channel' | 'multi'
const importMode = ref<CsvImportMode>('global')
const csvMultiFileInput = ref<HTMLInputElement | null>(null)

function parseMarkup(raw: string): { ok: boolean; value?: number | null } {
  const s = raw.trim()
  if (s === '') return { ok: true, value: null }
  const n = Number(s)
  if (!Number.isFinite(n) || n <= -100) return { ok: false }
  return { ok: true, value: n }
}

function effectiveMarkupPercentForRow(rowMarkupRaw: string): number | null {
  const s = rowMarkupRaw.trim()
  if (s !== '') {
    const n = Number(s)
    if (!Number.isFinite(n) || n <= -100) return null
    return n
  }
  return parseDeriveMargin()
}

function billingChargePreview(baseRaw: string, rowMarkupRaw: string): string {
  const s = baseRaw.trim()
  if (s === '') return ''
  const base = Number(s)
  if (!Number.isFinite(base) || !Number.isInteger(base) || base < 0) return ''
  const markup = effectiveMarkupPercentForRow(rowMarkupRaw)
  if (markup === null) return ''
  const charged = billingQuotaWithMarkup(base, markup)
  if (charged === null) return ''
  return String(charged)
}

/** 校验单行并构造 PUT 项；失败时返回错误文案（已本地化键路径由调用方包 t） */
function validateRowToItem(r: DraftRow): { ok: true; item: ApiPricingItem } | { ok: false; message: string } {
  const name = r.model_name.trim()
  if (!name) {
    return { ok: false, message: t('adminGlobalPricing.errEmptyModel') }
  }
  const qIn = parseQuota(r.quota_in_per_1k)
  const qOut = parseQuota(r.quota_out_per_1k)
  if (!qIn.ok) {
    return { ok: false, message: t('adminGlobalPricing.errQuotaIn') }
  }
  if (!qOut.ok) {
    return { ok: false, message: t('adminGlobalPricing.errQuotaOut') }
  }
  const inR = r.input_rate
  const outR = r.output_rate
  if (!(inR > 0) || !(outR > 0) || !Number.isFinite(inR) || !Number.isFinite(outR)) {
    return { ok: false, message: t('adminGlobalPricing.errRates') }
  }
  const mp = parseMarkup(r.markup_percent)
  if (!mp.ok) {
    return { ok: false, message: t('adminGlobalPricing.errMarkup') }
  }
  const item: ApiPricingItem = {
    model_name: name,
    enabled: r.enabled,
    input_rate: inR,
    output_rate: outR,
  }
  if (qIn.value !== undefined) item.quota_in_per_1k = qIn.value
  if (qOut.value !== undefined) item.quota_out_per_1k = qOut.value
  item.markup_percent = mp.value ?? null
  const bm = (r.billing_mode || 'token').trim() || 'token'
  item.billing_mode = bm
  if (bm !== 'token') {
    const qpr = parseQuota(r.quota_per_request)
    if (!qpr.ok || qpr.value === undefined || qpr.value <= 0) {
      return { ok: false, message: t('adminGlobalPricing.errQuotaPerRequest') }
    }
    item.quota_per_request = qpr.value
  }
  return { ok: true, item }
}

function draftsToPersistItems(drafts: DraftRow[]): ApiPricingItem[] | null {
  flushQuotaEdits(rows.value)
  flushQuotaEdits(drafts)
  const items: ApiPricingItem[] = []
  for (const r of drafts) {
    if (!shouldPersistRow(r)) continue
    const v = validateRowToItem(r)
    if (!v.ok) {
      flashWarn(v.message)
      return null
    }
    items.push(v.item)
  }
  return items
}

function apiImportItemToDraft(it: ApiImportPreviewRow): DraftRow {
  return {
    channel_id: it.channel_id,
    model_name: it.model_name,
    isCustom: false,
    locked: false,
    enabled: it.enabled,
    quota_in_per_1k:
      it.quota_in_per_1k != null && it.quota_in_per_1k !== undefined ? String(it.quota_in_per_1k) : '',
    quota_out_per_1k:
      it.quota_out_per_1k != null && it.quota_out_per_1k !== undefined ? String(it.quota_out_per_1k) : '',
    input_rate: it.input_rate,
    output_rate: it.output_rate,
    markup_percent:
      it.markup_percent != null && it.markup_percent !== undefined
        ? String(it.markup_percent)
        : '',
    billing_mode: (it.billing_mode || 'token').trim() || 'token',
    quota_per_request:
      it.quota_per_request != null && it.quota_per_request !== undefined
        ? String(it.quota_per_request)
        : '',
  }
}

/** 导入预览确认：跳过模型名为空的行；其余须通过 validateRowToItem */
function importPreviewDraftsToItems(drafts: DraftRow[]): ApiPricingItem[] | null {
  flushQuotaEdits(drafts)
  const items: ApiPricingItem[] = []
  for (const r of drafts) {
    if (!r.model_name.trim()) continue
    if (importMode.value === 'multi') {
      const cid = r.channel_id
      if (cid == null || !Number.isFinite(cid) || cid <= 0) {
        flashWarn(t('adminGlobalPricing.importPreviewChannelRequired'))
        return null
      }
    }
    const v = validateRowToItem(r)
    if (!v.ok) {
      flashWarn(v.message)
      return null
    }
    const item: ApiPricingItem = { ...v.item }
    if (importMode.value === 'multi' && r.channel_id != null) {
      item.channel_id = r.channel_id
    }
    items.push(item)
  }
  if (items.length === 0) {
    flashWarn(t('adminGlobalPricing.importPreviewNoRows'))
    return null
  }
  return items
}

function closeImportPreview() {
  importPreviewOpen.value = false
  importPreviewRows.value = []
  importMode.value = 'global'
}

const importPreviewIsMulti = computed(() => importMode.value === 'multi')

function removeImportPreviewRow(idx: number) {
  importPreviewRows.value.splice(idx, 1)
}

async function confirmImportPreview() {
  clearFlash()
  const items = importPreviewDraftsToItems(importPreviewRows.value)
  if (items === null) return
  importPreviewSubmitting.value = true
  try {
    if (importMode.value === 'multi') {
      await putChannelPricingItems(items)
    } else {
      await putPricingItems(items)
    }
    await load()
    closeImportPreview()
    flashSuccess(t('adminGlobalPricing.imported', { count: items.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    importPreviewSubmitting.value = false
  }
}

async function putChannelPricingItems(items: ApiPricingItem[]): Promise<void> {
  const payload = items.map((it) => {
    const cid = it.channel_id
    if (cid == null || cid <= 0) throw new Error('channel_id')
    const { channel_id: _c, ...rest } = it
    return { ...rest, channel_id: cid }
  })
  const { data } = await http.put<Envelope<unknown>>('/api/admin/channel-model-pricing', {
    items: payload,
  })
  if (data.code !== 0) throw new Error(data.message || 'save')
}

async function putPricingItems(items: ApiPricingItem[]): Promise<void> {
  if (isChannelScope.value) {
    const payload = items.map((it) => ({ ...it, channel_id: activeChannelId.value }))
    const { data } = await http.put<Envelope<unknown>>('/api/admin/channel-model-pricing', {
      items: payload,
    })
    if (data.code !== 0) throw new Error(data.message || 'save')
    return
  }
  const { data } = await http.put<Envelope<unknown>>('/api/admin/global-model-pricing', { items })
  if (data.code !== 0) throw new Error(data.message || 'save')
}

function syncRowSelectionFromInput(idx: number, ev: Event) {
  const el = ev.target as HTMLInputElement
  const on = el.checked
  const set = new Set(selectedRows.value)
  if (on) set.add(idx)
  else set.delete(idx)
  selectedRows.value = [...set].sort((a, b) => a - b)
}

function onToggleSelectAll(ev: Event) {
  const el = ev.target as HTMLInputElement
  if (!el.checked) {
    selectedRows.value = []
    return
  }
  selectedRows.value = rows.value.map((_, i) => i)
}

const allVisibleSelected = computed(() => {
  if (!rows.value.length) return false
  const set = new Set(selectedRows.value)
  return rows.value.every((_, i) => set.has(i))
})

const someVisibleSelected = computed(() => {
  if (!rows.value.length) return false
  const set = new Set(selectedRows.value)
  return rows.value.some((_, i) => set.has(i))
})

const anyListSaving = computed(
  () =>
    pageSaving.value ||
    batchSaving.value ||
    savingFull.value ||
    busyResetPage.value ||
    busyResetSelected.value ||
    busyResetFull.value ||
    importPreviewSubmitting.value,
)

const importTemplatePresets = [
  { currency: 'cny' as const, denom: 1_000_000, labelKey: 'deriveUnitCny1m' },
  { currency: 'cny' as const, denom: 1000, labelKey: 'deriveUnitCny1k' },
  { currency: 'cny' as const, denom: 1, labelKey: 'deriveUnitCny1' },
  { currency: 'usd' as const, denom: 1_000_000, labelKey: 'deriveUnitUsd1m' },
  { currency: 'usd' as const, denom: 1000, labelKey: 'deriveUnitUsd1k' },
  { currency: 'usd' as const, denom: 1, labelKey: 'deriveUnitUsd1' },
]

const showTemplateCurrentPreset = computed(() => deriveUnitPreset.value !== 'quota')

function sitePricingFactors() {
  return {
    quotaPerYuan: sitePerYuan.value ?? 0,
    quotaPerUsd: sitePerUsd.value ?? 0,
    cnyPerUsd: siteCnyPerUsd.value ?? 0,
  }
}

type CodeRefLine = { text: string; muted?: boolean }

function displayCodeRefCell(ref: number | null | undefined): { lines: CodeRefLine[]; title?: string } {
  if (ref == null) return { lines: [{ text: '—' }] }
  if (codeRefShowQuotaOnly.value) return { lines: [{ text: String(ref) }] }
  const quotaTitle = t('adminGlobalPricing.codeRefQuotaTitle', { q: ref })
  let upstreamText: string | null = null
  {
    const p = upstreamPriceFromQuotaPer1K(
      sitePricingFactors(),
      deriveCurrency.value,
      deriveDenom.value,
      ref,
      0,
    )
    if (p !== null) upstreamText = formatUpstreamPrice(p)
  }
  if (upstreamText === null) return { lines: [{ text: String(ref) }] }
  return {
    lines: [
      { text: upstreamText },
      { text: String(ref), muted: true },
    ],
    title: quotaTitle,
  }
}

const selectAllInput = ref<HTMLInputElement | null>(null)
watch(
  [allVisibleSelected, someVisibleSelected, () => rows.value.length],
  () => {
    const el = selectAllInput.value
    if (!el) return
    el.indeterminate = someVisibleSelected.value && !allVisibleSelected.value
  },
  { flush: 'post' },
)

/** 拉取当前筛选下全部工作区行（不修改分页状态） */
async function fetchAllWorkspaceDrafts(): Promise<DraftRow[]> {
  const chunk = 200
  let p = 1
  let totalCount = 0
  const byName = new Map<string, DraftRow>()
  const q = searchApplied.value.trim() || undefined
  const workspacePath = isChannelScope.value
    ? '/api/admin/channel-model-pricing/workspace'
    : '/api/admin/global-model-pricing/workspace'
  const baseParams: Record<string, string | number> = { page: p, page_size: chunk }
  if (q) baseParams.q = q
  if (isChannelScope.value) {
    baseParams.channel_id = activeChannelId.value
  }
  for (;;) {
    const { data } = await http.get<
      Envelope<{ items: WorkspaceRow[]; total: number; page: number; page_size: number }>
    >(workspacePath, {
      params: { ...baseParams, page: p },
    })
    if (data.code !== 0) throw new Error(data.message || 'load')
    totalCount = typeof data.data.total === 'number' ? data.data.total : 0
    const items = data.data.items || []
    for (const w of items) {
      const d = workspaceItemToDraft(w)
      const k = d.model_name.trim()
      if (k) byName.set(k, d)
    }
    if (items.length === 0 || byName.size >= totalCount || items.length < chunk) break
    p++
  }
  return Array.from(byName.values()).sort((a, b) => a.model_name.localeCompare(b.model_name))
}

/** 当前筛选下全量工作区草稿：远端分页合并 + 覆盖当前页编辑 + 覆盖/追加底部自定义行 */
async function buildFullSaveDrafts(): Promise<DraftRow[]> {
  const map = new Map<string, DraftRow>()
  for (const d of await fetchAllWorkspaceDrafts()) {
    const k = d.model_name.trim()
    if (k) map.set(k, d)
  }
  for (const d of workspacePageDrafts.value) {
    const k = d.model_name.trim()
    if (k) map.set(k, d)
  }
  for (const c of customDrafts.value) {
    const k = c.model_name.trim()
    if (k) map.set(k, c)
  }
  return Array.from(map.values()).sort((a, b) => a.model_name.localeCompare(b.model_name))
}

function sourceBadgeLabel(src: string): string {
  switch (src) {
    case 'builtin':
      return t('adminGlobalPricing.sourceBuiltin')
    case 'channel':
      return t('adminGlobalPricing.sourceChannel')
    case 'channel_declared':
      return t('adminGlobalPricing.sourceChannelDeclared')
    case 'channel_db':
      return t('adminGlobalPricing.sourceChannelDb')
    case 'catalog':
      return t('adminGlobalPricing.sourceCatalog')
    case 'global_db':
      return t('adminGlobalPricing.sourceGlobalDb')
    default:
      return src
  }
}

function sourceBadgeClass(src: string): string {
  const base = 'th-admin-pricing-source-badge'
  switch (src) {
    case 'catalog':
    case 'builtin':
      return `${base} th-admin-pricing-source-badge--catalog`
    case 'channel':
    case 'channel_declared':
    case 'channel_db':
      return `${base} th-admin-pricing-source-badge--channel`
    case 'global_db':
      return `${base} th-admin-pricing-source-badge--global`
    default:
      return `${base} th-admin-pricing-source-badge--default`
  }
}

async function loadChannelOptions() {
  channelsLoading.value = true
  try {
    const { data } = await http.get<Envelope<{ items: ChannelListItem[] }>>('/api/admin/channels')
    if (data.code !== 0 || !data.data) throw new Error(data.message || 'channels')
    channelOptions.value = (data.data.items || []).filter((c) => c.enabled !== false)
  } catch (e: unknown) {
    flashError(e)
  } finally {
    channelsLoading.value = false
  }
}

function onChannelScopeChange() {
  clearFlash()
  page.value = 1
  customDrafts.value = []
  void load()
}

function onModelFocus(idx: number) {
  if (rows.value[idx]?.locked) return
  focusModelSuggestRow(idx)
}

async function load() {
  if (isChannelScope.value && activeChannelId.value <= 0) {
    workspacePageDrafts.value = []
    workspaceSnapshotByModel.value = new Map()
    total.value = 0
    rebuildRows()
    return
  }
  clearFlash()
  await runListLoad(async () => {
    const workspacePath = isChannelScope.value
      ? '/api/admin/channel-model-pricing/workspace'
      : '/api/admin/global-model-pricing/workspace'
    const params: Record<string, string | number> = {
      page: page.value,
      page_size: pageSize.value,
    }
    const q = searchApplied.value.trim()
    if (q) params.q = q
    if (isChannelScope.value) {
      params.channel_id = activeChannelId.value
    }
    const { data } = await http.get<
      Envelope<{ items: WorkspaceRow[]; total: number; page: number; page_size: number }>
    >(workspacePath, { params })
    if (data.code !== 0) throw new Error(data.message || 'load')
    total.value = typeof data.data.total === 'number' ? data.data.total : 0
    if (typeof data.data.page === 'number' && data.data.page >= 1) {
      page.value = data.data.page
    }
    workspacePageDrafts.value = (data.data.items || []).map(workspaceItemToDraft)
    syncWorkspaceSnapshotsFromDrafts()
    rebuildRows()
    selectedRows.value = []
    void loadModelSuggestions()
  }).catch((e: unknown) => {
    flashError(e)
  })
}

function applySearch() {
  clearFlash()
  searchApplied.value = searchDraft.value.trim()
  page.value = 1
  void load()
}

function onPageSizeChange() {
  page.value = 1
  void load()
}

function prevPage() {
  if (page.value <= 1) return
  page.value--
  void load()
}

function nextPage() {
  if (page.value >= totalPages.value) return
  page.value++
  void load()
}

function addRow() {
  clearFlash()
  customDrafts.value.push({
    model_name: '',
    isCustom: true,
    locked: false,
    enabled: true,
    quota_in_per_1k: '',
    quota_out_per_1k: '',
    markup_percent: '',
    billing_mode: 'token',
    quota_per_request: '',
    input_rate: 1,
    output_rate: 1,
  })
  rebuildRows()
}

function planResetFromDrafts(drafts: DraftRow[]): { deleteNames: string[]; customToRemove: DraftRow[] } {
  const deleteNames = new Set<string>()
  const customToRemove: DraftRow[] = []
  for (const r of drafts) {
    if (r.isCustom) {
      customToRemove.push(r)
      continue
    }
    if (r.id != null) {
      const n = r.model_name.trim()
      if (n) deleteNames.add(n)
    }
  }
  return { deleteNames: [...deleteNames], customToRemove }
}

async function deleteGlobalPricingByName(modelName: string): Promise<void> {
  if (isChannelScope.value) {
    const { data } = await http.delete<Envelope<unknown>>('/api/admin/channel-model-pricing', {
      params: { channel_id: activeChannelId.value, model_name: modelName },
    })
    if (data.code !== 0) throw new Error(data.message || 'reset')
    return
  }
  const { data } = await http.delete<Envelope<unknown>>('/api/admin/global-model-pricing', {
    params: { model_name: modelName },
  })
  if (data.code !== 0) throw new Error(data.message || 'reset')
}

function removeCustomDraftRows(toRemove: DraftRow[]) {
  if (!toRemove.length) return
  const rm = new Set(toRemove)
  customDrafts.value = customDrafts.value.filter((x) => !rm.has(x))
  rebuildRows()
}

async function resetRow(idx: number) {
  const r = rows.value[idx]
  if (!r) return
  if (r.isCustom) {
    removeCustomDraftRows([r])
    flashSuccess(t('adminGlobalPricing.resetCustomRemoved'))
    return
  }
  if (r.locked && r.id == null) {
    const key = r.model_name.trim()
    if (!key) return
    const snap = workspaceSnapshotByModel.value.get(key)
    if (!snap) return
    const wi = workspacePageDrafts.value.findIndex((d) => d.model_name.trim() === key)
    if (wi < 0) return
    workspacePageDrafts.value[wi] = cloneDraftRow(snap)
    rebuildRows()
    flashSuccess(t('adminGlobalPricing.resetRevertDone', { name: key }))
    return
  }
  if (r.id != null && r.model_name.trim()) {
    if (!confirm(t('adminGlobalPricing.resetConfirm', { name: r.model_name.trim() }))) return
    clearFlash()
    try {
      await deleteGlobalPricingByName(r.model_name.trim())
      await load()
      flashSuccess(t('adminGlobalPricing.resetDone', { name: r.model_name.trim() }))
    } catch (e: unknown) {
      flashError(e)
    }
    return
  }
}

/** 当前行是否可执行重置（无操作时按钮仍展示但禁用） */
function canResetRow(r: DraftRow): boolean {
  if (!r.model_name.trim()) return false
  if (r.isCustom) return true
  if (r.id != null) return true
  if (r.locked === false || r.locked === undefined) return true
  return rowDiffersFromSnapshot(r)
}

async function resetCurrentPage() {
  clearFlash()
  const drafts = [...workspacePageDrafts.value, ...customDrafts.value]
  const { deleteNames, customToRemove } = planResetFromDrafts(drafts)
  if (!deleteNames.length && !customToRemove.length) {
    flashSuccess(t('adminGlobalPricing.nothingToResetPage'))
    return
  }
  if (
    !confirm(
      t('adminGlobalPricing.resetPageConfirm', {
        db: deleteNames.length,
        custom: customToRemove.length,
      }),
    )
  )
    return
  busyResetPage.value = true
  try {
    for (const name of deleteNames) {
      await deleteGlobalPricingByName(name)
    }
    removeCustomDraftRows(customToRemove)
    await load()
    flashSuccess(t('adminGlobalPricing.resetBatchDone', { db: deleteNames.length, custom: customToRemove.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    busyResetPage.value = false
  }
}

async function resetSelectedRows() {
  clearFlash()
  if (!selectedRows.value.length) {
    flashWarn(t('adminGlobalPricing.nothingSelected'))
    return
  }
  const drafts: DraftRow[] = []
  for (const i of selectedRows.value) {
    const r = rows.value[i]
    if (r) drafts.push(r)
  }
  const { deleteNames, customToRemove } = planResetFromDrafts(drafts)
  if (!deleteNames.length && !customToRemove.length) {
    flashSuccess(t('adminGlobalPricing.nothingToResetSelection'))
    return
  }
  if (
    !confirm(
      t('adminGlobalPricing.resetSelectedConfirm', {
        db: deleteNames.length,
        custom: customToRemove.length,
      }),
    )
  )
    return
  busyResetSelected.value = true
  try {
    for (const name of deleteNames) {
      await deleteGlobalPricingByName(name)
    }
    removeCustomDraftRows(customToRemove)
    await load()
    flashSuccess(t('adminGlobalPricing.resetBatchDone', { db: deleteNames.length, custom: customToRemove.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    busyResetSelected.value = false
  }
}

async function resetFullWorkspace() {
  clearFlash()
  busyResetFull.value = true
  try {
    const drafts = await buildFullSaveDrafts()
    const { deleteNames, customToRemove } = planResetFromDrafts(drafts)
    if (!deleteNames.length && !customToRemove.length) {
      flashSuccess(t('adminGlobalPricing.nothingToResetFull'))
      return
    }
    if (
      !confirm(
        t('adminGlobalPricing.resetFullConfirm', {
          db: deleteNames.length,
          custom: customToRemove.length,
        }),
      )
    )
      return
    for (const name of deleteNames) {
      await deleteGlobalPricingByName(name)
    }
    removeCustomDraftRows(customToRemove)
    await load()
    flashSuccess(t('adminGlobalPricing.resetBatchDone', { db: deleteNames.length, custom: customToRemove.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    busyResetFull.value = false
  }
}

async function saveCurrentPage() {
  clearFlash()
  const drafts = [...workspacePageDrafts.value, ...customDrafts.value]
  const items = draftsToPersistItems(drafts)
  if (items === null) return
  if (items.length === 0) {
    flashSuccess(t('adminGlobalPricing.nothingToSave'))
    return
  }
  pageSaving.value = true
  try {
    await putPricingItems(items)
    await load()
    flashSuccess(t('adminGlobalPricing.savedPage', { n: items.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    pageSaving.value = false
  }
}

async function saveFullWorkspace() {
  clearFlash()
  savingFull.value = true
  try {
    const drafts = await buildFullSaveDrafts()
    const items = draftsToPersistItems(drafts)
    if (items === null) return
    if (items.length === 0) {
      flashSuccess(t('adminGlobalPricing.nothingToSave'))
      return
    }
    await putPricingItems(items)
    await load()
    flashSuccess(t('adminGlobalPricing.savedFull', { n: items.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    savingFull.value = false
  }
}

async function saveSelectedRows() {
  clearFlash()
  if (!selectedRows.value.length) {
    flashWarn(t('adminGlobalPricing.nothingSelected'))
    return
  }
  const drafts: DraftRow[] = []
  for (const i of selectedRows.value) {
    const r = rows.value[i]
    if (r) drafts.push(r)
  }
  const items = draftsToPersistItems(drafts)
  if (items === null) return
  if (items.length === 0) {
    flashSuccess(t('adminGlobalPricing.nothingToSaveSelection'))
    return
  }
  batchSaving.value = true
  try {
    await putPricingItems(items)
    await load()
    flashSuccess(t('adminGlobalPricing.savedSelected', { n: items.length }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    batchSaving.value = false
  }
}

async function saveOneRow(idx: number) {
  const r = rows.value[idx]
  if (!r) return
  clearFlash()
  flushQuotaEdits(rows.value)
  if (!shouldPersistRow(r)) {
    flashSuccess(t('adminGlobalPricing.nothingToSaveRow'))
    return
  }
  const v = validateRowToItem(r)
  if (!v.ok) {
    flashWarn(v.message)
    return
  }
  rowSavingIdx.value = idx
  try {
    await putPricingItems([v.item])
    await load()
    flashSuccess(t('adminGlobalPricing.savedOne', { name: v.item.model_name }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    rowSavingIdx.value = null
  }
}

async function loadSitePricingMeta() {
  try {
    const { data } = await http.get<
      Envelope<{
        currency_quota_per_yuan?: number
        currency_quota_per_usd?: number
        pricing_cny_per_usd?: number
        default_pricing_markup_percent?: number
      }>
    >('/api/admin/site-settings')
    if (data.code !== 0 || !data.data) return
    const d = data.data
    if (typeof d.currency_quota_per_yuan === 'number') sitePerYuan.value = d.currency_quota_per_yuan
    if (typeof d.currency_quota_per_usd === 'number') sitePerUsd.value = d.currency_quota_per_usd
    if (typeof d.pricing_cny_per_usd === 'number') siteCnyPerUsd.value = d.pricing_cny_per_usd
    if (typeof d.default_pricing_markup_percent === 'number') {
      deriveMargin.value = formatDeriveMarginValue(d.default_pricing_markup_percent)
      deriveMarginDraft.value = deriveMargin.value
    }
  } catch {
    /* ignore */
  }
}

function parseDeriveMargin(): number | null {
  const margin = Number(String(deriveMargin.value).trim() || '0')
  if (!Number.isFinite(margin)) return null
  return margin
}

function parseMarginDraftText(raw: string): number | null {
  const margin = Number(String(raw).trim() || '0')
  if (!Number.isFinite(margin)) return null
  if (margin <= -100) return null
  return margin
}

function formatDeriveMarginValue(margin: number): string {
  if (Number.isInteger(margin)) return String(margin)
  return String(margin)
}

const deriveMarginAppliedLabel = computed(() => {
  const m = parseDeriveMargin()
  if (m === null || m === 0) return ''
  return formatDeriveMarginValue(m)
})

function onDeriveAdvancedToggle() {
  if (deriveAdvancedRef.value?.open) deriveMarginDraft.value = deriveMargin.value
}

const savingSiteMarkup = ref(false)

async function applyDeriveMargin() {
  const margin = parseMarginDraftText(deriveMarginDraft.value)
  if (margin === null) {
    const n = Number(String(deriveMarginDraft.value).trim() || '0')
    if (Number.isFinite(n) && n <= -100) {
      flashWarn(t('adminGlobalPricing.deriveErrMarginMin'))
    } else {
      flashWarn(t('adminGlobalPricing.deriveErrMargin'))
    }
    return
  }
  const next = formatDeriveMarginValue(margin)
  savingSiteMarkup.value = true
  try {
    const { data } = await http.put<Envelope<unknown>>('/api/admin/config', {
      items: [
        {
          key: 'site.default_pricing_markup_percent',
          value: next,
          description: t('adminGlobalPricing.deriveMarginConfigDesc'),
        },
      ],
    })
    if (data.code !== 0) throw new Error(data.message || 'save')
    deriveMargin.value = next
    deriveMarginDraft.value = next
    if (deriveAdvancedRef.value) deriveAdvancedRef.value.open = false
    flashSuccess(t('adminGlobalPricing.deriveMarginApplied', { m: next }))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    savingSiteMarkup.value = false
  }
}

function cancelDeriveMargin() {
  deriveMarginDraft.value = deriveMargin.value
  if (deriveAdvancedRef.value) deriveAdvancedRef.value.open = false
}

const {
  priceUnitLabel,
  quotaInputDisplay,
  onQuotaFocus,
  onQuotaInput,
  commitQuotaBlur,
  flushQuotaEdits,
} = useQuotaPer1kUpstreamField({
  codeRefShowQuotaOnly,
  deriveCurrency,
  deriveDenom,
  sitePricingFactors,
})

const thQuotaInLabel = computed(() =>
  codeRefShowQuotaOnly.value
    ? t('adminGlobalPricing.thQuotaIn')
    : t('adminGlobalPricing.thQuotaInUpstream', { unit: priceUnitLabel.value }),
)
const thQuotaOutLabel = computed(() =>
  codeRefShowQuotaOnly.value
    ? t('adminGlobalPricing.thQuotaOut')
    : t('adminGlobalPricing.thQuotaOutUpstream', { unit: priceUnitLabel.value }),
)
const quotaInPlaceholder = computed(() =>
  codeRefShowQuotaOnly.value
    ? t('adminGlobalPricing.phOptional')
    : t('adminGlobalPricing.phQuotaUpstreamIn', { cur: priceUnitLabel.value }),
)
const quotaOutPlaceholder = computed(() =>
  codeRefShowQuotaOnly.value
    ? t('adminGlobalPricing.phOptional')
    : t('adminGlobalPricing.phQuotaUpstreamOut', { cur: priceUnitLabel.value }),
)

function rowUsesTokenQuota(r: DraftRow) {
  return r.billing_mode === 'token' || r.billing_mode === 'hybrid_floor'
}

onMounted(() => {
  void loadChannelOptions()
  void load()
  void loadSitePricingMeta()
  document.addEventListener('click', onExportMenusDocClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onExportMenusDocClick)
  clearFlashDismissTimer()
})

function parseExportFilename(cd: string | undefined): string {
  if (!cd) return 'global-model-pricing.csv'
  const m = /filename\*=UTF-8''([^;\s]+)|filename="([^"]+)"|filename=([^;\s]+)/i.exec(cd)
  if (!m) return 'global-model-pricing.csv'
  const raw = (m[1] || m[2] || m[3] || '').trim()
  if (!raw) return 'global-model-pricing.csv'
  try {
    return decodeURIComponent(raw.replace(/^["']|["']$/g, ''))
  } catch {
    return raw.replace(/^["']|["']$/g, '') || 'global-model-pricing.csv'
  }
}

async function downloadCsvBlob(
  url: string,
  params: Record<string, string | number> | undefined,
  fallbackName: string,
  successKey: 'exported' | 'exportedTemplate',
) {
  const res = await http.get(url, { params, responseType: 'blob' })
  const name = parseExportFilename(res.headers['content-disposition'] as string | undefined) || fallbackName
  const blob =
    res.data instanceof Blob ? res.data : new Blob([res.data as BlobPart], { type: 'text/csv;charset=utf-8' })
  const objUrl = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = objUrl
  a.download = name
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(objUrl)
  flashSuccess(t(`adminGlobalPricing.${successKey}`))
}

function csvExportDataUrl() {
  if (isChannelScope.value) return '/api/admin/channel-model-pricing/export'
  return '/api/admin/global-model-pricing/export'
}

function csvExportTemplateUrl() {
  if (isChannelScope.value) return '/api/admin/channel-model-pricing/export/template'
  return '/api/admin/global-model-pricing/export/template'
}

function csvExportParams(currency: 'cny' | 'usd', denom: number): Record<string, string | number> {
  const p: Record<string, string | number> = { currency, denom }
  if (isChannelScope.value) p.channel_id = activeChannelId.value
  return p
}

function csvExportFallbackName(kind: 'data' | 'template', currency: 'cny' | 'usd') {
  if (isChannelScope.value) {
    return kind === 'data'
      ? `channel-${activeChannelId.value}-model-pricing-data-${currency}.csv`
      : `channel-${activeChannelId.value}-model-pricing-import-${currency}.csv`
  }
  return kind === 'data'
    ? `global-model-pricing-data-${currency}.csv`
    : `global-model-pricing-import-${currency}.csv`
}

async function exportDataCsv(currency: 'cny' | 'usd', denom: number) {
  clearFlash()
  exporting.value = true
  dataMenuOpen.value = false
  try {
    await downloadCsvBlob(
      csvExportDataUrl(),
      csvExportParams(currency, denom),
      csvExportFallbackName('data', currency),
      'exported',
    )
  } catch (e: unknown) {
    flashError(e)
  } finally {
    exporting.value = false
  }
}

async function exportDataCsvBatch(currency: 'cny' | 'usd', denom: number) {
  clearFlash()
  exporting.value = true
  dataMenuOpen.value = false
  try {
    await downloadCsvBlob(
      '/api/admin/channel-model-pricing/export/batch',
      { currency, denom },
      `channel-model-pricing-all-${currency}.csv`,
      'exported',
    )
  } catch (e: unknown) {
    flashError(e)
  } finally {
    exporting.value = false
  }
}

function exportDataCsvCurrentUnit() {
  void exportDataCsv(deriveCurrency.value, deriveDenom.value)
}

async function exportImportTemplate(currency: 'cny' | 'usd', denom: number) {
  clearFlash()
  exportingTemplate.value = true
  templateMenuOpen.value = false
  try {
    await downloadCsvBlob(
      csvExportTemplateUrl(),
      csvExportParams(currency, denom),
      csvExportFallbackName('template', currency),
      'exportedTemplate',
    )
  } catch (e: unknown) {
    flashError(e)
  } finally {
    exportingTemplate.value = false
  }
}

async function exportImportTemplateBatch(currency: 'cny' | 'usd', denom: number) {
  clearFlash()
  exportingTemplate.value = true
  templateMenuOpen.value = false
  try {
    await downloadCsvBlob(
      '/api/admin/channel-model-pricing/export/template/batch',
      { currency, denom },
      `channel-model-pricing-import-all-${currency}.csv`,
      'exportedTemplate',
    )
  } catch (e: unknown) {
    flashError(e)
  } finally {
    exportingTemplate.value = false
  }
}

function exportDataCsvBatchCurrentUnit() {
  void exportDataCsvBatch(deriveCurrency.value, deriveDenom.value)
}

function exportImportTemplateBatchCurrentUnit() {
  void exportImportTemplateBatch(deriveCurrency.value, deriveDenom.value)
}

function exportImportTemplateCurrentUnit() {
  void exportImportTemplate(deriveCurrency.value, deriveDenom.value)
}

function toggleDataMenu() {
  dataMenuOpen.value = !dataMenuOpen.value
  templateMenuOpen.value = false
}

function toggleTemplateMenu() {
  templateMenuOpen.value = !templateMenuOpen.value
  dataMenuOpen.value = false
}

function onExportMenusDocClick(ev: MouseEvent) {
  const target = ev.target as Node
  if (templateMenuOpen.value) {
    const el = templateMenuRef.value
    if (el && !el.contains(target)) templateMenuOpen.value = false
  }
  if (dataMenuOpen.value) {
    const el = dataMenuRef.value
    if (el && !el.contains(target)) dataMenuOpen.value = false
  }
}

function triggerCsvImport(mode: CsvImportMode = 'global') {
  clearFlash()
  if (mode === 'global' && isChannelScope.value) {
    mode = 'channel'
  }
  if (mode === 'multi' && isChannelScope.value) return
  importMode.value = mode
  if (mode === 'multi') {
    csvMultiFileInput.value?.click()
  } else {
    csvFileInput.value?.click()
  }
}

async function onCsvFileChange(ev: Event, mode: CsvImportMode) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  clearFlash()
  importMode.value = mode
  try {
    const fd = new FormData()
    fd.append('file', file)
    let previewUrl = '/api/admin/global-model-pricing/import/preview'
    const params: Record<string, number> = {}
    if (mode === 'channel') {
      previewUrl = '/api/admin/channel-model-pricing/import/preview'
      params.channel_id = activeChannelId.value
    } else if (mode === 'multi') {
      previewUrl = '/api/admin/channel-model-pricing/import/preview/batch'
    }
    const { data } = await http.post<Envelope<{ count: number; items: ApiImportPreviewRow[] }>>(
      previewUrl,
      fd,
      Object.keys(params).length ? { params } : undefined,
    )
    if (data.code !== 0) throw new Error(data.message || 'preview')
    const raw = data.data?.items
    if (!Array.isArray(raw) || raw.length === 0) {
      flashWarn(t('adminGlobalPricing.importPreviewEmpty'))
      return
    }
    importPreviewRows.value = raw.map(apiImportItemToDraft)
    importPreviewOpen.value = true
  } catch (e: unknown) {
    flashError(e)
  } finally {
    importing.value = false
    input.value = ''
  }
}
</script>

<template>
  <div>
    <article
      class="overflow-hidden rounded-2xl border border-slate-800/90 bg-slate-900/60 ring-1 ring-white/[0.04]"
    >
      <div class="border-b border-slate-800/80 px-5 py-4">
        <h3 class="text-lg font-semibold text-white">{{ t('adminGlobalPricing.title') }}</h3>
        <p class="mt-2 text-sm text-slate-400">{{ t('adminGlobalPricing.intro') }}</p>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminGlobalPricing.priorityHint') }}</p>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">{{ t('adminGlobalPricing.workspaceHint') }}</p>
        <p
          v-if="isChannelScope"
          class="mt-1 text-xs leading-relaxed text-cyan-300/80"
        >
          {{ t('adminGlobalPricing.channelScopeHint', { channel: selectedChannelLabel }) }}
        </p>
      </div>

      <div class="border-t border-slate-800/80 bg-slate-950/40 px-5 py-4">
        <ThTableSkeleton v-if="listUi && !rows.length" :rows="10" :columns="7" toolbar />
        <template v-else>
          <p class="mb-1 text-xs text-slate-500">{{ t('adminGlobalPricing.ratesTableHint') }}</p>
          <p class="mb-3 text-[11px] leading-relaxed text-slate-500">
            <span v-if="modelSuggestLoadingUi">{{ t('adminGroups.modelSuggestLoading') }}</span>
            <span v-else>{{ t('adminGlobalPricing.modelSuggestHint', { total: modelSuggestTotal }) }}</span>
          </p>

          <div
            class="th-admin-pricing-toolbar mb-3 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border px-3 py-2 text-xs"
          >
            <label class="flex min-w-0 max-w-full items-center gap-2 sm:max-w-[min(100%,20rem)]">
              <span class="th-admin-pricing-toolbar__label shrink-0">{{ t('adminGlobalPricing.channelFilter') }}</span>
              <select
                v-model="selectedChannelId"
                class="th-select th-select--inline th-select--pricing-channel min-w-0 flex-1"
                :disabled="busy || channelsLoading"
                @change="onChannelScopeChange"
              >
                <option value="">{{ t('adminGlobalPricing.channelFilterAll') }}</option>
                <option v-for="ch in channelOptions" :key="ch.id" :value="ch.id">
                  {{ (ch.channel_name || '').trim() || ch.channel_type }} (#{{ ch.id }})
                </option>
              </select>
            </label>
            <span class="th-admin-pricing-toolbar__sep" aria-hidden="true" />
            <label class="flex items-center gap-2">
              <span class="th-admin-pricing-toolbar__label shrink-0">{{ t('adminGlobalPricing.deriveBarLabel') }}</span>
              <select
                v-model="deriveUnitPreset"
                class="th-select th-select--inline th-select--pricing-unit"
              >
                <option value="quota">{{ t('adminGlobalPricing.deriveUnitQuota') }}</option>
                <option value="usd_1m">{{ t('adminGlobalPricing.deriveUnitUsd1m') }}</option>
                <option value="usd_1k">{{ t('adminGlobalPricing.deriveUnitUsd1k') }}</option>
                <option value="cny_1m">{{ t('adminGlobalPricing.deriveUnitCny1m') }}</option>
                <option value="cny_1k">{{ t('adminGlobalPricing.deriveUnitCny1k') }}</option>
              </select>
            </label>
            <details
              v-if="!codeRefShowQuotaOnly"
              ref="deriveAdvancedRef"
              class="th-pricing-advanced"
              @toggle="onDeriveAdvancedToggle"
            >
              <summary>
                {{ t('adminGlobalPricing.deriveAdvanced') }}
                <span
                  v-if="deriveMarginAppliedLabel"
                  class="ml-1 font-mono tabular-nums text-cyan-300/90"
                >{{ deriveMarginAppliedLabel }}%</span>
              </summary>
              <div class="th-pricing-advanced-panel">
                <span class="th-admin-pricing-toolbar__label whitespace-nowrap">{{
                  t('adminGlobalPricing.deriveMarginShort')
                }}</span>
                <input
                  v-model="deriveMarginDraft"
                  type="text"
                  inputmode="decimal"
                  class="th-input font-mono tabular-nums"
                  autocomplete="off"
                  :title="t('adminGlobalPricing.deriveMargin')"
                  @keydown.enter.prevent="applyDeriveMargin"
                  @keydown.esc.prevent="cancelDeriveMargin"
                />
                <div class="th-pricing-advanced-panel__actions">
                  <button
                    type="button"
                    class="th-pricing-advanced-panel__btn th-pricing-advanced-panel__btn--primary"
                    :disabled="savingSiteMarkup"
                    @click="applyDeriveMargin"
                  >
                    {{ savingSiteMarkup ? t('common.submitting') : t('adminCommon.apply') }}
                  </button>
                  <button type="button" class="th-pricing-advanced-panel__btn" @click="cancelDeriveMargin">
                    {{ t('adminCommon.cancel') }}
                  </button>
                </div>
              </div>
            </details>
            <span class="th-admin-pricing-toolbar__sep" aria-hidden="true" />
            <RouterLink
              :to="{ name: 'admin-billing-pricing-conversion' }"
              class="h-9 shrink-0 rounded-lg border border-slate-600 px-2.5 text-xs leading-9 text-cyan-300/90 hover:bg-slate-800"
            >
              {{ t('adminGlobalPricing.linkConversionSettings') }}
            </RouterLink>
            <span class="th-admin-pricing-toolbar__sep th-admin-pricing-toolbar__sep--before-search" aria-hidden="true" />
            <div class="th-admin-pricing-toolbar__search flex min-w-0 w-full basis-full items-center gap-2 sm:basis-auto sm:flex-1 sm:max-w-[min(100%,28rem)]">
              <span class="th-admin-pricing-toolbar__label shrink-0">{{ t('adminGlobalPricing.searchModel') }}</span>
              <input
                v-model="searchDraft"
                type="search"
                autocomplete="off"
                class="th-input th-admin-pricing-toolbar__search-input h-9 min-w-0 flex-1 font-mono text-xs"
                :placeholder="t('adminGlobalPricing.searchPlaceholder')"
                @keydown.enter.prevent="applySearch"
              />
              <button
                type="button"
                class="h-9 shrink-0 rounded-lg border border-slate-600 px-3 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                :disabled="busy"
                @click="applySearch"
              >
                {{ t('adminCommon.query') }}
              </button>
            </div>
          </div>

          <div class="mb-3 space-y-3 border-b border-slate-800/60 pb-4">
            <div
              class="th-admin-pricing-filter-bar flex flex-col gap-3 overflow-visible rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 pb-3 pt-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-1 sm:gap-y-2 sm:pb-2.5 sm:pt-2"
            >
              <div class="flex shrink-0 items-center gap-2">
                <span class="text-xs text-slate-500">{{ t('adminGlobalPricing.pageSize') }}</span>
                <select
                  v-model.number="pageSize"
                  class="th-select th-select--inline w-[4.25rem] !max-w-none py-0 pr-6 text-xs"
                  :disabled="busy"
                  @change="onPageSizeChange"
                >
                  <option :value="20">20</option>
                  <option :value="50">50</option>
                  <option :value="100">100</option>
                  <option :value="200">200</option>
                </select>
              </div>

              <div
                class="hidden h-6 w-px shrink-0 bg-slate-700/90 sm:block"
                aria-hidden="true"
              />

              <div class="flex min-w-0 flex-1 flex-wrap items-center justify-end gap-2 sm:flex-none sm:basis-auto">
                <span
                  class="min-w-0 truncate text-right text-xs tabular-nums text-slate-400 sm:max-w-[20rem]"
                  :title="`${t('adminGlobalPricing.pageTotal', { total })} · ${t('adminGlobalPricing.pageStatus', { page, pages: totalPages })}`"
                >
                  <span class="text-slate-300">{{ t('adminGlobalPricing.pageTotal', { total }) }}</span>
                  <span class="mx-1.5 text-slate-600" aria-hidden="true">·</span>
                  <span class="font-mono text-slate-300">{{
                    t('adminGlobalPricing.pageStatus', { page, pages: totalPages })
                  }}</span>
                </span>
                <div class="flex shrink-0 items-center gap-1">
                  <button
                    type="button"
                    class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                    :disabled="page <= 1 || busy"
                    @click="prevPage"
                  >
                    {{ t('adminCommon.prevPage') }}
                  </button>
                  <button
                    type="button"
                    class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                    :disabled="page >= totalPages || busy"
                    @click="nextPage"
                  >
                    {{ t('adminCommon.nextPage') }}
                  </button>
                </div>
                <div
                  class="hidden h-6 w-px shrink-0 bg-slate-700/90 sm:block"
                  aria-hidden="true"
                />
                <div ref="dataMenuRef" class="relative shrink-0">
                  <button
                    type="button"
                    class="flex h-9 items-center gap-1 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                    :disabled="exporting || exportingTemplate || importing || importPreviewSubmitting || importPreviewOpen || busy"
                    :aria-expanded="dataMenuOpen"
                    @click.stop="toggleDataMenu"
                  >
                    {{ exporting ? t('common.submitting') : t('adminGlobalPricing.exportDataMenu') }}
                    <span class="text-[10px] text-slate-500" aria-hidden="true">▾</span>
                  </button>
                  <div
                    v-if="dataMenuOpen"
                    class="absolute right-0 top-[calc(100%+4px)] z-30 min-w-[10.5rem] overflow-hidden rounded-lg border border-slate-700 bg-slate-900 py-1 shadow-xl ring-1 ring-black/20"
                    role="menu"
                    @click.stop
                  >
                    <button
                      v-if="showTemplateCurrentPreset"
                      type="button"
                      role="menuitem"
                      class="block w-full px-3 py-2 text-left text-xs text-cyan-200 hover:bg-slate-800"
                      @click="exportDataCsvCurrentUnit"
                    >
                      {{ t('adminGlobalPricing.exportDataCurrent', { unit: priceUnitLabel }) }}
                    </button>
                    <div
                      v-if="showTemplateCurrentPreset"
                      class="my-1 border-t border-slate-800"
                      role="separator"
                    />
                    <button
                      v-for="(p, di) in importTemplatePresets"
                      :key="`data-${p.currency}-${p.denom}`"
                      type="button"
                      role="menuitem"
                      class="block w-full px-3 py-2 text-left text-xs text-slate-200 hover:bg-slate-800"
                      :class="di > 0 && p.currency !== importTemplatePresets[di - 1]?.currency ? 'border-t border-slate-800' : ''"
                      @click="exportDataCsv(p.currency, p.denom)"
                    >
                      {{ t(`adminGlobalPricing.${p.labelKey}`) }}
                    </button>
                    <template v-if="!isChannelScope">
                      <div class="my-1 border-t border-slate-800" role="separator" />
                      <button
                        type="button"
                        role="menuitem"
                        class="block w-full px-3 py-2 text-left text-xs text-amber-200/90 hover:bg-slate-800"
                        @click="exportDataCsvBatchCurrentUnit"
                      >
                        {{ t('adminGlobalPricing.exportAllChannelsCurrent', { unit: priceUnitLabel }) }}
                      </button>
                      <button
                        v-for="(p, bi) in importTemplatePresets"
                        :key="`data-batch-${p.currency}-${p.denom}`"
                        type="button"
                        role="menuitem"
                        class="block w-full px-3 py-2 text-left text-xs text-slate-200 hover:bg-slate-800"
                        :class="bi > 0 && p.currency !== importTemplatePresets[bi - 1]?.currency ? 'border-t border-slate-800' : ''"
                        @click="exportDataCsvBatch(p.currency, p.denom)"
                      >
                        {{ t('adminGlobalPricing.exportAllChannels', { unit: t(`adminGlobalPricing.${p.labelKey}`) }) }}
                      </button>
                    </template>
                  </div>
                </div>
                <div ref="templateMenuRef" class="relative shrink-0">
                  <button
                    type="button"
                    class="flex h-9 items-center gap-1 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                    :disabled="exporting || exportingTemplate || importing || importPreviewSubmitting || importPreviewOpen || busy"
                    :aria-expanded="templateMenuOpen"
                    @click.stop="toggleTemplateMenu"
                  >
                    {{
                      exportingTemplate ? t('common.submitting') : t('adminGlobalPricing.exportTemplateMenu')
                    }}
                    <span class="text-[10px] text-slate-500" aria-hidden="true">▾</span>
                  </button>
                  <div
                    v-if="templateMenuOpen"
                    class="absolute right-0 top-[calc(100%+4px)] z-30 min-w-[10.5rem] overflow-hidden rounded-lg border border-slate-700 bg-slate-900 py-1 shadow-xl ring-1 ring-black/20"
                    role="menu"
                    @click.stop
                  >
                    <button
                      v-if="showTemplateCurrentPreset"
                      type="button"
                      role="menuitem"
                      class="block w-full px-3 py-2 text-left text-xs text-cyan-200 hover:bg-slate-800"
                      @click="exportImportTemplateCurrentUnit"
                    >
                      {{ t('adminGlobalPricing.exportTemplateCurrent', { unit: priceUnitLabel }) }}
                    </button>
                    <div
                      v-if="showTemplateCurrentPreset"
                      class="my-1 border-t border-slate-800"
                      role="separator"
                    />
                    <button
                      v-for="(p, ti) in importTemplatePresets"
                      :key="`tpl-${p.currency}-${p.denom}`"
                      type="button"
                      role="menuitem"
                      class="block w-full px-3 py-2 text-left text-xs text-slate-200 hover:bg-slate-800"
                      :class="ti > 0 && p.currency !== importTemplatePresets[ti - 1]?.currency ? 'border-t border-slate-800' : ''"
                      @click="exportImportTemplate(p.currency, p.denom)"
                    >
                      {{ t(`adminGlobalPricing.${p.labelKey}`) }}
                    </button>
                    <template v-if="!isChannelScope">
                      <div class="my-1 border-t border-slate-800" role="separator" />
                      <button
                        type="button"
                        role="menuitem"
                        class="block w-full px-3 py-2 text-left text-xs text-amber-200/90 hover:bg-slate-800"
                        @click="exportImportTemplateBatchCurrentUnit"
                      >
                        {{ t('adminGlobalPricing.exportAllChannelsTemplateCurrent', { unit: priceUnitLabel }) }}
                      </button>
                      <button
                        v-for="(p, bti) in importTemplatePresets"
                        :key="`tpl-batch-${p.currency}-${p.denom}`"
                        type="button"
                        role="menuitem"
                        class="block w-full px-3 py-2 text-left text-xs text-slate-200 hover:bg-slate-800"
                        :class="bti > 0 && p.currency !== importTemplatePresets[bti - 1]?.currency ? 'border-t border-slate-800' : ''"
                        @click="exportImportTemplateBatch(p.currency, p.denom)"
                      >
                        {{ t('adminGlobalPricing.exportAllChannelsTemplate', { unit: t(`adminGlobalPricing.${p.labelKey}`) }) }}
                      </button>
                    </template>
                  </div>
                </div>
                <button
                  type="button"
                  class="h-9 shrink-0 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="exporting || exportingTemplate || importing || importPreviewSubmitting || importPreviewOpen || anyListSaving || busy"
                  @click="triggerCsvImport(isChannelScope ? 'channel' : 'global')"
                >
                  {{ importing ? t('common.submitting') : t('adminGlobalPricing.importCsv') }}
                </button>
                <button
                  v-if="!isChannelScope"
                  type="button"
                  class="h-9 shrink-0 rounded-lg border border-slate-600 px-2.5 text-xs text-amber-200/90 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="exporting || exportingTemplate || importing || importPreviewSubmitting || importPreviewOpen || anyListSaving || busy"
                  @click="triggerCsvImport('multi')"
                >
                  {{ importing ? t('common.submitting') : t('adminGlobalPricing.importCsvMultiChannel') }}
                </button>
                <input
                  ref="csvFileInput"
                  type="file"
                  accept=".csv,text/csv"
                  class="pointer-events-none fixed left-0 top-0 h-px w-px opacity-0"
                  tabindex="-1"
                  aria-hidden="true"
                  @change="onCsvFileChange($event, isChannelScope ? 'channel' : 'global')"
                />
                <input
                  ref="csvMultiFileInput"
                  type="file"
                  accept=".csv,text/csv"
                  class="pointer-events-none fixed left-0 top-0 h-px w-px opacity-0"
                  tabindex="-1"
                  aria-hidden="true"
                  @change="onCsvFileChange($event, 'multi')"
                />
              </div>
            </div>
            <p class="th-admin-pricing-workspace-hint px-0.5 text-[11px] leading-relaxed text-slate-500">
              {{ t('adminGlobalPricing.customRowsHint') }}
            </p>
          </div>

          <div class="th-admin-table-shell relative overflow-x-auto rounded-xl border border-slate-800">
            <ThLoadingState v-if="listUi && rows.length > 0" variant="overlay" />
            <table class="th-admin-pricing-table w-full border-collapse text-left text-xs">
              <thead class="th-admin-table-head text-slate-500">
                <tr>
                  <th class="th-admin-pricing-sticky-check w-10 shrink-0 px-2 py-2 text-center align-middle" rowspan="2">
                    <input
                      ref="selectAllInput"
                      type="checkbox"
                      class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                      :checked="allVisibleSelected"
                      :title="t('adminGlobalPricing.selectAllVisible')"
                      :aria-label="t('adminGlobalPricing.selectAllVisible')"
                      @change="onToggleSelectAll"
                    />
                  </th>
                  <th class="th-admin-pricing-sticky-model min-w-[10rem] align-middle px-2 py-2" rowspan="2">{{ t('adminGroups.thModelId') }}</th>
                  <th class="w-[5.5rem] align-middle px-2 py-2" rowspan="2">{{ t('adminGlobalPricing.thSources') }}</th>
                  <th class="th-admin-pricing-col-billing min-w-[8rem] align-middle px-2 py-2" rowspan="2">
                    <span class="block leading-snug">{{ t('adminGlobalPricing.thBilling') }}</span>
                  </th>
                  <th class="th-admin-pricing-col-token px-2 py-1 text-center" colspan="2">{{ t('adminGlobalPricing.thTokenQuota') }}</th>
                  <th class="w-14 align-middle px-2 py-2" rowspan="2">
                    <span class="block">{{ t('adminGlobalPricing.thMarkup') }}</span>
                    <span class="mt-0.5 block text-[10px] font-normal text-slate-500">{{ t('adminGlobalPricing.thMarkupSub') }}</span>
                  </th>
                  <th class="th-admin-pricing-col-rates px-2 py-1 text-center" colspan="2">{{ t('adminGlobalPricing.thRates') }}</th>
                  <th class="th-admin-pricing-col-code px-2 py-1 text-center" colspan="2">{{ t('adminGlobalPricing.thCodeRef') }}</th>
                  <th class="w-10 align-middle px-2 py-2 text-center" rowspan="2">{{ t('adminGroups.thEn') }}</th>
                  <th class="th-admin-pricing-sticky-actions min-w-[8.75rem] whitespace-nowrap px-1 py-2 text-right align-middle" rowspan="2">{{ t('adminGlobalPricing.thActions') }}</th>
                </tr>
                <tr class="text-[10px] font-normal">
                  <th class="px-2 py-1">
                    <span class="block">{{ thQuotaInLabel }}</span>
                    <span v-if="!codeRefShowQuotaOnly" class="text-slate-600">{{ t('adminGlobalPricing.thQuotaBillingSub') }}</span>
                  </th>
                  <th class="px-2 py-1">
                    <span class="block">{{ thQuotaOutLabel }}</span>
                    <span v-if="!codeRefShowQuotaOnly" class="text-slate-600">{{ t('adminGlobalPricing.thQuotaBillingSub') }}</span>
                  </th>
                  <th class="px-2 py-1">{{ t('adminGroups.thIn') }}</th>
                  <th class="px-2 py-1">{{ t('adminGroups.thOut') }}</th>
                  <th class="px-2 py-1">{{ t('adminGlobalPricing.thCodeIn') }}</th>
                  <th class="px-2 py-1">{{ t('adminGlobalPricing.thCodeOut') }}</th>
                </tr>
              </thead>
              <tbody class="th-admin-table-body divide-y divide-slate-800/80">
                <tr v-for="(r, idx) in rows" :key="`gwp-${idx}-${r.model_name || 'new'}`">
                  <td class="th-admin-pricing-sticky-check w-10 shrink-0 px-2 py-2 text-center align-middle">
                    <input
                      type="checkbox"
                      class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                      :checked="isRowSelected(idx)"
                      @change.stop="syncRowSelectionFromInput(idx, $event)"
                    />
                  </td>
                  <td class="th-admin-pricing-sticky-model min-w-[11rem] max-w-[min(42rem,85vw)] align-top px-2 py-2">
                    <div
                      class="th-model-combobox relative w-full min-w-0"
                      :ref="(el) => setComboboxRef(idx, el)"
                    >
                      <div class="flex w-full min-w-0 items-start gap-1">
                        <span
                          v-if="r.locked"
                          class="th-admin-pricing-model-locked block select-text break-all py-1.5 font-mono text-xs leading-relaxed"
                          :title="r.model_name"
                        >{{ r.model_name }}</span>
                        <template v-else>
                          <input
                            v-model="r.model_name"
                            class="th-input h-9 min-w-[8rem] flex-1 overflow-x-auto py-1 font-mono text-xs leading-normal"
                            autocomplete="off"
                            spellcheck="false"
                            :placeholder="t('adminGroups.modelIdPh')"
                            :title="r.model_name.trim() ? r.model_name : undefined"
                            @focus="onModelFocus(idx)"
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
                        </template>
                      </div>
                    </div>
                  </td>
                  <td class="align-middle px-2 py-2">
                    <div class="flex max-w-[10rem] flex-wrap gap-1">
                      <span
                        v-for="src in r.sources || []"
                        :key="src"
                        :class="sourceBadgeClass(src)"
                      >
                        {{ sourceBadgeLabel(src) }}
                      </span>
                      <span v-if="!(r.sources && r.sources.length)" class="text-slate-600">—</span>
                    </div>
                  </td>
                  <td class="th-admin-pricing-col-billing align-middle px-2 py-2">
                    <div class="flex min-w-[7.5rem] flex-col gap-1">
                      <select
                        v-model="r.billing_mode"
                        class="th-select th-select--table-cell w-full"
                      >
                        <option value="token">{{ t('adminGlobalPricing.billingModeToken') }}</option>
                        <option value="per_request">{{ t('adminGlobalPricing.billingModePerRequest') }}</option>
                        <option value="hybrid_floor">{{ t('adminGlobalPricing.billingModeHybridFloor') }}</option>
                      </select>
                      <input
                        v-if="r.billing_mode !== 'token'"
                        v-model="r.quota_per_request"
                        type="text"
                        inputmode="numeric"
                        class="th-input th-input--table-cell w-full font-mono"
                        :placeholder="t('adminGlobalPricing.phQuotaPerRequest')"
                      />
                    </div>
                  </td>
                  <td class="th-admin-pricing-col-token align-middle px-2 py-2" :class="{ 'opacity-45': !rowUsesTokenQuota(r) }">
                    <div v-if="rowUsesTokenQuota(r)" class="flex flex-col gap-0.5">
                      <input
                        :value="quotaInputDisplay(r, idx, 'in')"
                        type="text"
                        :inputmode="codeRefShowQuotaOnly ? 'numeric' : 'decimal'"
                        class="th-input th-input--table-cell w-24 font-mono"
                        :placeholder="quotaInPlaceholder"
                        :title="!codeRefShowQuotaOnly && r.quota_in_per_1k.trim() ? t('adminGlobalPricing.quotaStoredHint', { q: r.quota_in_per_1k }) : undefined"
                        autocomplete="off"
                        @focus="onQuotaFocus(r, idx, 'in')"
                        @input="onQuotaInput(idx, 'in', $event)"
                        @blur="commitQuotaBlur(r, idx, 'in')"
                      />
                      <span
                        v-if="r.quota_in_per_1k.trim() && billingChargePreview(r.quota_in_per_1k, r.markup_percent)"
                        class="font-mono text-[10px] tabular-nums text-slate-500"
                      >{{
                        t('adminGlobalPricing.quotaBillingCell', {
                          q: billingChargePreview(r.quota_in_per_1k, r.markup_percent),
                        })
                      }}</span>
                    </div>
                    <span v-else class="block py-2 text-center text-slate-600">—</span>
                  </td>
                  <td class="th-admin-pricing-col-token align-middle px-2 py-2" :class="{ 'opacity-45': !rowUsesTokenQuota(r) }">
                    <div v-if="rowUsesTokenQuota(r)" class="flex flex-col gap-0.5">
                      <input
                        :value="quotaInputDisplay(r, idx, 'out')"
                        type="text"
                        :inputmode="codeRefShowQuotaOnly ? 'numeric' : 'decimal'"
                        class="th-input th-input--table-cell w-24 font-mono"
                        :placeholder="quotaOutPlaceholder"
                        :title="!codeRefShowQuotaOnly && r.quota_out_per_1k.trim() ? t('adminGlobalPricing.quotaStoredHint', { q: r.quota_out_per_1k }) : undefined"
                        autocomplete="off"
                        @focus="onQuotaFocus(r, idx, 'out')"
                        @input="onQuotaInput(idx, 'out', $event)"
                        @blur="commitQuotaBlur(r, idx, 'out')"
                      />
                      <span
                        v-if="r.quota_out_per_1k.trim() && billingChargePreview(r.quota_out_per_1k, r.markup_percent)"
                        class="font-mono text-[10px] tabular-nums text-slate-500"
                      >{{
                        t('adminGlobalPricing.quotaBillingCell', {
                          q: billingChargePreview(r.quota_out_per_1k, r.markup_percent),
                        })
                      }}</span>
                    </div>
                    <span v-else class="block py-2 text-center text-slate-600">—</span>
                  </td>
                  <td class="align-middle px-2 py-2">
                    <input
                      v-model="r.markup_percent"
                      type="text"
                      inputmode="decimal"
                      class="th-input th-input--table-cell w-16 font-mono"
                      :placeholder="t('adminGlobalPricing.phMarkupSiteDefault')"
                      :title="t('adminGlobalPricing.markupRowHint')"
                    />
                  </td>
                  <td class="align-middle px-2 py-2">
                    <input
                      v-model.number="r.input_rate"
                      type="number"
                      step="0.01"
                      class="th-input th-input--table-cell w-20 font-mono"
                    />
                  </td>
                  <td class="align-middle px-2 py-2">
                    <input
                      v-model.number="r.output_rate"
                      type="number"
                      step="0.01"
                      class="th-input th-input--table-cell w-20 font-mono"
                    />
                  </td>
                  <td class="th-admin-pricing-col-code align-middle px-2 py-2 font-mono text-slate-400">
                    <div
                      class="flex flex-col gap-0.5 tabular-nums text-[11px]"
                      :title="displayCodeRefCell(r.code_in_ref).title"
                    >
                      <span
                        v-for="(line, li) in displayCodeRefCell(r.code_in_ref).lines"
                        :key="`cin-${li}`"
                        :class="line.muted ? 'text-[10px] text-slate-500' : ''"
                      >{{ line.text }}</span>
                    </div>
                  </td>
                  <td class="th-admin-pricing-col-code align-middle px-2 py-2 font-mono text-slate-400">
                    <div
                      class="flex flex-col gap-0.5 tabular-nums text-[11px]"
                      :title="displayCodeRefCell(r.code_out_ref).title"
                    >
                      <span
                        v-for="(line, li) in displayCodeRefCell(r.code_out_ref).lines"
                        :key="`cout-${li}`"
                        :class="line.muted ? 'text-[10px] text-slate-500' : ''"
                      >{{ line.text }}</span>
                    </div>
                  </td>
                  <td class="align-middle px-2 py-2 text-center">
                    <input
                      v-model="r.enabled"
                      type="checkbox"
                      class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                    />
                  </td>
                  <td class="th-admin-pricing-sticky-actions align-middle px-1 py-2 text-right">
                    <div class="inline-flex flex-nowrap items-center justify-end gap-1">
                      <button
                        type="button"
                        class="th-btn-primary-flat th-admin-pricing-row-act shrink-0"
                        :disabled="
                          !shouldPersistRow(r) ||
                          rowSavingIdx !== null ||
                          anyListSaving ||
                          busy
                        "
                        :title="
                          !shouldPersistRow(r) ? t('adminGlobalPricing.saveRowDisabledTip') : t('adminGlobalPricing.saveRow')
                        "
                        @click="saveOneRow(idx)"
                      >
                        {{ rowSavingIdx === idx ? t('common.submitting') : t('adminGlobalPricing.saveRow') }}
                      </button>
                      <button
                        type="button"
                        class="th-pg-btn-ghost th-admin-pricing-row-act shrink-0"
                        :disabled="!canResetRow(r) || rowSavingIdx !== null || anyListSaving || busy"
                        :title="
                          !canResetRow(r)
                            ? t('adminGlobalPricing.resetRowDisabledTip')
                            : r.isCustom
                              ? t('adminGlobalPricing.resetCustomHint')
                              : r.id != null
                                ? t('adminGlobalPricing.resetHint')
                                : t('adminGlobalPricing.resetRevertHint')
                        "
                        @click="resetRow(idx)"
                      >
                        {{ t('adminGlobalPricing.reset') }}
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="!rows.length">
                  <td colspan="11" class="px-3 py-8 text-center text-slate-500">{{ t('adminGlobalPricing.empty') }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div
            class="mt-3 flex flex-col gap-2 rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 py-2 sm:flex-row sm:flex-nowrap sm:items-center sm:justify-between"
            role="group"
            :aria-label="t('adminGlobalPricing.bottomBarAria')"
          >
            <div class="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center">
              <div
                class="flex flex-wrap items-center gap-1.5"
                role="group"
                :aria-label="t('adminGlobalPricing.bottomToolbarGroupSave')"
              >
                <button type="button" class="th-admin-toolbar-btn" @click="addRow">
                  {{ t('adminGroups.addRow') }}
                </button>
                <button
                  type="button"
                  class="th-btn-primary-flat th-admin-toolbar-primary"
                  :disabled="anyListSaving || busy"
                  @click="saveCurrentPage"
                >
                  {{ pageSaving ? t('common.submitting') : t('adminGlobalPricing.saveCurrentPage') }}
                </button>
                <button
                  type="button"
                  class="th-admin-toolbar-btn"
                  :disabled="anyListSaving || busy"
                  :title="t('adminGlobalPricing.saveFullHint')"
                  @click="saveFullWorkspace"
                >
                  {{ savingFull ? t('common.submitting') : t('adminGlobalPricing.saveFull') }}
                </button>
                <button
                  type="button"
                  class="th-admin-toolbar-btn"
                  :disabled="anyListSaving || busy || !selectedRows.length || rowSavingIdx !== null"
                  @click="saveSelectedRows"
                >
                  {{ batchSaving ? t('common.submitting') : t('adminGlobalPricing.saveSelected') }}
                </button>
              </div>
              <div class="hidden h-6 w-px shrink-0 bg-slate-600/50 sm:block" aria-hidden="true" />
              <div
                class="flex flex-wrap items-center gap-1.5"
                role="group"
                :aria-label="t('adminGlobalPricing.bottomToolbarGroupReset')"
              >
                <button
                  type="button"
                  class="th-admin-toolbar-btn"
                  :disabled="anyListSaving || busy || rowSavingIdx !== null"
                  :title="t('adminGlobalPricing.resetPageHint')"
                  @click="resetCurrentPage"
                >
                  {{ busyResetPage ? t('common.submitting') : t('adminGlobalPricing.resetCurrentPage') }}
                </button>
                <button
                  type="button"
                  class="th-admin-toolbar-btn th-admin-toolbar-btn--danger"
                  :disabled="anyListSaving || busy || rowSavingIdx !== null"
                  :title="t('adminGlobalPricing.resetFullHint')"
                  @click="resetFullWorkspace"
                >
                  {{ busyResetFull ? t('common.submitting') : t('adminGlobalPricing.resetFull') }}
                </button>
                <button
                  type="button"
                  class="th-admin-toolbar-btn"
                  :disabled="anyListSaving || busy || !selectedRows.length || rowSavingIdx !== null"
                  :title="t('adminGlobalPricing.resetSelectedHint')"
                  @click="resetSelectedRows"
                >
                  {{ busyResetSelected ? t('common.submitting') : t('adminGlobalPricing.resetSelected') }}
                </button>
              </div>
              <div class="hidden h-6 w-px shrink-0 bg-slate-600/50 sm:block" aria-hidden="true" />
              <button
                type="button"
                class="th-admin-toolbar-btn sm:shrink-0"
                :disabled="anyListSaving || busy || rowSavingIdx !== null"
                @click="load"
              >
                {{ t('adminCommon.refresh') }}
              </button>
            </div>
            <div
              class="flex min-w-0 flex-wrap items-center justify-end gap-2 border-t border-slate-800/80 pt-2 sm:border-t-0 sm:pt-0"
              role="navigation"
              :aria-label="t('adminGlobalPricing.paginationBottomAria')"
            >
              <span
                class="min-w-0 truncate text-right text-xs tabular-nums text-slate-400 sm:max-w-[20rem]"
                :title="`${t('adminGlobalPricing.pageTotal', { total })} · ${t('adminGlobalPricing.pageStatus', { page, pages: totalPages })}`"
              >
                <span class="text-slate-300">{{ t('adminGlobalPricing.pageTotal', { total }) }}</span>
                <span class="mx-1.5 text-slate-600" aria-hidden="true">·</span>
                <span class="font-mono text-slate-300">{{
                  t('adminGlobalPricing.pageStatus', { page, pages: totalPages })
                }}</span>
              </span>
              <div class="flex shrink-0 items-center gap-1">
                <button
                  type="button"
                  class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="page <= 1 || busy"
                  @click="prevPage"
                >
                  {{ t('adminCommon.prevPage') }}
                </button>
                <button
                  type="button"
                  class="h-9 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="page >= totalPages || busy"
                  @click="nextPage"
                >
                  {{ t('adminCommon.nextPage') }}
                </button>
              </div>
            </div>
          </div>
          <p class="mt-2 text-[11px] leading-relaxed text-slate-600">{{ t('adminGlobalPricing.csvRoundTripHint') }}</p>
        </template>
      </div>
    </article>

    <Teleport to="body">
      <Transition name="th-admin-flash-fade">
        <div
          v-if="flashMsg"
          id="global-pricing-flash"
          class="pointer-events-none fixed left-1/2 top-20 z-[10050] max-w-[min(22rem,calc(100vw-2rem))] -translate-x-1/2 rounded-lg border px-4 py-2.5 text-center text-xs shadow-lg backdrop-blur-sm"
          :class="flashIsError ? 'th-admin-flash--error' : 'th-admin-flash--success'"
          role="status"
          aria-live="polite"
        >
          {{ flashMsg }}
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="importPreviewOpen"
        class="th-feedback-backdrop fixed inset-0 z-[10045] flex items-center justify-center p-4 sm:p-6"
        role="dialog"
        aria-modal="true"
        :aria-label="t('adminGlobalPricing.importPreviewTitle')"
        @click.self="closeImportPreview"
      >
        <div
          class="th-feedback-modal th-import-preview-modal thin-scrollbar flex max-h-[min(85vh,52rem)] w-full max-w-5xl flex-col overflow-hidden rounded-2xl"
          @click.stop
        >
          <div class="th-feedback-modal__head shrink-0 px-5 py-4">
            <h4 class="th-feedback-modal__title text-base font-semibold">{{ t('adminGlobalPricing.importPreviewTitle') }}</h4>
            <p class="th-feedback-modal__intro mt-1.5 text-xs leading-relaxed">{{
              importPreviewIsMulti
                ? t('adminGlobalPricing.importPreviewIntroMulti')
                : t('adminGlobalPricing.importPreviewIntro')
            }}</p>
            <p class="th-feedback-modal__muted mt-1 text-[11px]">
              {{ t('adminGlobalPricing.importPreviewCount', { n: importPreviewRows.length }) }}
            </p>
          </div>
          <!-- border-collapse 会破坏表头 sticky；表头样式见 .th-import-preview-th -->
          <div class="thin-scrollbar min-h-0 flex-1 overflow-auto px-3 pb-3 pt-0 sm:px-5 sm:pb-5">
            <div class="th-import-preview-table">
              <table class="w-full min-w-[640px] border-separate border-spacing-0 text-left text-xs">
                <thead>
                  <tr>
                    <th
                      v-if="importPreviewIsMulti"
                      class="th-import-preview-th w-16 px-2 py-2.5 text-left"
                    >
                      {{ t('adminGlobalPricing.importPreviewChannelCol') }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-left">
                      {{ t('adminGlobalPricing.importPreviewModelCol') }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-left">
                      {{ thQuotaInLabel }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-left">
                      {{ thQuotaOutLabel }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-left">
                      {{ t('adminGroups.thIn') }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-left">
                      {{ t('adminGroups.thOut') }}
                    </th>
                    <th class="th-import-preview-th px-2 py-2.5 text-center">
                      {{ t('adminGroups.thEn') }}
                    </th>
                    <th class="th-import-preview-th w-20 px-1 py-2.5 text-right">
                      {{ t('adminGlobalPricing.thActions') }}
                    </th>
                  </tr>
                </thead>
                <tbody>
                <tr v-for="(r, idx) in importPreviewRows" :key="`ipr-${idx}-${r.channel_id ?? ''}-${r.model_name}`">
                  <td v-if="importPreviewIsMulti" class="px-2 py-2 align-middle">
                    <span class="font-mono text-xs text-slate-300">{{ r.channel_id ?? '—' }}</span>
                  </td>
                  <td class="px-2 py-2 align-middle">
                    <input
                      v-model="r.model_name"
                      type="text"
                      class="th-input h-9 min-w-[10rem] max-w-[20rem] py-1 font-mono text-xs leading-normal"
                      autocomplete="off"
                      spellcheck="false"
                      :placeholder="t('adminGroups.modelIdPh')"
                    />
                  </td>
                  <td class="px-2 py-2 align-middle">
                    <div class="flex flex-col gap-0.5">
                      <input
                        :value="quotaInputDisplay(r, idx, 'in')"
                        type="text"
                        :inputmode="codeRefShowQuotaOnly ? 'numeric' : 'decimal'"
                        class="th-input h-9 w-28 py-1 font-mono text-xs leading-normal"
                        :placeholder="quotaInPlaceholder"
                        autocomplete="off"
                        @focus="onQuotaFocus(r, idx, 'in')"
                        @input="onQuotaInput(idx, 'in', $event)"
                        @blur="commitQuotaBlur(r, idx, 'in')"
                      />
                      <span
                        v-if="r.quota_in_per_1k.trim() && billingChargePreview(r.quota_in_per_1k, r.markup_percent)"
                        class="font-mono text-[10px] tabular-nums text-slate-500"
                      >{{
                        t('adminGlobalPricing.quotaBillingCell', {
                          q: billingChargePreview(r.quota_in_per_1k, r.markup_percent),
                        })
                      }}</span>
                    </div>
                  </td>
                  <td class="px-2 py-2 align-middle">
                    <div class="flex flex-col gap-0.5">
                      <input
                        :value="quotaInputDisplay(r, idx, 'out')"
                        type="text"
                        :inputmode="codeRefShowQuotaOnly ? 'numeric' : 'decimal'"
                        class="th-input h-9 w-28 py-1 font-mono text-xs leading-normal"
                        :placeholder="quotaOutPlaceholder"
                        autocomplete="off"
                        @focus="onQuotaFocus(r, idx, 'out')"
                        @input="onQuotaInput(idx, 'out', $event)"
                        @blur="commitQuotaBlur(r, idx, 'out')"
                      />
                      <span
                        v-if="r.quota_out_per_1k.trim() && billingChargePreview(r.quota_out_per_1k, r.markup_percent)"
                        class="font-mono text-[10px] tabular-nums text-slate-500"
                      >{{
                        t('adminGlobalPricing.quotaBillingCell', {
                          q: billingChargePreview(r.quota_out_per_1k, r.markup_percent),
                        })
                      }}</span>
                    </div>
                  </td>
                  <td class="px-2 py-2 align-middle">
                    <input
                      v-model.number="r.input_rate"
                      type="number"
                      step="0.01"
                      class="th-input th-input--table-cell w-20 font-mono"
                    />
                  </td>
                  <td class="px-2 py-2 align-middle">
                    <input
                      v-model.number="r.output_rate"
                      type="number"
                      step="0.01"
                      class="th-input th-input--table-cell w-20 font-mono"
                    />
                  </td>
                  <td class="px-2 py-2 text-center align-middle">
                    <input v-model="r.enabled" type="checkbox" class="h-4 w-4 cursor-pointer accent-cyan-600" />
                  </td>
                  <td class="px-1 py-2 text-right align-middle">
                    <button
                      type="button"
                      class="th-pg-btn-ghost th-admin-pricing-row-act"
                      :disabled="importPreviewSubmitting"
                      @click="removeImportPreviewRow(idx)"
                    >
                      {{ t('adminGlobalPricing.importPreviewRemoveRow') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
            </div>
          </div>
          <div
            class="th-modal-footer-bar th-modal-divider-t th-import-preview-footer flex shrink-0 flex-col gap-2 px-5 py-3 sm:flex-row sm:items-center sm:justify-end"
          >
            <button
              type="button"
              class="th-modal-secondary-btn inline-flex h-9 items-center justify-center px-4 text-xs disabled:opacity-40"
              :disabled="importPreviewSubmitting"
              @click="closeImportPreview"
            >
              {{ t('adminGlobalPricing.importPreviewCancel') }}
            </button>
            <button
              type="button"
              class="th-btn-primary-flat inline-flex h-9 items-center justify-center px-4 text-xs disabled:opacity-40"
              :disabled="importPreviewSubmitting || !importPreviewRows.length"
              @click="confirmImportPreview"
            >
              {{ importPreviewSubmitting ? t('common.submitting') : t('adminGlobalPricing.importPreviewConfirm') }}
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
            class="th-model-combobox-item flex items-center justify-between gap-2"
            role="option"
            @mousedown.prevent="pickOpenModelSuggestId(opt)"
          >
            <span class="min-w-0 truncate font-mono">{{ opt }}</span>
            <span
              v-if="channelCountForModel(opt) > 1"
              class="shrink-0 text-[10px] text-amber-300/90"
            >{{ t('adminModelChannelPick.multiBadge', { n: channelCountForModel(opt) }) }}</span>
          </li>
          <li
            v-if="suggestListTruncated"
            class="th-model-combobox-item th-model-combobox-item--muted cursor-default"
          >
            {{ t('adminModelChannelPick.suggestTruncate', { total: modelSuggestTotal }) }}
          </li>
        </template>
        <li v-else class="th-model-combobox-item th-model-combobox-item--muted cursor-default">
          {{ t('adminGroups.modelSuggestEmpty') }}
        </li>
      </ul>
    </Teleport>

    <AdminModelChannelPickModal
      :open="channelPickOpen"
      :model-id="channelPickModelId"
      :channels="channelPickChannels"
      :selected-channel-id="channelPickSelectedId"
      @update:selected-channel-id="channelPickSelectedId = $event"
      @confirm="confirmChannelPick"
      @cancel="cancelChannelPick"
    />
  </div>
</template>
