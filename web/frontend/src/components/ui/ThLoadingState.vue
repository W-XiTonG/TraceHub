<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ThLoadingSpinner from './ThLoadingSpinner.vue'

const props = withDefaults(
  defineProps<{
    label?: string
    /** panel：表格/卡片区域居中；inline：行内；overlay：半透明遮罩 */
    variant?: 'panel' | 'inline' | 'overlay'
    size?: 'sm' | 'md' | 'lg'
  }>(),
  { variant: 'panel', size: 'md' },
)

const { t } = useI18n()
const text = () => props.label ?? t('common.loading')
</script>

<template>
  <div
    v-if="variant === 'overlay'"
    class="th-loading-overlay absolute inset-0 z-20 flex items-center justify-center rounded-[inherit] bg-slate-950/55 backdrop-blur-[2px]"
    role="status"
    :aria-label="text()"
  >
    <div class="flex flex-col items-center gap-3 px-4 py-6">
      <ThLoadingSpinner :size="size" />
      <p class="text-sm text-slate-300">{{ text() }}</p>
    </div>
  </div>
  <p
    v-else-if="variant === 'inline'"
    class="inline-flex items-center gap-2 text-sm text-slate-400"
    role="status"
    :aria-label="text()"
  >
    <ThLoadingSpinner :size="size" tone="muted" />
    <span>{{ text() }}</span>
  </p>
  <div
    v-else
    class="th-loading-panel flex flex-col items-center justify-center gap-3 px-6 py-14 text-center"
    role="status"
    :aria-label="text()"
  >
    <ThLoadingSpinner :size="size === 'sm' ? 'md' : 'lg'" />
    <p class="text-sm text-slate-400">{{ text() }}</p>
    <slot />
  </div>
</template>
