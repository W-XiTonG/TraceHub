import { http, type Envelope } from '../api/http'

type WorkspaceItem = { model_name?: string }

/** 分页拉取全局或单渠道计价工作区中的全部 model_name（与 AdminSettingsGlobalPricingTab 一致）。 */
export async function fetchAllWorkspaceModelIds(opts?: {
  channelId?: number
  q?: string
}): Promise<string[]> {
  const chunk = 200
  let p = 1
  let totalCount = 0
  const byName = new Set<string>()
  const workspacePath = opts?.channelId
    ? '/api/admin/channel-model-pricing/workspace'
    : '/api/admin/global-model-pricing/workspace'
  const q = opts?.q?.trim() || undefined

  for (;;) {
    const params: Record<string, string | number> = { page: p, page_size: chunk }
    if (q) params.q = q
    if (opts?.channelId) params.channel_id = opts.channelId

    const { data } = await http.get<
      Envelope<{ items: WorkspaceItem[]; total: number; page: number; page_size: number }>
    >(workspacePath, { params })
    if (data.code !== 0) break
    totalCount = typeof data.data.total === 'number' ? data.data.total : 0
    const items = data.data.items || []
    for (const w of items) {
      const k = typeof w.model_name === 'string' ? w.model_name.trim() : ''
      if (k) byName.add(k)
    }
    if (items.length === 0 || byName.size >= totalCount || items.length < chunk) break
    p++
  }
  return Array.from(byName).sort((a, b) => a.localeCompare(b))
}
