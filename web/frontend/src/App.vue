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

/** 与「壳」 layout 绑定，避免 /console/*、/admin/* 子路由切换时整页 remount（只应由内层 RouterView 换右侧内容） */
const shellKey = computed(() => {
  const p = route.path
  if (p.startsWith('/console')) return 'shell:console'
  if (p.startsWith('/admin')) return 'shell:admin'
  return p
})
</script>

<template>
  <RouterView v-slot="{ Component }">
    <Transition name="th-page" mode="out-in">
      <component :is="Component" :key="shellKey" />
    </Transition>
  </RouterView>
  <ThemePicker />
  <LanguageSwitcher />
</template>
