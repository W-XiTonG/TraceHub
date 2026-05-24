<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import ThLoadingSpinner from '../ui/ThLoadingSpinner.vue'
import { downloadPlaygroundMedia } from '../../utils/playgroundMediaDownload'

const props = withDefaults(
  defineProps<{
    src: string
    kind: 'image' | 'video'
    filenameBase?: string
    variant?: 'modern' | 'classic'
  }>(),
  { variant: 'modern' },
)

const { t } = useI18n()
const downloading = ref(false)
const feedback = ref<'ok' | 'opened' | 'fail' | null>(null)
let feedbackTimer: ReturnType<typeof setTimeout> | undefined

function clearFeedbackSoon() {
  if (feedbackTimer) clearTimeout(feedbackTimer)
  feedbackTimer = setTimeout(() => {
    feedback.value = null
  }, 4000)
}

async function onDownload() {
  if (downloading.value || !props.src.trim()) return
  downloading.value = true
  feedback.value = null
  try {
    const result = await downloadPlaygroundMedia(props.src, props.kind, props.filenameBase)
    feedback.value = result
    if (result !== 'fail') clearFeedbackSoon()
  } finally {
    downloading.value = false
  }
}
</script>

<template>
  <div>
    <button
      type="button"
      class="inline-flex items-center gap-1.5"
      :class="variant === 'modern' ? 'th-pg-v2-link-btn mt-2' : 'th-pg-btn-ghost mt-2 px-2 py-1 text-xs'"
      :disabled="downloading || !src.trim()"
      @click="onDownload"
    >
      <ThLoadingSpinner v-if="downloading" size="sm" />
      <svg
        v-else
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        class="h-3.5 w-3.5 shrink-0"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12M12 16.5V3"
        />
      </svg>
      {{ kind === 'video' ? t('playground.downloadVideo') : t('playground.downloadImage') }}
    </button>
    <p
      v-if="feedback === 'opened'"
      class="mt-1 text-xs leading-relaxed text-amber-200/90"
    >
      {{ t('playground.downloadOpenedHint') }}
    </p>
    <p
      v-else-if="feedback === 'fail'"
      class="mt-1 text-xs leading-relaxed text-red-300/90"
    >
      {{ t('playground.downloadFailed') }}
    </p>
  </div>
</template>
