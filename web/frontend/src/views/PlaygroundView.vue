<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import PlaygroundUiVersionToggle from '../components/playground/PlaygroundUiVersionToggle.vue'
import PlaygroundClassic from './playground/PlaygroundClassic.vue'
import PlaygroundModern from './playground/PlaygroundModern.vue'
import { LS_PLAYGROUND_UI_VERSION, type PlaygroundUiVersion } from '../types/playground'

const uiVersion = ref<PlaygroundUiVersion>('classic')

function loadUiVersion() {
  try {
    const v = localStorage.getItem(LS_PLAYGROUND_UI_VERSION)
    if (v === 'modern' || v === 'classic') uiVersion.value = v
  } catch {
    /* ignore */
  }
}

function persistUiVersion(v: PlaygroundUiVersion) {
  uiVersion.value = v
  try {
    localStorage.setItem(LS_PLAYGROUND_UI_VERSION, v)
  } catch {
    /* ignore */
  }
}

onMounted(loadUiVersion)

watch(uiVersion, (v) => {
  try {
    localStorage.setItem(LS_PLAYGROUND_UI_VERSION, v)
  } catch {
    /* ignore */
  }
})
</script>

<template>
  <PlaygroundModern v-if="uiVersion === 'modern'" :ui-version="uiVersion" @update:ui-version="persistUiVersion" />
  <div v-else class="flex min-h-0 flex-1 flex-col">
    <div class="flex shrink-0 justify-end px-2 pt-2 lg:px-0">
      <PlaygroundUiVersionToggle :model-value="uiVersion" @update:model-value="persistUiVersion" />
    </div>
    <PlaygroundClassic class="min-h-0 flex-1" />
  </div>
</template>
