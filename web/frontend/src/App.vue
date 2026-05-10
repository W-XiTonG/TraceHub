<script setup lang="ts">
import { useRoute } from 'vue-router'
import ThemePicker from './components/ThemePicker.vue'

const route = useRoute()
</script>

<template>
  <RouterView v-slot="{ Component }">
    <Transition name="th-page" mode="out-in">
      <component :is="Component" :key="route.path" />
    </Transition>
  </RouterView>
  <ThemePicker />
</template>
