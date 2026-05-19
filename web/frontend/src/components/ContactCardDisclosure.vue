<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import contactCardImg from '../assets/xunji-contact-card.png'
import { useSiteMetaStore } from '../stores/siteMeta'

const siteMeta = useSiteMetaStore()

const props = withDefaults(
  defineProps<{
    /** i18n 键：各场景摘要文案，如 home.contactHintBilling */
    summaryKey?: string
    /** 控制台等左对齐页面用 start */
    align?: 'center' | 'start'
    /** 卡片略小，适合侧栏或窄栏 */
    compact?: boolean
  }>(),
  { summaryKey: 'home.contactHintGeneric', align: 'center', compact: false },
)

const { t } = useI18n()

const rootClass = computed(() => (props.align === 'start' ? 'text-left' : 'text-center'))

const rowClass = computed(() =>
  props.align === 'start' ? 'mt-4 flex justify-start' : 'mt-4 flex justify-center',
)

const imgClass = computed(() => {
  if (props.compact) {
    return 'h-auto w-full max-w-[200px] rounded-lg shadow-md ring-1 ring-white/[0.06]'
  }
  return 'h-auto w-full max-w-[240px] rounded-xl shadow-[0_16px_40px_-24px_rgba(0,0,0,0.65)] ring-1 ring-white/[0.06]'
})
</script>

<template>
  <details class="contact-card-disclosure" :class="rootClass">
    <summary
      class="list-none cursor-pointer select-none text-[11px] text-slate-600 underline decoration-slate-600/30 underline-offset-2 transition hover:text-slate-400 hover:decoration-slate-400/40 [&::-webkit-details-marker]:hidden"
    >
      {{ t(summaryKey) }}
    </summary>
    <div :class="rowClass">
      <img
        :src="siteMeta.displayContactCardUrl ?? contactCardImg"
        :alt="t('home.contactCardAlt')"
        :width="compact ? 200 : 240"
        loading="lazy"
        decoding="async"
        :class="imgClass"
      />
    </div>
    <p class="mt-3 text-[11px] leading-relaxed text-slate-600">
      {{ t('home.contactScanHint') }}
    </p>
  </details>
</template>
