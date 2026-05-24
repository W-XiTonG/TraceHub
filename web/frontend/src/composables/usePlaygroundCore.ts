import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useSmoothLoading } from './useSmoothLoading'
import { useI18n } from 'vue-i18n'
import * as pgw from '../api/playgroundGateway'
import type { PlaygroundModelRow, PlaygroundRouterHealth, PlaygroundServiceStatus } from '../api/playgroundGateway'
import {
  ALL_PLAYGROUND_IMAGE_SIZES,
  ALL_PLAYGROUND_VIDEO_SIZES,
  defaultPlaygroundImageSize,
  defaultPlaygroundVideoSize,
  inferPlaygroundMediaVendor,
  playgroundImageSizes,
  playgroundMediaHintKey,
  playgroundVideoSizes,
} from '../utils/playgroundMediaHints'
import { copyTextToClipboard } from '../utils/clipboard'
import { randomUUID } from '../utils/randomId'
import type { ChatMsg, PersistedVideoJob } from '../types/playground'
import {
  LS_PLAYGROUND_MESSAGES as LS_MSG,
  LS_PLAYGROUND_SETTINGS as LS_KEY,
  PG_AUDIO_MAX_BYTES,
  PG_IMAGE_ACCEPT_TYPES,
  PG_IMAGE_MAX_BYTES,
  PG_VIDEO_MAX_BYTES,
  PG_VIDEO_POLL_MAX,
  PG_VIDEO_POLL_MS,
  SS_PLAYGROUND_APIKEY as SS_KEY,
  SS_PLAYGROUND_VIDEO_JOB,
} from '../types/playground'

marked.use({
  async: false,
  gfm: true,
  breaks: true,
})

const MD_SANITIZE_OPTS: DOMPurify.Config = {
  ADD_TAGS: ['input'],
  ADD_ATTR: ['type', 'checked', 'disabled'],
}

export type UsePlaygroundCoreOptions = {
  /** 新版 UI：简化默认（b64 图片、Markdown 等） */
  modernUi?: boolean
}

