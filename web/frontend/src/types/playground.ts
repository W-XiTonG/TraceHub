export type PlaygroundUiVersion = 'classic' | 'modern'

export type PlaygroundPlayMode = 'chat' | 'image' | 'audio' | 'video'

export type PlaygroundAudioSub = 'speech' | 'transcribe' | 'translate'

export type ChatMsg = {
  id: string
  role: 'user' | 'assistant'
  content: string
  imageUrl?: string
  imageOutputs?: string[]
  imageLoadFailed?: number[]
  audioUrl?: string
  videoUrl?: string
  videoJobId?: string
  videoModel?: string
  videoStatus?: string
  /** 轮询期间展示用：取历次上游 progress 的最大值，避免回跳 */
  videoPollProgress?: number
  videoRefUrl?: string
}

export type PersistedVideoJob = {
  assistantId: string
  videoId: string
  modelName: string
  startedAt: number
}

export const LS_PLAYGROUND_SETTINGS = 'tracehub_playground_settings_v1'
export const SS_PLAYGROUND_APIKEY = 'tracehub_playground_apikey'
export const LS_PLAYGROUND_MESSAGES = 'tracehub_playground_messages_v1'
export const LS_PLAYGROUND_UI_VERSION = 'tracehub_playground_ui_version'
export const SS_PLAYGROUND_VIDEO_JOB = 'tracehub_playground_video_job'

export const PG_IMAGE_MAX_BYTES = 15 * 1024 * 1024
export const PG_AUDIO_MAX_BYTES = 25 * 1024 * 1024
export const PG_VIDEO_MAX_BYTES = 15 * 1024 * 1024
export const PG_VIDEO_POLL_MS = 3000
export const PG_VIDEO_POLL_MAX = 120

export const PG_IMAGE_ACCEPT_TYPES = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp'])
