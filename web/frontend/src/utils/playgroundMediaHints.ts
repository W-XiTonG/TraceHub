/** 试玩页文生图/文生视频：按模型推断参数习惯（OpenAI 优先，火山单独提示） */

export type PlaygroundMediaVendor = 'openai' | 'volc' | 'generic'

const OPENAI_IMAGE_SIZES = ['1024x1024', '1024x1792', '1792x1024', '512x512', '256x256'] as const
const VOLC_IMAGE_SIZES = ['2K', '4K', '2048x2048', '2560x1440', '1440x2560', '1024x1024', '1024x1792', '1792x1024'] as const
const GENERIC_IMAGE_SIZES = [
  '1024x1024',
  '1024x1792',
  '1792x1024',
  '2K',
  '4K',
  '2048x2048',
  '2560x1440',
  '1440x2560',
] as const

const OPENAI_VIDEO_SIZES = ['1280x720', '720x1280', '1920x1080', '1024x1024'] as const
const VOLC_VIDEO_SIZES = ['1280x720', '720x1280', '1920x1080', '1024x1024'] as const
const GENERIC_VIDEO_SIZES = ['1280x720', '720x1280', '1920x1080', '1024x1024'] as const

export function inferPlaygroundMediaVendor(
  modelId: string,
  labels?: string[],
  ownedBy?: string,
): PlaygroundMediaVendor {
  const id = modelId.trim().toLowerCase()
  if (!id) return 'generic'
  const ob = (ownedBy ?? '').trim().toLowerCase()
  const labs = new Set((labels ?? []).map((l) => l.trim().toLowerCase()).filter(Boolean))

  if (ob === 'bytedance' || /seedream|seedance|seed3d|3dgen|doubao-|skylark/.test(id)) {
    return 'volc'
  }
  if (/dall-e|dalle|gpt-image|^sora|o[134]-|gpt-[34]|chatgpt|openai\//.test(id)) {
    return 'openai'
  }
  if (labs.has('image') && /seedream|doubao/.test(id)) return 'volc'
  if (labs.has('image') && /dall-e|gpt-image|flux|stable-diffusion/.test(id)) return 'openai'
  if (labs.has('video') && /seedance|seed3d|doubao/.test(id)) return 'volc'
  if (labs.has('video') && /sora|veo-/.test(id)) return 'openai'
  return 'generic'
}

export function playgroundImageSizes(vendor: PlaygroundMediaVendor): readonly string[] {
  switch (vendor) {
    case 'openai':
      return OPENAI_IMAGE_SIZES
    case 'volc':
      return VOLC_IMAGE_SIZES
    default:
      return GENERIC_IMAGE_SIZES
  }
}

export function playgroundVideoSizes(vendor: PlaygroundMediaVendor): readonly string[] {
  switch (vendor) {
    case 'openai':
      return OPENAI_VIDEO_SIZES
    case 'volc':
      return VOLC_VIDEO_SIZES
    default:
      return GENERIC_VIDEO_SIZES
  }
}

export function defaultPlaygroundImageSize(vendor: PlaygroundMediaVendor): string {
  return vendor === 'volc' ? '2K' : '1024x1024'
}

export function defaultPlaygroundVideoSize(_vendor: PlaygroundMediaVendor): string {
  return '1280x720'
}

/** 导入/恢复本地设置时允许的尺寸并集（各 vendor 选项的超集） */
export const ALL_PLAYGROUND_IMAGE_SIZES: readonly string[] = GENERIC_IMAGE_SIZES
export const ALL_PLAYGROUND_VIDEO_SIZES: readonly string[] = GENERIC_VIDEO_SIZES

export function isVolcSeed3dModelId(modelId: string): boolean {
  const id = modelId.trim().toLowerCase()
  return id.includes('seed3d') || id.includes('3dgen')
}

export type PlaygroundMediaHintKey =
  | 'hintImageOpenAI'
  | 'hintImageVolc'
  | 'hintImageGeneric'
  | 'hintVideoOpenAI'
  | 'hintVideoVolc'
  | 'hintVideoVolcRef'
  | 'hintVideoGeneric'

export function playgroundMediaHintKey(
  mode: 'image' | 'video',
  vendor: PlaygroundMediaVendor,
  opts?: { hasVideoRef?: boolean; modelId?: string },
): PlaygroundMediaHintKey | null {
  if (mode === 'image') {
    if (vendor === 'openai') return 'hintImageOpenAI'
    if (vendor === 'volc') return 'hintImageVolc'
    return 'hintImageGeneric'
  }
  if (opts?.hasVideoRef && vendor === 'volc' && !isVolcSeed3dModelId(opts.modelId ?? '')) {
    return 'hintVideoVolcRef'
  }
  if (vendor === 'openai') return 'hintVideoOpenAI'
  if (vendor === 'volc') return 'hintVideoVolc'
  return 'hintVideoGeneric'
}
