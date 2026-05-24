<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import ThLoadingSpinner from '../ui/ThLoadingSpinner.vue'
import { isResolvableHttpImage, resolveImageUrlForDisplay } from '../../utils/playgroundMediaResolve'

const props = defineProps<{
  src: string
  alt?: string
  failed?: boolean
  showRetryB64?: boolean
}>()

const emit = defineEmits<{
  error: []
  retryB64: []
}>()

const displaySrc = ref('')
const loading = ref(true)
const resolvedFailed = ref(false)
let objectUrl: string | null = null

function revokeObject() {
  if (objectUrl) {
    URL.revokeObjectURL(objectUrl)
    objectUrl = null
  }
}

async function loadSrc(src: string) {
  loading.value = true
  resolvedFailed.value = false
  revokeObject()
  displaySrc.value = ''
  const t = src.trim()
  if (!t) {
    loading.value = false
    resolvedFailed.value = true
    return
  }
  if (isResolvableHttpImage(t)) {
    const blobUrl = await resolveImageUrlForDisplay(t)
    if (blobUrl && blobUrl.startsWith('blob:')) {
      objectUrl = blobUrl
      displaySrc.value = blobUrl
      loading.value = false
      return
    }
  }
  displaySrc.value = t
  loading.value = false
}

function onImgError() {
  resolvedFailed.value = true
  emit('error')
}

watch(
  () => props.src,
  (s) => {
    void loadSrc(s)
  },
  { immediate: true },
)

onBeforeUnmount(revokeObject)
</script>

<template>
  <div class="th-pg-v2-media-img relative">
    <div
      v-if="loading"
      class="flex min-h-[8rem] items-center justify-center rounded-lg border border-white/10 bg-slate-950/50"
    >
      <ThLoadingSpinner size="sm" />
      <span class="ml-2 text-xs th-pg-muted">{{ $t('playground.mediaLoading') }}</span>
    </div>
    <template v-else-if="failed || resolvedFailed">
      <p class="rounded-lg border border-amber-500/25 bg-amber-950/30 px-2.5 py-2 text-xs leading-relaxed text-amber-100/90">
        {{ $t('playground.imageLoadFailed') }}
      </p>
      <button
        v-if="showRetryB64"
        type="button"
        class="th-pg-btn-ghost mt-2 px-2 py-1 text-xs"
        @click="emit('retryB64')"
      >
        {{ $t('playground.retryB64') }}
      </button>
    </template>
    <img
      v-else
      :src="displaySrc"
      :alt="alt || ''"
      loading="lazy"
      class="max-h-72 max-w-full rounded-lg border border-white/10 object-contain"
      referrerpolicy="no-referrer"
      @error="onImgError"
    />
  </div>
</template>
