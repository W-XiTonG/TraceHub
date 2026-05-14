<script setup lang="ts">
import { ref } from 'vue'
import { openDateTimePickerField } from '../utils/datetimeLocal'

const model = defineModel<string>({ default: '' })

defineProps<{
  /** 与外层 `<label for>` 一致，便于点文字也能聚焦 */
  inputId: string
  /** 无障碍：读屏器朗读控件用途 */
  ariaLabel: string
}>()

const inputRef = ref<HTMLInputElement | null>(null)

function openPicker() {
  openDateTimePickerField(inputRef.value)
}

function onDateTimeChange() {
  window.setTimeout(() => {
    try {
      inputRef.value?.blur()
    } catch {
      /* ignore */
    }
  }, 0)
}
</script>

<template>
  <div class="relative h-10 w-full max-w-[11.5rem] overflow-hidden rounded-[0.5rem]">
    <input
      :id="inputId"
      ref="inputRef"
      v-model="model"
      type="datetime-local"
      step="60"
      tabindex="0"
      class="th-input th-datetime-local th-datetime-local--filter h-10 w-full max-w-full pr-2 text-sm leading-tight"
      :aria-label="ariaLabel"
      @keydown.enter.prevent="openPicker"
      @keydown.space.prevent="openPicker"
      @change="onDateTimeChange"
    />
  </div>
</template>
