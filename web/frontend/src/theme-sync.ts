export const THEME_STORAGE_KEY = 'tracehub-ui-theme'

export const THEMES = [
  { id: 'ocean', label: '极光青' },
  { id: 'dawn', label: '晨光' },
  { id: 'paper', label: '素纸' },
  { id: 'violet', label: '雾紫' },
  { id: 'ember', label: '暖烬' },
  { id: 'forest', label: '林雾' },
  { id: 'mono', label: '石墨' },
] as const

export type ThemeId = (typeof THEMES)[number]['id']

const ALLOWED = new Set<string>(THEMES.map((t) => t.id))

export function readStoredThemeId(): ThemeId {
  try {
    const raw = localStorage.getItem(THEME_STORAGE_KEY)
    if (raw && ALLOWED.has(raw)) return raw as ThemeId
  } catch {
    /* ignore */
  }
  return 'ocean'
}

/** 与 CSS 约定：ocean 不写 data-th-theme，其余写 data-th-theme="<id>" */
export function applyThemeToDocument(id: ThemeId) {
  if (id === 'ocean') document.documentElement.removeAttribute('data-th-theme')
  else document.documentElement.setAttribute('data-th-theme', id)
}
