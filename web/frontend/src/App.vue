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

onMounted(() => {
  void siteMeta.ensureLoaded()
})

watch(
  [locale, () => t('common.documentTitleTpl'), () => siteMeta.displayName],
  () => {
    document.title = t('common.documentTitleTpl', { brand: siteMeta.displayName })
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
