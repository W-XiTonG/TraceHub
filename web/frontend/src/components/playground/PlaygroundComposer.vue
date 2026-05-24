<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import type { PlaygroundAudioSub, PlaygroundPlayMode } from '../../types/playground'

defineProps<{
  playMode: PlaygroundPlayMode
  audioPlaySub: PlaygroundAudioSub
  placeholder: string
  sendDisabled: boolean
  sending: boolean
  showStopStream: boolean
  showCancelVideo: boolean
  imagePreview?: string
  videoRefPreview?: string
  audioFileName?: string
  imageUploadErr?: string
  videoUploadErr?: string
  audioUploadErr?: string
}>()

const emit = defineEmits<{
  send: []
  stopStream: []
  cancelVideo: []
  attachImage: []
  attachVideoRef: []
  attachAudio: []
  clearImage: []
  clearVideoRef: []
  clearAudio: []
}>()

const inputModel = defineModel<string>('input', { default: '' })
const inputEl = ref<HTMLTextAreaElement | null>(null)

const showAttach = (playMode: PlaygroundPlayMode, audioPlaySub: PlaygroundAudioSub) =>
  playMode === 'chat' || playMode === 'video' || (playMode === 'audio' && audioPlaySub !== 'speech')

function syncInputHeight() {
  const el = inputEl.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 120)}px`
}

function onEnter(ev: KeyboardEvent) {
  if (ev.shiftKey) return
  ev.preventDefault()
  emit('send')
}

watch(inputModel, () => nextTick(syncInputHeight))

onMounted(() => nextTick(syncInputHeight))

function focusInput() {
  inputEl.value?.focus()
}

defineExpose({ focusInput })
</script>

<template>
  <footer class="th-pg-v2-composer-wrap">
    <div class="th-pg-v2-composer">
      <div
        v-if="imagePreview || videoRefPreview || audioFileName"
        class="th-pg-v2-composer__preview"
      >
        <img v-if="imagePreview || videoRefPreview" :src="imagePreview || videoRefPreview" alt="" class="h-10 w-10 rounded-md object-cover" />
        <span v-if="audioFileName" class="text-xs th-pg-muted truncate">{{ audioFileName }}</span>
        <button v-if="imagePreview" type="button" class="th-pg-v2-composer__preview-clear" @click="emit('clearImage')">×</button>
        <button v-if="videoRefPreview" type="button" class="th-pg-v2-composer__preview-clear" @click="emit('clearVideoRef')">×</button>
        <button v-if="audioFileName" type="button" class="th-pg-v2-composer__preview-clear" @click="emit('clearAudio')">×</button>
      </div>

      <p v-if="imageUploadErr || videoUploadErr || audioUploadErr" class="th-pg-v2-composer__err">
        {{ imageUploadErr || videoUploadErr || audioUploadErr }}
      </p>

      <div class="th-pg-v2-composer__row">
        <button
          v-if="showAttach(playMode, audioPlaySub)"
          type="button"
          class="th-pg-v2-composer__attach"
          :title="
            playMode === 'chat'
              ? $t('playground.composerAttachImage')
              : playMode === 'video'
                ? $t('playground.composerAttachVideoRef')
                : $t('playground.composerAttachAudio')
          "
          @click="
            playMode === 'chat' ? emit('attachImage') : playMode === 'video' ? emit('attachVideoRef') : emit('attachAudio')
          "
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-5 w-5" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="m18.375 12.739-7.693 7.693a4.5 4.5 0 0 1-6.364-6.364l10.94-10.94A3 3 0 1 1 19.5 7.372L8.552 18.32m.009-.01-.01.01m5.106-9.87 2.54-2.54" />
          </svg>
        </button>

        <textarea
          ref="inputEl"
          v-model="inputModel"
          rows="1"
          class="th-pg-v2-composer__input"
          :placeholder="placeholder"
          @keydown.enter="onEnter"
          @input="syncInputHeight"
        />

        <div class="th-pg-v2-composer__actions">
          <button v-if="showStopStream" type="button" class="th-pg-v2-composer__secondary" @click="emit('stopStream')">
            {{ $t('playground.stop') }}
          </button>
          <button v-if="showCancelVideo" type="button" class="th-pg-v2-composer__secondary" @click="emit('cancelVideo')">
            {{ $t('playground.videoCancel') }}
          </button>
          <button
            type="button"
            class="th-pg-v2-composer__send"
            :disabled="sendDisabled"
            :aria-label="sending ? $t('playground.sending') : $t('playground.send')"
            @click="emit('send')"
          >
            <svg v-if="!sending" viewBox="0 0 24 24" fill="currentColor" class="h-5 w-5" aria-hidden="true">
              <path d="M3.478 2.405a.75.75 0 0 0-.926.94l2.432 7.905H13.5a.75.75 0 0 1 0 1.5H4.984l-2.432 7.905a.75.75 0 0 0 .926.94 60.519 60.519 0 0 0 18.445-8.986.75.75 0 0 0 0-1.218A60.517 60.517 0 0 0 3.478 2.405Z" />
            </svg>
            <span v-else class="th-pg-v2-composer__send-dots" aria-hidden="true" />
          </button>
        </div>
      </div>
      <p class="th-pg-v2-composer__hint">{{ $t('playground.composerHint') }}</p>
    </div>
  </footer>
</template>
