/** 调用日志：将同一视频任务（创建 + 轮询/查询/精调）合并为可展开的一组 */

import { resolveUsageBillingMode, type UsageLogBillingRow } from './usageLedgerDetail'

export type UsageLogGroupRow = {
  id: number
  request_time: string
  model: string
  request_id?: string
  prompt_tokens?: number
  completion_tokens?: number
  total_cost?: number
  status_code?: number
  error_message?: string
  ip_address?: string
  response_time_ms?: number
  billing_mode?: string
  upstream_attempt?: number
  is_client_outcome?: boolean
  is_failover_attempt?: boolean
}

export type UsageLogGroup = {
  key: string
  primary: UsageLogGroupRow
  children: UsageLogGroupRow[]
  videoJobId: string
}

export function isFailoverIntermediateRow(row: {
  is_failover_attempt?: boolean
}): boolean {
  return row.is_failover_attempt === true
}

export function isFailoverOutcomeRow(row: {
  is_failover_attempt?: boolean
  is_client_outcome?: boolean
  upstream_attempt?: number
}): boolean {
  return (
    row.is_failover_attempt !== true &&
    row.is_client_outcome !== false &&
    (row.upstream_attempt ?? 1) > 1
  )
}

export function failoverAttemptBadgeKey(row: {
  upstream_attempt?: number
  is_failover_attempt?: boolean
}): string | null {
  if (!isFailoverIntermediateRow(row)) return null
  const n = row.upstream_attempt ?? 1
  return `failoverAttempt:${n}`
}

const VIDEO_TRACE_PREFIXES = [
  'video_job:',
  'video_settle_extra:',
  'video_settle_refund:',
  'kling_settle_extra:',
  'kling_settle_refund:',
]

const VIDEO_GROUP_WINDOW_MS = 30 * 60 * 1000

/** 从 error_message 解析视频任务 ID（网关写入的追踪字段，非用户可见错误） */
export function parseVideoJobId(errorMessage?: string): string {
  const m = (errorMessage ?? '').trim()
  if (!m) return ''
  for (const p of VIDEO_TRACE_PREFIXES) {
    if (m.startsWith(p)) {
      return m.slice(p.length).trim()
    }
  }
  return ''
}

export function isVideoJobTraceMessage(errorMessage?: string): boolean {
  const m = (errorMessage ?? '').trim()
  if (!m) return false
  return VIDEO_TRACE_PREFIXES.some((p) => m.startsWith(p))
}

function numTok(v?: number): number {
  if (typeof v === 'number' && Number.isFinite(v)) return Math.trunc(v)
  return 0
}

/** 与 resolveUsageBillingMode 一致：completion_tokens≠0 的 Token 对话不参与视频分组 */
function isVideoBillingRow(row: UsageLogGroupRow): boolean {
  return resolveUsageBillingMode(row as UsageLogBillingRow) === 'video_duration'
}

function isVideoPrimaryRow(row: UsageLogGroupRow): boolean {
  const model = row.model.trim()
  if (model.startsWith('route:') || !isVideoBillingRow(row)) return false
  const cost = row.total_cost ?? 0
  const sec = numTok(row.prompt_tokens)
  return cost > 0 || (row.status_code === 200 && sec > 0)
}

function isVideoChildRow(row: UsageLogGroupRow, head: UsageLogGroupRow, jobId: string): boolean {
  if (numTok(row.completion_tokens) > 0) return false
  if (!isVideoBillingRow(head)) return false
  const rowJob = parseVideoJobId(row.error_message)
  if (jobId && rowJob === jobId) return true
  const headTime = new Date(head.request_time).getTime()
  const rowTime = new Date(row.request_time).getTime()
  if (!Number.isFinite(headTime) || !Number.isFinite(rowTime)) return false
  if (Math.abs(rowTime - headTime) > VIDEO_GROUP_WINDOW_MS) return false
  const model = row.model.trim()
  if (model.startsWith('route:') && model.includes('/videos') && isVideoBillingRow(row)) return true
  const headModel = head.model.trim()
  if (isVideoBillingRow(row) && model === headModel && (row.total_cost ?? 0) === 0) return true
  return false
}

