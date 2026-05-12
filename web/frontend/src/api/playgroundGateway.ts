/** 同源调用 TraceHub OpenAI 兼容网关 /v1（使用 API Key，而非控制台 JWT） */

const STICKY_HDR = 'X-TraceHub-Session'

export type PlaygroundRouterHealth = 'healthy' | 'unhealthy' | 'catalog_only'

export type PlaygroundModelRow = {
  id: string
  routerHealth?: PlaygroundRouterHealth
}

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

function parseOpenAIError(text: string): string {
  try {
    const j = JSON.parse(text) as { error?: { message?: string } }
    return j?.error?.message || ''
  } catch {
    return text.slice(0, 500)
  }
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
    throw new Error(parseOpenAIError(raw) || `${res.status}`)
  }
  const json = JSON.parse(raw) as {
    choices?: Array<{ message?: { content?: string | null } }>
  }
  const content = json.choices?.[0]?.message?.content ?? ''
  return { content: typeof content === 'string' ? content : '', raw }
}