export function usePlaygroundCore(options: UsePlaygroundCoreOptions = {}) {
  const modernUi = options.modernUi === true
  const { t } = useI18n()

const apiKey = ref('')
const stickySession = ref('')
const models = ref<PlaygroundModelRow[]>([])
const model = ref('')
const modelsErr = ref('')
const { busy: modelsLoading, visible: modelsLoadingUi, run: runModelsLoad } = useSmoothLoading()
/** 已成功拉取控制台「可用模型」元数据并与列表合并（含空表），用于展示登记说明 */
const playgroundRegistryMerged = ref(false)
/** 控制台返回过元数据的模型 id；仅对这些行展示登记角标 */
const playgroundRegistryIds = ref<Set<string> | null>(null)

const modelPickerOpen = ref(false)
const modelPickerWrap = ref<HTMLElement | null>(null)
const modelPickerPanel = ref<HTMLElement | null>(null)
const modelPickerPanelStyle = ref<Record<string, string>>({})
const modelSearchQuery = ref('')
const modelPickerSearchInput = ref<HTMLInputElement | null>(null)

function closeModelPicker() {
  modelPickerOpen.value = false
  modelSearchQuery.value = ''
}

function updateModelPanelPosition() {
  const wrap = modelPickerWrap.value
  if (!wrap) return
  const rect = wrap.getBoundingClientRect()
  const gap = 6
  const pad = 12
  const vh = window.innerHeight
  const spaceBelow = vh - rect.bottom - gap - pad
  const spaceAbove = rect.top - gap - pad
  const preferBelow = spaceBelow >= 148 || spaceBelow >= spaceAbove
  const maxH = Math.min(288, Math.max(100, preferBelow ? spaceBelow : spaceAbove))
  if (preferBelow) {
    modelPickerPanelStyle.value = {
      position: 'fixed',
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      top: `${rect.bottom + gap}px`,
      maxHeight: `${maxH}px`,
    }
  } else {
    modelPickerPanelStyle.value = {
      position: 'fixed',
      left: `${rect.left}px`,
      width: `${rect.width}px`,
      bottom: `${vh - rect.top + gap}px`,
      maxHeight: `${maxH}px`,
    }
  }
}

function toggleModelPicker() {
  if (!models.value.length) return
  modelPickerOpen.value = !modelPickerOpen.value
}

function pickModel(id: string) {
  const row = models.value.find((r) => r.id === id)
  if (row?.callable === false) return
  model.value = id
  closeModelPicker()
}

function isPlaygroundModelCallable(m: { callable?: boolean }): boolean {
  return m.callable !== false
}

const hasCatalogOnlyPlaygroundModels = computed(() =>
  models.value.some((r) => r.catalogOnly || r.callable === false),
)

const showCatalogOnlyModelHint = computed(() => {
  const row = selectedModelRow.value
  return !!row && row.callable === false
})

function onDocMousedown(ev: MouseEvent) {
  if (!modelPickerOpen.value) return
  const t = ev.target as Node
  const wrap = modelPickerWrap.value
  const panel = modelPickerPanel.value
  if (wrap?.contains(t) || panel?.contains(t)) return
  // 新版 PlaygroundModelSelect 将面板 Teleport 到 body，wrap/panel ref 不在 composable 中
  if (t instanceof Element) {
    if (t.closest('.th-pg-model-picker__panel')) return
    if (t.closest('.th-pg-v2-model-select')) return
  }
  closeModelPicker()
}

function onDocKeydown(ev: KeyboardEvent) {
  if (ev.key === 'Escape' && modelPickerOpen.value) closeModelPicker()
}

function onWinReposition() {
  if (modelPickerOpen.value) updateModelPanelPosition()
}

const selectedModelRow = computed(() => {
  const rows = models.value
  if (!rows.length) return null
  return rows.find((r) => r.id === model.value) ?? rows[0]
})

const selectedModelDisplayId = computed(() => {
  if (!models.value.length) return t('playground.modelEmpty')
  return model.value || models.value[0]?.id || ''
})

const showSelectedModelMeta = computed(() => {
  const row = selectedModelRow.value
  if (!row) return false
  return (
    (row.labels?.length ?? 0) > 0 ||
    !!row.serviceStatus ||
    !!row.routerHealth ||
    !!(playgroundRegistryIds.value && playgroundRegistryIds.value.has(row.id))
  )
})

/** 试玩模型下拉里「路由健康 / 服务登记」共用色板（与可用模型页登记胶囊一致） */
const PG_PILL_UNKNOWN =
  'th-models-upstream-pill th-models-upstream-pill--unknown border border-dashed border-slate-600/55 bg-slate-950/70 text-slate-500 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]'
const PG_PILL_AVAILABLE =
  'th-models-upstream-pill th-models-upstream-pill--available border border-emerald-400/25 bg-gradient-to-br from-emerald-500/[0.22] via-teal-500/10 to-slate-900/40 text-emerald-50 ring-1 ring-inset ring-white/[0.06] shadow-[0_0_28px_-12px_rgba(16,185,129,0.55),inset_0_1px_0_rgba(255,255,255,0.06)]'
const PG_PILL_UNAVAILABLE =
  'th-models-upstream-pill th-models-upstream-pill--unavailable border border-rose-400/25 bg-gradient-to-br from-rose-500/[0.18] via-fuchsia-900/10 to-slate-900/45 text-rose-50 ring-1 ring-inset ring-white/[0.05] shadow-[0_0_26px_-12px_rgba(244,63,94,0.45),inset_0_1px_0_rgba(255,255,255,0.05)]'
const PG_PILL_PARTIAL =
  'th-models-upstream-pill th-models-upstream-pill--partial border border-amber-400/30 bg-gradient-to-br from-amber-500/[0.2] via-violet-600/10 to-slate-900/40 text-amber-50 ring-1 ring-inset ring-white/[0.06] shadow-[0_0_28px_-12px_rgba(245,158,11,0.4),inset_0_1px_0_rgba(255,255,255,0.06)]'
const PG_DOT_AVAILABLE = 'th-models-upstream-pill-dot bg-emerald-400 shadow-[0_0_10px_rgba(52,211,153,0.75)]'
const PG_DOT_UNAVAILABLE = 'th-models-upstream-pill-dot bg-rose-400 shadow-[0_0_10px_rgba(251,113,133,0.55)]'
const PG_DOT_PARTIAL =
  'th-models-upstream-pill-dot bg-gradient-to-br from-amber-300 to-violet-400 shadow-[0_0_10px_rgba(251,191,36,0.5)]'
const PG_DOT_UNKNOWN_DIM = 'th-models-upstream-pill-dot bg-slate-600'
const PG_DOT_UNKNOWN = 'th-models-upstream-pill-dot bg-slate-500'

function pgRegistryCell(m: PlaygroundModelRow): {
  text: string
  title: string
  variant: 'unknown' | 'available' | 'unavailable' | 'partial'
  pillClass: string
  dotClass: string
} {
  if (m.in_gateway === false) {
    return {
      text: t('modelsPage.upstreamUnknown'),
      title: '',
      variant: 'unknown',
      pillClass: PG_PILL_UNKNOWN,
      dotClass: PG_DOT_UNKNOWN_DIM,
    }
  }
  const st = (m.upstream_list_status ?? '').trim()
  const checked = (m.upstream_list_checked_at ?? '').trim()
  const hint = t('modelsPage.thUpstreamListHint')
  const timeTitle = checked ? `${checked}\n${hint}` : hint
  switch (st) {
    case 'listed':
      return {
        text: t('modelsPage.upstreamAvailable'),
        title: timeTitle,
        variant: 'available',
        pillClass: PG_PILL_AVAILABLE,
        dotClass: PG_DOT_AVAILABLE,
      }
    case 'not_listed':
      return {
        text: t('modelsPage.upstreamUnavailable'),
        title: timeTitle,
        variant: 'unavailable',
        pillClass: PG_PILL_UNAVAILABLE,
        dotClass: PG_DOT_UNAVAILABLE,
      }
    case 'mixed':
      return {
        text: t('modelsPage.upstreamPartial'),
        title: timeTitle,
        variant: 'partial',
        pillClass: PG_PILL_PARTIAL,
        dotClass: PG_DOT_PARTIAL,
      }
    default:
      return {
        text: t('modelsPage.upstreamUnknown'),
        title: m.in_gateway === true ? hint : '',
        variant: 'unknown',
        pillClass: PG_PILL_UNKNOWN,
        dotClass: PG_DOT_UNKNOWN,
      }
  }
}

function pgServiceStatusCell(st: PlaygroundServiceStatus): {
  text: string
  title: string
  pillClass: string
  dotClass: string
} {
  const title = t('playground.serviceStatusHint')
  switch (st) {
    case 'operational':
      return {
        text: t('playground.serviceStatus.operational'),
        title,
        pillClass: PG_PILL_AVAILABLE,
        dotClass: PG_DOT_AVAILABLE,
      }
    case 'stale':
      return {
        text: t('playground.serviceStatus.stale'),
        title,
        pillClass: PG_PILL_UNKNOWN,
        dotClass: PG_DOT_UNKNOWN,
      }
    case 'degraded':
      return {
        text: t('playground.serviceStatus.degraded'),
        title,
        pillClass: PG_PILL_PARTIAL,
        dotClass: PG_DOT_PARTIAL,
      }
    case 'catalog_only':
      return {
        text: t('playground.serviceStatus.catalog_only'),
        title,
        pillClass: PG_PILL_UNKNOWN,
        dotClass: PG_DOT_UNKNOWN_DIM,
      }
  }
}

function pgRouterHealthCell(h: PlaygroundRouterHealth): {
  text: string
  title: string
  pillClass: string
  dotClass: string
} {
  const title = t('playground.routerHealthHint')
  switch (h) {
    case 'healthy':
      return {
        text: t('playground.routerHealth.healthy'),
        title,
        pillClass: PG_PILL_AVAILABLE,
        dotClass: PG_DOT_AVAILABLE,
      }
    case 'unhealthy':
      return {
        text: t('playground.routerHealth.unhealthy'),
        title,
        pillClass: PG_PILL_PARTIAL,
        dotClass: PG_DOT_PARTIAL,
      }
    case 'catalog_only':
      return {
        text: t('playground.routerHealth.catalog_only'),
        title,
        pillClass: PG_PILL_UNKNOWN,
        dotClass: PG_DOT_UNKNOWN,
      }
  }
}

function pgModelKindLabel(k: string): string {
  switch (k) {
    case 'chat':
      return t('modelsPage.labelChat')
    case 'embedding':
      return t('modelsPage.labelEmbedding')
    case 'image':
      return t('modelsPage.labelImage')
    case 'video':
      return t('modelsPage.labelVideo')
    case 'audio':
      return t('modelsPage.labelAudio')
    case 'moderation':
      return t('modelsPage.labelModeration')
    case 'rerank':
      return t('modelsPage.labelRerank')
    default:
      return k
  }
}

const filteredPlaygroundModels = computed(() => {
  const rows = models.value
  const raw = modelSearchQuery.value.trim().toLowerCase()
  let list = rows
  if (raw) {
    const parts = raw.split(/\s+/).filter(Boolean)
    const idSet = playgroundRegistryIds.value
    list = rows.filter((r) => {
      const id = r.id.toLowerCase()
      const regText = idSet?.has(r.id) ? pgRegistryCell(r).text.toLowerCase() : ''
      const rhText = r.routerHealth ? pgRouterHealthCell(r.routerHealth).text.toLowerCase() : ''
      const kindBlob = (r.labels ?? []).map((k) => pgModelKindLabel(k).toLowerCase()).join(' ')
      return parts.every(
        (p) => id.includes(p) || regText.includes(p) || rhText.includes(p) || kindBlob.includes(p),
      )
    })
  }
  if (playMode.value === 'video') {
    list = [...list].sort((a, b) => {
      const av = (a.labels ?? []).includes('video') ? 0 : 1
      const bv = (b.labels ?? []).includes('video') ? 0 : 1
      if (av !== bv) return av - bv
      return a.id.localeCompare(b.id)
    })
  }
  return list
})

const showVideoModelHint = computed(() => {
  if (playMode.value !== 'video') return false
  const row = selectedModelRow.value
  if (!row?.id) return false
  return !(row.labels ?? []).includes('video')
})

const pgMediaVendor = computed(() => {
  const row = selectedModelRow.value
  return inferPlaygroundMediaVendor(model.value, row?.labels, row?.ownedBy)
})

const pgImageSizeOptions = computed(() => playgroundImageSizes(pgMediaVendor.value))

const pgVideoSizeOptions = computed(() => playgroundVideoSizes(pgMediaVendor.value))

const pgMediaParamHint = computed(() => {
  const mode = playMode.value
  if (mode !== 'image' && mode !== 'video') return ''
  const key = playgroundMediaHintKey(mode, pgMediaVendor.value, {
    hasVideoRef: !!videoRefFile.value,
    modelId: model.value,
  })
  return key ? t(`playground.${key}`) : ''
})

function syncPlaygroundMediaParamsToModel() {
  const v = pgMediaVendor.value
  if (playMode.value === 'image') {
    const sizes = pgImageSizeOptions.value
    if (!sizes.includes(imageGenSize.value)) {
      imageGenSize.value = defaultPlaygroundImageSize(v)
    }
  } else if (playMode.value === 'video') {
    const sizes = pgVideoSizeOptions.value
    if (!sizes.includes(videoSize.value)) {
      videoSize.value = defaultPlaygroundVideoSize(v)
    }
  }
}

/** 对话补全 / 文生图 / 文生视频 / 音频（OpenAI 兼容 /v1/audio/*） */
const playMode = ref<'chat' | 'image' | 'audio' | 'video'>('chat')
/** 音频试玩子模式 */
const audioPlaySub = ref<'speech' | 'transcribe' | 'translate'>('speech')
const imageGenSize = ref('1024x1024')
const imageResponseFormat = ref<'url' | 'b64_json'>('url')

const systemPrompt = ref('')
const customBody = ref(false)
const customBodyJson = ref('{\n  "model": "gpt-4o-mini",\n  "messages": []\n}')

const useStream = ref(true)
const useImage = ref(false)
const imageUrl = ref('')
/** 本地上传错误提示（与 modelsErr 分离，避免混在模型区） */
const imageUploadErr = ref('')
const imageFileInputRef = ref<HTMLInputElement | null>(null)
const audioFileInputRef = ref<HTMLInputElement | null>(null)

const videoSeconds = ref('8')
const videoSize = ref('1280x720')
const videoRefFile = ref<File | null>(null)
const videoRefPreviewUrl = ref('')
const videoUploadErr = ref('')
const videoFileInputRef = ref<HTMLInputElement | null>(null)
const videoPollAbort = ref<AbortController | null>(null)
const activeVideoJobId = ref('')
const activeVideoAssistantId = ref('')

function persistActiveVideoJob(job: PersistedVideoJob) {
  try {
    sessionStorage.setItem(SS_PLAYGROUND_VIDEO_JOB, JSON.stringify(job))
  } catch {
    /* ignore */
  }
}

function clearPersistedVideoJob() {
  try {
    sessionStorage.removeItem(SS_PLAYGROUND_VIDEO_JOB)
  } catch {
    /* ignore */
  }
}

function readPersistedVideoJob(): PersistedVideoJob | null {
  try {
    const raw = sessionStorage.getItem(SS_PLAYGROUND_VIDEO_JOB)
    if (!raw) return null
    const j = JSON.parse(raw) as PersistedVideoJob
    if (!j.assistantId || !j.videoId || !j.modelName) return null
    return j
  } catch {
    return null
  }
}

async function resumePersistedVideoJob() {
  const job = readPersistedVideoJob()
  if (!job || !apiKey.value.trim()) return
  const msg = messages.value.find((m) => m.id === job.assistantId)
  if (!msg) {
    clearPersistedVideoJob()
    return
  }
  const st = (msg.videoStatus || '').toLowerCase()
  if (st === 'completed' || st === 'failed' || st === 'cancelled' || st === 'canceled') {
    clearPersistedVideoJob()
    return
  }
  activeVideoAssistantId.value = job.assistantId
  activeVideoJobId.value = job.videoId
  stopVideoPoll()
  videoPollAbort.value = new AbortController()
  sending.value = true
  try {
    await pollVideoUntilDone(job.assistantId, job.videoId, job.modelName)
  } finally {
    sending.value = false
    activeVideoJobId.value = ''
    activeVideoAssistantId.value = ''
    stopVideoPoll()
    clearPersistedVideoJob()
  }
}

/** 刷新后按 videoJobId 重新拉取成品时的消息 id */
const videoRestoringIds = ref<Set<string>>(new Set())

/** 语音合成：音色、语速（与上游 OpenAI 风格字段一致） */
const audioSpeechVoice = ref('alloy')
const audioSpeechSpeed = ref(1)
const sttLanguage = ref('')
const audioFile = ref<File | null>(null)
const audioUploadErr = ref('')

const useTemp = ref(true)
const temperature = ref(0.7)
const useTopP = ref(true)
const topP = ref(1)
const useFreqPen = ref(false)
const freqPen = ref(0)
const usePresPen = ref(false)
const presPen = ref(0)
const useMaxTokens = ref(false)
const maxTokens = ref(2048)
const useSeed = ref(false)
const seed = ref(0)

const messages = ref<ChatMsg[]>([])
const input = ref('')
const sending = ref(false)
const abortCtrl = ref<AbortController | null>(null)

const showDebug = ref(false)
/** 助理气泡：开启后以 Markdown 渲染；关闭时与原先一致的纯文本换行 */
const assistantMarkdown = ref(false)
const lastRequest = ref('')
const lastResponse = ref('')
const streamLog = ref('')

const editingId = ref<string | null>(null)
const editDraft = ref('')

function uid() {
  return randomUUID()
}

function parseErr(text: string) {
  try {
    const j = JSON.parse(text) as { error?: { message?: string }; message?: string; code?: number }
    if (j.error?.message) return j.error.message
    if (typeof j.message === 'string' && j.message.trim()) {
      if (typeof j.code === 'number') return `[${j.code}] ${j.message}`
      return j.message
    }
    return text
  } catch {
    return text
  }
}

onMounted(() => {
  stickySession.value = uid()
  try {
    apiKey.value = sessionStorage.getItem(SS_KEY) || ''
  } catch {
    /* ignore */
  }
  loadSettings()
  if (modernUi) applyModernUiDefaults()
  try {
    const raw = localStorage.getItem(LS_MSG)
    if (raw) {
      const arr = JSON.parse(raw) as ChatMsg[]
      if (Array.isArray(arr)) messages.value = arr
    }
  } catch {
    /* ignore */
  }
  if (apiKey.value) {
    void refreshModels().then(() => {
      void restorePersistedVideoMedia()
      void resumePersistedVideoJob()
    })
  } else {
    void restorePersistedVideoMedia()
    void resumePersistedVideoJob()
  }
  document.addEventListener('mousedown', onDocMousedown)
  document.addEventListener('keydown', onDocKeydown)
  document.addEventListener('visibilitychange', onPlaygroundVisibility)
})

function onPlaygroundVisibility() {
  if (document.visibilityState !== 'visible') return
  if (apiKey.value.trim()) void refreshModels()
}

onBeforeUnmount(() => {
  document.removeEventListener('mousedown', onDocMousedown)
  document.removeEventListener('keydown', onDocKeydown)
  document.removeEventListener('visibilitychange', onPlaygroundVisibility)
  window.removeEventListener('resize', onWinReposition)
  window.removeEventListener('scroll', onWinReposition, true)
  cancelVideoGeneration()
  clearVideoRefPreview()
  for (const m of messages.value) revokeMsgMedia(m)
})

let apiKeyRefreshTimer: ReturnType<typeof setTimeout> | null = null
watch(apiKey, (v) => {
  try {
    if (v.trim()) sessionStorage.setItem(SS_KEY, v.trim())
    else sessionStorage.removeItem(SS_KEY)
  } catch {
    /* ignore */
  }
  if (apiKeyRefreshTimer) clearTimeout(apiKeyRefreshTimer)
  apiKeyRefreshTimer = setTimeout(() => {
    apiKeyRefreshTimer = null
    const t = v.trim()
    if (!t) {
      models.value = []
      modelsErr.value = ''
      playgroundRegistryMerged.value = false
      playgroundRegistryIds.value = null
      return
    }
    void refreshModels()
  }, 400)
})

/** 勾选「自定义请求体」时，把当前下拉的 model 写入 JSON，避免模板里仍是旧 model 导致上游 20012 */
watch(customBody, (on, prev) => {
  if (!on) return
  if (prev === true) return
  const picked = model.value.trim()
  if (!picked) return
  try {
    const j = JSON.parse(customBodyJson.value) as Record<string, unknown>
    j.model = picked
    customBodyJson.value = JSON.stringify(j, null, 2)
  } catch {
    /* ignore */
  }
})

watch([model, playMode], () => {
  syncPlaygroundMediaParamsToModel()
})

watch(playMode, (m, prev) => {
  if (m === 'image' || m === 'audio' || m === 'video') {
    customBody.value = false
    useStream.value = false
  }
  if (prev === 'video' && m !== 'video') {
    cancelVideoGeneration()
    clearVideoRefPreview()
  }
  if (m !== 'video') videoUploadErr.value = ''
})

watch(
  () => models.value.length,
  (n) => {
    if (n === 0) closeModelPicker()
  },
)

watch(modelPickerOpen, async (open) => {
  if (open) {
    await nextTick()
    modelPickerSearchInput.value?.focus()
    updateModelPanelPosition()
    window.addEventListener('resize', onWinReposition)
    window.addEventListener('scroll', onWinReposition, true)
  } else {
    window.removeEventListener('resize', onWinReposition)
    window.removeEventListener('scroll', onWinReposition, true)
  }
})

watch(messages, persistConversation, { deep: true })

watch(apiKey, (k) => {
  if (k.trim()) void restorePersistedVideoMedia()
})

watch(useImage, (on) => {
  if (!on) imageUploadErr.value = ''
})

watch(playMode, () => {
  audioUploadErr.value = ''
})

watch(imageUrl, () => {
  imageUploadErr.value = ''
})


function applyModernUiDefaults() {
  if (!localStorage.getItem(LS_KEY)) {
    assistantMarkdown.value = true
    temperature.value = 1
    imageResponseFormat.value = 'b64_json'
  }
}

function loadSettings() {
  try {
    const raw = localStorage.getItem(LS_KEY)
    if (!raw) return
    const j = JSON.parse(raw) as Record<string, unknown>
    if (typeof j.systemPrompt === 'string') systemPrompt.value = j.systemPrompt
    if (typeof j.model === 'string') model.value = j.model
    if (typeof j.temperature === 'number') temperature.value = j.temperature
    if (typeof j.topP === 'number') topP.value = j.topP
    if (typeof j.freqPen === 'number') freqPen.value = j.freqPen
    if (typeof j.presPen === 'number') presPen.value = j.presPen
    if (typeof j.maxTokens === 'number') maxTokens.value = j.maxTokens
    if (typeof j.useTemp === 'boolean') useTemp.value = j.useTemp
    if (typeof j.useTopP === 'boolean') useTopP.value = j.useTopP
    if (typeof j.useFreqPen === 'boolean') useFreqPen.value = j.useFreqPen
    if (typeof j.usePresPen === 'boolean') usePresPen.value = j.usePresPen
    if (typeof j.useMaxTokens === 'boolean') useMaxTokens.value = j.useMaxTokens
    if (typeof j.useStream === 'boolean') useStream.value = j.useStream
    if (typeof j.useImage === 'boolean') useImage.value = j.useImage
    if (j.playMode === 'chat' || j.playMode === 'image' || j.playMode === 'audio' || j.playMode === 'video')
      playMode.value = j.playMode
    if (j.audioPlaySub === 'speech' || j.audioPlaySub === 'transcribe' || j.audioPlaySub === 'translate')
      audioPlaySub.value = j.audioPlaySub
    if (typeof j.audioSpeechVoice === 'string') audioSpeechVoice.value = j.audioSpeechVoice
    if (typeof j.audioSpeechSpeed === 'number' && j.audioSpeechSpeed > 0) audioSpeechSpeed.value = j.audioSpeechSpeed
    if (typeof j.sttLanguage === 'string') sttLanguage.value = j.sttLanguage
    if (typeof j.imageGenSize === 'string' && ALL_PLAYGROUND_IMAGE_SIZES.includes(j.imageGenSize))
      imageGenSize.value = j.imageGenSize
    if (j.imageResponseFormat === 'url' || j.imageResponseFormat === 'b64_json')
      imageResponseFormat.value = j.imageResponseFormat
    if (typeof j.videoSeconds === 'string' && j.videoSeconds.trim()) videoSeconds.value = j.videoSeconds.trim()
    if (typeof j.videoSize === 'string' && ALL_PLAYGROUND_VIDEO_SIZES.includes(j.videoSize))
      videoSize.value = j.videoSize
    if (typeof j.useSeed === 'boolean') useSeed.value = j.useSeed
    if (typeof j.seed === 'number') seed.value = j.seed
    if (typeof j.assistantMarkdown === 'boolean') assistantMarkdown.value = j.assistantMarkdown
    if (modernUi) {
      if (typeof j.showDebugModern === 'boolean') showDebug.value = j.showDebugModern
    } else if (typeof j.showDebugClassic === 'boolean') {
      showDebug.value = j.showDebugClassic
    } else if (typeof j.showDebug === 'boolean') {
      showDebug.value = j.showDebug
    }
  } catch {
    /* ignore */
  }
  syncPlaygroundMediaParamsToModel()
}

function persistSettings() {
  const payload = {
    systemPrompt: systemPrompt.value,
    model: model.value,
    temperature: temperature.value,
    topP: topP.value,
    freqPen: freqPen.value,
    presPen: presPen.value,
    maxTokens: maxTokens.value,
    useTemp: useTemp.value,
    useTopP: useTopP.value,
    useFreqPen: useFreqPen.value,
    usePresPen: usePresPen.value,
    useMaxTokens: useMaxTokens.value,
    useStream: useStream.value,
    useImage: useImage.value,
    playMode: playMode.value,
    audioPlaySub: audioPlaySub.value,
    audioSpeechVoice: audioSpeechVoice.value,
    audioSpeechSpeed: audioSpeechSpeed.value,
    sttLanguage: sttLanguage.value,
    imageGenSize: imageGenSize.value,
    imageResponseFormat: imageResponseFormat.value,
    videoSeconds: videoSeconds.value,
    videoSize: videoSize.value,
    useSeed: useSeed.value,
    seed: seed.value,
    assistantMarkdown: assistantMarkdown.value,
    ...(modernUi ? { showDebugModern: showDebug.value } : { showDebugClassic: showDebug.value }),
  }
  try {
    localStorage.setItem(LS_KEY, JSON.stringify(payload))
  } catch {
    /* ignore */
  }
}

watch(
  [
    systemPrompt,
    model,
    temperature,
    topP,
    freqPen,
    presPen,
    maxTokens,
    useTemp,
    useTopP,
    useFreqPen,
    usePresPen,
    useMaxTokens,
    useStream,
    useImage,
    playMode,
    audioPlaySub,
    audioSpeechVoice,
    audioSpeechSpeed,
    sttLanguage,
    imageGenSize,
    imageResponseFormat,
    videoSeconds,
    videoSize,
    useSeed,
    seed,
    assistantMarkdown,
    showDebug,
  ],
  () => persistSettings(),
  { deep: true },
)

async function refreshModels() {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  modelsErr.value = ''
  await runModelsLoad(async () => {
    const [rows, avail] = await Promise.all([
      pgw.fetchPlaygroundModels(k, stickySession.value),
      pgw.fetchPlaygroundAvailItems(),
    ])
    playgroundRegistryMerged.value = avail !== null
    playgroundRegistryIds.value = avail ? new Set(avail.map((it) => it.id)) : null
    models.value = pgw.buildPlaygroundModelList(rows, avail)
    const callable = models.value.filter((r) => r.callable !== false)
    const pickIds = callable.length ? callable.map((r) => r.id) : models.value.map((r) => r.id)
    if (!model.value && pickIds.length) model.value = pickIds[0]
    if (model.value && !pickIds.includes(model.value) && pickIds.length) model.value = pickIds[0]
    syncPlaygroundMediaParamsToModel()
    void restorePersistedVideoMedia()
  }).catch((e: unknown) => {
    modelsErr.value = e instanceof Error ? e.message : t('playground.modelsFail')
  })
}

async function pasteKey() {
  try {
    const text = await navigator.clipboard.readText()
    if (text.trim()) apiKey.value = text.trim()
  } catch {
    /* ignore */
  }
}

function openPlaygroundImagePicker() {
  imageUploadErr.value = ''
  imageFileInputRef.value?.click()
}

function onPlaygroundImageFile(ev: Event) {
  const el = ev.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  imageUploadErr.value = ''
  if (!file) return
  const extOk = /\.(jpe?g|png|gif|webp)$/i.test(file.name)
  const typeOk = PG_IMAGE_ACCEPT_TYPES.has(file.type) || (file.type === '' && extOk)
  if (!typeOk) {
    imageUploadErr.value = t('playground.imageUploadType')
    return
  }
  if (file.size > PG_IMAGE_MAX_BYTES) {
    imageUploadErr.value = t('playground.imageUploadSize', { maxMb: Math.floor(PG_IMAGE_MAX_BYTES / 1024 / 1024) })
    return
  }
  const reader = new FileReader()
  reader.onload = () => {
    const data = String(reader.result || '')
    if (!data.startsWith('data:image/')) {
      imageUploadErr.value = t('playground.imageUploadReadFail')
      return
    }
    useImage.value = true
    imageUrl.value = data
  }
  reader.onerror = () => {
    imageUploadErr.value = t('playground.imageUploadReadFail')
  }
  reader.readAsDataURL(file)
}

function revokeMsgMedia(m: ChatMsg) {
  if (m.audioUrl?.startsWith('blob:')) URL.revokeObjectURL(m.audioUrl)
  if (m.videoUrl?.startsWith('blob:')) URL.revokeObjectURL(m.videoUrl)
}

function clearVideoRefPreview() {
  if (videoRefPreviewUrl.value.startsWith('blob:')) URL.revokeObjectURL(videoRefPreviewUrl.value)
  videoRefPreviewUrl.value = ''
  videoRefFile.value = null
  if (videoFileInputRef.value) videoFileInputRef.value.value = ''
}

function openPlaygroundVideoRefPicker() {
  videoUploadErr.value = ''
  videoFileInputRef.value?.click()
}

function onPlaygroundVideoRefFile(ev: Event) {
  const el = ev.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  if (!file) return
  if (!PG_IMAGE_ACCEPT_TYPES.has(file.type)) {
    videoUploadErr.value = t('playground.imageUploadType')
    return
  }
  if (file.size > PG_VIDEO_MAX_BYTES) {
    videoUploadErr.value = t('playground.imageUploadSize', {
      maxMb: Math.floor(PG_VIDEO_MAX_BYTES / 1024 / 1024),
    })
    return
  }
  videoUploadErr.value = ''
  clearVideoRefPreview()
  videoRefFile.value = file
  videoRefPreviewUrl.value = URL.createObjectURL(file)
}

function stopVideoPoll() {
  videoPollAbort.value?.abort()
  videoPollAbort.value = null
}

function cancelVideoGeneration() {
  clearPersistedVideoJob()
  stopVideoPoll()
  const jobId = activeVideoJobId.value
  const assistantId = activeVideoAssistantId.value
  const m = model.value.trim()
  const k = apiKey.value.trim()
  if (jobId && m && k) {
    void pgw.videoDelete(k, stickySession.value, m, jobId).catch(() => {})
  }
  if (assistantId) {
    patchAssistantVideo(assistantId, t('playground.videoStatusCancelled'), {
      videoJobId: jobId || undefined,
      videoStatus: 'cancelled',
    })
  }
  activeVideoJobId.value = ''
  activeVideoAssistantId.value = ''
  if (sending.value) sending.value = false
}

function videoStatusLabel(status: string): string {
  const s = status.toLowerCase()
  if (s === 'queued') return t('playground.videoStatusQueued')
  if (s === 'in_progress' || s === 'processing') return t('playground.videoStatusInProgress')
  if (s === 'completed') return t('playground.videoStatusCompleted')
  if (s === 'failed') return t('playground.videoStatusFailed')
  if (s === 'cancelled' || s === 'canceled') return t('playground.videoStatusCancelled')
  return status
}

function formatVideoPollingCaption(snap: pgw.VideoJobSnapshot): string {
  const status = videoStatusLabel(snap.status)
  const progress =
    snap.progress != null && Number.isFinite(snap.progress) ? ` · ${Math.round(snap.progress)}%` : ''
  return t('playground.videoPolling', { status, progress })
}

function patchAssistantVideo(
  id: string,
  content: string,
  extra?: Partial<Pick<ChatMsg, 'videoUrl' | 'videoJobId' | 'videoModel' | 'videoStatus'>>,
) {
  messages.value = messages.value.map((m) => {
    if (m.id !== id) return m
    if (extra?.videoUrl !== undefined && m.videoUrl?.startsWith('blob:') && m.videoUrl !== extra.videoUrl) {
      URL.revokeObjectURL(m.videoUrl)
    }
    return {
      ...m,
      content,
      ...(extra?.videoUrl !== undefined ? { videoUrl: extra.videoUrl } : {}),
      ...(extra?.videoJobId !== undefined ? { videoJobId: extra.videoJobId } : {}),
      ...(extra?.videoModel !== undefined ? { videoModel: extra.videoModel } : {}),
      ...(extra?.videoStatus !== undefined ? { videoStatus: extra.videoStatus } : {}),
    }
  })
}

async function pollVideoUntilDone(assistantId: string, videoId: string, modelName: string) {
  const k = apiKey.value.trim()
  let polls = 0
  while (polls < PG_VIDEO_POLL_MAX) {
    if (videoPollAbort.value?.signal.aborted) return
    await new Promise((r) => setTimeout(r, PG_VIDEO_POLL_MS))
    if (videoPollAbort.value?.signal.aborted) return
    polls++
    try {
      const snap = await pgw.videoRetrieve(
        k,
        stickySession.value,
        modelName,
        videoId,
        videoPollAbort.value?.signal,
      )
      lastResponse.value = snap.raw
      patchAssistantVideo(assistantId, formatVideoPollingCaption(snap), {
        videoJobId: videoId,
        videoModel: modelName,
        videoStatus: snap.status,
      })
      const st = snap.status.toLowerCase()
      if (st === 'completed') {
        clearPersistedVideoJob()
        const { blob } = await pgw.videoContent(
          k,
          stickySession.value,
          modelName,
          videoId,
          videoPollAbort.value?.signal,
        )
        const blobUrl = URL.createObjectURL(blob)
        patchAssistantVideo(assistantId, t('playground.videoCompleted'), {
          videoUrl: blobUrl,
          videoJobId: videoId,
          videoModel: modelName,
          videoStatus: 'completed',
        })
        return
      }
      if (st === 'failed' || st === 'cancelled' || st === 'canceled') {
        clearPersistedVideoJob()
        const err = snap.error?.trim() || t('playground.videoFailed')
        patchAssistantVideo(assistantId, `【${t('playground.error')}】\n\n${err}`, {
          videoJobId: videoId,
          videoModel: modelName,
          videoStatus: snap.status,
        })
        return
      }
    } catch (e: unknown) {
      if (videoPollAbort.value?.signal.aborted) return
      const msg = e instanceof Error ? e.message : String(e)
      patchAssistantVideo(assistantId, `【${t('playground.error')}】\n\n${msg}`, {
        videoJobId: videoId,
        videoModel: modelName,
        videoStatus: 'failed',
      })
      return
    }
  }
  clearPersistedVideoJob()
  patchAssistantVideo(assistantId, `【${t('playground.error')}】\n\n${t('playground.videoPollTimeout')}`, {
    videoJobId: videoId,
    videoModel: modelName,
    videoStatus: 'failed',
  })
}

function isVideoRestorePending(m: ChatMsg): boolean {
  if (m.role !== 'assistant' || !m.videoJobId || m.videoUrl) return false
  const st = (m.videoStatus || '').toLowerCase()
  return st === 'completed'
}

async function fetchVideoBlobForMessage(m: ChatMsg): Promise<boolean> {
  const k = apiKey.value.trim()
  const modelName = (m.videoModel || model.value).trim()
  if (!k || !m.videoJobId || !modelName) return false
  const next = new Set(videoRestoringIds.value)
  next.add(m.id)
  videoRestoringIds.value = next
  try {
    const { blob } = await pgw.videoContent(k, stickySession.value, modelName, m.videoJobId)
    const cap = m.content?.trim() || t('playground.videoCompleted')
    patchAssistantVideo(m.id, cap, {
      videoUrl: URL.createObjectURL(blob),
      videoJobId: m.videoJobId,
      videoModel: modelName,
      videoStatus: 'completed',
    })
    return true
  } catch {
    return false
  } finally {
    const done = new Set(videoRestoringIds.value)
    done.delete(m.id)
    videoRestoringIds.value = done
  }
}

/** 刷新后 blob URL 失效；对已完成且存了 jobId 的记录重新下载成品。 */
async function restorePersistedVideoMedia() {
  const pending = messages.value.filter(isVideoRestorePending)
  if (!pending.length) return
  if (!apiKey.value.trim()) return
  for (const m of pending) {
    await fetchVideoBlobForMessage(m)
  }
}

async function retryVideoMessage(m: ChatMsg) {
  if (!m.videoJobId || !model.value.trim() || !apiKey.value.trim()) return
  const assistantId = m.id
  activeVideoAssistantId.value = assistantId
  activeVideoJobId.value = m.videoJobId
  stopVideoPoll()
  videoPollAbort.value = new AbortController()
  sending.value = true
  modelsErr.value = ''
  patchAssistantVideo(assistantId, formatVideoPollingCaption({ videoId: m.videoJobId, status: 'in_progress', raw: '' }), {
    videoJobId: m.videoJobId,
    videoStatus: 'in_progress',
  })
  try {
    await pollVideoUntilDone(assistantId, m.videoJobId, model.value.trim())
  } finally {
    sending.value = false
    activeVideoJobId.value = ''
    activeVideoAssistantId.value = ''
    stopVideoPoll()
  }
}

async function sendVideoGeneration(prompt: string) {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }

  const modelName = model.value.trim()
  const refSnap = videoRefPreviewUrl.value || undefined
  messages.value = [
    ...messages.value,
    { id: uid(), role: 'user', content: prompt, videoRefUrl: refSnap },
  ]
  input.value = ''

  const assistantId = uid()
  messages.value = [
    ...messages.value,
    { id: assistantId, role: 'assistant', content: '', videoStatus: 'queued' },
  ]
  scrollChatToBottom(true)

  lastRequest.value = ''
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true
  stopVideoPoll()
  videoPollAbort.value = new AbortController()
  activeVideoAssistantId.value = assistantId

  try {
    let snap: pgw.VideoJobSnapshot
    if (videoRefFile.value) {
      const fdOpts = {
        model: modelName,
        prompt,
        seconds: videoSeconds.value.trim() || '8',
        size: videoSize.value,
        file: videoRefFile.value,
      }
      lastRequest.value = `[multipart] model=${modelName} prompt=${prompt} seconds=${fdOpts.seconds} size=${fdOpts.size}`
      snap = await pgw.videoCreateMultipart(k, stickySession.value, fdOpts, videoPollAbort.value.signal)
    } else {
      const body: Record<string, unknown> = {
        model: modelName,
        prompt,
        seconds: videoSeconds.value.trim() || '8',
        size: videoSize.value,
      }
      lastRequest.value = JSON.stringify(body, null, 2)
      snap = await pgw.videoCreate(k, stickySession.value, body, videoPollAbort.value.signal)
    }
    lastResponse.value = snap.raw
    if (!snap.videoId) {
      throw new Error(t('playground.videoFailed'))
    }
    activeVideoJobId.value = snap.videoId
    persistActiveVideoJob({
      assistantId,
      videoId: snap.videoId,
      modelName,
      startedAt: Date.now(),
    })
    patchAssistantVideo(assistantId, t('playground.videoCreated', { id: snap.videoId }), {
      videoJobId: snap.videoId,
      videoModel: modelName,
      videoStatus: snap.status,
    })
    await pollVideoUntilDone(assistantId, snap.videoId, modelName)
  } catch (e: unknown) {
    if (videoPollAbort.value?.signal.aborted) return
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistantVideo(assistantId, `【${t('playground.error')}】\n\n${msg}`, { videoStatus: 'failed' })
  } finally {
    sending.value = false
    activeVideoJobId.value = ''
    activeVideoAssistantId.value = ''
    stopVideoPoll()
  }
}

