import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const extracted = fs.readFileSync(path.join(__dirname, '../src/composables/_pg_script_extract.ts'), 'utf8')

let body = extracted
  .replace(/^import ThLoadingSpinner.*\n/m, '')
  .replace(/^import \{ RouterLink \}.*\n/m, '')
  .replace(
    /^const LS_KEY = 'tracehub_playground_settings_v1'\nconst SS_KEY = 'tracehub_playground_apikey'\nconst LS_MSG = 'tracehub_playground_messages_v1'\n\n/s,
    '',
  )
  .replace(/^type ChatMsg = \{[\s\S]*?\n\}\n\n/m, '')

const header = `import DOMPurify from 'dompurify'
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
  breaks: false,
})

export type UsePlaygroundCoreOptions = {
  /** 新版 UI：简化默认（b64 图片、Markdown 等） */
  modernUi?: boolean
}

export function usePlaygroundCore(options: UsePlaygroundCoreOptions = {}) {
  const modernUi = options.modernUi === true
  const { t } = useI18n()
`

// Remove duplicate marked.use and const { t }
body = body.replace(/^marked\.use\([\s\S]*?\)\n\n/m, '')
body = body.replace(/^const \{ t \} = useI18n\(\)\n\n/m, '')

// Remove local PG_* constants (now imported)
body = body.replace(
  /^const PG_IMAGE_MAX_BYTES = 15 \* 1024 \* 1024\nconst PG_AUDIO_MAX_BYTES = 25 \* 1024 \* 1024\nconst PG_IMAGE_ACCEPT_TYPES = new Set\(\[[^\]]+\]\)\nconst PG_VIDEO_MAX_BYTES = 15 \* 1024 \* 1024\nconst PG_VIDEO_POLL_MS = 3000\nconst PG_VIDEO_POLL_MAX = 120\n\n/m,
  '',
)

// Fix exportSettings - add video params
body = body.replace(
  /imageResponseFormat: imageResponseFormat\.value,\n          \},/,
  `imageResponseFormat: imageResponseFormat.value,
            videoSeconds: videoSeconds.value,
            videoSize: videoSize.value,
          },`,
)

// Fix importSettings playMode to include video
body = body.replace(
  /if \(f\.playMode === 'chat' \|\| f\.playMode === 'image' \|\| f\.playMode === 'audio'\) playMode\.value = f\.playMode/,
  "if (f.playMode === 'chat' || f.playMode === 'image' || f.playMode === 'audio' || f.playMode === 'video') playMode.value = f.playMode as typeof playMode.value",
)

// Fix import flags type and add video import
body = body.replace(
  /flags\?: Record<string, boolean>/,
  'flags?: Record<string, unknown>',
)
body = body.replace(
  /if \(f\.imageResponseFormat === 'url' \|\| f\.imageResponseFormat === 'b64_json'\)\n        imageResponseFormat\.value = f\.imageResponseFormat\n      persistSettings\(\)/,
  `if (f.imageResponseFormat === 'url' || f.imageResponseFormat === 'b64_json')
        imageResponseFormat.value = f.imageResponseFormat as 'url' | 'b64_json'
      if (typeof f.videoSeconds === 'string' && f.videoSeconds.trim()) videoSeconds.value = f.videoSeconds.trim()
      if (typeof f.videoSize === 'string' && ALL_PLAYGROUND_VIDEO_SIZES.includes(f.videoSize))
        videoSize.value = f.videoSize
      persistSettings()`,
)

// Add video job persistence helpers after activeVideoAssistantId
const videoJobHelpers = `
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
`
body = body.replace(
  /const activeVideoAssistantId = ref\(''\)\n/,
  `const activeVideoAssistantId = ref('')\n${videoJobHelpers}\n`,
)

