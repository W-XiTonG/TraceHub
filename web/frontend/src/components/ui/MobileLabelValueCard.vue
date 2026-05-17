<script setup lang="ts">
defineProps<{
  title?: string
}>()
</script>

<template>
  <article class="th-glass rounded-[var(--th-radius-lg)] p-4">
    <header v-if="title || $slots.title" class="mb-3 border-b border-white/[0.06] pb-2">
      <slot name="title">
        <p v-if="title" class="font-medium text-slate-200">{{ title }}</p>
      </slot>
    </header>
    <dl class="space-y-2.5 text-sm">
      <slot />
    </dl>
  </article>
</template>