function openPlaygroundAudioPicker() {
  audioUploadErr.value = ''
  audioFileInputRef.value?.click()
}

function onPlaygroundAudioFile(ev: Event) {
  const el = ev.target as HTMLInputElement
  const file = el.files?.[0]
  el.value = ''
  audioUploadErr.value = ''
  if (!file) {
    audioFile.value = null
    return
  }
  if (file.size > PG_AUDIO_MAX_BYTES) {
    audioUploadErr.value = t('playground.audioUploadSize', { maxMb: Math.floor(PG_AUDIO_MAX_BYTES / 1024 / 1024) })
    audioFile.value = null
    return
  }
  audioFile.value = file
}

function parseTranscriptionJson(raw: string): string {
  try {
    const j = JSON.parse(raw) as { text?: string }
    if (typeof j.text === 'string') return j.text
  } catch {
    /* ignore */
  }
  return raw
}

async function sendAudioSpeech(text: string) {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }

  const body: Record<string, unknown> = {
    model: model.value.trim(),
    input: text,
    voice: audioSpeechVoice.value.trim() || 'alloy',
  }
  const sp = Number(audioSpeechSpeed.value)
  if (Number.isFinite(sp) && sp > 0 && sp !== 1) body.speed = sp

  messages.value = [...messages.value, { id: uid(), role: 'user', content: text }]
  input.value = ''

  const assistantId = uid()
  messages.value = [...messages.value, { id: assistantId, role: 'assistant', content: '' }]
  scrollChatToBottom(true)

  lastRequest.value = JSON.stringify(body, null, 2)
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true

  try {
    const r = await pgw.audioSpeech(k, stickySession.value, body)
    const blobUrl = URL.createObjectURL(r.blob)
    patchAssistantAudio(assistantId, t('playground.audioSpeechDoneCaption'), blobUrl)
    lastResponse.value = `[audio] ${r.contentType}; ${r.blob.size} bytes`
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistantAudio(assistantId, `【${t('playground.error')}】\n\n${msg}`, undefined)
  } finally {
    sending.value = false
  }
}

