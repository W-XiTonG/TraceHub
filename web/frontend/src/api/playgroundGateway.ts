/** 同源调用 TraceHub OpenAI 兼容网关 /v1（使用 API Key，而非控制台 JWT） */

import { http, type Envelope } from './http'

const STICKY_HDR = 'X-TraceHub-Session'

export type PlaygroundRouterHealth = 'healthy' | 'unhealthy' | 'catalog_only'

export type PlaygroundServiceStatus = 'operational' | 'stale' | 'catalog_only' | 'degraded'

export type PlaygroundRegistryMeta = {
  upstream_list_status?: string
  upstream_list_checked_at?: string
  in_gateway?: boolean
  labels?: string[]
  service_status?: string
}

export type PlaygroundModelRow = {
  id: string
  ownedBy?: string
  routerHealth?: PlaygroundRouterHealth
  serviceStatus?: PlaygroundServiceStatus
  labels?: string[]
  /** 仅模型市场目录、分组渠道未配置；展示但不可在试玩中调用 */
  catalogOnly?: boolean
  /** 是否可在试玩中发起 API 请求（默认可调用） */
  callable?: boolean
} & PlaygroundRegistryMeta

function parseRouterHealth(v: unknown): PlaygroundRouterHealth | undefined {
  if (v === 'healthy' || v === 'unhealthy' || v === 'catalog_only') return v
  return undefined
}

function parseServiceStatus(v: unknown): PlaygroundServiceStatus | undefined {
  if (v === 'operational' || v === 'stale' || v === 'catalog_only' || v === 'degraded') return v
  return undefined
}

function parseTracehubLabels(v: unknown): string[] | undefined {
  if (!Array.isArray(v)) return undefined
  const out: string[] = []
  for (const x of v) {
    if (typeof x === 'string' && x.trim()) out.push(x.trim())
  }
  return out.length ? out : undefined
}

/** GET /v1/models，带路由健康扩展字段（与网关选路 health_status 一致，非实时探测上游）。 */
export async function fetchPlaygroundModels(apiKey: string, sticky: string): Promise<PlaygroundModelRow[]> {
  const res = await fetch('/v1/models?tracehub_include_router_health=1&tracehub_include_unavailable=1', {
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      [STICKY_HDR]: sticky,
    },
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(parseOpenAIError(text) || `${res.status} ${res.statusText}`)
  }
  const data = (await res.json()) as {
    data?: Array<{
      id?: string
      owned_by?: string
      tracehub?: { router_health?: string; service_status?: string; labels?: unknown }
    }>
  }
  const rows: PlaygroundModelRow[] = []
  const seen = new Set<string>()
  for (const m of data.data || []) {
    const id = typeof m.id === 'string' ? m.id.trim() : ''
    if (!id) continue
    const key = id.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    const ownedBy = typeof m.owned_by === 'string' ? m.owned_by.trim() : ''
    rows.push({
      id,
      ownedBy: ownedBy || undefined,
      routerHealth: parseRouterHealth(m.tracehub?.router_health),
      serviceStatus: parseServiceStatus(m.tracehub?.service_status),
      labels: parseTracehubLabels(m.tracehub?.labels),
      callable: true,
      catalogOnly: false,
    })
  }
  rows.sort((a, b) => a.id.localeCompare(b.id))
  return rows
}

export type AvailModelItem = {
  id: string
  upstream_list_status?: string
  upstream_list_checked_at?: string
  in_gateway?: boolean
  labels?: string[]
  service_status?: string
  owned_by?: string
}

/**
 * 与控制台「可用模型」同源；用于补全试玩下拉里 GET /v1/models 未返回的目录项。
 */
