<script setup lang="ts">
import axios from 'axios'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAdminModelIdCombobox } from '../composables/useAdminModelIdCombobox'
import { http, type Envelope } from '../api/http'

const { t } = useI18n()

type DraftRow = {
  id?: number
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
}

type WorkspaceRow = {
  model_name: string
  code_in_per_1k: number
  code_out_per_1k: number
  sources: string[]
  global?: ApiRow | null
}

const customDrafts = ref<DraftRow[]>([])
const workspacePageDrafts = ref<DraftRow[]>([])
const rows = ref<DraftRow[]>([])
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)
const searchDraft = ref('')
const searchApplied = ref('')
const busy = ref(true)
const pageSaving = ref(false)
const batchSaving = ref(false)
const exporting = ref(false)
const importing = ref(false)
const csvFileInput = ref<HTMLInputElement | null>(null)
const flashMsg = ref('')
const flashIsError = ref(false)

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
  openModelSuggestRow,
  modelSuggestPanelStyle,
  setComboboxRef,
  filteredOpenModelSuggestIds,
  scheduleSuggestLayout,
  toggleModelSuggestRow,
  focusModelSuggestRow,
  pickOpenModelSuggestId,
  loadModelSuggestions,
} = useAdminModelIdCombobox(rows)

const totalPages = computed(() => {
  if (total.value <= 0) return 1
  return Math.max(1, Math.ceil(total.value / pageSize.value))
})

function rebuildRows() {
  rows.value = [...workspacePageDrafts.value, ...customDrafts.value]
}

function clearFlash() {
  flashMsg.value = ''
  flashIsError.value = false
}

function flashSuccess(text: string) {
  flashIsError.value = false
  flashMsg.value = text
}

function httpErr(e: unknown): string {
  if (axios.isAxiosError(e) && e.response?.data && typeof e.response.data === 'object') {
    const d = e.response.data as { message?: string }
    if (typeof d.message === 'string' && d.message.trim()) return d.message.trim()
  }
  return e instanceof Error ? e.message : t('adminCommon.requestFail')
}

function flashError(e: unknown) {
  flashIsError.value = true
  flashMsg.value = httpErr(e)
}

function workspaceItemToDraft(w: WorkspaceRow): DraftRow {
  const g = w.global
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
  model_name: string
  enabled: boolean
  quota_in_per_1k?: number | null
  quota_out_per_1k?: number | null
  input_rate: number
  output_rate: number
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
  const item: ApiPricingItem = {
    model_name: name,
    enabled: r.enabled,
    input_rate: inR,
    output_rate: outR,
  }
  if (qIn.value !== undefined) item.quota_in_per_1k = qIn.value
  if (qOut.value !== undefined) item.quota_out_per_1k = qOut.value
  return { ok: true, item }
}

function draftsToPersistItems(drafts: DraftRow[]): ApiPricingItem[] | null {
  const items: ApiPricingItem[] = []
  for (const r of drafts) {
    if (!shouldPersistRow(r)) continue
    const v = validateRowToItem(r)
    if (!v.ok) {
      flashMsg.value = v.message
      flashIsError.value = true
      return null
    }
    items.push(v.item)
  }
  return items
}

