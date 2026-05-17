<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { globalFabHidden } from '../composables/shellNavState'
import { persistLocale, type AppLocale, SUPPORTED_LOCALES } from '../i18n'

const { locale, t } = useI18n()
const route = useRoute()
const inShell = computed(() => route.path.startsWith('/console') || route.path.startsWith('/admin'))
const open = ref(false)
const root = ref<HTMLElement | null>(null)

const currentLabel = computed(() => {
  const loc = locale.value as AppLocale
  if (loc === 'zh-TW') return '繁體中文'
  if (loc === 'en') return 'English'
  return '简体中文'
})

function setLocale(loc: AppLocale) {
  locale.value = loc
  persistLocale(loc)
  open.value = false
}

function toggle() {
  open.value = !open.value
}

function onDocPointerDown(e: PointerEvent) {
  if (!open.value || !root.value) return
  if (!root.value.contains(e.target as Node)) open.value = false
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocPointerDown, true)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointerDown, true)
})
</script>

<template>
  <div
    ref="root"
    class="th-lang-picker pointer-events-none fixed bottom-5 right-[4.75rem] z-[100] flex flex-col items-end gap-2 transition-opacity duration-200 max-lg:bottom-[calc(1.25rem+env(safe-area-inset-bottom,0px))] md:bottom-6 md:right-[5.75rem]"
    :class="{
      'opacity-0': globalFabHidden,
      'max-lg:scale-90': inShell,
    }"
  >
    <Transition
      enter-active-class="transition duration-220 ease-[cubic-bezier(0.22,1,0.36,1)]"
      leave-active-class="transition duration-160 ease-in"
      enter-from-class="translate-y-2 opacity-0 scale-[0.98]"
      leave-to-class="translate-y-1 opacity-0 scale-[0.99]"
    >
      <div
        v-show="open"
        class="pointer-events-auto w-[min(12rem,calc(100vw-6rem))] rounded-xl border border-white/[0.1] bg-slate-950/95 p-2 shadow-2xl shadow-black/50 backdrop-blur-xl"
        role="listbox"
        :aria-label="t('lang.switch')"
      >
        <p class="px-2.5 pb-1.5 pt-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-500">{{ t('lang.title') }}</p>
        <ul class="space-y-0.5">
          <li v-for="loc in SUPPORTED_LOCALES" :key="loc">
            <button
              type="button"
              class="flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm text-slate-200 transition hover:bg-white/[0.06]"
              :class="{ 'bg-white/[0.07]': locale === loc }"
              role="option"
              :aria-selected="locale === loc"
              @click="setLocale(loc)"
            >
              <span v-if="loc === 'zh-CN'">简体中文</span>
              <span v-else-if="loc === 'zh-TW'">繁體中文</span>
              <span v-else>English</span>
              <span v-if="locale === loc" class="text-xs text-cyan-400/90">✓</span>
            </button>
          </li>
        </ul>
      </div>
    </Transition>

    <button
      type="button"
      class="pointer-events-auto flex h-11 w-11 items-center justify-center rounded-full border border-white/[0.1] bg-slate-950/90 text-slate-200 shadow-lg shadow-black/40 backdrop-blur-md transition hover:border-white/[0.16] hover:bg-slate-900/95 hover:text-white"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :aria-label="t('lang.switch')"
      :title="currentLabel"
      @click.stop="toggle"
    >
      <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M12 21a9 9 0 100-18 9 9 0 000 18zM3.6 9h16.8M3.6 15h16.8M12 3a15.3 15.3 0 014 9 15.3 15.3 0 01-4 9 15.3 15.3 0 01-4-9 15.3 15.3 0 014-9z"
        />
      </svg>
    </button>
  </div>
</template>
