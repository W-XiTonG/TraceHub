<script setup lang="ts">
import { computed, ref } from 'vue'
import ThLoadingSpinner from '../ui/ThLoadingSpinner.vue'
import PlaygroundMediaBubble from './PlaygroundMediaBubble.vue'
import PlaygroundMediaDownloadBtn from './PlaygroundMediaDownloadBtn.vue'
import PlaygroundUserAvatar from './PlaygroundUserAvatar.vue'
import type { ChatMsg } from '../../types/playground'

const props = defineProps<{
  messages: ChatMsg[]
  assistantMarkdown: boolean
  playMode: string
  videoRestoringIds: Set<string>
  apiKey: string
  model: string
  showRegenerate: boolean
  sending: boolean
  bubbleText: (m: ChatMsg) => string
  renderMarkdown: (src: string) => string
  videoStatusLabel: (status: string) => string
  isVideoRestorePending: (m: ChatMsg) => boolean
  imageOutputFailed: (m: ChatMsg, index: number) => boolean
}>()

const emit = defineEmits<{
  copy: [text: string]
  edit: [m: ChatMsg]
  remove: [id: string]
  regen: []
  retryVideo: [m: ChatMsg]
  reloadVideo: [m: ChatMsg]
  saveEdit: []
  imageError: [msgId: string, index: number]
  retryB64: []
}>()

const editingId = defineModel<string | null>('editingId', { default: null })
const editDraft = defineModel<string>('editDraft', { default: '' })

const chatScrollEl = ref<HTMLElement | null>(null)
defineExpose({ chatScrollEl })

function isErrorBubble(m: ChatMsg): boolean {
  return m.role === 'assistant' && (m.content.includes('【') || m.videoStatus === 'failed')
}

function avatarLetter(m: ChatMsg): string {
  return m.role === 'assistant' ? 'AI' : ''
}

function isTypingBubble(m: ChatMsg): boolean {
  if (!props.sending || m.role !== 'assistant') return false
  if (m.content.trim() || m.videoUrl || m.audioUrl || m.imageOutputs?.length) return false
  if (m.videoStatus && m.videoStatus !== 'failed' && m.videoStatus !== 'cancelled') return false
  return true
}

const showStandaloneTyping = computed(() => {
  if (!props.sending || !props.messages.length) return props.sending
  const last = props.messages[props.messages.length - 1]
  return last.role === 'user'
})
</script>