async function putPricingItems(items: ApiPricingItem[]): Promise<void> {
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
    busyResetFull.value,
)

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
  for (;;) {
    const { data } = await http.get<
      Envelope<{ items: WorkspaceRow[]; total: number; page: number; page_size: number }>
    >('/api/admin/global-model-pricing/workspace', {
      params: { page: p, page_size: chunk, q },
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
    case 'catalog':
      return t('adminGlobalPricing.sourceCatalog')
    case 'global_db':
      return t('adminGlobalPricing.sourceGlobalDb')
    default:
      return src
  }
}

function onModelFocus(idx: number) {
  if (rows.value[idx]?.locked) return
  focusModelSuggestRow(idx)
}

async function load() {
  clearFlash()
  busy.value = true
  try {
    const { data } = await http.get<
      Envelope<{ items: WorkspaceRow[]; total: number; page: number; page_size: number }>
    >('/api/admin/global-model-pricing/workspace', {
      params: {
        page: page.value,
        page_size: pageSize.value,
        q: searchApplied.value || undefined,
      },
    })
    if (data.code !== 0) throw new Error(data.message || 'load')
    total.value = typeof data.data.total === 'number' ? data.data.total : 0
    if (typeof data.data.page === 'number' && data.data.page >= 1) {
      page.value = data.data.page
    }
    workspacePageDrafts.value = (data.data.items || []).map(workspaceItemToDraft)
    rebuildRows()
    selectedRows.value = []
    void loadModelSuggestions()
  } catch (e: unknown) {
    flashError(e)
  } finally {
    busy.value = false
  }
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
  if (r.locked && r.id == null) return
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

function canShowReset(r: DraftRow): boolean {
  return r.isCustom === true || r.id != null || r.locked === false || r.locked === undefined
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
    flashMsg.value = t('adminGlobalPricing.nothingSelected')
    flashIsError.value = true
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
    flashMsg.value = t('adminGlobalPricing.nothingSelected')
    flashIsError.value = true
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
  if (!shouldPersistRow(r)) {
    flashSuccess(t('adminGlobalPricing.nothingToSaveRow'))
    return
  }
  const v = validateRowToItem(r)
  if (!v.ok) {
    flashMsg.value = v.message
    flashIsError.value = true
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

onMounted(() => void load())

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

async function exportCsv() {
  clearFlash()
  exporting.value = true
  try {
    const res = await http.get('/api/admin/global-model-pricing/export', { responseType: 'blob' })
    const name = parseExportFilename(res.headers['content-disposition'] as string | undefined)
    const blob = res.data instanceof Blob ? res.data : new Blob([res.data as BlobPart], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.rel = 'noopener'
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    flashSuccess(t('adminGlobalPricing.exported'))
  } catch (e: unknown) {
    flashError(e)
  } finally {
    exporting.value = false
  }
}

function triggerCsvImport() {
  clearFlash()
  csvFileInput.value?.click()
}

async function onCsvFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  importing.value = true
  clearFlash()
  try {
    const fd = new FormData()
    fd.append('file', file)
    const { data } = await http.post<Envelope<{ count: number }>>('/api/admin/global-model-pricing/import', fd)
    if (data.code !== 0) throw new Error(data.message || 'import')
    const n = typeof data.data?.count === 'number' ? data.data.count : 0
    await load()
    flashSuccess(t('adminGlobalPricing.imported', { count: n }))
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
      </div>

      <div class="border-t border-slate-800/80 bg-slate-950/40 px-5 py-4">
        <p v-if="busy" class="text-sm text-slate-500">{{ t('adminGroups.loading') }}</p>
        <template v-else>
          <p class="mb-1 text-xs text-slate-500">{{ t('adminGlobalPricing.ratesTableHint') }}</p>
          <p class="mb-3 text-[11px] leading-relaxed text-slate-500">
            <span v-if="modelSuggestLoading">{{ t('adminGroups.modelSuggestLoading') }}</span>
            <span v-else>{{ t('adminGlobalPricing.modelSuggestHint') }}</span>
          </p>

          <div class="mb-3 space-y-2 border-b border-slate-800/60 pb-3">
            <div
              class="flex flex-col gap-3 rounded-lg border border-slate-800/80 bg-slate-900/40 px-3 py-2.5 sm:flex-row sm:flex-wrap sm:items-center sm:gap-x-1 sm:gap-y-2 sm:py-2"
            >
              <!-- 筛选 -->
              <div class="flex min-w-0 flex-1 items-center gap-2 sm:max-w-md">
                <span class="shrink-0 text-xs text-slate-500">{{ t('adminGlobalPricing.searchModel') }}</span>
                <input
                  v-model="searchDraft"
                  type="search"
                  autocomplete="off"
                  class="th-input h-9 min-w-0 flex-1 font-mono text-xs sm:max-w-[14rem]"
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

              <div
                class="hidden h-6 w-px shrink-0 bg-slate-700/90 sm:block"
                aria-hidden="true"
              />

              <!-- 每页条数 -->
              <div class="flex shrink-0 items-center gap-2">
                <span class="text-xs text-slate-500">{{ t('adminGlobalPricing.pageSize') }}</span>
                <select
                  v-model.number="pageSize"
                  class="th-input h-9 w-[4.25rem] py-0 pr-6 text-xs leading-none"
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

              <!-- 分页 -->
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
                <button
                  type="button"
                  class="h-9 shrink-0 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="exporting || importing || busy"
                  @click="exportCsv"
                >
                  {{ exporting ? t('common.submitting') : t('adminGlobalPricing.exportCsv') }}
                </button>
                <button
                  type="button"
                  class="h-9 shrink-0 rounded-lg border border-slate-600 px-2.5 text-xs text-slate-200 hover:bg-slate-800 disabled:opacity-40"
                  :disabled="exporting || importing || anyListSaving || busy"
                  @click="triggerCsvImport"
                >
                  {{ importing ? t('common.submitting') : t('adminGlobalPricing.importCsv') }}
                </button>
                <input
                  ref="csvFileInput"
                  type="file"
                  accept=".csv,text/csv"
                  class="pointer-events-none fixed left-0 top-0 h-px w-px opacity-0"
                  tabindex="-1"
                  aria-hidden="true"
                  @change="onCsvFileChange"
                />
              </div>
            </div>
            <p class="text-[11px] leading-relaxed text-slate-500">{{ t('adminGlobalPricing.customRowsHint') }}</p>
          </div>

          <div class="overflow-x-auto rounded-xl border border-slate-800">
            <table class="w-full min-w-[1080px] border-collapse text-left text-xs">
              <thead class="bg-slate-900 text-slate-500">
                <tr>
                  <th class="w-10 shrink-0 px-2 py-2 text-center align-middle">
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
                  <th class="min-w-[12rem] max-w-[min(42rem,85vw)] align-middle px-3 py-2">{{ t('adminGroups.thModelId') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGlobalPricing.thSources') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGlobalPricing.thCodeIn') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGlobalPricing.thCodeOut') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGlobalPricing.thQuotaIn') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGlobalPricing.thQuotaOut') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGroups.thIn') }}</th>
                  <th class="align-middle px-3 py-2">{{ t('adminGroups.thOut') }}</th>
                  <th class="align-middle px-3 py-2 text-center">{{ t('adminGroups.thEn') }}</th>
                  <th class="min-w-[5.25rem] whitespace-nowrap px-1 py-2 text-right align-middle">{{ t('adminGlobalPricing.thActions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-slate-800">
                <tr v-for="(r, idx) in rows" :key="`gwp-${idx}-${r.model_name || 'new'}`">
                  <td class="w-10 shrink-0 px-2 py-2 text-center align-middle">
                    <input
                      type="checkbox"
                      class="h-4 w-4 cursor-pointer align-middle accent-cyan-600"
                      :checked="isRowSelected(idx)"
                      @change.stop="syncRowSelectionFromInput(idx, $event)"
                    />
                  </td>
                  <td class="min-w-[11rem] max-w-[min(42rem,85vw)] align-top px-2 py-2">
                    <div
                      class="th-model-combobox relative w-full min-w-0"
                      :ref="(el) => setComboboxRef(idx, el)"
                    >
                      <div class="flex w-full min-w-0 items-start gap-1">
                        <span
                          v-if="r.locked"
                          class="block select-text break-all py-1.5 font-mono text-xs leading-relaxed text-slate-200"
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
                        class="inline-block rounded border border-slate-700/90 bg-slate-800/60 px-1.5 py-0.5 font-mono text-[10px] text-slate-300"
                      >
                        {{ sourceBadgeLabel(src) }}
                      </span>
                      <span v-if="!(r.sources && r.sources.length)" class="text-slate-600">—</span>
                    </div>
                  </td>
                  <td class="align-middle px-2 py-2 font-mono text-slate-400">
                    {{ r.code_in_ref != null ? r.code_in_ref : '—' }}
                  </td>
                  <td class="align-middle px-2 py-2 font-mono text-slate-400">
                    {{ r.code_out_ref != null ? r.code_out_ref : '—' }}
                  </td>
                  <td class="align-middle px-2 py-2">
                    <input
                      v-model="r.quota_in_per_1k"
                      type="text"
                      inputmode="numeric"
                      class="th-input h-9 w-24 py-1 font-mono text-xs leading-normal"
                      :placeholder="t('adminGlobalPricing.phOptional')"
                      autocomplete="off"
                    />
                  </td>
                  <td class="align-middle px-2 py-2">
                    <input
                      v-model="r.quota_out_per_1k"
                      type="text"
                      inputmode="numeric"
                      class="th-input h-9 w-24 py-1 font-mono text-xs leading-normal"
                      :placeholder="t('adminGlobalPricing.phOptional')"
                      autocomplete="off"
                    />
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
                  <td class="align-middle px-1 py-2 text-right">
                    <div class="inline-flex max-w-full flex-nowrap items-center justify-end gap-1">
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
                        v-if="canShowReset(r)"
                        type="button"
                        class="th-pg-btn-ghost th-admin-pricing-row-act shrink-0"
                        :title="r.isCustom ? t('adminGlobalPricing.resetCustomHint') : t('adminGlobalPricing.resetHint')"
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
