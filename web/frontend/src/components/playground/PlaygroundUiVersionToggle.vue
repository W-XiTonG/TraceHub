<script setup lang="ts">
import type { PlaygroundUiVersion } from '../../types/playground'

defineProps<{
  modelValue: PlaygroundUiVersion
}>()

const emit = defineEmits<{
  'update:modelValue': [value: PlaygroundUiVersion]
}>()
</script>

<template>
  <div class="th-pg-v2-version th-pg-seg" role="group" :aria-label="$t('playground.uiVersionToggle')">
    <button
      type="button"
      class="th-pg-seg__btn text-[11px]"
      :class="{ 'th-pg-seg__btn--active': modelValue === 'classic' }"
      :aria-pressed="modelValue === 'classic'"
      @click="emit('update:modelValue', 'classic')"
    >
      {{ $t('playground.uiVersionClassic') }}
    </button>
    <button
      type="button"
      class="th-pg-seg__btn text-[11px]"
      :class="{ 'th-pg-seg__btn--active': modelValue === 'modern' }"
      :aria-pressed="modelValue === 'modern'"
      @click="emit('update:modelValue', 'modern')"
    >
      {{ $t('playground.uiVersionModern') }}
    </button>
  </div>
</template>
