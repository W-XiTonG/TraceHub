<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

defineProps<{
  /** 窄屏下压缩为更少条目与更小字号 */
  compact?: boolean
}>()

const { t } = useI18n()

const items = computed(() => [
  { t: t('authAside.i0t'), d: t('authAside.i0d') },
  { t: t('authAside.i1t'), d: t('authAside.i1d') },
  { t: t('authAside.i2t'), d: t('authAside.i2d') },
])
</script>

<template>
  <aside
    class="text-slate-400"
    :class="compact ? 'max-w-md' : 'max-w-md lg:max-w-sm xl:max-w-md'"
    :aria-label="t('authAside.aria')"
  >
    <RouterLink
      to="/"
      class="group mb-6 inline-flex items-center gap-2 rounded-lg text-sm font-medium text-slate-500 transition hover:text-cyan-400/90"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4 -translate-x-0.5 transition group-hover:-translate-x-1" aria-hidden="true">
        <path fill-rule="evenodd" d="M17 10a.75.75 0 01-.75.75H5.612l4.158 3.96a.75.75 0 11-1.04 1.08l-5.5-5.25a.75.75 0 010-1.08l5.5-5.25a.75.75 0 111.04 1.08L5.612 9.25H16.25A.75.75 0 0117 10z" clip-rule="evenodd" />
      </svg>
      {{ t('authAside.back') }}
    </RouterLink>
    <p class="th-kicker mb-2"><span class="th-kicker-accent">{{ t('authAside.kicker') }}</span></p>
    <h2 class="text-xl font-semibold tracking-tight text-slate-100" :class="compact ? 'text-lg' : 'md:text-2xl'">{{ t('authAside.headline') }}</h2>
    <p class="mt-2 text-sm leading-relaxed text-slate-500" :class="compact && 'text-xs'">
      {{ t('authAside.desc') }}
    </p>
    <ul class="mt-8 space-y-4" :class="compact && 'mt-5 space-y-3'">
      <li
        v-for="(it, i) in items"
        :key="it.t"
        v-show="!compact || i < 2"
        class="flex gap-3"
      >
        <span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-cyan-500/75" aria-hidden="true" />
        <div>
          <p class="text-sm font-medium text-slate-300">{{ it.t }}</p>
          <p class="mt-0.5 text-xs leading-relaxed text-slate-600">{{ it.d }}</p>
        </div>
      </li>
    </ul>
    <RouterLink
      to="/#features"
      class="mt-8 inline-flex items-center gap-1 text-sm font-medium text-cyan-400/90 transition hover:text-cyan-300"
      :class="compact && 'mt-5 text-xs'"
    >
      {{ t('authAside.cta') }}
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-4 w-4" aria-hidden="true">
        <path fill-rule="evenodd" d="M3 10a.75.75 0 01.75-.75h10.638L10.23 5.29a.75.75 0 111.04-1.08l5.5 5.25a.75.75 0 010 1.08l-5.5 5.25a.75.75 0 11-1.04-1.08l4.158-3.96H3.75A.75.75 0 013 10z" clip-rule="evenodd" />
      </svg>
    </RouterLink>
    <p class="mt-6 text-[11px] leading-relaxed text-slate-600" :class="compact && 'mt-4'">{{ t('authAside.company') }}</p>
  </aside>
</template>
