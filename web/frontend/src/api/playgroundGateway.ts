/** 同源调用 TraceHub OpenAI 兼容网关 /v1（使用 API Key，而非控制台 JWT） */

import { http, type Envelope } from './http'

const STICKY_HDR = 'X-TraceHub-Session'

export type PlaygroundRouterHealth = 'healthy' | 'unhealthy' | 'catalog_only'

export type PlaygroundRegistryMeta = {
  upstream_list_status?: string
  upstream_list_checked_at?: string
  in_gateway?: boolean
}

export type PlaygroundModelRow = {
  id: string
  routerHealth?: PlaygroundRouterHealth
} & PlaygroundRegistryMeta

function parseRouterHealth(v: unknown): PlaygroundRouterHealth | undefined {
  if (v === 'healthy' || v === 'unhealthy' || v === 'catalog_only') return v
  return undefined
}

/** GET /v1/models，带路由健康扩展字段（与网关选路 health_status 一致，非实时探测上游）。 */
export async function fetchPlaygroundModels(apiKey: string, sticky: string): Promise<PlaygroundModelRow[]> {
  const res = await fetch('/v1/models?tracehub_include_router_health=1', {
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
    data?: Array<{ id?: string; tracehub?: { router_health?: string } }>
  }
  const rows: PlaygroundModelRow[] = []
  for (const m of data.data || []) {
    const id = typeof m.id === 'string' ? m.id : ''
    if (!id) continue
    rows.push({ id, routerHealth: parseRouterHealth(m.tracehub?.router_health) })
  }
  rows.sort((a, b) => a.id.localeCompare(b.id))
  return rows
}

type AvailModelItem = {
  id: string
  upstream_list_status?: string
  upstream_list_checked_at?: string
  in_gateway?: boolean
}

/**
 * 拉取当前登录账户在「可用模型」页的登记元数据，用于与试玩 GET /v1/models 结果按 id 合并。
 * 未登录、无令牌或请求失败时返回 null（不影响网关模型列表）。
 */
export async function fetchPlaygroundRegistryMap(): Promise<Map<string, PlaygroundRegistryMeta> | null> {
  try {
    const { data } = await http.get<Envelope<{ items: AvailModelItem[] }>>('/api/user/available-models')
    if (data.code !== 0 || !data.data?.items) return null
    const map = new Map<string, PlaygroundRegistryMeta>()
    for (const it of data.data.items) {
      const id = typeof it.id === 'string' ? it.id.trim() : ''
      if (!id) continue
      map.set(id, {
        upstream_list_status: it.upstream_list_status,
        upstream_list_checked_at: it.upstream_list_checked_at,
        in_gateway: it.in_gateway,
      })
    }
    return map
  } catch {
    return null
  }
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
    }
  })
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

/** 网关在上游 4xx 或计费 402 时返回的诊断头（需 CORS Expose-Headers） */
export function traceHubGatewayErrorDetails(res: Response): string {
  const r = res.headers.get('X-TraceHub-Routing-Model')?.trim()
  const u = res.headers.get('X-TraceHub-Upstream-Model')?.trim()
  const cid = res.headers.get('X-TraceHub-Channel-Id')?.trim()
  let out = ''
  if (r || u || cid) {
    out += `\n\n[TraceHub 选路] routing_model=${r ?? '—'} | upstream_model=${u ?? '—'} | channel_id=${cid ?? '—'}\n若 upstream 与上游控制台不一致，请检查渠道「模型 ID / 别名」配置。`
  }
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
  const json = JSON.parse(raw) as { data?: Array<{ url?: string; b64_json?: string }> }
  const imageOutputs: string[] = []
  for (const d of json.data || []) {
    if (typeof d.url === 'string' && d.url.trim()) imageOutputs.push(d.url.trim())
    else if (typeof d.b64_json === 'string' && d.b64_json)
      imageOutputs.push(`data:image/png;base64,${d.b64_json}`)
  }
  return { raw, imageOutputs }
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
