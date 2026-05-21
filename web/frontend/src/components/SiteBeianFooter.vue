<script setup lang="ts">
import { computed } from 'vue'
import { useSiteMetaStore } from '../stores/siteMeta'

const props = withDefaults(
  defineProps<{
    /** 控制台/管理后台壳层：细边框顶部分隔 */
    variant?: 'default' | 'shell'
  }>(),
  { variant: 'default' },
)

const siteMeta = useSiteMetaStore()

function isHttpUrl(line: string): boolean {
  try {
    const u = new URL(line)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

const shellClass = computed(() =>
  props.variant === 'shell'
    ? 'border-t border-white/[0.06] px-4 py-3 sm:px-5 md:px-10'
    : '',
)
</script>

<template>
  <footer
    v-if="siteMeta.hasBeianFooter"
    class="shrink-0 text-center"
    :class="shellClass"
    aria-label="Beian"
  >
    <p
      v-for="(line, idx) in siteMeta.beianFooterLines"
      :key="idx"
      class="text-[11px] leading-relaxed tracking-wide text-slate-500"
      :class="idx > 0 ? 'mt-1' : ''"
    >
      <a
        v-if="isHttpUrl(line)"
        :href="line"
        target="_blank"
        rel="noopener noreferrer"
        class="text-slate-500 no-underline transition hover:text-slate-400"
      >{{ line }}</a>
      <span v-else>{{ line }}</span>
    </p>
  </footer>
</template>
