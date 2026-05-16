import { computed, nextTick, onUnmounted, ref, watch, type Ref } from 'vue'
import { http, type Envelope } from '../api/http'

/** 行上至少含 model_name，供模型 ID 建议与下拉面板使用（与 AdminGroups 模型倍率一致） */
export type ModelNameRow = { model_name: string }

let cachedCatalogModelIds: string[] | null = null

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

/**
 * 与 AdminGroups 内「模型 ID」下拉相同的定位与滚动行为；建议来源：模型目录 + 全渠道类型预设 + 当前表内 ID。
 */
export function useAdminModelIdCombobox(rowsRef: Ref<ModelNameRow[]>) {
  const modelSuggestIds = ref<string[]>([])
  const modelSuggestLoading = ref(false)
  const openModelSuggestRow = ref<number | null>(null)
  const comboboxAnchorEls = new Map<number, HTMLElement>()
  const modelSuggestPanelStyle = ref<Record<string, string>>({})

  function setComboboxRef(rowIdx: number, el: unknown) {
    if (el instanceof HTMLElement) comboboxAnchorEls.set(rowIdx, el)
    else comboboxAnchorEls.delete(rowIdx)
  }

  function updateModelSuggestPanelLayout() {
    const i = openModelSuggestRow.value
    if (i === null) {
      modelSuggestPanelStyle.value = {}
      return
    }
    const anchor = comboboxAnchorEls.get(i)
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
    const anchor = comboboxAnchorEls.get(i)
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
    return rowsRef.value[i]?.model_name ?? ''
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
    const row = rowsRef.value[rowIdx]
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

  async function loadModelSuggestions() {
    modelSuggestLoading.value = true
    try {
      const set = new Set<string>()
      for (const r of rowsRef.value) {
        const m = r.model_name.trim()
        if (m) set.add(m)
      }
      const catalog = await fetchCatalogModelIds()
      for (const id of catalog) set.add(id)

      const types = new Set<string>()
      try {
        const { data } = await http.get<Envelope<{ items: Array<{ channel_type?: string }> }>>('/api/admin/channels')
        if (data.code === 0) {
          for (const ch of data.data.items || []) {
            const ct = (ch.channel_type || '').trim().toLowerCase()
            if (ct) types.add(ct)
          }
        }
      } catch {
        /* ignore */
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
            /* ignore */
          }
        }),
      )
      modelSuggestIds.value = Array.from(set).sort((a, b) => a.localeCompare(b))
    } finally {
      modelSuggestLoading.value = false
    }
  }

  return {
    modelSuggestIds,
    modelSuggestLoading,
    openModelSuggestRow,
    modelSuggestPanelStyle,
    setComboboxRef,
    filteredOpenModelSuggestIds,
    scheduleSuggestLayout,
    toggleModelSuggestRow,
    focusModelSuggestRow,
    pickOpenModelSuggestId,
    updateModelSuggestPanelLayout,
    loadModelSuggestions,
  }
}