async function sendAudioMultipart() {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }
  const file = audioFile.value
  if (!file) {
    modelsErr.value = t('playground.audioNeedFile')
    return
  }

  const promptOpt = input.value.trim()
  const userLine = promptOpt
    ? `${t('playground.audioFile')}: ${file.name}\n${t('playground.audioSttPrompt')}: ${promptOpt}`
    : `${t('playground.audioFile')}: ${file.name}`

  messages.value = [...messages.value, { id: uid(), role: 'user', content: userLine }]
  input.value = ''

  const assistantId = uid()
  messages.value = [...messages.value, { id: assistantId, role: 'assistant', content: '' }]
  scrollChatToBottom(true)

  const rel = audioPlaySub.value === 'translate' ? 'audio/translations' : 'audio/transcriptions'
  const fdPreview = {
    model: model.value.trim(),
    file: file.name,
    language: sttLanguage.value.trim() || undefined,
    prompt: promptOpt || undefined,
  }
  lastRequest.value = `[multipart ${rel}]\n${JSON.stringify(fdPreview, null, 2)}`
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true

  try {
    const r = await pgw.audioMultipart(k, stickySession.value, rel, {
      model: model.value.trim(),
      file,
      language: sttLanguage.value.trim() || undefined,
      prompt: promptOpt || undefined,
    })
    lastResponse.value = r.raw
    const text = parseTranscriptionJson(r.raw)
    patchAssistant(assistantId, text)
    audioFile.value = null
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistant(assistantId, `【${t('playground.error')}】\n\n${msg}`)
  } finally {
    sending.value = false
  }
}

