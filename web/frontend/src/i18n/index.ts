import { createI18n } from 'vue-i18n'
import zhCN from '../locales/zh-CN'
import zhTW from '../locales/zh-TW'
import en from '../locales/en'

export const LOCALE_STORAGE_KEY = 'tracehub-locale'
export const SUPPORTED_LOCALES = ['zh-CN', 'zh-TW', 'en'] as const
export type AppLocale = (typeof SUPPORTED_LOCALES)[number]

function readStoredLocale(): AppLocale {
  try {
    const raw = localStorage.getItem(LOCALE_STORAGE_KEY)
    if (raw && SUPPORTED_LOCALES.includes(raw as AppLocale)) return raw as AppLocale
  } catch {
    /* ignore */
  }
  if (typeof navigator !== 'undefined') {
    const lang = (navigator.language || '').toLowerCase()
    if (lang === 'zh-tw' || lang === 'zh-hk' || lang === 'zh-mo') return 'zh-TW'
    if (lang.startsWith('zh')) return 'zh-CN'
    if (lang.startsWith('en')) return 'en'
  }
  return 'zh-CN'
}

function applyDocumentLocale(locale: AppLocale) {
  const htmlLang = locale === 'zh-TW' ? 'zh-Hant' : locale === 'en' ? 'en' : 'zh-CN'
  document.documentElement.lang = htmlLang
}

const initial = readStoredLocale()
applyDocumentLocale(initial)

export const i18n = createI18n({
  legacy: false,
  locale: initial,
  fallbackLocale: 'zh-CN',
  globalInjection: true,
  messages: {
    'zh-CN': zhCN,
    'zh-TW': zhTW,
    en,
  },
})

export function persistLocale(locale: AppLocale) {
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  } catch {
    /* ignore */
  }
  applyDocumentLocale(locale)
}
