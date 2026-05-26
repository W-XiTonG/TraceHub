/** 额度流水 / 调用日志：API 扣费说明格式化（含视频按秒计费） */

function numTok(v: unknown): number {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v)
  if (typeof v === 'string' && v.trim() !== '') {
    const n = Number(v)
    return Number.isFinite(n) ? Math.trunc(n) : 0
  }
  return 0
}

/** 与 internal/pkg/modelkind 启发式对齐，用于流水展示 */
export function modelLooksVideo(model: string): boolean {
  const id = model.trim().toLowerCase()
  if (!id) return false
  return (
    id.includes('text-to-video') ||
    id.includes('image-to-video') ||
    id.includes('video-gen') ||
    id.includes('t2v') ||
    id.includes('i2v') ||
    id.includes('-video') ||
    id.includes('/videos') ||
    id.includes('/video') ||
    id.includes('_video') ||
    (id.startsWith('route:') && (id.includes('/videos') || id.includes('/video/'))) ||
    id.includes('seedance') ||
    id.includes('doubao-seed') ||
    id.includes('seed3d') ||
    id.includes('sora') ||
    id.includes('kling') ||
    id.includes('hunyuan-video') ||
    id.includes('hunyuan_video') ||
    id.includes('veo-') ||
    id.startsWith('veo') ||
    id.includes('cogvideox') ||
    id.includes('wan2.1') ||
    id.includes('wan-2.1')
  )
}

/** 视频任务：秒数写入 prompt_tokens，completion_tokens 恒为 0 */
export function isVideoDurationBilling(d: Record<string, unknown>): boolean {
  if (d.billing_mode === 'video_duration') return true
  if (numTok(d.completion_tokens) !== 0) return false
  const sec = numTok(d.prompt_tokens)
  if (sec <= 0 || sec > 3600) return false
  return modelLooksVideo(String(d.model ?? ''))
}

export type UsageLogBillingRow = {
  model?: string
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
  billing_mode?: string
  error_message?: string
}

export function resolveUsageBillingMode(row: UsageLogBillingRow): 'video_duration' | 'token' {
  if (row.billing_mode === 'video_duration') return 'video_duration'
  const modelStr = String(row.model ?? '')
  if (modelLooksVideo(modelStr)) return 'video_duration'
  if (isVideoDurationBilling(row as Record<string, unknown>)) return 'video_duration'
  return 'token'
}

/** 调用日志表格：计费类型短标签 */
export function usageBillingBadgeKey(mode: 'video_duration' | 'token'): string {
  return mode === 'video_duration' ? 'usageLog.billingVideo' : 'usageLog.billingToken'
}

/** 网关写入的用量说明（usage_note:*），非错误 */
export function parseUsageNote(errorMessage?: string): string {
  const m = (errorMessage ?? '').trim()
  if (!m.startsWith('usage_note:')) return ''
  return m.slice('usage_note:'.length)
}

export function isUsageNoteMessage(errorMessage?: string): boolean {
  return parseUsageNote(errorMessage) !== '' || (errorMessage ?? '').trim().startsWith('usage_note:')
}

function usageNoteSuffix(
  t: (key: string, params?: Record<string, unknown>) => string,
  errorMessage?: string,
): string {
  switch (parseUsageNote(errorMessage)) {
    case 'stream_content':
      return t('usageLog.noteStreamContent')
    case 'max_tokens':
      return t('usageLog.noteMaxTokens')
    case 'client_disconnect':
      return t('usageLog.noteClientDisconnect')
    case 'no_output':
      return t('usageLog.noteNoOutput')
    default:
      return ''
  }
}

/** 调用日志表格：用量单元格文案 */
export function formatUsageLogUsageLine(
  t: (key: string, params?: Record<string, unknown>) => string,
  row: UsageLogBillingRow & { total_cost?: number },
): string {
  const modelStr = String(row.model ?? '')
  if (modelStr.startsWith('route:')) {
    return t('usageLog.usageRouteFailed')
  }
  const mode = resolveUsageBillingMode(row)
  let base: string
  if (mode === 'video_duration') {
    const sec = numTok(row.prompt_tokens)
    const cost = numTok(row.total_cost)
    if (cost === 0 && sec === 0) {
      base = t('usageLog.usageVideoNoCharge')
    } else if (cost === 0 && sec > 0) {
      base = t('usageLog.usageVideoSeconds', { seconds: sec }) + ' · ' + t('usageLog.usageVideoNoCharge')
    } else {
      base = t('usageLog.usageVideoSeconds', { seconds: sec })
    }
  } else {
    const pt = numTok(row.prompt_tokens)
    const ct = numTok(row.completion_tokens)
    const total = numTok(row.total_tokens)
    if (total > 0 && total !== pt + ct) {
      base = t('usageLog.usageTokenTotal', { in: pt, out: ct, total })
    } else {
      base = t('usageLog.usageTokenInOut', { in: pt, out: ct })
    }
  }
  const note = usageNoteSuffix(t, row.error_message)
  return note ? `${base} · ${note}` : base
}

export function formatApiUsageLedgerLine(
  t: (key: string, params?: Record<string, unknown>) => string,
  d: Record<string, unknown>,
  keys: { detailApi: string; detailApiVideo: string },
): string {
  const model = String(d.model ?? '—')
  const cost = numTok(d.total_cost)
  if (isVideoDurationBilling(d)) {
    return t(keys.detailApiVideo, { model, seconds: numTok(d.prompt_tokens), cost })
  }
  return t(keys.detailApi, {
    model,
    inTok: numTok(d.prompt_tokens),
    outTok: numTok(d.completion_tokens),
    cost,
  })
}
