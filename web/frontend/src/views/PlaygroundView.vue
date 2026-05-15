<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import * as pgw from '../api/playgroundGateway'
import type { PlaygroundModelRow, PlaygroundRouterHealth } from '../api/playgroundGateway'
import { copyTextToClipboard } from '../utils/clipboard'
import { randomUUID } from '../utils/randomId'

marked.use({
  async: false,
  gfm: true,
  breaks: false,
})

const { t } = useI18n()

const LS_KEY = 'tracehub_playground_settings_v1'
const SS_KEY = 'tracehub_playground_apikey'
const LS_MSG = 'tracehub_playground_messages_v1'

type ChatMsg = {
  id: string
  role: 'user' | 'assistant'
  content: string
  imageUrl?: string
  /** 文生图：展示用 URL 或 data URL，避免把超长 base64 塞进 Markdown */
  imageOutputs?: string[]
  /** 语音合成：浏览器 blob URL，仅本会话有效 */
  audioUrl?: string
}

const apiKey = ref('')
const stickySession = ref('')
const models = ref<PlaygroundModelRow[]>([])
const model = ref('')
const modelsErr = ref('')
const modelsLoading = ref(false)
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
  model.value = id
  closeModelPicker()
}

function onDocMousedown(ev: MouseEvent) {
  if (!modelPickerOpen.value) return
  const t = ev.target as Node
  const wrap = modelPickerWrap.value
  const panel = modelPickerPanel.value
  if (wrap?.contains(t) || panel?.contains(t)) return
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
  if (!raw) return rows
  const parts = raw.split(/\s+/).filter(Boolean)
  const idSet = playgroundRegistryIds.value
  return rows.filter((r) => {
    const id = r.id.toLowerCase()
    const regText = idSet?.has(r.id) ? pgRegistryCell(r).text.toLowerCase() : ''
    const rhText = r.routerHealth ? pgRouterHealthCell(r.routerHealth).text.toLowerCase() : ''
    const kindBlob = (r.labels ?? []).map((k) => pgModelKindLabel(k).toLowerCase()).join(' ')
    return parts.every(
      (p) => id.includes(p) || regText.includes(p) || rhText.includes(p) || kindBlob.includes(p),
    )
  })
})

/** 对话补全 / 文生图 / 音频（OpenAI 兼容 /v1/audio/*） */
const playMode = ref<'chat' | 'image' | 'audio'>('chat')
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

const PG_IMAGE_MAX_BYTES = 15 * 1024 * 1024
const PG_AUDIO_MAX_BYTES = 25 * 1024 * 1024
const PG_IMAGE_ACCEPT_TYPES = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp'])
const PG_IMAGE_GEN_SIZES = ['256x256', '512x512', '1024x1024', '1024x1792', '1792x1024'] as const

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
  try {
    const raw = localStorage.getItem(LS_MSG)
    if (raw) {
      const arr = JSON.parse(raw) as ChatMsg[]
      if (Array.isArray(arr)) messages.value = arr
    }
  } catch {
    /* ignore */
  }
  if (apiKey.value) void refreshModels()
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

