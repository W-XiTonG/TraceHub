import { computed, nextTick, onUnmounted, ref, watch, type ComputedRef, type Ref } from 'vue'
import { http, type Envelope } from '../api/http'
import {
  buildModelChannelMap,
  type AdminModelChannelRef,
} from '../utils/adminModelChannelMap'
import { fetchAllWorkspaceModelIds } from '../utils/adminWorkspaceModels'
import {
  comboboxAnchorKey,
  resolveComboboxAnchor,
  type ComboboxAnchorVariant,
} from '../utils/comboboxAnchor'

/** 行上至少含 model_name，供模型 ID 建议与下拉面板使用 */
export type ModelNameRow = { model_name: string }

export type AdminModelIdComboboxOptions = {
  /** 工作区范围：'global' 与全局计价一致；正整数为单渠道计价工作区 */
  workspaceScope?: Ref<'global' | number> | ComputedRef<'global' | number>
  /** 选中出现在多个渠道上的模型时弹出渠道选择 */
  enableChannelPick?: boolean
}

const SUGGEST_EMPTY_LIMIT = 200
const SUGGEST_FILTER_LIMIT = 250

/**
 * 模型 ID 下拉：默认从全局/渠道计价工作区加载（与全局模型计价数量一致）；
 * 可选在多渠道重复时弹出渠道选择后再写入 model_name。
 */
export function useAdminModelIdCombobox(
  rowsRef: Ref<ModelNameRow[]>,
  options: AdminModelIdComboboxOptions = {},
) {
  const modelSuggestIds = ref<string[]>([])
  const modelSuggestTotal = ref(0)
  const modelSuggestLoading = ref(false)
  const modelChannelMap = ref<Map<string, AdminModelChannelRef[]>>(new Map())
  const openModelSuggestRow = ref<number | null>(null)
  const comboboxAnchorEls = new Map<string, HTMLElement>()
  const modelSuggestPanelStyle = ref<Record<string, string>>({})

  const channelPickOpen = ref(false)
  const channelPickRowIdx = ref<number | null>(null)
  const channelPickModelId = ref('')
  const channelPickChannels = ref<AdminModelChannelRef[]>([])
  const channelPickSelectedId = ref<number | ''>('')

  function setComboboxRef(rowIdx: number, el: unknown, variant: ComboboxAnchorVariant = 'desktop') {
    const key = comboboxAnchorKey(rowIdx, variant)
    if (el instanceof HTMLElement) comboboxAnchorEls.set(key, el)
    else comboboxAnchorEls.delete(key)
  }

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
    if (!s) return all.slice(0, SUGGEST_EMPTY_LIMIT)
    const out: string[] = []
    for (const id of all) {
      if (id.toLowerCase().includes(s)) {
        out.push(id)
        if (out.length >= SUGGEST_FILTER_LIMIT) break
      }
    }
    return out
  }

  function channelsForModel(modelId: string): AdminModelChannelRef[] {
    return modelChannelMap.value.get(modelId.trim()) || []
  }

  function channelCountForModel(modelId: string): number {
    return channelsForModel(modelId).length
  }

  const activeModelSuggestQuery = computed(() => {
    const i = openModelSuggestRow.value
    if (i === null) return ''
    return rowsRef.value[i]?.model_name ?? ''
  })

  const filteredOpenModelSuggestIds = computed(() => filterModelSuggestIds(activeModelSuggestQuery.value))

  const suggestListTruncated = computed(() => {
    const q = activeModelSuggestQuery.value.trim()
    const total = modelSuggestTotal.value
    const shown = filteredOpenModelSuggestIds.value.length
    if (!q) return total > SUGGEST_EMPTY_LIMIT && shown >= SUGGEST_EMPTY_LIMIT
    return shown >= SUGGEST_FILTER_LIMIT
  })

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

  function applyModelPick(rowIdx: number, id: string) {
    const row = rowsRef.value[rowIdx]
    if (row) row.model_name = id
    openModelSuggestRow.value = null
  }

  function openChannelPick(rowIdx: number, modelId: string, channels: AdminModelChannelRef[]) {
    channelPickRowIdx.value = rowIdx
    channelPickModelId.value = modelId
    channelPickChannels.value = channels
    channelPickSelectedId.value = channels[0]?.id ?? ''
    channelPickOpen.value = true
    openModelSuggestRow.value = null
  }

  function cancelChannelPick() {
    channelPickOpen.value = false
    channelPickRowIdx.value = null
    channelPickModelId.value = ''
    channelPickChannels.value = []
    channelPickSelectedId.value = ''
  }

  function confirmChannelPick() {
    const rowIdx = channelPickRowIdx.value
    const modelId = channelPickModelId.value.trim()
    const chId = channelPickSelectedId.value
    if (rowIdx === null || !modelId || typeof chId !== 'number' || chId <= 0) return
    applyModelPick(rowIdx, modelId)
    cancelChannelPick()
  }

  function pickModelRateId(rowIdx: number, id: string) {
    const trimmed = id.trim()
    if (!trimmed) return
    if (options.enableChannelPick) {
      const channels = channelsForModel(trimmed)
      if (channels.length > 1) {
        openChannelPick(rowIdx, trimmed, channels)
        return
      }
    }
    applyModelPick(rowIdx, trimmed)
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

  async function loadChannelsForMap() {
    try {
      const { data } = await http.get<
        Envelope<{
          items: Array<{
            id: number
            channel_name?: string
            channel_type?: string
            enabled?: boolean
            health_status?: number
            models?: string[]
            model_alias?: Record<string, string>
          }>
        }>
      >('/api/admin/channels')
      if (data.code !== 0) return
      modelChannelMap.value = buildModelChannelMap(data.data.items || [])
    } catch {
      modelChannelMap.value = new Map()
    }
  }

  async function loadModelSuggestions() {
    modelSuggestLoading.value = true
    try {
      const set = new Set<string>()
      for (const r of rowsRef.value) {
        const m = r.model_name.trim()
        if (m) set.add(m)
      }

      const scope = options.workspaceScope?.value ?? 'global'
      const channelId = typeof scope === 'number' && scope > 0 ? scope : undefined
      const workspaceIds = await fetchAllWorkspaceModelIds(
        channelId ? { channelId } : undefined,
      )
      for (const id of workspaceIds) set.add(id)

      if (options.enableChannelPick) {
        await loadChannelsForMap()
      }

      const ids = Array.from(set).sort((a, b) => a.localeCompare(b))
      modelSuggestIds.value = ids
      modelSuggestTotal.value = ids.length
    } finally {
      modelSuggestLoading.value = false
    }
  }

  if (options.workspaceScope) {
    watch(options.workspaceScope, () => {
      void loadModelSuggestions()
    })
  }

  return {
    modelSuggestIds,
    modelSuggestTotal,
    modelSuggestLoading,
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
    updateModelSuggestPanelLayout,
    loadModelSuggestions,
    channelPickOpen,
    channelPickModelId,
    channelPickChannels,
    channelPickSelectedId,
    confirmChannelPick,
    cancelChannelPick,
  }
}