function collectChildrenForPrimary(
  rows: UsageLogGroupRow[],
  primary: UsageLogGroupRow,
  used: Set<number>,
): UsageLogGroupRow[] {
  const jobId = parseVideoJobId(primary.error_message)
  const children: UsageLogGroupRow[] = []
  for (const other of rows) {
    if (other.id === primary.id || used.has(other.id)) continue
    if (isVideoPrimaryRow(other)) continue
    if (!isVideoChildRow(other, primary, jobId)) continue
    children.push(other)
  }
  children.sort((a, b) => new Date(b.request_time).getTime() - new Date(a.request_time).getTime())
  return children
}

function pushGroup(
  groups: UsageLogGroup[],
  used: Set<number>,
  primary: UsageLogGroupRow,
  children: UsageLogGroupRow[],
  keyPrefix: string,
  jobId: string,
) {
  used.add(primary.id)
  for (const c of children) used.add(c.id)
  groups.push({
    key: `${keyPrefix}-${jobId || primary.id}`,
    videoJobId: jobId,
    primary,
    children,
  })
}

/**
 * 将当前页日志按视频任务分组（API 按时间倒序：轮询行往往排在创建行之前）。
 * 主行：带扣费或带秒数的视频创建；子行：同 job_id 或时间窗内的轮询/零扣费同行模型。
 */
export function groupVideoUsageLogs(rows: UsageLogGroupRow[]): {
  groups: UsageLogGroup[]
  standalone: UsageLogGroupRow[]
} {
  const used = new Set<number>()
  const groups: UsageLogGroup[] = []

  // 先按显式 video_job / settle 标签聚合
  const byJob = new Map<string, UsageLogGroupRow[]>()
  for (const row of rows) {
    const jid = parseVideoJobId(row.error_message)
    if (jid) {
      const list = byJob.get(jid) ?? []
      list.push(row)
      byJob.set(jid, list)
    }
  }
  for (const [jid, list] of byJob) {
    if (list.length < 2) continue
    const primary =
      list.find((r) => isVideoPrimaryRow(r)) ??
      list.find((r) => isVideoBillingRow(r) && !r.model.trim().startsWith('route:'))
    if (!primary) continue
    const children = list.filter(
      (r) => r.id !== primary.id && isVideoChildRow(r, primary, jid),
    )
    if (children.length === 0) continue
    children.sort((a, b) => new Date(b.request_time).getTime() - new Date(a.request_time).getTime())
    pushGroup(groups, used, primary, children, 'job', jid)
  }

  // 启发式：主行可能在列表任意位置，子行（轮询）常在主行上方
  const primaries = rows.filter((r) => !used.has(r.id) && isVideoPrimaryRow(r))
  primaries.sort((a, b) => new Date(b.request_time).getTime() - new Date(a.request_time).getTime())
  for (const primary of primaries) {
    if (used.has(primary.id)) continue
    const children = collectChildrenForPrimary(rows, primary, used)
    if (children.length === 0) continue
    pushGroup(groups, used, primary, children, 'row', parseVideoJobId(primary.error_message))
  }

  groups.sort(
    (a, b) => new Date(b.primary.request_time).getTime() - new Date(a.primary.request_time).getTime(),
  )

  const standalone: UsageLogGroupRow[] = []
  for (const row of rows) {
    if (!used.has(row.id)) standalone.push(row)
  }
  return { groups, standalone }
}

export type FlatUsageLogEntry =
  | { kind: 'group-primary'; group: UsageLogGroup }
  | { kind: 'group-child'; group: UsageLogGroup; row: UsageLogGroupRow }
  | { kind: 'standalone'; row: UsageLogGroupRow }

export function flattenGroupedUsageLogs(
  groups: UsageLogGroup[],
  standalone: UsageLogGroupRow[],
  expandedKeys: Set<string>,
): FlatUsageLogEntry[] {
  const out: FlatUsageLogEntry[] = []
  const all = [
    ...groups.map((g) => ({ t: new Date(g.primary.request_time).getTime(), g })),
    ...standalone.map((r) => ({ t: new Date(r.request_time).getTime(), r })),
  ].sort((a, b) => b.t - a.t)

  for (const item of all) {
    if ('g' in item && item.g) {
      const g = item.g
      out.push({ kind: 'group-primary', group: g })
      if (expandedKeys.has(g.key) && g.children.length) {
        for (const child of g.children) {
          out.push({ kind: 'group-child', group: g, row: child })
        }
      }
    } else if ('r' in item && item.r) {
      out.push({ kind: 'standalone', row: item.r })
    }
  }
  return out
}
