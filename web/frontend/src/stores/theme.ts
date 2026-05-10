import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  applyThemeToDocument,
  readStoredThemeId,
  THEMES,
  THEME_STORAGE_KEY,
  type ThemeId,
} from '../theme-sync'

export const useThemeStore = defineStore('theme', () => {
  const themeId = ref<ThemeId>(readStoredThemeId())

  function setTheme(id: ThemeId) {
    themeId.value = id
    try {
      localStorage.setItem(THEME_STORAGE_KEY, id)
    } catch {
      /* ignore */
    }
    applyThemeToDocument(id)
  }

  function init() {
    themeId.value = readStoredThemeId()
    applyThemeToDocument(themeId.value)
  }

  return {
    themeId,
    themes: THEMES,
    setTheme,
    init,
  }
})
