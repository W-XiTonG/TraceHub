<script setup lang="ts">
import type { PlaygroundPlayMode } from '../../types/playground'

defineProps<{
  modelValue: PlaygroundPlayMode
}>()

const emit = defineEmits<{
  'update:modelValue': [value: PlaygroundPlayMode]
}>()

const modes: { id: PlaygroundPlayMode; icon: string }[] = [
  { id: 'chat', icon: 'M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z' },
  { id: 'image', icon: 'M2.25 15.75l5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909M12 6.75h.008v.008H12V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z' },
  { id: 'video', icon: 'm15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z' },
  { id: 'audio', icon: 'M19.114 5.636a9 9 0 0 1 0 12.728M16.463 8.288a5.25 5.25 0 0 1 0 7.424M6.75 8.25l4.72-4.72a.75.75 0 0 1 1.28.53v15.88a.75.75 0 0 1-1.28.53l-4.72-4.72A4.5 4.5 0 0 0 2.25 12c0-1.77.715-3.37 1.87-4.52L6.75 8.25Z' },
]

function label(id: PlaygroundPlayMode): string {
  if (id === 'chat') return 'playground.modeChat'
  if (id === 'image') return 'playground.modeImage'
  if (id === 'video') return 'playground.modeVideo'
  return 'playground.modeAudio'
}
</script>

<template>
  <div class="th-pg-v2-tabs" role="tablist" :aria-label="$t('playground.playMode')">
    <button
      v-for="m in modes"
      :key="m.id"
      type="button"
      role="tab"
      class="th-pg-v2-tabs__btn"
      :class="{ 'th-pg-v2-tabs__btn--active': modelValue === m.id }"
      :aria-selected="modelValue === m.id"
      :title="$t(label(m.id))"
      @click="emit('update:modelValue', m.id)"
    >
      <svg class="th-pg-v2-tabs__icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
        <path stroke-linecap="round" stroke-linejoin="round" :d="m.icon" />
      </svg>
      <span class="th-pg-v2-tabs__label">{{ $t(label(m.id)) }}</span>
    </button>
  </div>
</template>