export async function fetchPlaygroundAvailItems(): Promise<AvailModelItem[] | null> {
  try {
    const { data } = await http.get<Envelope<{ items: AvailModelItem[] }>>('/api/user/available-models')
    if (data.code !== 0 || !data.data?.items) return null
    const out: AvailModelItem[] = []
    for (const it of data.data.items) {
      const id = typeof it.id === 'string' ? it.id.trim() : ''
      if (!id) continue
      out.push({
        id,
        upstream_list_status: it.upstream_list_status,
        upstream_list_checked_at: it.upstream_list_checked_at,
        in_gateway: it.in_gateway,
        labels: it.labels,
        service_status: it.service_status,
        owned_by: typeof it.owned_by === 'string' ? it.owned_by : undefined,
      })
    }
    return out
  } catch {
    return null
  }
}

/** @deprecated 使用 fetchPlaygroundAvailItems + availItemsToRegistryMap */
export async function fetchPlaygroundRegistryMap(): Promise<Map<string, PlaygroundRegistryMeta> | null> {
  const items = await fetchPlaygroundAvailItems()
  return items ? availItemsToRegistryMap(items) : null
}

export function availItemsToRegistryMap(items: AvailModelItem[]): Map<string, PlaygroundRegistryMeta> {
  const map = new Map<string, PlaygroundRegistryMeta>()
  for (const it of items) {
    map.set(it.id, {
      upstream_list_status: it.upstream_list_status,
      upstream_list_checked_at: it.upstream_list_checked_at,
      in_gateway: it.in_gateway,
      labels: it.labels,
      service_status: it.service_status,
    })
  }
  return map
}

function rowFromAvailItem(it: AvailModelItem, opts: { catalogOnly: boolean; callable: boolean }): PlaygroundModelRow {
  const ownedBy = typeof it.owned_by === 'string' ? it.owned_by.trim() : ''
  return {
    id: it.id,
    ownedBy: ownedBy || undefined,
    upstream_list_status: it.upstream_list_status,
    upstream_list_checked_at: it.upstream_list_checked_at,
    in_gateway: it.in_gateway,
    labels: it.labels,
    serviceStatus: parseServiceStatus(it.service_status),
    routerHealth: it.in_gateway === false ? 'catalog_only' : undefined,
    catalogOnly: opts.catalogOnly,
    callable: opts.callable,
  }
}

/**
 * 将「可用模型」中试玩 GET /v1/models 未包含的项并入下拉（仅目录 / 网关侧被状态过滤的 in_gateway 项）。
 */
export function appendPlaygroundAvailGap(
  gatewayRows: PlaygroundModelRow[],
  availItems: AvailModelItem[] | null,
): PlaygroundModelRow[] {
  if (!availItems?.length) return gatewayRows
  const byKey = new Map<string, PlaygroundModelRow>()
  for (const r of gatewayRows) {
    byKey.set(r.id.toLowerCase(), { ...r, callable: r.callable !== false, catalogOnly: !!r.catalogOnly })
  }
  for (const it of availItems) {
    const id = it.id.trim()
    if (!id) continue
    const key = id.toLowerCase()
    const existing = byKey.get(key)
    if (existing) {
      byKey.set(key, {
        ...existing,
        upstream_list_status: it.upstream_list_status ?? existing.upstream_list_status,
        upstream_list_checked_at: it.upstream_list_checked_at ?? existing.upstream_list_checked_at,
        in_gateway: it.in_gateway ?? existing.in_gateway,
        labels: it.labels?.length ? it.labels : existing.labels,
        serviceStatus: parseServiceStatus(it.service_status) ?? existing.serviceStatus,
      })
      continue
    }
    if (it.in_gateway === false) {
      byKey.set(key, rowFromAvailItem(it, { catalogOnly: true, callable: false }))
      continue
    }
    byKey.set(key, rowFromAvailItem(it, { catalogOnly: false, callable: true }))
  }
  return [...byKey.values()].sort((a, b) => a.id.localeCompare(b.id))
}