function patchAssistantAudio(id: string, content: string, audioUrl?: string) {
  messages.value = messages.value.map((m) => {
    if (m.id !== id) return m
    revokeMsgMedia(m)
    return { ...m, content, audioUrl }
  })
}

function buildOpenAIMessagesForHistory(): Array<Record<string, unknown>> {
  const out: Array<Record<string, unknown>> = []
  if (systemPrompt.value.trim()) {
    out.push({ role: 'system', content: systemPrompt.value.trim() })
  }
  for (const m of messages.value) {
    if (m.role === 'user' && m.imageUrl?.trim()) {
      out.push({
        role: 'user',
        content: [
          { type: 'text', text: m.content },
          { type: 'image_url', image_url: { url: m.imageUrl.trim() } },
        ],
      })
    } else {
      out.push({ role: m.role, content: m.content })
    }
  }
  return out
}

function buildNormalBody(appendUser: { text: string; img?: string }): Record<string, unknown> {
  const history = buildOpenAIMessagesForHistory()
  let userMsg: Record<string, unknown>
  if (appendUser.img?.trim()) {
    userMsg = {
      role: 'user',
      content: [
        { type: 'text', text: appendUser.text },
        { type: 'image_url', image_url: { url: appendUser.img.trim() } },
      ],
    }
  } else {
    userMsg = { role: 'user', content: appendUser.text }
  }
  const body: Record<string, unknown> = {
    model: model.value.trim(),
    messages: [...history, userMsg],
  }
  if (useTemp.value) body.temperature = temperature.value
  if (useTopP.value) body.top_p = topP.value
  if (useFreqPen.value) body.frequency_penalty = freqPen.value
  if (usePresPen.value) body.presence_penalty = presPen.value
  if (useMaxTokens.value && maxTokens.value > 0) body.max_tokens = maxTokens.value
  if (useSeed.value) body.seed = Math.floor(seed.value)
  return body
}

