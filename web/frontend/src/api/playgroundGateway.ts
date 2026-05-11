/** 同源调用 TraceHub OpenAI 兼容网关 /v1（使用 API Key，而非控制台 JWT） */

const STICKY_HDR = 'X-TraceHub-Session'

export async function fetchModelIds(apiKey: string, sticky: string): Promise<string[]> {
  const res = await fetch('/v1/models', {
    headers: {
      Authorization: `Bearer ${apiKey.trim()}`,
      [STICKY_HDR]: sticky,
    },
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(parseOpenAIError(text) || `${res.status} ${res.statusText}`)
  }
  const data = (await res.json()) as { data?: Array<{ id?: string }> }
  const ids = (data.data || []).map((m) => m.id).filter(Boolean) as string[]
  return ids.sort()
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
