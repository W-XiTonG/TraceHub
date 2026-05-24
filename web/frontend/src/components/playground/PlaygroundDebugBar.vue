<script setup lang="ts">
import { computed } from 'vue'
import { usePlaygroundDebugResize } from '../../composables/usePlaygroundDebugResize'

const props = defineProps<{
  lastRequest: string
  lastResponse: string
  streamLog: string
}>()

const emit = defineEmits<{
  copy: [text: string]
  collapse: []
}>()

const responseDisplay = computed(
  () => props.lastResponse || props.streamLog.slice(0, 4000) || '—',
)

function onCopy(text: string) {
  const v = text.trim()
  if (!v || v === '—') return
  emit('copy', v)
}

const { resizing, debugBodyStyle, onResizePointerDown } = usePlaygroundDebugResize()
</script>

<template>
  <div
    class="th-pg-debug th-pg-v2-debug shrink-0"
    :class="{ 'th-pg-v2-debug--resizing': resizing }"
    :style="debugBodyStyle"
  >
    <div
      class="th-pg-v2-debug__resize"
      role="separator"
      aria-orientation="horizontal"
      :aria-label="$t('playground.debugResizeHint')"
      :title="$t('playground.debugResizeHint')"
      @pointerdown="onResizePointerDown"
    >
      <span class="th-pg-v2-debug__resize-grip" aria-hidden="true" />
    </div>
    <div class="th-pg-v2-debug__toolbar">
      <span class="th-pg-v2-debug__toolbar-title">{{ $t('playground.debugPanelTitle') }}</span>
      <button type="button" class="th-pg-v2-debug__toolbar-btn" @click="emit('collapse')">
        {{ $t('playground.hideDebug') }}
      </button>
    </div>
    <div class="th-pg-v2-debug__body">
      <div class="th-pg-v2-debug__grid">
        <div class="th-pg-v2-debug__block">
          <div class="th-pg-v2-debug__head">
            <p class="th-pg-v2-debug__label">{{ $t('playground.debugReq') }}</p>
            <button type="button" class="th-pg-v2-debug__copy" @click="onCopy(lastRequest)">
              {{ $t('playground.copy') }}
            </button>
          </div>
          <pre class="th-pg-v2-debug__pre">{{ lastRequest || '—' }}</pre>
        </div>
        <div class="th-pg-v2-debug__block">
          <div class="th-pg-v2-debug__head">
            <p class="th-pg-v2-debug__label">{{ $t('playground.debugRes') }}</p>
            <button type="button" class="th-pg-v2-debug__copy" @click="onCopy(responseDisplay)">
              {{ $t('playground.copy') }}
            </button>
          </div>
          <pre class="th-pg-v2-debug__pre">{{ responseDisplay }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>