export function mergePlaygroundRegistry(
  rows: PlaygroundModelRow[],
  reg: Map<string, PlaygroundRegistryMeta> | null,
): PlaygroundModelRow[] {
  if (!reg?.size) return rows
  return rows.map((r) => {
    const m = reg.get(r.id)
    if (!m) return r
    return {
      ...r,
      upstream_list_status: m.upstream_list_status,
      upstream_list_checked_at: m.upstream_list_checked_at,
      in_gateway: m.in_gateway,
      labels: m.labels != null && m.labels.length > 0 ? m.labels : r.labels,
      serviceStatus: parseServiceStatus(m.service_status) ?? r.serviceStatus,
    }
  })
}

/** 网关列表 + 登记元数据 + 可用模型页补全 */
export function buildPlaygroundModelList(
  gatewayRows: PlaygroundModelRow[],
  availItems: AvailModelItem[] | null,
): PlaygroundModelRow[] {
  const reg = availItems ? availItemsToRegistryMap(availItems) : null
  return appendPlaygroundAvailGap(mergePlaygroundRegistry(gatewayRows, reg), availItems)
}

function parseOpenAIError(text: string): string {
  try {
    const j = JSON.parse(text) as { error?: { message?: string }; message?: string; code?: number }
    if (j.error?.message) return j.error.message
    if (typeof j.message === 'string' && j.message.trim()) {
      if (typeof j.code === 'number') return `[${j.code}] ${j.message}`
      return j.message
    }
    return ''
  } catch {
    return text.slice(0, 500)
  }
}

/** 网关计费诊断头（需 CORS Expose-Headers；正式 API 与控制台试玩共用） */
export function traceHubGatewayErrorDetails(res: Response): string {
  let out = ''
  const phase = res.headers.get('X-TraceHub-Billing-Phase')?.trim()
  const bal = res.headers.get('X-TraceHub-Balance')?.trim()
  const est = res.headers.get('X-TraceHub-Estimated-Cost')?.trim()
  if (phase || bal || est) {
    const pt = res.headers.get('X-TraceHub-Estimated-Prompt-Tokens')?.trim()
    const ctok = res.headers.get('X-TraceHub-Estimated-Completion-Tokens')?.trim()
    const debit = res.headers.get('X-TraceHub-Debit-Cost')?.trim()
    out += `\n\n[TraceHub 计费] phase=${phase ?? '—'}`
    if (bal) out += ` | balance=${bal}`
    if (est) out += ` | estimated_cost=${est}`
    if (debit) out += ` | debit_cost=${debit}`
    if (pt || ctok) out += ` | est_prompt_tokens=${pt ?? '—'} | est_completion_tokens=${ctok ?? '—'}`
    // 与 error.message 不重复展开；完整说明以 JSON error.message 为准，本行便于复制/工单。
    out += '\n说明以响应 JSON 的 error.message 为准。'
  }
  return out
}

export async function chatCompletion(
  apiKey: string,
  sticky: string,
  body: Record<string, unknown>,
): Promise<{ content: string; raw: string }> {
  const res = await fetch('/v1/chat/completions', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      'Content-Type': 'application/json',
      [STICKY_HDR]: sticky,
    },
    body: JSON.stringify({ ...body, stream: false }),
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  const json = JSON.parse(raw) as {
    choices?: Array<{ message?: { content?: string | null } }>
  }
  const content = json.choices?.[0]?.message?.content ?? ''
  return { content: typeof content === 'string' ? content : '', raw }
}

/** POST /v1/images/generations（非流式；网关可能 chunked 写出，浏览器仍收完整 JSON） */
export async function imageGenerations(
  apiKey: string,
  sticky: string,
  body: Record<string, unknown>,
): Promise<{ raw: string; imageOutputs: string[] }> {
  const res = await fetch('/v1/images/generations', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      'Content-Type': 'application/json',
      [STICKY_HDR]: sticky,
    },
    body: JSON.stringify(body),
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  let json: unknown
  try {
    json = JSON.parse(raw)
  } catch {
    return { raw, imageOutputs: [] }
  }
  return { raw, imageOutputs: parseImageGenerationOutputs(json) }
}

