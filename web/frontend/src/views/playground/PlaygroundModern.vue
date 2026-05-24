<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import ThLoadingSpinner from '../../components/ui/ThLoadingSpinner.vue'
import PlaygroundModeTabs from '../../components/playground/PlaygroundModeTabs.vue'
import PlaygroundModelSelect from '../../components/playground/PlaygroundModelSelect.vue'
import PlaygroundMessageList from '../../components/playground/PlaygroundMessageList.vue'
import PlaygroundEmptyPanel from '../../components/playground/PlaygroundEmptyPanel.vue'
import PlaygroundComposer from '../../components/playground/PlaygroundComposer.vue'
import PlaygroundDebugBar from '../../components/playground/PlaygroundDebugBar.vue'
import PlaygroundToolbarQuickParams from '../../components/playground/PlaygroundToolbarQuickParams.vue'
import { usePlaygroundCore } from '../../composables/usePlaygroundCore'
import { PG_VIDEO_MAX_BYTES } from '../../types/playground'
import type { PlaygroundUiVersion } from '../../types/playground'

defineProps<{
  uiVersion: PlaygroundUiVersion
}>()

const emit = defineEmits<{
  'update:uiVersion': [v: PlaygroundUiVersion]
}>()

const {
  apiKey,
  assistantMarkdown,
  audioFile,
  audioFileInputRef,
  audioPlaySub,
  audioSpeechVoice,
  audioUploadErr,
  cancelVideoGeneration,
  chatHeaderSubtitle,
  chatScrollEl,
  clearVideoRefPreview,
  copyText,
  customBody,
  customBodyJson,
  editingId,
  editDraft,
  emptyPanelHint,
  exportSettings,
  fmtNum,
  fetchVideoBlobForMessage,
  filteredPlaygroundModels,
  imageFileInputRef,
  imageGenSize,
  imageOutputFailed,
  imageResponseFormat,
  imageUploadErr,
  imageUrl,
  importSettings,
  input,
  inputFooterPh,
  isPlaygroundModelCallable,
  isVideoRestorePending,
  lastRequest,
  lastResponse,
  markImageLoadFailed,
  maxTokens,
  model,
  modelPickerOpen,
  modelSearchQuery,
  models,
  modelsErr,
  modelsLoading,
  modelsLoadingUi,
  messages,
  onPlaygroundAudioFile,
  onPlaygroundImageFile,
  onPlaygroundVideoRefFile,
  openPlaygroundAudioPicker,
  openPlaygroundImagePicker,
  openPlaygroundVideoRefPicker,
  pasteKey,
  pgImageSizeOptions,
  pgMediaParamHint,
  pgVideoSizeOptions,
  pgModelKindLabel,
  pgRegistryCell,
  pgRouterHealthCell,
  pgServiceStatusCell,
  pickModel,
  playgroundRegistryIds,
  playMode,
  playgroundBubbleText,
  refreshModels,
  regenerate,
  removeMessage,
  renderAssistantMarkdown,
  retryImageWithB64,
  retryVideoMessage,
  saveEdit,
  selectedModelDisplayId,
  selectedModelRow,
  showSelectedModelMeta,
  sendDisabled,
  sendMessage,
  sending,
  showDebug,
  showRegenerateForCurrentMode,
  startEdit,
  startNewPlaygroundSession,
  stopStream,
  streamLog,
  systemPrompt,
  t,
  temperature,
  useImage,
  useMaxTokens,
  useStream,
  useTemp,
  videoFileInputRef,
  videoRefFile,
  videoRefPreviewUrl,
  videoSeconds,
  videoSize,
  videoStatusLabel,
  videoRestoringIds,
  videoUploadErr,
} = usePlaygroundCore({ modernUi: true })

const settingsOpen = ref(false)
const debugPanelCollapsed = ref(false)
const messageListRef = ref<InstanceType<typeof PlaygroundMessageList> | null>(null)
const composerWrapRef = ref<InstanceType<typeof PlaygroundComposer> | null>(null)

const examplePromptKeys = ['playground.examplePrompt1', 'playground.examplePrompt2', 'playground.examplePrompt3'] as const

function applyExamplePrompt(key: (typeof examplePromptKeys)[number]) {
  input.value = t(key)
  nextTick(() => composerWrapRef.value?.focusInput?.())
}

