<script setup lang="ts">
import ThLoadingSpinner from '../../components/ui/ThLoadingSpinner.vue'
import PlaygroundMediaDownloadBtn from '../../components/playground/PlaygroundMediaDownloadBtn.vue'
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { usePlaygroundCore } from '../../composables/usePlaygroundCore'

const {
  LS_KEY,
  LS_MSG,
  PG_DOT_AVAILABLE,
  PG_DOT_UNAVAILABLE,
  PG_DOT_UNKNOWN,
  PG_DOT_UNKNOWN_DIM,
  SS_KEY,
  abortCtrl,
  activeVideoAssistantId,
  activeVideoJobId,
  apiKey,
  applyModernUiDefaults,
  assistantMarkdown,
  audioFile,
  audioFileInputRef,
  audioPlaySub,
  audioSpeechSpeed,
  audioSpeechVoice,
  audioUploadErr,
  buildCustomBody,
  buildNormalBody,
  buildOpenAIMessagesForHistory,
  cancelEdit,
  cancelVideoGeneration,
  chatHeaderSubtitle,
  chatHeaderTitle,
  chatScrollEl,
  clearPersistedVideoJob,
  clearVideoRefPreview,
  closeModelPicker,
  copyText,
  customBody,
  customBodyJson,
  downloadBlob,
  editDraft,
  editingId,
  emptyPanelHint,
  escapeHtml,
  exportConversation,
  exportSettings,
  fetchVideoBlobForMessage,
  pasteKey,
  pollVideoUntilDone,
  refreshModels,
  regenerate,
  restorePersistedVideoMedia,
  resumePersistedVideoJob,
  retryImageWithB64,
  retryVideoMessage,
  sendAudioMultipart,
  sendAudioSpeech,
  sendImageGeneration,
  sendMessage,
  sendVideoGeneration,
  filteredPlaygroundModels,
  fmtNum,
  formatVideoPollingCaption,
  freqPen,
  hasCatalogOnlyPlaygroundModels,
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
  loadSettings,
  markImageLoadFailed,
  maxTokens,
  messages,
  model,
  modelPickerOpen,
  modelPickerPanel,
  modelPickerPanelStyle,
  modelPickerSearchInput,
  modelPickerWrap,
  modelSearchQuery,
  models,
  modelsErr,
  modelsLoading,
  modelsLoadingUi,
  onDocKeydown,
  onDocMousedown,
  onPlaygroundAudioFile,
  onPlaygroundImageFile,
  onPlaygroundVideoRefFile,
  onPlaygroundVisibility,
  onWinReposition,
  openPlaygroundAudioPicker,
  openPlaygroundImagePicker,
  openPlaygroundVideoRefPicker,
  parseErr,
  parseTranscriptionJson,
  patchAssistant,
  patchAssistantAudio,
  patchAssistantImages,
  patchAssistantVideo,
  persistActiveVideoJob,
  persistConversation,
  persistSettings,
  pgImageSizeOptions,
  pgMediaParamHint,
  pgMediaVendor,
  pgModelKindLabel,
  pgRegistryCell,
  pgRouterHealthCell,
  pgServiceStatusCell,
  pgVideoSizeOptions,
  pickModel,
  playMode,
  playgroundBubbleText,
  playgroundRegistryIds,
  playgroundRegistryMerged,
  presPen,
  readPersistedVideoJob,
  removeMessage,
  renderAssistantMarkdown,
  revokeMsgMedia,
  rewriteAssistantLinks,
  saveEdit,
  scrollChatToBottom,
  seed,
  selectedModelDisplayId,
  selectedModelRow,
  sendDisabled,
  sending,
  showCatalogOnlyModelHint,
  showDebug,
  showRegenerateForCurrentMode,
  showSelectedModelMeta,
  showVideoModelHint,
  sidebarPlaygroundDesc,
  startEdit,
  startNewPlaygroundSession,
  stickySession,
  stopStream,
  stopVideoPoll,
  streamLog,
  sttLanguage,
  switchToUser,
  syncPlaygroundMediaParamsToModel,
  systemPrompt,
  t,
  temperature,
  toggleModelPicker,
  topP,
  uid,
  updateModelPanelPosition,
  useFreqPen,
  useImage,
  useMaxTokens,
  usePresPen,
  useSeed,
  useStream,
  useTemp,
  useTopP,
  videoFileInputRef,
  videoPollAbort,
  videoRefFile,
  videoRefPreviewUrl,
  videoRestoringIds,
  videoSeconds,
  videoSize,
  videoStatusLabel,
  videoUploadErr,
} = usePlaygroundCore({ modernUi: false })

const debugPanelCollapsed = ref(false)
watch(showDebug, (enabled) => {
  if (enabled) {
    debugPanelCollapsed.value = false
  } else {
    debugPanelCollapsed.value = true
  }
})
</script>

