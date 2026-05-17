/** 响应式布局下同一行可能存在 desktop / mobile 两套锚点，定位时需选用可见的那一个。 */
export type ComboboxAnchorVariant = 'desktop' | 'mobile'

export function comboboxAnchorKey(rowIdx: number, variant: ComboboxAnchorVariant = 'desktop'): string {
  return `${variant}:${rowIdx}`
}

export function isComboboxAnchorVisible(el: HTMLElement): boolean {
  if (!el.isConnected) return false
  const st = window.getComputedStyle(el)
  if (st.display === 'none' || st.visibility === 'hidden') return false
  const r = el.getBoundingClientRect()
  return r.width > 0 && r.height > 0
}

export function resolveComboboxAnchor(
  anchors: Map<string, HTMLElement>,
  rowIdx: number,
): HTMLElement | null {
  for (const variant of ['desktop', 'mobile'] as const) {
    const el = anchors.get(comboboxAnchorKey(rowIdx, variant))
    if (el && isComboboxAnchorVisible(el)) return el
  }
  return (
    anchors.get(comboboxAnchorKey(rowIdx, 'desktop')) ??
    anchors.get(comboboxAnchorKey(rowIdx, 'mobile')) ??
    null
  )
}