function buildCustomBody(text: string, img?: string): Record<string, unknown> {
  const j = JSON.parse(customBodyJson.value) as Record<string, unknown>
  const mergePickerModel = (o: Record<string, unknown>) => {
    const picked = model.value.trim()
    if (picked) return { ...o, model: picked }
    return o
  }
  const msgs = j.messages
  if (Array.isArray(msgs) && text) {
    if (img?.trim()) {
      return mergePickerModel({
        ...j,
        messages: [
          ...msgs,
          {
            role: 'user',
            content: [
              { type: 'text', text },
              { type: 'image_url', image_url: { url: img.trim() } },
            ],
          },
        ],
      })
    }
    return mergePickerModel({ ...j, messages: [...msgs, { role: 'user', content: text }] })
  }
  return mergePickerModel({ ...j })
}


async function retryImageWithB64() {
  imageResponseFormat.value = 'b64_json'
  const lastUser = [...messages.value].reverse().find((m) => m.role === 'user')
  if (!lastUser?.content.trim()) return
  const before = messages.value.slice(0, messages.value.findIndex((m) => m.id === lastUser.id))
  messages.value = before
  input.value = lastUser.content
  await sendImageGeneration(lastUser.content)
}

async function sendImageGeneration(prompt: string) {
  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }

  const body: Record<string, unknown> = {
    model: model.value.trim(),
    prompt,
    n: 1,
    size: imageGenSize.value,
    response_format: imageResponseFormat.value,
  }

  messages.value = [...messages.value, { id: uid(), role: 'user', content: prompt }]
  input.value = ''

  const assistantId = uid()
  messages.value = [...messages.value, { id: assistantId, role: 'assistant', content: '', imageOutputs: [] }]
  scrollChatToBottom(true)

  lastRequest.value = JSON.stringify(body, null, 2)
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true

  try {
    const r = await pgw.imageGenerations(k, stickySession.value, body)
    lastResponse.value = r.raw
    let cap = t('playground.imageDoneCaption')
    if (!r.imageOutputs.length) cap += `\n\n${t('playground.imageNoUrlInResponse')}`
    patchAssistantImages(assistantId, cap, r.imageOutputs)
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistantImages(assistantId, `【${t('playground.error')}】\n\n${msg}`, [])
  } finally {
    sending.value = false
  }
}

