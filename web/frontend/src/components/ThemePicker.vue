<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useThemeStore } from '../stores/theme'
import type { ThemeId } from '../theme-sync'

const theme = useThemeStore()
const open = ref(false)
const root = ref<HTMLElement | null>(null)

const swatches: Record<ThemeId, string> = {
  ocean: 'linear-gradient(135deg,#7dd3fc,#0369a1)',
  dawn: 'linear-gradient(135deg,#f8fafc,#bae6fd)',
  paper: 'linear-gradient(135deg,#f1f5f9,#e2e8f0)',
  violet: 'linear-gradient(135deg,#ddd6fe,#5b21b6)',
  ember: 'linear-gradient(135deg,#fecdd3,#9f1239)',
  forest: 'linear-gradient(135deg,#6ee7b7,#065f46)',
  mono: 'linear-gradient(135deg,#e4e4e7,#3f3f46)',
}

function toggle() {
  open.value = !open.value
}

function pick(id: ThemeId) {
  theme.setTheme(id)
  open.value = false
}

function onDocPointerDown(e: PointerEvent) {
  if (!open.value || !root.value) return
  if (!root.value.contains(e.target as Node)) open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') open.value = false
}

onMounted(() => {
  document.addEventListener('pointerdown', onDocPointerDown, true)
  document.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  document.removeEventListener('pointerdown', onDocPointerDown, true)
  document.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <div ref="root" class="th-theme-picker pointer-events-none fixed bottom-5 right-5 z-[100] flex flex-col items-end gap-2 md:bottom-6 md:right-6">
    <Transition
      enter-active-class="transition duration-220 ease-[cubic-bezier(0.22,1,0.36,1)]"
      leave-active-class="transition duration-160 ease-in"
      enter-from-class="translate-y-2 opacity-0 scale-[0.98]"
      leave-to-class="translate-y-1 opacity-0 scale-[0.99]"
    >
      <div
        v-show="open"
        class="pointer-events-auto w-[min(18rem,calc(100vw-2.5rem))] rounded-xl border border-white/[0.1] bg-slate-950/95 p-2 shadow-2xl shadow-black/50 backdrop-blur-xl"
        role="listbox"
        aria-label="选择界面主题"
      >
        <p class="px-2.5 pb-1.5 pt-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-slate-500">主题</p>
        <ul class="space-y-0.5">
          <li v-for="t in theme.themes" :key="t.id">
            <button
              type="button"
              class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left text-sm text-slate-200 transition hover:bg-white/[0.06]"
              :class="{ 'bg-white/[0.07]': theme.themeId === t.id }"
              role="option"
              :aria-selected="theme.themeId === t.id"
              @click="pick(t.id)"
            >
              <span
                class="h-8 w-8 shrink-0 rounded-lg border border-white/10 shadow-inner"
                :style="{ background: swatches[t.id] }"
                aria-hidden="true"
              />
              <span class="min-w-0 flex-1 font-medium">{{ t.label }}</span>
              <span v-if="theme.themeId === t.id" class="shrink-0 text-xs text-slate-500">当前</span>
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
      aria-label="打开主题选择"
      @click.stop="toggle"
    >
      <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"
        />
      </svg>
    </button>
  </div>
</template>