/** 从 OpenAI 及常见兼容体的 images/generations JSON 提取可展示的 URL / data URL。 */
export function parseImageGenerationOutputs(json: unknown): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  const push = (s: string) => {
    const t = s.trim()
    if (!t || seen.has(t)) return
    seen.add(t)
    out.push(t)
  }
  const pushUrl = (u: unknown) => {
    if (typeof u !== 'string') return
    const t = u.trim()
    if (t.startsWith('http://') || t.startsWith('https://') || t.startsWith('data:image/')) push(t)
  }
  const pushB64 = (b: unknown) => {
    if (typeof b !== 'string') return
    const t = b.trim()
    if (!t) return
    if (t.startsWith('data:image/')) push(t)
    else push(`data:image/png;base64,${t}`)
  }
  const walkRow = (row: unknown) => {
    if (!row || typeof row !== 'object') return
    const r = row as Record<string, unknown>
    pushUrl(r.url)
    pushUrl(r.image_url)
    pushB64(r.b64_json)
    pushB64(r.image)
  }
  if (!json || typeof json !== 'object') return out
  const root = json as Record<string, unknown>
  if (Array.isArray(root.data)) {
    for (const row of root.data) walkRow(row)
  }
  if (Array.isArray(root.images)) {
    for (const x of root.images) {
      if (typeof x === 'string') pushUrl(x)
      else walkRow(x)
    }
  }
  if (Array.isArray(root.output)) {
    for (const x of root.output) {
      if (typeof x === 'string') pushUrl(x)
      else walkRow(x)
    }
  }
  return out
}