watch(playMode, (m) => {
  if (m === 'image' || m === 'audio') {
    customBody.value = false
    useStream.value = false
  }
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

watch(useImage, (on) => {
  if (!on) imageUploadErr.value = ''
})

watch(playMode, () => {
  audioUploadErr.value = ''
})

watch(imageUrl, () => {
  imageUploadErr.value = ''
})

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
    if (j.playMode === 'chat' || j.playMode === 'image' || j.playMode === 'audio') playMode.value = j.playMode
    if (j.audioPlaySub === 'speech' || j.audioPlaySub === 'transcribe' || j.audioPlaySub === 'translate')
      audioPlaySub.value = j.audioPlaySub
    if (typeof j.audioSpeechVoice === 'string') audioSpeechVoice.value = j.audioSpeechVoice
    if (typeof j.audioSpeechSpeed === 'number' && j.audioSpeechSpeed > 0) audioSpeechSpeed.value = j.audioSpeechSpeed
    if (typeof j.sttLanguage === 'string') sttLanguage.value = j.sttLanguage
    if (typeof j.imageGenSize === 'string' && (PG_IMAGE_GEN_SIZES as readonly string[]).includes(j.imageGenSize))
      imageGenSize.value = j.imageGenSize
    if (j.imageResponseFormat === 'url' || j.imageResponseFormat === 'b64_json')
      imageResponseFormat.value = j.imageResponseFormat
    if (typeof j.useSeed === 'boolean') useSeed.value = j.useSeed
    if (typeof j.seed === 'number') seed.value = j.seed
    if (typeof j.assistantMarkdown === 'boolean') assistantMarkdown.value = j.assistantMarkdown
  } catch {
    /* ignore */
  }
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
    useSeed: useSeed.value,
    seed: seed.value,
    assistantMarkdown: assistantMarkdown.value,
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
    useSeed,
    seed,
    assistantMarkdown,
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
  modelsLoading.value = true
  try {
    const [rows, reg] = await Promise.all([
      pgw.fetchPlaygroundModels(k, stickySession.value),
      pgw.fetchPlaygroundRegistryMap(),
    ])
    playgroundRegistryMerged.value = reg !== null
    playgroundRegistryIds.value = reg ? new Set(reg.keys()) : null
    models.value = pgw.mergePlaygroundRegistry(rows, reg)
    const ids = models.value.map((r) => r.id)
    if (!model.value && ids.length) model.value = ids[0]
    if (model.value && !ids.includes(model.value) && ids.length) model.value = ids[0]
  } catch (e: unknown) {
    modelsErr.value = e instanceof Error ? e.message : t('playground.modelsFail')
  } finally {
    modelsLoading.value = false
  }
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

function revokeMsgAudio(m: ChatMsg) {
  const u = m.audioUrl
  if (u?.startsWith('blob:')) URL.revokeObjectURL(u)
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
    revokeMsgAudio(m)
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
      const reader = res.body?.getReader()
      if (!reader) throw new Error('No body')
      const dec = new TextDecoder()
      let buf = ''
      let acc = ''
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buf += dec.decode(value, { stream: true })
        const lines = buf.split('\n')
        buf = lines.pop() || ''
        for (const line of lines) {
          streamLog.value += `${line}\n`
          const s = line.trim()
          if (!s.startsWith('data:')) continue
          const data = s.slice(5).trim()
          if (data === '[DONE]') continue
          try {
            const json = JSON.parse(data) as { choices?: Array<{ delta?: { content?: string } }> }
            const piece = json.choices?.[0]?.delta?.content
            if (piece) {
              acc += piece
              patchAssistant(assistantId, acc)
            }
          } catch {
            /* ignore */
          }
        }
      }
      lastResponse.value = acc || streamLog.value.slice(0, 8000)
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

function patchAssistantImages(id: string, content: string, imageOutputs: string[]) {
  messages.value = messages.value.map((m) =>
    m.id === id ? { ...m, content, imageOutputs: imageOutputs.length ? imageOutputs : undefined } : m,
  )
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
    const clean = DOMPurify.sanitize(dirty)
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
  for (const m of messages.value) revokeMsgAudio(m)
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
      const { audioUrl, ...rest } = m
      if (audioUrl?.startsWith('blob:')) return rest
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
  if (target) revokeMsgAudio(target)
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
        flags?: Record<string, boolean>
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
      if (f.playMode === 'chat' || f.playMode === 'image' || f.playMode === 'audio') playMode.value = f.playMode
      if (f.audioPlaySub === 'speech' || f.audioPlaySub === 'transcribe' || f.audioPlaySub === 'translate')
        audioPlaySub.value = f.audioPlaySub
      if (typeof f.audioSpeechVoice === 'string') audioSpeechVoice.value = f.audioSpeechVoice
      if (typeof f.audioSpeechSpeed === 'number' && f.audioSpeechSpeed > 0) audioSpeechSpeed.value = f.audioSpeechSpeed
      if (typeof f.sttLanguage === 'string') sttLanguage.value = f.sttLanguage
      if (typeof f.imageGenSize === 'string' && (PG_IMAGE_GEN_SIZES as readonly string[]).includes(f.imageGenSize))
        imageGenSize.value = f.imageGenSize
      if (f.imageResponseFormat === 'url' || f.imageResponseFormat === 'b64_json')
        imageResponseFormat.value = f.imageResponseFormat
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

const sidebarPlaygroundDesc = computed(() => {
  if (playMode.value === 'image') return t('playground.descImage')
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech') return t('playground.descAudioSpeech')
    if (audioPlaySub.value === 'transcribe') return t('playground.descAudioTranscribe')
    return t('playground.descAudioTranslate')
  }
  return t('playground.desc')
})

const chatHeaderTitle = computed(() => {
  if (playMode.value === 'image') return t('playground.chatTitleImage')
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
  if (playMode.value === 'audio') {
    if (audioPlaySub.value === 'speech')
      return `${model.value || '—'} · ${t('playground.audioVoice')}: ${audioSpeechVoice.value.trim() || 'alloy'}`
    return `${model.value || '—'} · ${t('playground.audioLanguage')}: ${sttLanguage.value.trim() || t('playground.audioLangAuto')}`
  }
  return `${model.value || '—'} · ${useStream.value ? t('playground.streamOn') : t('playground.streamOff')}`
})

const emptyPanelHint = computed(() => {
  if (playMode.value === 'image') return t('playground.emptyImage')
  if (playMode.value === 'audio' && audioPlaySub.value === 'speech') return t('playground.emptyAudioSpeech')
  if (playMode.value === 'audio' && audioPlaySub.value === 'transcribe') return t('playground.emptyAudioTranscribe')
  if (playMode.value === 'audio') return t('playground.emptyAudioTranslate')
  return t('playground.emptyChat')
})

const inputFooterPh = computed(() => {
  if (playMode.value === 'image') return t('playground.inputPhImage')
  if (playMode.value === 'audio' && audioPlaySub.value === 'speech') return t('playground.inputPhAudioSpeech')
  if (playMode.value === 'audio' && audioPlaySub.value === 'transcribe') return t('playground.inputPhAudioStt')
  if (playMode.value === 'audio') return t('playground.inputPhAudioTranslate')
  return t('playground.inputPh')
})

const showRegenerateForCurrentMode = computed(
  () =>
    playMode.value === 'chat' ||
    playMode.value === 'image' ||
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

function scrollChatToBottom() {
  nextTick(() => {
    const el = chatScrollEl.value
    if (el) el.scrollTop = el.scrollHeight
  })
}

watch(
  () => [messages.value.length, messages.value.map((m) => m.content).join('\x00'), sending.value],
  () => scrollChatToBottom(),
)

function fmtNum(n: number) {
  return Number.isInteger(n) ? String(n) : String(n)
}
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
          {{ t('playground.billingHint') }}
        </p>
      </div>

      <div class="thin-scrollbar min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-y-contain px-4 py-4">
        <div class="th-pg-card th-pg-card--emph space-y-2 p-3">
          <p class="text-[11px] font-medium uppercase tracking-wide text-slate-500">{{ t('playground.playMode') }}</p>
          <div class="th-pg-seg flex w-full" role="group" :aria-label="t('playground.playMode')">
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-xs"
              :class="{ 'th-pg-seg__btn--active': playMode === 'chat' }"
              :aria-pressed="playMode === 'chat'"
              @click="playMode = 'chat'"
            >
              {{ t('playground.modeChat') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-xs"
              :class="{ 'th-pg-seg__btn--active': playMode === 'image' }"
              :aria-pressed="playMode === 'image'"
              @click="playMode = 'image'"
            >
              {{ t('playground.modeImage') }}
            </button>
            <button
              type="button"
              class="th-pg-seg__btn min-w-0 flex-1 text-xs"
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
            {{ modelsLoading ? t('common.loading') : t('playground.refreshModels') }}
          </button>
          <label v-if="playMode === 'chat'" class="flex cursor-pointer items-center gap-2 text-xs th-pg-muted">
            <input v-model="useStream" type="checkbox" class="h-3.5 w-3.5 rounded border-slate-600 text-cyan-500" />
            {{ t('playground.stream') }}
          </label>
        </div>
        <p v-if="modelsErr" class="text-xs text-red-400">{{ modelsErr }}</p>

        <template v-if="!customBody">
          <div>
            <label class="text-xs text-slate-500">{{ t('playground.model') }}</label>
            <p class="mt-0.5 text-[10px] leading-snug th-pg-muted">{{ t('playground.routerHealthHint') }}</p>
            <p v-if="playgroundRegistryMerged" class="mt-0.5 text-[10px] leading-snug th-pg-muted">
              {{ t('playground.registryHint') }}
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
                @click="toggleModelPicker"
              >
                <span
                  v-for="lab in selectedModelRow?.labels ?? []"
                  :key="'pg-sel-k-' + lab"
                  class="th-pg-model-picker__meta-pill th-pg-model-picker__kind-pill"
                  :title="t('modelsPage.labelsHint')"
                >
                  {{ pgModelKindLabel(lab) }}
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
                <span class="th-pg-model-picker__value min-w-0 flex-1 truncate text-left font-mono text-[11px] leading-snug">{{
                  models.length ? model || models[0].id : t('playground.modelEmpty')
                }}</span>
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
                    class="th-pg-model-picker__option"
                    :class="{ 'th-pg-model-picker__option--active': model === m.id }"
                    @click="pickModel(m.id)"
                  >
                    <span class="th-pg-model-picker__opt-id">{{ m.id }}</span>
                    <div class="th-pg-model-picker__opt-aside">
                      <span
                        v-for="lab in m.labels ?? []"
                        :key="`${m.id}-k-${lab}`"
                        class="th-pg-model-picker__meta-pill th-pg-model-picker__kind-pill"
                        :title="t('modelsPage.labelsHint')"
                      >
                        {{ pgModelKindLabel(lab) }}
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
          </div>

          <div v-if="playMode === 'image'" class="th-pg-card space-y-3 p-3">
            <div>
              <label class="text-xs text-slate-500">{{ t('playground.imageGenSize') }}</label>
              <select
                v-model="imageGenSize"
                class="mt-1 w-full rounded-lg border border-slate-600 bg-slate-950 px-2 py-1.5 font-mono text-xs text-slate-100"
              >
                <option v-for="s in PG_IMAGE_GEN_SIZES" :key="s" :value="s">{{ s }}</option>
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
            type="button"
            class="th-pg-btn-ghost px-3 py-1.5"
            @click="showDebug = !showDebug"
          >
            {{ showDebug ? t('playground.hideDebug') : t('playground.showDebug') }}
          </button>
          <button type="button" class="th-pg-btn-ghost px-3 py-1.5" @click="exportConversation">
            {{ t('playground.exportChat') }}
          </button>
        </div>
      </header>

      <div v-if="showDebug" class="th-pg-debug shrink-0 px-4 py-2">
        <p class="text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugReq') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastRequest || '—' }}</pre>
        <p class="mt-2 text-[10px] font-medium uppercase text-slate-500">{{ t('playground.debugRes') }}</p>
        <pre class="mt-1 max-h-32 overflow-auto font-mono text-[10px] text-slate-400">{{ lastResponse || streamLog.slice(0, 4000) || '—' }}</pre>
      </div>

      <div
        ref="chatScrollEl"
        class="thin-scrollbar min-h-0 flex-1 overflow-y-auto overscroll-y-contain px-4 py-4"
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
            class="flex max-w-[min(100%,42rem)] flex-col gap-1"
            :class="m.role === 'user' ? 'items-end' : 'items-start'"
          >
            <div
              class="th-pg-msg flex min-w-0 flex-col gap-0 rounded-2xl border px-3.5 py-2 text-sm"
              :class="m.role === 'user' ? 'th-pg-msg--user' : 'th-pg-msg--assistant'"
            >
              <div v-if="m.role === 'assistant' && m.audioUrl" class="mb-2 w-full max-w-md">
                <audio :src="m.audioUrl" controls class="w-full rounded-lg border border-white/10 bg-slate-950/80" />
              </div>
              <div v-if="m.role === 'assistant' && m.imageOutputs?.length" class="mb-2 flex flex-col gap-2">
                <img
                  v-for="(src, ix) in m.imageOutputs"
                  :key="ix"
                  :src="src"
                  alt=""
                  class="max-h-72 max-w-full rounded-lg border border-white/10 object-contain"
                  @error="(($event.target as HTMLImageElement).style.display = 'none')"
                />
              </div>
              <div v-if="m.imageUrl" class="mb-2 shrink-0">
                <img
                  :src="m.imageUrl"
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