<template>
  <div class="th-playground flex min-h-0 flex-1 flex-col gap-4 lg:flex-row lg:gap-0 lg:items-stretch">
    <section
      class="th-pg-sidebar flex max-h-[min(52vh,26rem)] w-full shrink-0 flex-col overflow-hidden rounded-2xl lg:max-h-none lg:max-w-[22rem] lg:self-stretch lg:rounded-r-none"
    >
      <div class="th-pg-sidebar-head px-4 py-3">
        <p class="text-[11px] font-semibold uppercase tracking-wider text-slate-500">{{ t('playground.configKicker') }}</p>
        <h1 class="text-lg font-semibold text-slate-100">{{ t('playground.title') }}</h1>
        <p class="mt-1 text-xs leading-relaxed text-slate-500">
          {{ sidebarPlaygroundDesc }}
        </p>
        <p class="mt-2 border-l-2 border-cyan-500/45 pl-2.5 text-[11px] leading-relaxed th-pg-muted">
          {{ playMode === 'video' ? t('playground.billingHintVideo') : t('playground.billingHint') }}
        </p>
      </div>

      <div class="thin-scrollbar min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-y-contain px-4 py-4">
        <div class="th-pg-card th-pg-card--emph space-y-2 p-3">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('playground.playMode') }}</p>
          <div class="th-pg-seg th-pg-seg--modes" role="group" :aria-label="t('playground.playMode')">
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': playMode === 'chat' }"
              :aria-pressed="playMode === 'chat'"
              @click="playMode = 'chat'"
            >
              {{ t('playground.modeChat') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': playMode === 'image' }"
              :aria-pressed="playMode === 'image'"
              @click="playMode = 'image'"
            >
              {{ t('playground.modeImage') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': playMode === 'video' }"
              :aria-pressed="playMode === 'video'"
              @click="playMode = 'video'"
            >
              {{ t('playground.modeVideo') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': playMode === 'audio' }"
              :aria-pressed="playMode === 'audio'"
              @click="playMode = 'audio'"
            >
              {{ t('playground.modeAudio') }}
            </button>
          </div>
          <div v-if="playMode === 'audio'" class="th-pg-seg mt-2 flex w-full" role="group" :aria-label="t('playground.audioSubAria')">
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-[11px] leading-tight"
              :class="{ 'th-pg-seg__btn--active': audioPlaySub === 'speech' }"
              :aria-pressed="audioPlaySub === 'speech'"
              @click="audioPlaySub = 'speech'"
            >
              {{ t('playground.audioSubSpeech') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-[11px] leading-tight"
              :class="{ 'th-pg-seg__btn--active': audioPlaySub === 'transcribe' }"
              :aria-pressed="audioPlaySub === 'transcribe'"
              @click="audioPlaySub = 'transcribe'"
            >
              {{ t('playground.audioSubTranscribe') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-[11px] leading-tight"
              :class="{ 'th-pg-seg__btn--active': audioPlaySub === 'translate' }"
              :aria-pressed="audioPlaySub === 'translate'"
              @click="audioPlaySub = 'translate'"
            >
              {{ t('playground.audioSubTranslate') }}
            </button>
          </div>
        </div>

        <div v-if="playMode === 'chat'" class="th-pg-card th-pg-card--emph p-3">
          <label class="flex cursor-pointer items-center justify-between gap-2 text-sm th-pg-muted">
            <span>{{ t('playground.customBody') }}</span>
            <input v-model="customBody" type="checkbox" class="h-4 w-4 rounded border-slate-600 text-cyan-500" />
          </label>
          <p class="mt-1 text-[11px] th-pg-muted">{{ t('playground.customBodyHint') }}</p>
          <textarea
            v-if="customBody"
            v-model="customBodyJson"
            rows="8"
            class="mt-2 w-full rounded-lg border border-slate-700 bg-slate-950 px-2 py-1.5 font-mono text-[11px] text-slate-200"
          />
        </div>

        <div>
          <label class="text-xs text-slate-500">{{ t('playground.apiKey') }}</label>
          <div class="mt-1 flex gap-2">
            <input
              v-model="apiKey"
              type="password"
              autocomplete="off"
              :placeholder="t('playground.apiKeyPh')"
              class="min-w-0 flex-1 rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs"
            />
            <button type="button" class="th-pg-btn-ghost shrink-0 px-2 py-1" @click="pasteKey">
              {{ t('playground.paste') }}
            </button>
          </div>
          <RouterLink to="/console/keys" class="mt-1 inline-block text-[11px] text-cyan-400 hover:underline">
            {{ t('playground.goKeys') }}
          </RouterLink>
        </div>

        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="th-btn-primary-flat !px-3 !py-1.5 !text-xs"
            :disabled="modelsLoading"
            @click="refreshModels"
          >
            <span class="inline-flex items-center gap-2">
              <ThLoadingSpinner v-if="modelsLoadingUi" size="sm" />
              {{ modelsLoadingUi ? t('common.loading') : t('playground.refreshModels') }}
            </span>
          </button>
          <label v-if="playMode === 'chat'" class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
            <input v-model="useStream" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
            {{ t('playground.stream') }}
          </label>
          <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
            <input v-model="showDebug" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
            {{ t('playground.showDebug') }}
          </label>
        </div>
        <p v-if="showDebug" class="text-[11px] th-pg-muted">{{ t('playground.showDebugHint') }}</p>
        <p v-if="modelsErr" class="text-xs text-red-400">{{ modelsErr }}</p>

        <template v-if="!customBody">
          <div>
            <label class="text-xs text-slate-500">{{ t('playground.model') }}</label>
            <p class="mt-0.5 text-[10px] leading-snug th-pg-muted">{{ t('playground.routerHealthHint') }}</p>
            <p v-if="playgroundRegistryMerged" class="mt-0.5 text-[10px] leading-snug th-pg-muted">
              {{ t('playground.registryHint') }}
            </p>
            <p v-if="hasCatalogOnlyPlaygroundModels" class="mt-0.5 text-[10px] leading-snug text-amber-400/90">
              {{ t('playground.catalogOnlyListHint') }}
            </p>
            <div ref="modelPickerWrap" class="th-pg-model-picker mt-1">
              <button
                id="pg-model-picker-trigger"
                type="button"
                class="th-pg-model-picker__trigger"
                :disabled="!models.length"
                :aria-expanded="modelPickerOpen"
                aria-haspopup="listbox"
                aria-controls="pg-model-picker-listbox"
                :title="selectedModelDisplayId"
                @click="toggleModelPicker"
              >
                <span class="th-pg-model-picker__value">{{ selectedModelDisplayId }}</span>
                <div v-if="showSelectedModelMeta" class="th-pg-model-picker__meta-row">
                  <span
                    v-for="lab in selectedModelRow?.labels ?? []"
                    :key="'pg-sel-k-' + lab"
                    class="th-pg-model-picker__meta-pill th-pg-model-picker__kind-pill"
                    :title="t('modelsPage.labelsHint')"
                  >
                    {{ pgModelKindLabel(lab) }}
                  </span>
                  <span
                    v-for="cell in selectedModelRow?.serviceStatus ? [pgServiceStatusCell(selectedModelRow.serviceStatus)] : []"
                    :key="'pg-sel-ss'"
                    class="th-pg-model-picker__meta-pill"
                    :class="cell.pillClass"
                    :title="cell.title"
                  >
                    <span class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20" :class="cell.dotClass" aria-hidden="true" />
                    <span class="min-w-0 truncate">{{ cell.text }}</span>
                  </span>
                  <span
                    v-for="cell in selectedModelRow?.routerHealth ? [pgRouterHealthCell(selectedModelRow.routerHealth)] : []"
                    :key="'pg-sel-rh'"
                    class="th-pg-model-picker__meta-pill"
                    :class="cell.pillClass"
                    :title="cell.title"
                  >
                    <span
                      class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20"
                      :class="cell.dotClass"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 truncate">{{ cell.text }}</span>
                  </span>
                  <span
                    v-for="cell in selectedModelRow && playgroundRegistryIds?.has(selectedModelRow.id) ? [pgRegistryCell(selectedModelRow)] : []"
                    :key="'pg-sel-reg'"
                    class="th-pg-model-picker__meta-pill"
                    :class="cell.pillClass"
                    :title="cell.title"
                  >
                    <span
                      class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20"
                      :class="cell.dotClass"
                      aria-hidden="true"
                    />
                    <span class="min-w-0 truncate">{{ cell.text }}</span>
                  </span>
                </div>
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                  class="th-pg-model-picker__chev"
                  :class="{ 'th-pg-model-picker__chev--open': modelPickerOpen }"
                  aria-hidden="true"
                >
                  <path
                    fill-rule="evenodd"
                    d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.94a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
                    clip-rule="evenodd"
                  />
                </svg>
              </button>
            </div>
            <Teleport to="body">
              <div
                v-show="modelPickerOpen"
                ref="modelPickerPanel"
                class="th-pg-model-picker__panel"
                role="region"
                :aria-label="t('playground.modelPickerRegion')"
                :style="modelPickerPanelStyle"
              >
                <div class="th-pg-model-picker__search-wrap">
                  <input
                    ref="modelPickerSearchInput"
                    v-model="modelSearchQuery"
                    type="search"
                    autocomplete="off"
                    autocorrect="off"
                    spellcheck="false"
                    class="th-pg-model-picker__search"
                    :placeholder="t('playground.modelSearchPh')"
                    aria-controls="pg-model-picker-listbox"
                  />
                </div>
                <div
                  id="pg-model-picker-listbox"
                  class="th-pg-model-picker__options thin-scrollbar"
                  role="listbox"
                  aria-labelledby="pg-model-picker-trigger"
                >
                  <button
                    v-for="m in filteredPlaygroundModels"
                    :key="m.id"
                    type="button"
                    role="option"
                    :aria-selected="model === m.id"
                    :aria-disabled="!isPlaygroundModelCallable(m)"
                    :disabled="!isPlaygroundModelCallable(m)"
                    class="th-pg-model-picker__option"
                    :class="{
                      'th-pg-model-picker__option--active': model === m.id,
                      'th-pg-model-picker__option--disabled': !isPlaygroundModelCallable(m),
                    }"
                    :title="!isPlaygroundModelCallable(m) ? t('playground.catalogOnlyOptionTitle') : undefined"
                    @click="pickModel(m.id)"
                  >
                    <span class="th-pg-model-picker__opt-id">{{ m.id }}</span>
                    <div class="th-pg-model-picker__opt-aside">
                      <span
                        v-if="!isPlaygroundModelCallable(m)"
                        class="th-pg-model-picker__meta-pill border border-amber-500/35 bg-amber-500/10 text-amber-200/90"
                      >
                        {{ t('playground.catalogOnlyBadge') }}
                      </span>
                      <span
                        v-for="lab in m.labels ?? []"
                        :key="`${m.id}-k-${lab}`"
                        class="th-pg-model-picker__meta-pill th-pg-model-picker__kind-pill"
                        :title="t('modelsPage.labelsHint')"
                      >
                        {{ pgModelKindLabel(lab) }}
                      </span>
                      <span
                        v-for="cell in m.serviceStatus ? [pgServiceStatusCell(m.serviceStatus)] : []"
                        :key="'pg-ss-' + m.id"
                        class="th-pg-model-picker__meta-pill"
                        :class="cell.pillClass"
                        :title="cell.title"
                      >
                        <span class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20" :class="cell.dotClass" aria-hidden="true" />
                        <span class="min-w-0 truncate">{{ cell.text }}</span>
                      </span>
                      <span
                        v-for="cell in m.routerHealth ? [pgRouterHealthCell(m.routerHealth)] : []"
                        :key="`${m.id}-rh`"
                        class="th-pg-model-picker__meta-pill"
                        :class="cell.pillClass"
                        :title="cell.title"
                      >
                        <span
                          class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20"
                          :class="cell.dotClass"
                          aria-hidden="true"
                        />
                        <span class="min-w-0 truncate">{{ cell.text }}</span>
                      </span>
                      <span
                        v-for="cell in playgroundRegistryIds?.has(m.id) ? [pgRegistryCell(m)] : []"
                        :key="`${m.id}-reg`"
                        class="th-pg-model-picker__meta-pill"
                        :class="cell.pillClass"
                        :title="cell.title"
                      >
                        <span
                          class="h-1.5 w-1.5 shrink-0 rounded-full ring-1 ring-black/20"
                          :class="cell.dotClass"
                          aria-hidden="true"
                        />
                        <span class="min-w-0 truncate">{{ cell.text }}</span>
                      </span>
                    </div>
                  </button>
                  <p
                    v-if="!filteredPlaygroundModels.length"
                    class="th-pg-model-picker__empty"
                  >
                    {{ t('playground.modelSearchEmpty') }}
                  </p>
                </div>
              </div>
            </Teleport>
            <p v-if="showCatalogOnlyModelHint" class="mt-1.5 text-[11px] text-amber-400/95">
              {{ t('playground.catalogOnlySelectedHint') }}
            </p>
            <p v-else-if="showVideoModelHint" class="mt-1.5 text-[11px] text-amber-400/95">{{ t('playground.videoModelHint') }}</p>
          </div>

          <div v-if="playMode === 'image'" class="th-pg-card space-y-3 p-3">
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.imageGenSize') }}</label>
              <select
                v-model="imageGenSize"
                class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
              >
                <option v-for="s in pgImageSizeOptions" :key="s" :value="s">{{ s }}</option>
              </select>
            </div>
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.imageResponseFormat') }}</label>
              <select
                v-model="imageResponseFormat"
                class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
              >
                <option value="url">{{ t('playground.imageFormatUrl') }}</option>
                <option value="b64_json">{{ t('playground.imageFormatB64') }}</option>
              </select>
            </div>
            <p v-if="pgMediaParamHint" class="text-[11px] leading-relaxed text-sky-400/90">{{ pgMediaParamHint }}</p>
          </div>

          <div v-if="playMode === 'video'" class="th-pg-card space-y-3 p-3">
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.videoSeconds') }}</label>
              <input
                v-model="videoSeconds"
                type="text"
                class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
                placeholder="8"
              />
            </div>
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.videoSize') }}</label>
              <select
                v-model="videoSize"
                class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
              >
                <option v-for="s in pgVideoSizeOptions" :key="s" :value="s">{{ s }}</option>
              </select>
            </div>
            <p v-if="pgMediaParamHint" class="text-[11px] leading-relaxed text-sky-400/90">{{ pgMediaParamHint }}</p>
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.videoRefImage') }}</label>
              <input
                ref="videoFileInputRef"
                type="file"
                class="sr-only"
                accept="image/jpeg,image/png,image/webp"
                :aria-label="t('playground.videoRefImage')"
                @change="onPlaygroundVideoRefFile"
              />
              <div class="mt-1 flex flex-wrap items-center gap-2">
                <button type="button" class="th-pg-btn-ghost px-2 py-1 text-xs" @click="openPlaygroundVideoRefPicker">
                  {{ t('playground.videoRefPick') }}
                </button>
                <button
                  v-if="videoRefFile"
                  type="button"
                  class="th-pg-btn-ghost px-2 py-1 text-xs"
                  @click="clearVideoRefPreview"
                >
                  {{ t('playground.videoRefClear') }}
                </button>
              </div>
              <img
                v-if="videoRefPreviewUrl"
                :src="videoRefPreviewUrl"
                alt=""
                class="mt-2 max-h-28 max-w-full rounded-lg border border-white/10 object-contain"
              />
              <p v-if="videoUploadErr" class="mt-1 text-[11px] text-rose-400/95">{{ videoUploadErr }}</p>
              <p class="mt-1 text-[11px] th-pg-muted">
                {{ t('playground.videoRefHint', { maxMb: Math.floor(PG_VIDEO_MAX_BYTES / 1024 / 1024) }) }}
              </p>
            </div>
          </div>

          <div v-if="playMode === 'audio'" class="th-pg-card space-y-3 p-3">
            <template v-if="audioPlaySub === 'speech'">
              <div>
                <label class="text-xs text-slate-500">{{ t('playground.audioVoice') }}</label>
                <input
                  v-model="audioSpeechVoice"
                  type="text"
                  class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
                  :placeholder="t('playground.audioVoicePh')"
                />
              </div>
              <div>
                <label class="text-xs text-slate-500">{{ t('playground.audioSpeed') }}</label>
                <input
                  v-model.number="audioSpeechSpeed"
                  type="number"
                  min="0.25"
                  max="4"
                  step="0.05"
                  class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
                />
                <p class="mt-1 text-[11px] th-pg-muted">{{ t('playground.audioSpeedHint') }}</p>
              </div>
            </template>
            <template v-else>
              <div>
                <label class="text-xs text-slate-500">{{ t('playground.audioLanguage') }}</label>
                <input
                  v-model="sttLanguage"
                  type="text"
                  class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
                  :placeholder="t('playground.audioLanguagePh')"
                />
                <p class="mt-1 text-[11px] th-pg-muted">{{ t('playground.audioLanguageHint') }}</p>
              </div>
              <input
                ref="audioFileInputRef"
                type="file"
                class="sr-only"
                accept="audio/*,.mp3,.wav,.m4a,.webm,.flac,.ogg,.mpeg"
                :aria-label="t('playground.audioUpload')"
                @change="onPlaygroundAudioFile"
              />
              <div class="flex flex-wrap items-center gap-2">
                <button type="button" class="th-pg-btn-ghost px-2 py-1 text-xs" @click="openPlaygroundAudioPicker">
                  {{ t('playground.audioPickFile') }}
                </button>
                <span v-if="audioFile" class="truncate font-mono text-[11px] text-slate-300">{{ audioFile.name }}</span>
              </div>
              <p v-if="audioUploadErr" class="text-[11px] text-rose-400/95">{{ audioUploadErr }}</p>
              <p class="text-[11px] th-pg-muted">{{ t('playground.audioFileHint', { maxMb: Math.floor(PG_AUDIO_MAX_BYTES / 1024 / 1024) }) }}</p>
            </template>
          </div>

          <template v-if="playMode === 'chat'">
          <div>
            <label class="text-xs text-slate-500">{{ t('playground.system') }}</label>
            <textarea
              v-model="systemPrompt"
              rows="3"
              class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 text-xs"
              :placeholder="t('playground.systemPh')"
            />
          </div>

          <div class="th-pg-card p-3">
            <label class="flex cursor-pointer items-center justify-between text-sm th-pg-muted">
              <span>{{ t('playground.imageUrl') }}</span>
              <input v-model="useImage" type="checkbox" class="h-4 w-4 rounded border-slate-600 text-cyan-500" />
            </label>
            <template v-if="useImage">
              <input
                ref="imageFileInputRef"
                type="file"
                class="sr-only"
                accept="image/jpeg,image/png,image/gif,image/webp"
                :aria-label="t('playground.imageUpload')"
                @change="onPlaygroundImageFile"
              />
              <div class="mt-2 flex flex-wrap items-center gap-2">
                <button type="button" class="th-pg-btn-ghost px-2 py-1 text-xs" @click="openPlaygroundImagePicker">
                  {{ t('playground.imageUpload') }}
                </button>
                <span class="text-[11px] th-pg-muted">{{ t('playground.imageOrUrl') }}</span>
              </div>
              <input
                v-model="imageUrl"
                type="text"
                class="mt-2 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 text-xs"
                :placeholder="t('playground.imagePh')"
              />
              <p v-if="imageUploadErr" class="mt-1 text-[11px] text-rose-400/95">{{ imageUploadErr }}</p>
            </template>
            <p class="mt-1 text-[11px] th-pg-muted">{{
              t('playground.imageHint', { maxMb: Math.floor(PG_IMAGE_MAX_BYTES / 1024 / 1024) })
            }}</p>
          </div>

          <div class="th-pg-card space-y-3 p-3">
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
                  <input v-model="useTemp" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.temp') }}</span>
                </label>
                <span class="th-pg-accent-num">{{ fmtNum(temperature) }}</span>
              </div>
              <input
                v-model.number="temperature"
                type="range"
                min="0"
                max="2"
                step="0.05"
                :disabled="!useTemp"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
                  <input v-model="useTopP" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.topP') }}</span>
                </label>
                <span class="th-pg-accent-num">{{ fmtNum(topP) }}</span>
              </div>
              <input
                v-model.number="topP"
                type="range"
                min="0"
                max="1"
                step="0.01"
                :disabled="!useTopP"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
                  <input v-model="useFreqPen" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.freqPen') }}</span>
                </label>
                <span class="th-pg-accent-num">{{ fmtNum(freqPen) }}</span>
              </div>
              <input
                v-model.number="freqPen"
                type="range"
                min="-2"
                max="2"
                step="0.1"
                :disabled="!useFreqPen"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
            <div class="space-y-1">
              <div class="flex items-center justify-between gap-2">
                <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
                  <input v-model="usePresPen" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
                  <span>{{ t('playground.presPen') }}</span>
                </label>
                <span class="th-pg-accent-num">{{ fmtNum(presPen) }}</span>
              </div>
              <input
                v-model.number="presPen"
                type="range"
                min="-2"
                max="2"
                step="0.1"
                :disabled="!usePresPen"
                class="w-full accent-cyan-500 disabled:opacity-40"
              />
            </div>
          </div>

          <div class="th-pg-card flex flex-wrap items-center gap-3 px-3 py-2">
            <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
              <input v-model="useMaxTokens" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
              {{ t('playground.maxTokens') }}
            </label>
            <input
              v-model.number="maxTokens"
              type="number"
              min="1"
              class="w-24 rounded border border-slate-600 bg-slate-950 px-2 py-1 font-mono text-xs"
            />
          </div>

          <div class="th-pg-card flex flex-wrap items-center gap-3 px-3 py-2">
            <label class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
              <input v-model="useSeed" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
              {{ t('playground.seed') }}
            </label>
            <input v-model.number="seed" type="number" class="w-28 rounded border border-slate-600 bg-slate-950 px-2 py-1 font-mono text-xs" />
          </div>
          </template>
        </template>

        <div v-else class="rounded-xl border border-amber-500/20 bg-amber-950/20 p-3 text-xs text-amber-200/90">
          {{ t('playground.customBodyWarn') }}
        </div>

        <div class="th-pg-divider-top flex flex-wrap gap-2 pt-3">
          <button type="button" class="th-btn-primary-flat !px-3 !py-1.5 !text-xs" @click="exportSettings">
            {{ t('playground.export') }}
          </button>
          <label class="th-pg-btn-ghost inline-flex cursor-pointer items-center px-3 py-1.5">
            {{ t('playground.import') }}
            <input type="file" accept="application/json" class="hidden" @change="importSettings" />
          </label>
        </div>
      </div>
    </section>

    <section
      class="th-pg-main flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-2xl lg:min-h-0 lg:rounded-l-none"
    >
      <header class="th-pg-chat-head flex shrink-0 flex-wrap items-center justify-between gap-3 px-4 py-3">
        <div>
          <p class="text-sm font-semibold text-slate-100">
            {{ chatHeaderTitle }}
          </p>
          <p class="text-[11px] text-slate-500">
            {{ chatHeaderSubtitle }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="th-pg-btn-ghost px-3 py-1.5 text-xs font-medium"
            :title="t('playground.newSessionHint')"
            @click="startNewPlaygroundSession"
          >
            {{ t('playground.newSession') }}
          </button>
          <div
            class="th-pg-seg"
            role="group"
            :aria-label="t('playground.renderModeGroup')"
          >
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': !assistantMarkdown }"
              :aria-pressed="!assistantMarkdown"
              @click="assistantMarkdown = false"
            >
              {{ t('playground.renderPlain') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn"
              :class="{ 'th-pg-seg__btn--active': assistantMarkdown }"
              :aria-pressed="assistantMarkdown"
              @click="assistantMarkdown = true"
            >
              {{ t('playground.renderMd') }}
            </button>
          </div>
          <button
            v-if="showDebug && !debugPanelCollapsed"
            type="button"
            class="th-pg-btn-ghost px-3 py-1.5"
            @click="debugPanelCollapsed = true"
          >
            {{ t('playground.hideDebug') }}
          </button>
          <button
            v-if="showDebug && debugPanelCollapsed"
            type="button"
            class="th-pg-btn-ghost px-3 py-1.5"
            @click="debugPanelCollapsed = false"
          >
            {{ t('playground.showDebug') }}
          </button>
          <button type="button" class="th-pg-btn-ghost px-3 py-1.5" @click="exportConversation">
            {{ t('playground.exportChat') }}
          </button>
        </div>
      </header>

      <div v-if="showDebug && !debugPanelCollapsed" class="th-pg-debug shrink-0 px-4 py-2">
        <p class="text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugReq') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastRequest || '—' }}</pre>
        <p class="mt-2 text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugRes') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastResponse || streamLog.slice(0, 4000) || '—' }}</pre>
      </div>

      <div
        ref="chatScrollEl"
        class="th-pg-classic-messages thin-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-y-contain px-4 py-4"
      >
        <div class="flex flex-col gap-3">
        <div v-if="!messages.length" class="flex flex-1 flex-col items-center justify-center py-16 text-center text-sm text-slate-500">
          <p>{{ emptyPanelHint }}</p>
        </div>
        <div
          v-for="m in messages"
          :key="m.id"
          class="flex"
          :class="m.role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <div
            class="flex flex-col gap-1"
            :class="m.role === 'user' ? 'items-end max-w-[min(100%,44rem)]' : 'items-start max-w-[min(100%,64rem)]'"
          >
            <div
              class="th-pg-msg flex min-w-0 flex-col gap-0 rounded-2xl border px-3.5 py-2 text-sm"
              :class="m.role === 'user' ? 'th-pg-msg--user' : 'th-pg-msg--assistant'"
            >
              <div v-if="m.role === 'assistant' && m.videoStatus" class="mb-1.5">
                <span
                  class="inline-flex items-center rounded-full border border-cyan-500/35 bg-cyan-500/10 px-2 py-0.5 text-[10px] font-medium text-cyan-200/95"
                >
                  {{ videoStatusLabel(m.videoStatus) }}
                  <span v-if="m.videoJobId" class="ml-1 font-mono opacity-70">· {{ m.videoJobId.slice(0, 12) }}…</span>
                </span>
              </div>
              <div
                v-if="m.role === 'assistant' && isVideoRestorePending(m)"
                class="mb-2 w-full max-w-md rounded-lg border border-white/10 bg-slate-950/50 px-3 py-2.5 text-xs"
              >
                <p v-if="videoRestoringIds.has(m.id)" class="inline-flex items-center gap-2 th-pg-muted">
                  <ThLoadingSpinner size="sm" />
                  {{ t('playground.videoRestoring') }}
                </p>
                <template v-else>
                  <p class="th-pg-muted leading-relaxed">{{ t('playground.videoReloadHint') }}</p>
                  <button
                    type="button"
                    class="th-pg-btn-ghost mt-2 px-2 py-1 text-xs"
                    :disabled="!apiKey.trim() || !(m.videoModel || model).trim()"
                    @click="fetchVideoBlobForMessage(m)"
                  >
                    {{ t('playground.videoReload') }}
                  </button>
                </template>
              </div>
              <div v-else-if="m.role === 'assistant' && m.videoUrl" class="mb-2 w-full max-w-md">
                <video :src="m.videoUrl" controls class="max-w-full rounded-lg border border-white/10 bg-slate-950/80" />
                <PlaygroundMediaDownloadBtn
                  :src="m.videoUrl"
                  kind="video"
                  variant="classic"
                  :filename-base="m.videoJobId || m.id"
                />
              </div>
              <div v-if="m.role === 'assistant' && m.audioUrl" class="mb-2 w-full max-w-md">
                <audio :src="m.audioUrl" controls class="w-full rounded-lg border border-white/10 bg-slate-950/80" />
              </div>
              <div v-if="m.role === 'assistant' && m.imageOutputs?.length" class="mb-2 flex flex-col gap-2">
                <template v-for="(src, ix) in m.imageOutputs" :key="`${m.id}-img-${ix}`">
                  <template v-if="!imageOutputFailed(m, ix)">
                    <img
                      :src="src"
                      alt=""
                      class="max-h-72 max-w-full rounded-lg border border-white/10 object-contain"
                      referrerpolicy="no-referrer"
                      @error="markImageLoadFailed(m.id, ix)"
                    />
                    <PlaygroundMediaDownloadBtn
                      :src="src"
                      kind="image"
                      variant="classic"
                      :filename-base="`${m.id}-${ix + 1}`"
                    />
                  </template>
                  <p
                    v-else
                    class="rounded-lg border border-amber-500/25 bg-amber-950/30 px-2.5 py-2 text-xs leading-relaxed text-amber-100/90"
                  >
                    {{ t('playground.imageLoadFailed') }}
                  </p>
                </template>
              </div>
              <div v-if="m.imageUrl || m.videoRefUrl" class="mb-2 shrink-0">
                <img
                  :src="m.imageUrl || m.videoRefUrl"
                  alt=""
                  class="max-h-40 max-w-full rounded-lg border border-white/10 object-contain"
                  @error="(($event.target as HTMLImageElement).style.display = 'none')"
                />
              </div>
              <div v-if="editingId === m.id" class="min-w-0 space-y-2">
                <textarea v-model="editDraft" rows="4" class="w-full rounded border border-slate-600 bg-slate-950 px-2 py-1 text-xs" />
                <div class="flex gap-2">
                  <button type="button" class="text-xs text-cyan-400 hover:underline" @click="saveEdit">{{ t('common.save') }}</button>
                  <button type="button" class="text-xs text-slate-500 hover:underline" @click="cancelEdit">{{ t('common.cancel') }}</button>
                </div>
              </div>
              <div
                v-else-if="m.role === 'assistant' && assistantMarkdown"
                class="playground-bubble-md min-w-0 break-words leading-normal"
                v-html="renderAssistantMarkdown(playgroundBubbleText(m))"
              />
              <div v-else class="min-w-0 whitespace-pre-wrap break-words leading-normal">{{ playgroundBubbleText(m) }}</div>
            </div>
            <div
              v-if="editingId !== m.id"
              class="flex flex-wrap items-center gap-0.5"
              :class="m.role === 'user' ? 'justify-end' : 'justify-start'"
              role="toolbar"
              :aria-label="t('playground.msgActionsAria')"
            >
              <button
                type="button"
                class="th-pg-icon-btn"
                :title="t('playground.copy')"
                :aria-label="t('playground.copy')"
                @click="copyText(m.content)"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M15.666 3.888A2.25 2.25 0 0 0 13.5 2.25h-3c-1.03 0-1.9.693-2.166 1.638m7.332 0c.055.194.084.4.084.612v0a.75.75 0 0 1-.75.75H9a.75.75 0 0 1-.75-.75v0c0-.212.03-.418.084-.612m7.332 0c.646.049 1.288.11 1.927.184 1.1.128 1.907 1.077 1.907 2.185V19.5a2.25 2.25 0 0 1-2.25 2.25H6.75A2.25 2.25 0 0 1 4.5 19.5V6.257c0-1.108.806-2.057 1.907-2.185a48.208 48.208 0 0 1 1.927-.184"
                  />
                  <path stroke-linecap="round" stroke-linejoin="round" d="M16.5 2.25H18a2.25 2.25 0 0 1 2.25 2.25V18A2.25 2.25 0 0 1 18 20.25H7.5a2.25 2.25 0 0 1-2.25-2.25V7.875a2.25 2.25 0 0 1 2.25-2.25H9" />
                </svg>
              </button>
              <button
                type="button"
                class="th-pg-icon-btn"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                @click="startEdit(m)"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10"
                  />
                </svg>
              </button>
              <button
                type="button"
                class="th-pg-icon-btn th-pg-icon-btn--del"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                @click="removeMessage(m.id)"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0"
                  />
                </svg>
              </button>
              <button
                v-if="
                  m.role === 'assistant' &&
                  playMode === 'video' &&
                  m.videoJobId &&
                  !m.videoUrl &&
                  (m.videoStatus === 'failed' || m.videoStatus === 'cancelled')
                "
                type="button"
                class="th-pg-icon-btn"
                :title="t('playground.videoRetry')"
                :aria-label="t('playground.videoRetry')"
                @click="retryVideoMessage(m)"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
                  />
                </svg>
              </button>
              <button
                v-if="m.role === 'assistant' && showRegenerateForCurrentMode"
                type="button"
                class="th-pg-icon-btn th-pg-icon-btn--regen"
                :title="t('playground.regen')"
                :aria-label="t('playground.regen')"
                @click="regenerate"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99"
                  />
                </svg>
              </button>
              <button
                v-if="m.role === 'assistant'"
                type="button"
                class="th-pg-icon-btn th-pg-icon-btn--asuser"
                :title="t('playground.asUser')"
                :aria-label="t('playground.asUser')"
                @click="switchToUser(m.id)"
              >
                <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                </svg>
              </button>
            </div>
          </div>
        </div>
        </div>
      </div>

      <footer class="th-pg-footer shrink-0 p-4">
        <div class="flex gap-2">
          <button
            type="button"
            class="th-pg-btn-ghost th-pg-btn-ghost--icon th-pg-btn-ghost--danger"
            :title="t('playground.newSessionHint')"
            :aria-label="t('playground.newSession')"
            @click="startNewPlaygroundSession"
          >
            <svg class="h-5 w-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"
              />
            </svg>
          </button>
          <textarea
            v-model="input"
            rows="2"
            class="min-h-[3rem] flex-1 resize-y rounded-xl border border-slate-600 bg-slate-950 px-3 py-2 text-sm text-slate-100 placeholder:text-slate-600"
            :placeholder="inputFooterPh"
            @keydown.enter.exact.prevent="sendMessage"
          />
          <div class="flex flex-col gap-1">
            <button
              v-if="sending && useStream && playMode === 'chat'"
              type="button"
              class="th-pg-btn-amber"
              @click="stopStream"
            >
              {{ t('playground.stop') }}
            </button>
            <button
              v-if="sending && playMode === 'video'"
              type="button"
              class="th-pg-btn-amber"
              @click="cancelVideoGeneration"
            >
              {{ t('playground.videoCancel') }}
            </button>
            <button
              type="button"
              :disabled="sendDisabled"
              class="th-pg-btn-primary th-pg-btn-primary--md w-full min-w-[5.5rem] justify-center rounded-xl"
              @click="sendMessage"
            >
              {{ sending ? t('playground.sending') : t('playground.send') }}
            </button>
          </div>
        </div>
      </footer>
    </section>
  </div>
</template>

<style scoped>
.playground-bubble-md :deep(h1),
.playground-bubble-md :deep(h2),
.playground-bubble-md :deep(h3),
.playground-bubble-md :deep(h4) {
  font-weight: 600;
  color: rgb(241 245 249);
  margin: 0.65em 0 0.35em;
  line-height: 1.35;
}
.playground-bubble-md :deep(h1) {
  font-size: 1.125rem;
}
.playground-bubble-md :deep(h2) {
  font-size: 1.05rem;
}
.playground-bubble-md :deep(h3),
.playground-bubble-md :deep(h4) {
  font-size: 1rem;
}
.playground-bubble-md :deep(h1:first-child),
.playground-bubble-md :deep(h2:first-child),
.playground-bubble-md :deep(h3:first-child) {
  margin-top: 0;
}
.playground-bubble-md :deep(p) {
  margin: 0.45em 0;
}
.playground-bubble-md :deep(p:first-child) {
  margin-top: 0;
}
.playground-bubble-md :deep(p:last-child) {
  margin-bottom: 0;
}
.playground-bubble-md :deep(ul),
.playground-bubble-md :deep(ol) {
  margin: 0.4em 0;
  padding-left: 1.5em;
  list-style-position: outside;
}
/* Preflight 对 ul/ol 使用 list-style:none；此处恢复 Markdown 列表符号 */
.playground-bubble-md :deep(ul) {
  list-style-type: disc;
}
.playground-bubble-md :deep(ol) {
  list-style-type: decimal;
}
.playground-bubble-md :deep(ul ul) {
  list-style-type: circle;
}
.playground-bubble-md :deep(li) {
  display: list-item;
  margin: 0.2em 0;
}
.playground-bubble-md :deep(blockquote) {
  margin: 0.5em 0;
  padding-left: 0.85em;
  border-left: 3px solid rgb(100 116 139 / 0.6);
  color: rgb(203 213 225);
}
.playground-bubble-md :deep(hr) {
  margin: 0.75em 0;
  border: 0;
  border-top: 1px solid rgb(51 65 85 / 0.7);
}
.playground-bubble-md :deep(a) {
  color: rgb(34 211 238);
  text-decoration: underline;
  text-underline-offset: 2px;
}
.playground-bubble-md :deep(a:hover) {
  color: rgb(103 232 249);
}
.playground-bubble-md :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.85em;
  padding: 0.1em 0.35em;
  border-radius: 0.25rem;
  background: rgb(15 23 42 / 0.85);
  color: rgb(226 232 240);
}
.playground-bubble-md :deep(pre) {
  margin: 0.5em 0;
  padding: 0.65rem 0.75rem;
  border-radius: 0.5rem;
  overflow-x: auto;
  background: rgb(2 6 23 / 0.65);
  border: 1px solid rgb(51 65 85 / 0.45);
}
.playground-bubble-md :deep(pre code) {
  padding: 0;
  background: transparent;
  font-size: 0.8rem;
  line-height: 1.5;
}
.playground-bubble-md :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
  font-size: 0.8rem;
}
.playground-bubble-md :deep(th),
.playground-bubble-md :deep(td) {
  border: 1px solid rgb(51 65 85 / 0.55);
  padding: 0.35em 0.5em;
  text-align: left;
}
.playground-bubble-md :deep(th) {
  background: rgb(15 23 42 / 0.75);
  font-weight: 600;
}
.playground-bubble-md :deep(.playground-md-fallback) {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.8rem;
}
</style>