/** POST /v1/audio/speech（JSON；响应多为音频二进制） */
export async function audioSpeech(
  apiKey: string,
  sticky: string,
  body: Record<string, unknown>,
): Promise<{ blob: Blob; contentType: string }> {
  const res = await fetch('/v1/audio/speech', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      'Content-Type': 'application/json',
      Accept: '*/*',
      [STICKY_HDR]: sticky,
    },
    body: JSON.stringify(body),
  })
  const contentType = res.headers.get('Content-Type')?.trim() || 'application/octet-stream'
  if (!res.ok) {
    const text = await res.text()
    throw new Error((parseOpenAIError(text) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  const blob = await res.blob()
  return { blob, contentType }
}

/** POST /v1/audio/transcriptions 或 /v1/audio/translations（multipart，与 OpenAI 一致） */
export async function audioMultipart(
  apiKey: string,
  sticky: string,
  relPath: 'audio/transcriptions' | 'audio/translations',
  opts: { model: string; file: File; language?: string; prompt?: string },
): Promise<{ raw: string }> {
  const fd = new FormData()
  fd.append('model', opts.model.trim())
  fd.append('file', opts.file, opts.file.name)
  if (opts.language?.trim()) fd.append('language', opts.language.trim())
  if (opts.prompt?.trim()) fd.append('prompt', opts.prompt.trim())

  const res = await fetch(`/v1/${relPath}`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      [STICKY_HDR]: sticky,
    },
    body: fd,
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  return { raw }
}

export type VideoJobSnapshot = {
  videoId: string
  status: string
  progress?: number
  seconds?: number
  error?: string
  raw: string
}

function videoAuthHeaders(apiKey: string, sticky: string, model: string): Record<string, string> {
  const h: Record<string, string> = {
    Authorization: `Bearer ${apiKey.trim()}`,
    [STICKY_HDR]: sticky,
  }
  const m = model.trim()
  if (m) h['X-TraceHub-Model'] = m
  return h
}

/** 解析创建/查询视频任务响应中的 id 与 status。 */
export function parseVideoJobResponse(raw: string): VideoJobSnapshot {
  const j = JSON.parse(raw) as Record<string, unknown>
  let videoId = ''
  for (const k of ['id', 'video_id', 'task_id']) {
    const v = j[k]
    if (typeof v === 'string' && v.trim()) {
      videoId = v.trim()
      break
    }
  }
  let status = 'queued'
  if (typeof j.status === 'string' && j.status.trim()) {
    status = j.status.trim().toLowerCase()
  }
  let progress: number | undefined
  if (typeof j.progress === 'number' && Number.isFinite(j.progress)) progress = j.progress
  let seconds: number | undefined
  if (typeof j.seconds === 'string') {
    const n = parseInt(j.seconds, 10)
    if (Number.isFinite(n)) seconds = n
  } else if (typeof j.seconds === 'number') {
    seconds = j.seconds
  } else if (typeof j.duration === 'number') {
    seconds = j.duration
  }
  let errMsg = ''
  const err = j.error
  if (err && typeof err === 'object' && err !== null) {
    const em = (err as { message?: string }).message
    if (typeof em === 'string') errMsg = em
  } else if (typeof err === 'string') {
    errMsg = err
  }
  return { videoId, status, progress, seconds, error: errMsg || undefined, raw }
}

/** POST /v1/videos（JSON） */
export async function videoCreate(
  apiKey: string,
  sticky: string,
  body: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<VideoJobSnapshot> {
  const res = await fetch('/v1/videos', {
    method: 'POST',
    headers: {
      ...videoAuthHeaders(apiKey, sticky, String(body.model ?? '')),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
    signal,
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  return parseVideoJobResponse(raw)
}

/** POST /v1/videos（multipart，含 input_reference 首帧图） */
export async function videoCreateMultipart(
  apiKey: string,
  sticky: string,
  opts: { model: string; prompt: string; seconds: string; size: string; file?: File },
  signal?: AbortSignal,
): Promise<VideoJobSnapshot> {
  const fd = new FormData()
  fd.append('model', opts.model.trim())
  fd.append('prompt', opts.prompt.trim())
  fd.append('seconds', opts.seconds.trim())
  if (opts.size.trim()) fd.append('size', opts.size.trim())
  if (opts.file) fd.append('input_reference', opts.file, opts.file.name)

  const res = await fetch('/v1/videos', {
    method: 'POST',
    headers: videoAuthHeaders(apiKey, sticky, opts.model),
    body: fd,
    signal,
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  return parseVideoJobResponse(raw)
}

/** GET /v1/videos/{id} */
export async function videoRetrieve(
  apiKey: string,
  sticky: string,
  model: string,
  videoId: string,
  signal?: AbortSignal,
): Promise<VideoJobSnapshot> {
  const q = new URLSearchParams({ model: model.trim() })
  const res = await fetch(`/v1/videos/${encodeURIComponent(videoId)}?${q}`, {
    method: 'GET',
    headers: videoAuthHeaders(apiKey, sticky, model),
    signal,
  })
  const raw = await res.text()
  if (!res.ok) {
    throw new Error((parseOpenAIError(raw) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  return parseVideoJobResponse(raw)
}

/** GET /v1/videos/{id}/content */
export async function videoContent(
  apiKey: string,
  sticky: string,
  model: string,
  videoId: string,
  signal?: AbortSignal,
): Promise<{ blob: Blob; contentType: string }> {
  const q = new URLSearchParams({ model: model.trim() })
  const res = await fetch(`/v1/videos/${encodeURIComponent(videoId)}/content?${q}`, {
    method: 'GET',
    headers: videoAuthHeaders(apiKey, sticky, model),
    signal,
  })
  const contentType = res.headers.get('Content-Type')?.trim() || 'video/mp4'
  if (!res.ok) {
    const text = await res.text()
    throw new Error((parseOpenAIError(text) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
  const blob = await res.blob()
  return { blob, contentType }
}

/** DELETE /v1/videos/{id} */
export async function videoDelete(
  apiKey: string,
  sticky: string,
  model: string,
  videoId: string,
): Promise<void> {
  const q = new URLSearchParams({ model: model.trim() })
  const res = await fetch(`/v1/videos/${encodeURIComponent(videoId)}?${q}`, {
    method: 'DELETE',
    headers: videoAuthHeaders(apiKey, sticky, model),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error((parseOpenAIError(text) || `${res.status}`) + traceHubGatewayErrorDetails(res))
  }
}
