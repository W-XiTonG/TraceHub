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

onMounted(() => {
  void siteMeta.ensureLoaded()
})

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
