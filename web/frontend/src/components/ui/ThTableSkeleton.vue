<script setup lang="ts">
withDefaults(
  defineProps<{
    rows?: number
    columns?: number
    toolbar?: boolean
  }>(),
  { rows: 8, columns: 6, toolbar: false },
)
</script>

<template>
  <div class="th-table-skeleton w-full" aria-hidden="true">
    <div v-if="toolbar" class="mb-4 flex flex-wrap gap-3">
      <div class="th-skeleton-bar h-10 min-w-[12rem] flex-1 max-w-md rounded-lg" />
      <div class="th-skeleton-bar h-10 w-24 rounded-lg" />
      <div class="th-skeleton-bar h-10 w-28 rounded-lg" />
    </div>
    <div class="th-glass overflow-hidden rounded-xl border border-white/[0.06]">
        <div
        class="hidden border-b border-white/[0.06] px-4 py-3 lg:grid"
        :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }"
      >
        <div v-for="c in columns" :key="`h-${c}`" class="th-skeleton-bar h-3 w-16 rounded" />
      </div>
      <div
        v-for="r in rows"
        :key="r"
        class="grid gap-4 border-b border-white/[0.04] px-4 py-4 last:border-0"
        :style="{ gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }"
      >
        <div v-for="c in columns" :key="`${r}-${c}`" class="min-w-0 space-y-2">
          <div class="th-skeleton-bar h-4 rounded" :class="c === 1 ? 'w-[85%]' : 'w-full max-w-[7rem]'" />
          <div v-if="c === 1" class="th-skeleton-bar h-3 w-2/3 max-w-[10rem] rounded" />
        </div>
      </div>
    </div>
  </div>
</template>