async function sendMessage() {
  if (sending.value) return

  if (playMode.value === 'audio' && audioPlaySub.value === 'speech') {
    const t0 = input.value.trim()
    if (!t0) return
    await sendAudioSpeech(t0)
    return
  }
  if (playMode.value === 'audio' && (audioPlaySub.value === 'transcribe' || audioPlaySub.value === 'translate')) {
    await sendAudioMultipart()
    return
  }

  const text = input.value.trim()
  if (!text) return

  if (playMode.value === 'image') {
    await sendImageGeneration(text)
    return
  }
  if (playMode.value === 'video') {
    await sendVideoGeneration(text)
    return
  }

  const k = apiKey.value.trim()
  if (!k) {
    modelsErr.value = t('playground.needKey')
    return
  }
  if (!customBody.value && !model.value.trim()) {
    modelsErr.value = t('playground.needModel')
    return
  }

  let body: Record<string, unknown>
  try {
    if (customBody.value) {
      body = buildCustomBody(text, useImage.value ? imageUrl.value : undefined)
    } else {
      body = buildNormalBody({
        text,
        img: useImage.value && imageUrl.value.trim() ? imageUrl.value : undefined,
      })
    }
  } catch {
    modelsErr.value = t('playground.badCustomJson')
    return
  }

  const imgSnap = useImage.value && imageUrl.value.trim() ? imageUrl.value.trim() : undefined
  messages.value = [
    ...messages.value,
    { id: uid(), role: 'user', content: text, imageUrl: imgSnap },
  ]
  input.value = ''
  imageUrl.value = ''

  const assistantId = uid()
  messages.value = [...messages.value, { id: assistantId, role: 'assistant', content: '' }]
  scrollChatToBottom(true)

  lastRequest.value = JSON.stringify({ ...body, stream: useStream.value }, null, 2)
  lastResponse.value = ''
  streamLog.value = ''
  modelsErr.value = ''
  sending.value = true

  try {
    if (useStream.value) {
      abortCtrl.value = new AbortController()
      const res = await fetch('/v1/chat/completions', {
        method: 'POST',
        signal: abortCtrl.value.signal,
        headers: {
          Authorization: `Bearer ${k}`,
          'Content-Type': 'application/json',
          'X-TraceHub-Session': stickySession.value,
        },
        body: JSON.stringify({ ...body, stream: true }),
      })
      if (!res.ok) {
        const errText = await res.text()
        throw new Error(parseErr(errText) + pgw.traceHubGatewayErrorDetails(res))
      }
      const streamed = await pgw.consumeOpenAIChatCompletionStream(res, (display) => {
        patchAssistantStream(assistantId, display)
      })
      streamLog.value = streamed.streamLog
      let acc = streamed.content || streamed.reasoning
      if (!acc) {
        const r = await pgw.chatCompletion(k, stickySession.value, body)
        acc = r.content
        lastResponse.value = r.raw
        patchAssistant(assistantId, acc)
      } else {
        lastResponse.value = acc
      }
      if (!acc.trim()) {
        lastResponse.value = streamed.streamLog.slice(0, 8000) || lastResponse.value
        patchAssistant(
          assistantId,
          `【${t('playground.error')}】\n\n${t('playground.emptyStreamReply')}`,
        )
      }
    } else {
      const r = await pgw.chatCompletion(k, stickySession.value, body)
      lastResponse.value = r.raw
      patchAssistant(assistantId, r.content)
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e)
    modelsErr.value = msg
    patchAssistant(assistantId, `【${t('playground.error')}】\n\n${msg}`)
  } finally {
    sending.value = false
    abortCtrl.value = null
    if (customBody.value) {
      try {
        customBodyJson.value = JSON.stringify(body, null, 2)
      } catch {
        /* ignore */
      }
    }
  }
}

function patchAssistant(id: string, content: string) {
  messages.value = messages.value.map((m) => (m.id === id ? { ...m, content } : m))
}

/** 流式回调：每片 chunk 后让 Vue 刷 DOM，避免同批次更新被合并 */
async function patchAssistantStream(id: string, content: string) {
  patchAssistant(id, content)
  await nextTick()
  scrollChatToBottom()
}

function patchAssistantImages(id: string, content: string, imageOutputs: string[]) {
  messages.value = messages.value.map((m) =>
    m.id === id
      ? { ...m, content, imageOutputs: imageOutputs.length ? imageOutputs : undefined, imageLoadFailed: undefined }
      : m,
  )
}

function markImageLoadFailed(msgId: string, index: number) {
  messages.value = messages.value.map((m) => {
    if (m.id !== msgId) return m
    const prev = m.imageLoadFailed ?? []
    if (prev.includes(index)) return m
    return { ...m, imageLoadFailed: [...prev, index] }
  })
}

function imageOutputFailed(m: ChatMsg, index: number): boolean {
  return (m.imageLoadFailed ?? []).includes(index)
}

/** 气泡内展示：去掉前导换行/BOM，避免 whitespace-pre-wrap 在框顶出现大块空白（模型常带 \n 开头） */
function playgroundBubbleText(m: ChatMsg) {
  const raw = m.content.replace(/^\uFEFF/, '').replace(/^[\u200B-\u200D]+/, '').replace(/^(?:\r?\n)+/, '')
  if (m.role === 'assistant' && sending.value && !raw) return '…'
  return raw
}

function escapeHtml(s: string) {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function rewriteAssistantLinks(html: string): string {
  try {
    const doc = new DOMParser().parseFromString(html, 'text/html')
    doc.querySelectorAll('a[href]').forEach((a) => {
      a.setAttribute('target', '_blank')
      a.setAttribute('rel', 'noopener noreferrer')
    })
    return doc.body.innerHTML
  } catch {
    return html
  }
}

/** 助理 Markdown：经 DOMPurify sanitize；非 Markdown 正文多数仍按段落显示 */
function renderAssistantMarkdown(src: string): string {
  if (!src) return ''
  try {
    const dirty = marked.parse(src, { async: false }) as string
    const clean = DOMPurify.sanitize(dirty, MD_SANITIZE_OPTS)
    return rewriteAssistantLinks(clean)
  } catch {
    const safe = escapeHtml(src)
    return DOMPurify.sanitize(`<pre class="playground-md-fallback text-left">${safe}</pre>`)
  }
}

function stopStream() {
  abortCtrl.value?.abort()
}

/** 新建会话：清空右侧消息与本地缓存；自定义 JSON 模式下同时把请求体里的 messages 置空，避免旧多模态历史仍被发出。 */
function startNewPlaygroundSession() {
  cancelVideoGeneration()
  clearVideoRefPreview()
  for (const m of messages.value) revokeMsgMedia(m)
  messages.value = []
  input.value = ''
  lastResponse.value = ''
  streamLog.value = ''
  useImage.value = false
  imageUrl.value = ''
  audioFile.value = null
  audioUploadErr.value = ''
  try {
    localStorage.removeItem(LS_MSG)
  } catch {
    /* ignore */
  }
  if (customBody.value) {
    try {
      const j = JSON.parse(customBodyJson.value) as Record<string, unknown>
      if (Array.isArray(j.messages)) {
        j.messages = []
        customBodyJson.value = JSON.stringify(j, null, 2)
      }
    } catch {
      /* 保留用户 JSON，不强行改写 */
    }
  }
}

function persistConversation() {
  try {
    const safe = messages.value.map((m) => {
      const { audioUrl, videoUrl, ...rest } = m
      if (audioUrl?.startsWith('blob:') || videoUrl?.startsWith('blob:')) return rest
      return m
    })
    localStorage.setItem(LS_MSG, JSON.stringify(safe))
  } catch {
    /* ignore */
  }
}

function copyText(s: string) {
  void copyTextToClipboard(s)
}

function removeMessage(id: string) {
  const target = messages.value.find((m) => m.id === id)
  if (target) revokeMsgMedia(target)
  messages.value = messages.value.filter((m) => m.id !== id)
}

function startEdit(m: ChatMsg) {
  editingId.value = m.id
  editDraft.value = m.content
}

function saveEdit() {
  if (!editingId.value) return
  messages.value = messages.value.map((m) =>
    m.id === editingId.value ? { ...m, content: editDraft.value } : m,
  )
  editingId.value = null
}

function cancelEdit() {
  editingId.value = null
}

function switchToUser(id: string) {
  messages.value = messages.value.map((m) => (m.id === id ? { ...m, role: 'user' } : m))
}

async function regenerate() {
  const lastAi = [...messages.value].map((m, i) => ({ m, i })).filter((x) => x.m.role === 'assistant').pop()
  if (!lastAi) return
  const before = messages.value.slice(0, lastAi.i)
  const lastUser = before[before.length - 1]
  if (!lastUser || lastUser.role !== 'user') return
  messages.value = before.slice(0, -1)
  input.value = lastUser.content
  if (lastUser.imageUrl) {
    useImage.value = true
    imageUrl.value = lastUser.imageUrl
  } else {
    imageUrl.value = ''
  }
  await sendMessage()
}

function exportSettings() {
  const blob = new Blob(
    [
      JSON.stringify(
        {
          version: 1,
          systemPrompt: systemPrompt.value,
          model: model.value,
          params: {
            temperature: useTemp.value ? temperature.value : null,
            top_p: useTopP.value ? topP.value : null,
            frequency_penalty: useFreqPen.value ? freqPen.value : null,
            presence_penalty: usePresPen.value ? presPen.value : null,
            max_tokens: useMaxTokens.value ? maxTokens.value : null,
            seed: useSeed.value ? seed.value : null,
          },
          flags: {
            useStream: useStream.value,
            useImage: useImage.value,
            customBody: customBody.value,
            assistantMarkdown: assistantMarkdown.value,
            playMode: playMode.value,
            audioPlaySub: audioPlaySub.value,
            audioSpeechVoice: audioSpeechVoice.value,
            audioSpeechSpeed: audioSpeechSpeed.value,
            sttLanguage: sttLanguage.value,
            imageGenSize: imageGenSize.value,
            imageResponseFormat: imageResponseFormat.value,
            videoSeconds: videoSeconds.value,
            videoSize: videoSize.value,
          },
        },
        null,
        2,
      ),
    ],
    { type: 'application/json' },
  )
  downloadBlob(blob, 'tracehub-playground-settings.json')
}

function importSettings(ev: Event) {
  const inputEl = ev.target as HTMLInputElement
  const file = inputEl.files?.[0]
  inputEl.value = ''
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    try {
      const j = JSON.parse(String(reader.result)) as {
        systemPrompt?: string
        model?: string
        params?: Record<string, number | null>
        flags?: Record<string, unknown>
      }
      if (typeof j.systemPrompt === 'string') systemPrompt.value = j.systemPrompt
      if (typeof j.model === 'string') model.value = j.model
      const p = j.params || {}
      if (p.temperature != null) {
        useTemp.value = true
        temperature.value = p.temperature
      }
      if (p.top_p != null) {
        useTopP.value = true
        topP.value = p.top_p
      }
      if (p.frequency_penalty != null) {
        useFreqPen.value = true
        freqPen.value = p.frequency_penalty
      }
      if (p.presence_penalty != null) {
        usePresPen.value = true
        presPen.value = p.presence_penalty
      }
      if (p.max_tokens != null) {
        useMaxTokens.value = true
        maxTokens.value = Math.floor(p.max_tokens)
      }
      if (p.seed != null) {
        useSeed.value = true
        seed.value = Math.floor(p.seed)
      }
      const f = j.flags || {}
      if (typeof f.useStream === 'boolean') useStream.value = f.useStream
      if (typeof f.useImage === 'boolean') useImage.value = f.useImage
      if (typeof f.customBody === 'boolean') customBody.value = f.customBody
      if (typeof f.assistantMarkdown === 'boolean') assistantMarkdown.value = f.assistantMarkdown
      if (f.playMode === 'chat' || f.playMode === 'image' || f.playMode === 'audio' || f.playMode === 'video') playMode.value = f.playMode as typeof playMode.value
      if (f.audioPlaySub === 'speech' || f.audioPlaySub === 'transcribe' || f.audioPlaySub === 'translate')
        audioPlaySub.value = f.audioPlaySub
      if (typeof f.audioSpeechVoice === 'string') audioSpeechVoice.value = f.audioSpeechVoice
      if (typeof f.audioSpeechSpeed === 'number' && f.audioSpeechSpeed > 0) audioSpeechSpeed.value = f.audioSpeechSpeed
      if (typeof f.sttLanguage === 'string') sttLanguage.value = f.sttLanguage
      if (typeof f.imageGenSize === 'string' && ALL_PLAYGROUND_IMAGE_SIZES.includes(f.imageGenSize))
        imageGenSize.value = f.imageGenSize
      if (f.imageResponseFormat === 'url' || f.imageResponseFormat === 'b64_json')
        imageResponseFormat.value = f.imageResponseFormat as 'url' | 'b64_json'
      if (typeof f.videoSeconds === 'string' && f.videoSeconds.trim()) videoSeconds.value = f.videoSeconds.trim()
      if (typeof f.videoSize === 'string' && ALL_PLAYGROUND_VIDEO_SIZES.includes(f.videoSize))
        videoSize.value = f.videoSize
      persistSettings()
    } catch {
      modelsErr.value = t('playground.importFail')
    }
  }
  reader.readAsText(file)
}

