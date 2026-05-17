<script setup lang="ts">
withDefaults(
  defineProps<{
    size?: 'sm' | 'md' | 'lg'
    tone?: 'cyan' | 'amber' | 'muted'
  }>(),
  { size: 'md', tone: 'cyan' },
)

const sizeClass = {
  sm: 'h-4 w-4',
  md: 'h-5 w-5',
  lg: 'h-8 w-8',
} as const

const toneClass = {
  cyan: 'border-slate-600/80 border-t-cyan-400/95',
  amber: 'border-slate-500 border-t-amber-400',
  muted: 'border-slate-700 border-t-slate-400',
} as const
</script>

<template>
  <span
    class="th-loading-spinner inline-block shrink-0 animate-spin rounded-full border-2"
    :class="[sizeClass[size], toneClass[tone]]"
    role="status"
    aria-hidden="true"
  />
</template>
