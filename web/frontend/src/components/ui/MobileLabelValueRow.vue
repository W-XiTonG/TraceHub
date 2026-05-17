<script setup lang="ts">
defineProps<{
  label: string
}>()
</script>

<template>
  <div class="flex flex-col gap-0.5 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
    <dt class="shrink-0 text-xs font-medium uppercase tracking-wide text-slate-500">{{ label }}</dt>
    <dd class="min-w-0 text-slate-300">
      <slot />
    </dd>
  </div>
</template>