const toolbarModeHint = computed(() => {
  if (playMode.value === 'chat') return ''
  const sub = chatHeaderSubtitle.value
  const prefix = `${model.value || '—'} · `
  return sub.startsWith(prefix) ? sub.slice(prefix.length) : sub
})

watch(
  showDebug,
  (enabled) => {
    if (enabled) {
      debugPanelCollapsed.value = false
    } else {
      debugPanelCollapsed.value = true
    }
  },
)

watch(
  () => messageListRef.value?.chatScrollEl ?? null,
  (el) => {
    chatScrollEl.value = el
  },
  { flush: 'post' },
)

function onAttachImage() {
  useImage.value = true
  openPlaygroundImagePicker()
}

function onClearImage() {
  useImage.value = false
  imageUrl.value = ''
}
</script>

<template>
  <div class="th-playground th-pg-v2-root flex min-h-0 flex-1 flex-col overflow-hidden">
    <header class="th-pg-v2-header th-pg-v2-header--compact th-pg-v2-header--unified shrink-0">
      <div class="th-pg-v2-header__top">
        <div class="th-pg-v2-header__brand">
          <h1 class="th-pg-v2-header__title">{{ t('playground.title') }}</h1>
        </div>
        <div class="th-pg-v2-header__actions">
          <div class="th-pg-v2-version-pills" role="group" :aria-label="t('playground.uiVersionToggle')">
            <button
              type="button"
              class="th-pg-v2-version-pills__btn"
              :class="{ 'th-pg-v2-version-pills__btn--active': uiVersion === 'classic' }"
              @click="emit('update:uiVersion', 'classic')"
            >
              {{ t('playground.uiVersionClassic') }}
            </button>
            <button
              type="button"
              class="th-pg-v2-version-pills__btn"
              :class="{ 'th-pg-v2-version-pills__btn--active': uiVersion === 'modern' }"
              @click="emit('update:uiVersion', 'modern')"
            >
              {{ t('playground.uiVersionModern') }}
            </button>
          </div>
          <button type="button" class="th-pg-v2-header__btn" :title="t('playground.newSession')" @click="startNewPlaygroundSession">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-4 w-4 sm:hidden" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
            <span class="hidden sm:inline">{{ t('playground.newSession') }}</span>
          </button>
          <button type="button" class="th-pg-v2-header__btn th-pg-v2-header__btn--primary" @click="settingsOpen = true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-4 w-4" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.47 6.47 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a7.722 7.722 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
              <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
            </svg>
            <span class="hidden sm:inline">{{ t('playground.settingsDrawer') }}</span>
          </button>
        </div>
      </div>

      <div v-if="!apiKey.trim()" class="th-pg-v2-alert th-pg-v2-alert--warn th-pg-v2-alert--compact">
        <span class="th-pg-v2-alert__text">{{ t('playground.needKey') }}</span>
        <button type="button" class="th-pg-v2-alert__link" @click="settingsOpen = true">{{ t('playground.configureKey') }}</button>
        <RouterLink to="/console/keys" class="th-pg-v2-alert__link">{{ t('playground.goKeys') }}</RouterLink>
      </div>

      <div class="th-pg-v2-toolbar th-pg-v2-toolbar--compact">
        <div class="th-pg-v2-toolbar__main">
          <div class="th-pg-v2-toolbar__cluster">
            <PlaygroundModeTabs v-model="playMode" />

            <template v-if="playMode === 'audio'">
              <div
                class="th-pg-v2-version-pills th-pg-v2-audio-sub-pills"
                role="group"
                :aria-label="t('playground.audioSubAria')"
              >
                <button
                  type="button"
                  class="th-pg-v2-version-pills__btn"
                  :class="{ 'th-pg-v2-version-pills__btn--active': audioPlaySub === 'speech' }"
                  @click="audioPlaySub = 'speech'"
                >
                  {{ t('playground.audioSubSpeech') }}
                </button>
                <button
                  type="button"
                  class="th-pg-v2-version-pills__btn"
                  :class="{ 'th-pg-v2-version-pills__btn--active': audioPlaySub === 'transcribe' }"
                  @click="audioPlaySub = 'transcribe'"
                >
                  {{ t('playground.audioSubTranscribe') }}
                </button>
                <button
                  type="button"
                  class="th-pg-v2-version-pills__btn"
                  :class="{ 'th-pg-v2-version-pills__btn--active': audioPlaySub === 'translate' }"
                  @click="audioPlaySub = 'translate'"
                >
                  {{ t('playground.audioSubTranslate') }}
                </button>
              </div>
              <label v-if="audioPlaySub === 'speech'" class="th-pg-v2-voice-field">
                <span class="th-pg-v2-voice-field__label">{{ t('playground.audioVoiceShort') }}</span>
                <input
                  v-model="audioSpeechVoice"
                  type="text"
                  class="th-pg-v2-voice-field__input"
                  :placeholder="t('playground.audioVoicePh')"
                  autocomplete="off"
                />
              </label>
            </template>

            <PlaygroundToolbarQuickParams
              v-else
              v-model:temperature="temperature"
              v-model:use-temp="useTemp"
              v-model:image-gen-size="imageGenSize"
              v-model:video-seconds="videoSeconds"
              v-model:video-size="videoSize"
              :play-mode="playMode"
              :image-size-options="pgImageSizeOptions"
              :video-size-options="pgVideoSizeOptions"
              :fmt-num="fmtNum"
            />
          </div>

          <div class="th-pg-v2-toolbar__spacer" aria-hidden="true" />

          <div class="th-pg-v2-model-bar">
          <label class="sr-only">{{ t('playground.model') }}</label>
          <PlaygroundModelSelect
            v-model:open="modelPickerOpen"
            v-model:search-query="modelSearchQuery"
            v-model:model="model"
            :models="models"
            :filtered-models="filteredPlaygroundModels"
            :selected-display-id="selectedModelDisplayId"
            :selected-model-row="selectedModelRow"
            :show-selected-model-meta="showSelectedModelMeta"
            :playground-registry-ids="playgroundRegistryIds"
            :is-callable="isPlaygroundModelCallable"
            :catalog-only-title="t('playground.catalogOnlyOptionTitle')"
            :model-kind-label="pgModelKindLabel"
            :service-status-cell="pgServiceStatusCell"
            :router-health-cell="pgRouterHealthCell"
            :registry-cell="pgRegistryCell"
            @pick="pickModel"
          />
          <button
            type="button"
            class="th-pg-v2-model-bar__refresh"
            :disabled="modelsLoading"
            :title="t('playground.refreshModels')"
            @click="refreshModels"
          >
            <ThLoadingSpinner v-if="modelsLoadingUi" size="sm" />
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" class="h-4 w-4" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" />
            </svg>
          </button>
            <div
              v-if="playMode === 'chat'"
              class="th-pg-v2-version-pills th-pg-v2-stream-pills"
              role="group"
              :aria-label="t('playground.stream')"
            >
              <button
                type="button"
                class="th-pg-v2-version-pills__btn"
                :class="{ 'th-pg-v2-version-pills__btn--active': !useStream }"
                @click="useStream = false"
              >
                {{ t('playground.streamOff') }}
              </button>
              <button
                type="button"
                class="th-pg-v2-version-pills__btn"
                :class="{ 'th-pg-v2-version-pills__btn--active': useStream }"
                @click="useStream = true"
              >
                {{ t('playground.streamOn') }}
              </button>
            </div>
            <span
              v-if="toolbarModeHint && !(playMode === 'audio' && audioPlaySub === 'speech')"
              class="th-pg-v2-model-bar__meta"
            >{{ toolbarModeHint }}</span>
          </div>
        </div>

        <p v-if="modelsErr" class="th-pg-v2-toolbar__err">{{ modelsErr }}</p>
      </div>
    </header>

    <div class="th-pg-v2-body flex min-h-0 flex-1 flex-col" :class="{ 'th-pg-v2-body--idle': !messages.length }">
      <PlaygroundMessageList
        v-if="messages.length"
        ref="messageListRef"
        v-model:editing-id="editingId"
        v-model:edit-draft="editDraft"
        :messages="messages"
        :assistant-markdown="assistantMarkdown"
        :play-mode="playMode"
        :video-restoring-ids="videoRestoringIds"
        :api-key="apiKey"
        :model="model"
        :show-regenerate="showRegenerateForCurrentMode"
        :sending="sending"
        :bubble-text="playgroundBubbleText"
        :render-markdown="renderAssistantMarkdown"
        :video-status-label="videoStatusLabel"
        :is-video-restore-pending="isVideoRestorePending"
        :image-output-failed="imageOutputFailed"
        @copy="copyText"
        @edit="startEdit"
        @remove="removeMessage"
        @regen="regenerate"
        @retry-video="retryVideoMessage"
        @reload-video="fetchVideoBlobForMessage"
        @save-edit="saveEdit"
        @image-error="markImageLoadFailed"
        @retry-b64="retryImageWithB64"
      />

      <PlaygroundEmptyPanel v-else variant="idle">
        {{ emptyPanelHint }}
        <template v-if="playMode === 'chat'" #actions>
          <div class="th-pg-v2-empty__chips">
            <button
              v-for="key in examplePromptKeys"
              :key="key"
              type="button"
              class="th-pg-v2-empty__chip"
              @click="applyExamplePrompt(key)"
            >
              {{ t(key) }}
            </button>
          </div>
        </template>
      </PlaygroundEmptyPanel>

      <div class="th-pg-v2-body__footer">
        <div class="th-pg-v2-bottom-dock">
          <PlaygroundDebugBar
            v-if="showDebug && !debugPanelCollapsed"
            :last-request="lastRequest"
            :last-response="lastResponse"
            :stream-log="streamLog"
            @copy="copyText"
            @collapse="debugPanelCollapsed = true"
          />
          <div v-else-if="showDebug && debugPanelCollapsed" class="th-pg-v2-debug-toggle">
            <button type="button" class="th-pg-v2-debug-toggle__btn" @click="debugPanelCollapsed = false">
              {{ t('playground.showDebug') }}
            </button>
          </div>

          <PlaygroundComposer
            ref="composerWrapRef"
            v-model:input="input"
            :play-mode="playMode"
            :audio-play-sub="audioPlaySub"
            :placeholder="inputFooterPh"
            :send-disabled="sendDisabled"
            :sending="sending"
            :show-stop-stream="sending && useStream && playMode === 'chat'"
            :show-cancel-video="sending && playMode === 'video'"
            :image-preview="useImage && imageUrl ? imageUrl : undefined"
            :video-ref-preview="videoRefPreviewUrl || undefined"
            :audio-file-name="audioFile?.name"
            :image-upload-err="imageUploadErr"
            :video-upload-err="videoUploadErr"
            :audio-upload-err="audioUploadErr"
            @send="sendMessage"
            @stop-stream="stopStream"
            @cancel-video="cancelVideoGeneration"
            @attach-image="onAttachImage"
            @attach-video-ref="openPlaygroundVideoRefPicker"
            @attach-audio="openPlaygroundAudioPicker"
            @clear-image="onClearImage"
            @clear-video-ref="clearVideoRefPreview"
            @clear-audio="audioFile = null"
          />
        </div>
      </div>
    </div>

    <input ref="imageFileInputRef" type="file" class="sr-only" accept="image/jpeg,image/png,image/gif,image/webp" @change="onPlaygroundImageFile" />
    <input ref="videoFileInputRef" type="file" class="sr-only" accept="image/jpeg,image/png,image/gif,image/webp" @change="onPlaygroundVideoRefFile" />
    <input ref="audioFileInputRef" type="file" class="sr-only" accept="audio/*" @change="onPlaygroundAudioFile" />

    <Teleport to="body">
      <div v-if="settingsOpen" class="th-pg-v2-drawer-backdrop" @click="settingsOpen = false" />
      <aside v-if="settingsOpen" class="th-pg-v2-drawer thin-scrollbar">
        <div class="th-pg-v2-drawer__head">
          <h2>{{ t('playground.settingsDrawer') }}</h2>
          <button type="button" class="th-pg-v2-drawer__close" :aria-label="t('common.cancel')" @click="settingsOpen = false">×</button>
        </div>
        <div class="th-pg-v2-drawer__body">
          <section class="th-pg-v2-drawer__section">
            <label class="th-pg-v2-drawer__label">{{ t('playground.apiKey') }}</label>
            <div class="flex gap-2">
              <input v-model="apiKey" type="password" autocomplete="off" class="th-pg-v2-drawer__input flex-1 font-mono" />
              <button type="button" class="th-pg-v2-header__btn shrink-0" @click="pasteKey">{{ t('playground.paste') }}</button>
            </div>
          </section>
          <section v-if="playMode === 'chat'" class="th-pg-v2-drawer__section space-y-3">
            <div>
              <label class="th-pg-v2-drawer__label">{{ t('playground.system') }}</label>
              <input
                v-model="systemPrompt"
                type="text"
                class="th-pg-v2-drawer__input mt-1"
                :placeholder="t('playground.systemPh')"
                autocomplete="off"
              />
            </div>
            <label class="th-pg-v2-check">
              <input v-model="useMaxTokens" type="checkbox" />
              {{ t('playground.maxTokens') }}
            </label>
            <input
              v-model.number="maxTokens"
              type="number"
              min="1"
              class="th-pg-v2-drawer__input w-full font-mono"
              :disabled="!useMaxTokens"
            />
          </section>
          <section v-if="playMode === 'image'" class="th-pg-v2-drawer__section">
            <label class="th-pg-v2-drawer__label">{{ t('playground.imageResponseFormat') }}</label>
            <select v-model="imageResponseFormat" class="th-pg-v2-drawer__input mt-1">
              <option value="url">{{ t('playground.imageFormatUrl') }}</option>
              <option value="b64_json">{{ t('playground.imageFormatB64') }}</option>
            </select>
          </section>
          <section class="th-pg-v2-drawer__section th-pg-v2-drawer__checks">
            <label class="th-pg-v2-check">
              <input v-model="assistantMarkdown" type="checkbox" />
              {{ t('playground.renderMd') }}
            </label>
            <label class="th-pg-v2-check">
              <input v-model="showDebug" type="checkbox" />
              {{ t('playground.showDebug') }}
            </label>
            <p v-if="showDebug" class="th-pg-v2-drawer__hint">{{ t('playground.showDebugHint') }}</p>
          </section>
          <section v-if="playMode === 'video'" class="th-pg-v2-drawer__section space-y-2">
            <p v-if="pgMediaParamHint" class="th-pg-v2-drawer__hint">{{ t(pgMediaParamHint) }}</p>
            <div>
              <label class="th-pg-v2-drawer__label">{{ t('playground.videoRefImage') }}</label>
              <div class="mt-1 flex flex-wrap items-center gap-2">
                <button type="button" class="th-pg-v2-header__btn" @click="openPlaygroundVideoRefPicker">
                  {{ t('playground.videoRefPick') }}
                </button>
                <button v-if="videoRefFile" type="button" class="th-pg-v2-header__btn" @click="clearVideoRefPreview">
                  {{ t('playground.videoRefClear') }}
                </button>
              </div>
              <img
                v-if="videoRefPreviewUrl"
                :src="videoRefPreviewUrl"
                alt=""
                class="mt-2 max-h-28 max-w-full rounded-lg border border-white/10 object-contain"
              />
              <p class="th-pg-v2-drawer__hint mt-1">
                {{ t('playground.videoRefHint', { maxMb: Math.floor(PG_VIDEO_MAX_BYTES / 1024 / 1024) }) }}
              </p>
            </div>
          </section>
          <details v-if="playMode === 'chat'" class="th-pg-v2-drawer__section">
            <summary class="th-pg-v2-drawer__label cursor-pointer">{{ t('playground.devOptions') }}</summary>
            <label class="th-pg-v2-check mt-2">
              <input v-model="customBody" type="checkbox" />
              {{ t('playground.customBody') }}
            </label>
            <textarea v-if="customBody" v-model="customBodyJson" rows="6" class="th-pg-v2-drawer__input mt-2 font-mono text-[11px]" />
          </details>
          <div class="th-pg-v2-drawer__section flex flex-wrap gap-2">
            <button type="button" class="th-pg-v2-header__btn th-pg-v2-header__btn--primary" @click="exportSettings">{{ t('playground.export') }}</button>
            <label class="th-pg-v2-header__btn cursor-pointer">
              {{ t('playground.import') }}
              <input type="file" accept="application/json" class="hidden" @change="importSettings" />
            </label>
          </div>
        </div>
      </aside>
    </Teleport>
  </div>
</template>
