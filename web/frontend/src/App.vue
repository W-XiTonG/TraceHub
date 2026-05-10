<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import ThemePicker from './components/ThemePicker.vue'

const route = useRoute()

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
</template>