// persist job when video created
body = body.replace(
  /activeVideoJobId\.value = snap\.videoId\n    patchAssistantVideo\(assistantId, t\('playground\.videoCreated'/,
  `activeVideoJobId.value = snap.videoId
    persistActiveVideoJob({
      assistantId,
      videoId: snap.videoId,
      modelName,
      startedAt: Date.now(),
    })
    patchAssistantVideo(assistantId, t('playground.videoCreated'`,
)

// clear job on poll complete paths in pollVideoUntilDone - add clearPersistedVideoJob before return on completed/failed
body = body.replace(
  /if \(st === 'completed'\) \{\n        const \{ blob \} = await pgw\.videoContent/,
  `if (st === 'completed') {
        clearPersistedVideoJob()
        const { blob } = await pgw.videoContent`,
)
body = body.replace(
  /if \(st === 'failed' \|\| st === 'cancelled' \|\| st === 'canceled'\) \{\n        const err = snap\.error/,
  `if (st === 'failed' || st === 'cancelled' || st === 'canceled') {
        clearPersistedVideoJob()
        const err = snap.error`,
)

// clear on poll timeout and catch
body = body.replace(
  /patchAssistantVideo\(assistantId, `\【\$\{t\('playground\.error'\)\}\】\\n\\n\$\{t\('playground\.videoPollTimeout'\)\}`/,
  `clearPersistedVideoJob()
  patchAssistantVideo(assistantId, \`【\${t('playground.error')}】\\n\\n\${t('playground.videoPollTimeout')}\``,
)

// cancelVideoGeneration clear job
body = body.replace(
  /function cancelVideoGeneration\(\) \{\n  stopVideoPoll\(\)/,
  `function cancelVideoGeneration() {
  clearPersistedVideoJob()
  stopVideoPoll()`,
)

// onMounted resume video job
body = body.replace(
  /void restorePersistedVideoMedia\(\)\n  \}\n  document\.addEventListener\('mousedown'/,
  `void restorePersistedVideoMedia()
    void resumePersistedVideoJob()
  } else {
    void restorePersistedVideoMedia()
    void resumePersistedVideoJob()
  }
  document.addEventListener('mousedown'`,
)

// Fix duplicate else branch - the original had:
// if (apiKey.value) { refreshModels().then(() => restore...) } else { restore... }
// My replace might have broken it. Let me check the onMounted section in body

// Modern defaults after loadSettings in onMounted
body = body.replace(
  /loadSettings\(\)\n  try \{\n    const raw = localStorage\.getItem\(LS_MSG\)/,
  `loadSettings()
  if (modernUi) applyModernUiDefaults()
  try {
    const raw = localStorage.getItem(LS_MSG)`,
)

// Add applyModernUiDefaults before loadSettings function
const modernDefaultsFn = `
function applyModernUiDefaults() {
  if (!localStorage.getItem(LS_KEY)) {
    assistantMarkdown.value = true
    temperature.value = 1
    imageResponseFormat.value = 'b64_json'
  }
}
`
body = body.replace(/^function loadSettings\(\)/m, `${modernDefaultsFn}\nfunction loadSettings()`)

// Add retryImageWithB64 function for media hardening
const retryB64Fn = `
async function retryImageWithB64() {
  imageResponseFormat.value = 'b64_json'
  const lastUser = [...messages.value].reverse().find((m) => m.role === 'user')
  if (!lastUser?.content.trim()) return
  const before = messages.value.slice(0, messages.value.findIndex((m) => m.id === lastUser.id))
  messages.value = before
  input.value = lastUser.content
  await sendImageGeneration(lastUser.content)
}
`
body = body.replace(/^async function sendImageGeneration/m, `${retryB64Fn}\nasync function sendImageGeneration`)

// Collect exports for return statement
const exportNames = new Set()
for (const m of body.matchAll(/^const (\w+) = /gm)) exportNames.add(m[1])
for (const m of body.matchAll(/^function (\w+)/gm)) exportNames.add(m[1])
// Remove fmtNum if we want - keep it
const sorted = [...exportNames].sort()

const footer = `
  return {
    ${sorted.join(',\n    ')},
  }
}
`

const out = header + body + footer
fs.writeFileSync(path.join(__dirname, '../src/composables/usePlaygroundCore.ts'), out)
console.log('wrote usePlaygroundCore.ts with', sorted.length, 'exports')
