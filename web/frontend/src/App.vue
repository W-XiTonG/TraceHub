<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import ThemePicker from './components/ThemePicker.vue'
import LanguageSwitcher from './components/LanguageSwitcher.vue'
import { useSiteMetaStore } from './stores/siteMeta'

const route = useRoute()
const { t, locale } = useI18n()
const siteMeta = useSiteMetaStore()

/** 与标题后缀「迹径」同义时避免重复，改用 common.documentTitleTaglineAlt */
const titleBrandDupesProductMark = (brand: string) => {
  const b = brand.trim()
  return b === '迹径' || b === '跡徑' || b === '跡径'
}

/** 站点名与产品代号 TraceHub 相同时，不再写「{brand} · TraceHub」，避免出现两个 TraceHub */
const titleBrandIsTraceHubCode = (brand: string) => brand.trim().toLowerCase() === 'tracehub'

function imageMimeFromUrl(url: string): string {
  const u = url.split('?')[0]?.toLowerCase() ?? ''
  if (u.endsWith('.svg')) return 'image/svg+xml'
  if (u.endsWith('.png')) return 'image/png'
  if (u.endsWith('.jpg') || u.endsWith('.jpeg')) return 'image/jpeg'
  if (u.endsWith('.gif')) return 'image/gif'
  if (u.endsWith('.webp')) return 'image/webp'
  return 'image/png'
}

function upsertHeadLink(rel: string, href: string, type?: string) {
  if (typeof document === 'undefined') return
  let el = document.querySelector<HTMLLinkElement>(`link[rel="${rel}"]`)
  if (!el) {
    el = document.createElement('link')
    el.rel = rel
    document.head.appendChild(el)
  }
  const current = el.getAttribute('href') ?? ''
  if (current === href && (!type || el.type === type)) return
  el.setAttribute('href', href)
  if (type) el.type = type
  else el.removeAttribute('type')
}

function upsertMetaProperty(property: string, content: string) {
  if (typeof document === 'undefined') return
  let el = document.querySelector<HTMLMetaElement>(`meta[property="${property}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute('property', property)
    document.head.appendChild(el)
  }
  if (el.content === content) return
  el.content = content
}

function removeMetaProperty(property: string) {
  if (typeof document === 'undefined') return
  document.querySelector<HTMLMetaElement>(`meta[property="${property}"]`)?.remove()
}

onMounted(() => {
  void siteMeta.ensureLoaded()
})

watch(
  () => siteMeta.displayFaviconUrl,
  (href) => {
    const type = imageMimeFromUrl(href)
    upsertHeadLink('icon', href, type)
    upsertHeadLink('apple-touch-icon', href)
  },
  { immediate: true },
)

watch(
  () => siteMeta.displayOgImageUrl,
  (og) => {
    if (og) {
      upsertMetaProperty('og:image', og)
      upsertMetaProperty('twitter:image', og)
    } else {
      removeMetaProperty('og:image')
      removeMetaProperty('twitter:image')
    }
  },
  { immediate: true },
)

watch(
  [
    locale,
    () => t('common.documentTitleTpl'),
    () => t('common.documentTitleTplTraceHubSingle'),
    () => t('common.documentTitleTagline'),
    () => t('common.documentTitleTaglineAlt'),
    () => siteMeta.displayName,
  ],
  () => {
    const brand = siteMeta.displayName
    const tagline = titleBrandDupesProductMark(brand)
      ? t('common.documentTitleTaglineAlt')
      : t('common.documentTitleTagline')
    if (titleBrandIsTraceHubCode(brand)) {
      document.title = t('common.documentTitleTplTraceHubSingle', { tagline })
    } else {
      document.title = t('common.documentTitleTpl', { brand, tagline })
    }
  },
  { immediate: true },
)

/** 与「壳」 layout 绑定：/console/*、/admin/* 使用稳定 key，避免子路由切换时外层误 remount（内层由各 Layout 的 RouterView 切换）。 */
const shellKey = computed(() => {
  const p = route.path
  if (p.startsWith('/console')) return 'shell:console'
  if (p.startsWith('/admin')) return 'shell:admin'
  return p
})
</script>

<template>
  <!-- 根级不用 Transition：避免 appear/duration 与路由异步组件在少数环境下触发控制台错误并导致整页不渲染 -->
  <RouterView v-slot="{ Component }">
    <component :is="Component" :key="shellKey" />
  </RouterView>
  <ThemePicker />
  <LanguageSwitcher />
</template>
