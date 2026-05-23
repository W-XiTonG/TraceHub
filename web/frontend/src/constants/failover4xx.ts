/** 管理端可勾选的 4xx 状态码（429 始终转移，不在此列表）。 */
export const FAILOVER_4XX_CODE_OPTIONS = [400, 401, 403, 404, 408, 413, 422] as const

export const DEFAULT_SELECTIVE_FAILOVER_4XX_CODES = [400, 401, 403, 408]

export function parseFailover4xxCodes(raw: unknown): number[] {
  const allowed = new Set<number>(FAILOVER_4XX_CODE_OPTIONS)
  const out = new Set<number>()
  if (Array.isArray(raw)) {
    for (const item of raw) {
      const n = Number(item)
      if (Number.isFinite(n) && allowed.has(n)) out.add(n)
    }
  } else if (typeof raw === 'string' && raw.trim()) {
    for (const part of raw.split(/[,;\s]+/)) {
      const n = parseInt(part.trim(), 10)
      if (Number.isFinite(n) && allowed.has(n)) out.add(n)
    }
  }
  return [...out].sort((a, b) => a - b)
}

export function effectiveFailover4xxCodes(mode: string, configured: number[]): number[] {
  if (configured.length > 0) return configured
  if (mode === 'selective') return [...DEFAULT_SELECTIVE_FAILOVER_4XX_CODES]
  return []
}
