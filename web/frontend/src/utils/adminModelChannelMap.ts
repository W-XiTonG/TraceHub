export type AdminModelChannelRef = {
  id: number
  channel_name: string
  channel_type: string
  healthy: boolean
}

type AdminChannelListItem = {
  id: number
  channel_name?: string
  channel_type?: string
  enabled?: boolean
  health_status?: number
  models?: string[]
  model_alias?: Record<string, string>
}

/** 汇总全站已启用渠道：逻辑模型 ID → 声明该模型的渠道列表。 */
export function buildModelChannelMap(channels: AdminChannelListItem[]): Map<string, AdminModelChannelRef[]> {
  const map = new Map<string, AdminModelChannelRef[]>()

  for (const ch of channels) {
    if (ch.enabled === false) continue
    const ref: AdminModelChannelRef = {
      id: ch.id,
      channel_name: (ch.channel_name || '').trim(),
      channel_type: (ch.channel_type || '').trim(),
      healthy: ch.health_status === 1,
    }
    const keys = new Set<string>()
    for (const m of ch.models || []) {
      const id = typeof m === 'string' ? m.trim() : ''
      if (id) keys.add(id)
    }
    if (ch.model_alias) {
      for (const alias of Object.keys(ch.model_alias)) {
        const a = alias.trim()
        if (a) keys.add(a)
      }
    }
    for (const modelId of keys) {
      const list = map.get(modelId) || []
      if (!list.some((x) => x.id === ref.id)) {
        list.push(ref)
        map.set(modelId, list)
      }
    }
  }

  for (const [k, list] of map) {
    list.sort((a, b) => {
      const la = a.channel_name || `#${a.id}`
      const lb = b.channel_name || `#${b.id}`
      return la.localeCompare(lb)
    })
    map.set(k, list)
  }
  return map
}

export function channelRefLabel(ch: AdminModelChannelRef): string {
  const name = ch.channel_name.trim()
  if (name) return `${name} (#${ch.id})`
  return `#${ch.id} · ${ch.channel_type || '?'}`
}