function exportConversation() {
  const blob = new Blob([JSON.stringify({ version: 1, messages: messages.value }, null, 2)], {
    type: 'application/json',
  })
  downloadBlob(blob, 'tracehub-playground-chat.json')
}

function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

const chatScrollEl = ref<HTMLElement | null>(null)

const SCROLL_BOTTOM_THRESHOLD = 96
const userPinnedToBottom = ref(true)

function isNearBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.clientHeight - el.scrollTop <= SCROLL_BOTTOM_THRESHOLD
}

function onChatScrollEvent() {
  const el = chatScrollEl.value
  if (!el) return
  userPinnedToBottom.value = isNearBottom(el)
}

watch(chatScrollEl, (el, prev) => {
  prev?.removeEventListener('scroll', onChatScrollEvent)
  if (el) {
    el.addEventListener('scroll', onChatScrollEvent, { passive: true })
    userPinnedToBottom.value = isNearBottom(el)
  }
})

function scrollChatToBottom(force = false) {
  nextTick(() => {
    const el = chatScrollEl.value
    if (!el) return
    if (force || userPinnedToBottom.value) {
      el.scrollTop = el.scrollHeight
      if (force) userPinnedToBottom.value = true
    }
  })
}

const sidebarPlaygroundDesc = computed(() => {
  if (playMode.value === 'image') return t('playground.descImage')
  if (playMode.value === 'video') return t('playground.descVideo')
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech') return t('playground.descAudioSpeech')
    if (audioPlaySub.value === 'transcribe') return t('playground.descAudioTranscribe')
    return t('playground.descAudioTranslate')
  }
  return t('playground.desc')
})

const chatHeaderTitle = computed(() => {
  if (playMode.value === 'image') return t('playground.chatTitleImage')
  if (playMode.value === 'video') return t('playground.chatTitleVideo')
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech') return t('playground.chatTitleAudioSpeech')
    if (audioPlaySub.value === 'transcribe') return t('playground.chatTitleAudioTranscribe')
    return t('playground.chatTitleAudioTranslate')
  }
  return t('playground.chatTitle')
})

const chatHeaderSubtitle = computed(() => {
  if (playMode.value === 'image') {
    return `${model.value || '—'} · ${imageGenSize.value} · ${
      imageResponseFormat.value === 'url' ? t('playground.imageFormatUrl') : t('playground.imageFormatB64')
    }`
  }
  if (playMode.value === 'video') {
    return `${model.value || '—'} · ${videoSeconds.value}s · ${videoSize.value}`
  }
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech')
      return `${model.value || '—'} · ${t('playground.audioVoice')}: ${audioSpeechVoice.value.trim() || 'alloy'}`
    return `${model.value || '—'} · ${t('playground.audioLanguage')}: ${sttLanguage.value.trim() || t('playground.audioLangAuto')}`
  }
  return `${model.value || '—'} · ${useStream.value ? t('playground.streamOn') : t('playground.streamOff')}`
})

const emptyPanelHint = computed(() => {
  if (playMode.value === 'image') return t('playground.emptyImage')
  if (playMode.value === 'video') return t('playground.emptyVideo')
  if (playMode.value === 'audio' && audioPlaySub.value === 'speech') return t('playground.emptyAudioSpeech')
  if (playMode.value === 'audio' && audioPlaySub.value === 'transcribe') return t('playground.emptyAudioTranscribe')
  if (playMode.value === 'audio') return t('playground.emptyAudioTranslate')
  return t('playground.emptyChat')
})

const inputFooterPh = computed(() => {
  if (playMode.value === 'image') return t('playground.inputPhImage')
  if (playMode.value === 'video') return t('playground.inputPhVideo')
  if (playMode.value === 'audio' && audioPlaySub.value === 'speech') return t('playground.inputPhAudioSpeech')
  if (playMode.value === 'audio' && audioPlaySub.value === 'transcribe') return t('playground.inputPhAudioStt')
  if (playMode.value === 'audio') return t('playground.inputPhAudioTranslate')
  return t('playground.inputPh')
})

const showRegenerateForCurrentMode = computed(
  () =>
    playMode.value === 'chat' ||
    playMode.value === 'image' ||
    playMode.value === 'video' ||
    (playMode.value === 'audio' && audioPlaySub.value === 'speech'),
)

const sendDisabled = computed(() => {
  if (!apiKey.value.trim() || sending.value) return true
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech') return !input.value.trim()
    return !audioFile.value
  }
  return !input.value.trim()
})

watch(
  () => [messages.value.length, messages.value.map((m) => m.content).join('\x00'), sending.value],
  () => scrollChatToBottom(),
)

function fmtNum(n: number) {
  return Number.isInteger(n) ? String(n) : String(n)
}

  return {
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
  }
}
