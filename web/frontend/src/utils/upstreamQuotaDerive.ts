/** 与 internal/service/billing/upstream_quota_derive.go 正向/反向公式保持一致（展示用） */

export type SitePricingFactors = {
  quotaPerYuan: number
  quotaPerUsd: number
  cnyPerUsd: number
}

export function upstreamPriceFromQuotaPer1K(
  factors: SitePricingFactors,
  currency: 'cny' | 'usd',
  denomTokens: number,
  quotaPer1k: number,
  marginPercent: number,
): number | null {
  if (quotaPer1k < 0 || !Number.isFinite(quotaPer1k)) return null
  if (quotaPer1k === 0) return 0
  if (denomTokens !== 1 && denomTokens !== 1000 && denomTokens !== 1_000_000) return null
  if (!Number.isFinite(marginPercent) || marginPercent <= -100) return null

  const mul = 1 + marginPercent / 100
  const costPer1k = quotaPer1k / mul
  const scale = denomTokens / 1000

  if (currency === 'cny') {
    if (factors.quotaPerYuan <= 0) return null
    return (costPer1k / factors.quotaPerYuan) * scale
  }
  if (factors.quotaPerUsd > 0) {
    return (costPer1k / factors.quotaPerUsd) * scale
  }
  if (factors.cnyPerUsd > 0 && factors.quotaPerYuan > 0) {
    return (costPer1k / (factors.cnyPerUsd * factors.quotaPerYuan)) * scale
  }
  return null
}

export function quotaPer1KFromUpstreamPrice(
  factors: SitePricingFactors,
  currency: 'cny' | 'usd',
  denomTokens: number,
  price: number,
  marginPercent: number,
): number | null {
  if (price < 0 || !Number.isFinite(price)) return null
  if (denomTokens !== 1 && denomTokens !== 1000 && denomTokens !== 1_000_000) return null
  if (!Number.isFinite(marginPercent) || marginPercent <= -100) return null

  const costPer1k = price * (1000 / denomTokens)
  const mul = 1 + marginPercent / 100
  let raw: number
  if (currency === 'cny') {
    if (factors.quotaPerYuan <= 0) return null
    raw = costPer1k * factors.quotaPerYuan * mul
  } else if (factors.quotaPerUsd > 0) {
    raw = costPer1k * factors.quotaPerUsd * mul
  } else if (factors.cnyPerUsd > 0 && factors.quotaPerYuan > 0) {
    raw = costPer1k * factors.cnyPerUsd * factors.quotaPerYuan * mul
  } else {
    return null
  }
  return Math.max(0, Math.ceil(raw))
}

export function formatUpstreamPrice(n: number): string {
  if (!Number.isFinite(n)) return '—'
  const abs = Math.abs(n)
  if (abs >= 100) return n.toFixed(2)
  if (abs >= 1) return n.toFixed(4)
  if (abs >= 0.0001) return n.toFixed(6)
  return n.toExponential(3)
}

export function parseUpstreamPrice(raw: string): number | null {
  const s = raw.trim()
  if (s === '') return null
  const n = Number(s)
  if (!Number.isFinite(n) || n < 0) return null
  return n
}