<template>
  <div ref="chatScrollEl" class="th-pg-v2-messages thin-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-y-contain">
    <div class="th-pg-v2-thread">
      <article
        v-for="m in messages"
        :key="m.id"
        class="th-pg-v2-turn group"
        :class="[
          m.role === 'user' ? 'th-pg-v2-turn--user' : 'th-pg-v2-turn--assistant',
          isErrorBubble(m) ? 'th-pg-v2-turn--error' : '',
        ]"
      >
        <PlaygroundUserAvatar v-if="m.role === 'user'" />
        <div v-else class="th-pg-v2-turn__avatar th-pg-v2-turn__avatar--ai">
          {{ avatarLetter(m) }}
        </div>

        <div class="th-pg-v2-turn__body">
          <div
            class="th-pg-v2-bubble"
            :class="[
              m.role === 'user' ? 'th-pg-v2-bubble--user' : 'th-pg-v2-bubble--assistant',
              isErrorBubble(m) ? 'th-pg-v2-bubble--error' : '',
            ]"
          >
            <div v-if="m.role === 'assistant' && m.videoStatus" class="th-pg-v2-status-pill">
              {{ videoStatusLabel(m.videoStatus) }}
              <span v-if="m.videoJobId" class="font-mono opacity-70">· {{ m.videoJobId.slice(0, 10) }}…</span>
            </div>

            <div v-if="m.role === 'assistant' && isVideoRestorePending(m)" class="th-pg-v2-media-block text-xs">
              <p v-if="videoRestoringIds.has(m.id)" class="inline-flex items-center gap-2 th-pg-muted">
                <ThLoadingSpinner size="sm" />
                {{ $t('playground.videoRestoring') }}
              </p>
              <template v-else>
                <p class="th-pg-muted leading-relaxed">{{ $t('playground.videoReloadHint') }}</p>
                <button type="button" class="th-pg-v2-link-btn mt-2" @click="emit('reloadVideo', m)">
                  {{ $t('playground.videoReload') }}
                </button>
              </template>
            </div>
            <div
              v-else-if="m.role === 'assistant' && m.videoUrl"
              class="th-pg-v2-media-block"
            >
              <video
                :src="m.videoUrl"
                controls
                preload="metadata"
                class="max-w-full rounded-lg"
              />
              <PlaygroundMediaDownloadBtn
                :src="m.videoUrl"
                kind="video"
                :filename-base="m.videoJobId || m.id"
              />
            </div>
            <audio v-if="m.role === 'assistant' && m.audioUrl" :src="m.audioUrl" controls preload="metadata" class="th-pg-v2-media-block w-full" />
            <div v-if="m.role === 'assistant' && m.imageOutputs?.length" class="th-pg-v2-media-block flex flex-col gap-2">
              <PlaygroundMediaBubble
                v-for="(src, ix) in m.imageOutputs"
                :key="`${m.id}-img-${ix}`"
                :src="src"
                :failed="imageOutputFailed(m, ix)"
                :show-retry-b64="playMode === 'image' && ix === 0"
                :download-name="`${m.id}-${ix + 1}`"
                @error="emit('imageError', m.id, ix)"
                @retry-b64="emit('retryB64')"
              />
            </div>
            <img
              v-if="m.imageUrl || m.videoRefUrl"
              :src="m.imageUrl || m.videoRefUrl"
              alt=""
              class="th-pg-v2-media-block max-h-44 max-w-full rounded-lg object-contain"
            />

            <div v-if="editingId === m.id" class="space-y-2">
              <textarea v-model="editDraft" rows="4" class="th-pg-v2-edit-area" />
              <div class="flex gap-2">
                <button type="button" class="th-pg-v2-link-btn" @click="emit('saveEdit')">{{ $t('common.save') }}</button>
                <button type="button" class="th-pg-v2-link-btn th-pg-v2-link-btn--muted" @click="editingId = null">
                  {{ $t('common.cancel') }}
                </button>
              </div>
            </div>
            <div v-else-if="isTypingBubble(m)" class="th-pg-v2-typing-dots" aria-live="polite">
              <span />
              <span />
              <span />
              <span class="sr-only">{{ $t('playground.sending') }}</span>
            </div>
            <div
              v-else-if="m.role === 'assistant' && assistantMarkdown"
              class="playground-bubble-md th-pg-v2-bubble__text"
              v-html="renderMarkdown(bubbleText(m))"
            />
            <div v-else class="th-pg-v2-bubble__text whitespace-pre-wrap">{{ bubbleText(m) }}</div>
          </div>

          <div v-if="editingId !== m.id" class="th-pg-v2-turn__actions" role="toolbar" :aria-label="$t('playground.msgActionsAria')">
            <button type="button" class="th-pg-v2-action" :title="$t('playground.copy')" @click="emit('copy', m.content)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-3.5 w-3.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184" />
              </svg>
            </button>
            <button type="button" class="th-pg-v2-action" :title="$t('common.edit')" @click="emit('edit', m)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-3.5 w-3.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
              </svg>
            </button>
            <button type="button" class="th-pg-v2-action th-pg-v2-action--danger" :title="$t('common.delete')" @click="emit('remove', m.id)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-3.5 w-3.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
              </svg>
            </button>
            <button
              v-if="m.role === 'assistant' && playMode === 'video' && m.videoJobId && !m.videoUrl && (m.videoStatus === 'failed' || m.videoStatus === 'cancelled')"
              type="button"
              class="th-pg-v2-action"
              :title="$t('playground.videoRetry')"
              @click="emit('retryVideo', m)"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-3.5 w-3.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" />
              </svg>
            </button>
            <button
              v-if="m.role === 'assistant' && showRegenerate"
              type="button"
              class="th-pg-v2-action"
              :title="$t('playground.regen')"
              @click="emit('regen')"
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-3.5 w-3.5" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" />
              </svg>
            </button>
          </div>
        </div>
      </article>

      <article v-if="showStandaloneTyping" class="th-pg-v2-turn th-pg-v2-turn--assistant th-pg-v2-turn--typing" aria-live="polite">
        <div class="th-pg-v2-turn__avatar th-pg-v2-turn__avatar--ai">AI</div>
        <div class="th-pg-v2-turn__body">
          <div class="th-pg-v2-bubble th-pg-v2-bubble--assistant th-pg-v2-bubble--typing">
            <span class="th-pg-v2-typing-dots" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            <span class="sr-only">{{ $t('playground.sending') }}</span>
          </div>
        </div>
      </article>
    </div>
  </div>
</template>
