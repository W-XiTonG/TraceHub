<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { dismissPromoForScope, fingerprintForPromo, isPromoDismissedForScope } from '../utils/registerPromoDismiss'

const props = withDefaults(
  defineProps<{
    text: string
    /** header：顶栏内联；card：注册卡片右上角悬浮 */
    variant?: 'header' | 'card'
  }>(),
  { variant: 'header' },
)

const { t } = useI18n()
const rootRef = ref<HTMLElement | null>(null)
const open = ref(false)
const revision = ref(0)

const fp = computed(() => fingerprintForPromo(props.text))

const visible = computed(() => {
  void revision.value
  return Boolean(fp.value)
})

const showUnreadDot = computed(() => {
  void revision.value
  const f = fp.value
  if (!f) return false
  return !isPromoDismissedForScope(f)
})

watch(
  () => props.text,
  () => {
    revision.value++
    if (!visible.value) open.value = false
  },
)

function toggle() {
  if (!visible.value) return
  open.value = !open.value
}

function onGotIt() {
  const f = fp.value
  if (f) dismissPromoForScope(f)
  revision.value++
  open.value = false
}

function onDocMouseDown(ev: MouseEvent) {
  if (!open.value) return
  const el = rootRef.value
  if (el && !el.contains(ev.target as Node)) open.value = false
}

function onStorage(ev: StorageEvent) {
  if (ev.key?.startsWith('tracehub_register_promo_dismiss:v2:')) revision.value++
}

onMounted(() => {
  document.addEventListener('mousedown', onDocMouseDown)
  window.addEventListener('storage', onStorage)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMouseDown)
  window.removeEventListener('storage', onStorage)
})
</script>

<template>
  <div
    v-if="visible"
    ref="rootRef"
    class="shrink-0"
    :class="variant === 'card' ? 'absolute right-4 top-4 z-20' : 'relative self-center'"
  >
    <button
      type="button"
      class="th-site-promo-bell relative flex h-9 w-9 items-center justify-center rounded-lg border border-white/[0.1] bg-white/[0.04] text-slate-300 transition hover:border-cyan-500/25 hover:bg-cyan-500/10 hover:text-cyan-100"
      :class="variant === 'header' ? 'th-site-promo-bell--header' : 'th-site-promo-bell--card'"
      :aria-label="t('sitePromo.aria')"
      :aria-expanded="open"
      :title="t('sitePromo.aria')"
      @click="toggle"
    >
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" class="h-5 w-5" aria-hidden="true">
        <path
          d="M10 2a6 6 0 00-6 6v3.586l-.707.707A1 1 0 004 14h12a1 1 0 00.707-1.707L16 11.586V8a6 6 0 00-6-6zM10 18a3 3 0 01-3-3h6a3 3 0 01-3 3z"
        />
      </svg>
      <span
        v-if="showUnreadDot"
        class="th-site-promo-bell__dot absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-cyan-400 shadow-[0_0_0_2px_rgba(15,23,42,0.85)]"
        :class="variant === 'header' ? 'th-site-promo-bell__dot--header' : 'th-site-promo-bell__dot--card'"
        aria-hidden="true"
      />
    </button>

    <div
      v-if="open"
      class="th-site-promo-panel absolute right-0 z-50 mt-2 w-[min(100vw-2rem,22rem)] origin-top-right rounded-xl border border-white/[0.1] bg-slate-950/95 p-4 shadow-xl backdrop-blur-md"
      :class="variant === 'header' ? 'th-site-promo-panel--header' : 'th-site-promo-panel--card'"
      role="dialog"
      :aria-label="t('sitePromo.panelTitle')"
      @click.stop
    >
      <div class="mb-3 flex items-start justify-between gap-2">
        <p class="th-site-promo-panel__title text-sm font-semibold text-slate-100">{{ t('sitePromo.panelTitle') }}</p>
        <button
          type="button"
          class="th-site-promo-panel__close rounded px-1.5 py-0.5 text-xs text-slate-500 hover:bg-white/10 hover:text-slate-300"
          @click="open = false"
        >
          {{ t('common.close') }}
        </button>
      </div>
      <p class="th-site-promo-panel__body max-h-48 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed text-slate-300">
        {{ text.trim() }}
      </p>
      <div class="mt-4 flex justify-end">
        <button
          type="button"
          class="th-site-promo-panel__cta rounded-lg border border-cyan-500/30 bg-cyan-500/15 px-4 py-2 text-xs font-medium text-cyan-100 transition hover:border-cyan-400/40 hover:bg-cyan-500/25"
          @click="onGotIt"
        >
          {{ t('sitePromo.gotIt') }}
        </button>
      </div>
    </div>
  </div>
</template>
