/** 用户侧模型价格展示（与 GET /api/user/available-models 的 pricing 字段对齐）。 */

export type ModelPricing = {
  billable: boolean
  quota_in_per_1k?: number
  quota_out_per_1k?: number
  quota_per_request?: number
  pricing_unit?: string
  billing_mode?: string
}

export type PriceCurrency = 'usd' | 'cny'

export type SiteCurrencyFactors = {
  quotaPerYuan: number
  quotaPerUsd: number
  cnyPerUsd: number
}

export type ModelPricingDisplay = {
  /** 主行：法币（每百万 token 或统一价） */
  fiatPrimary: string
  /** 副行：额度/千 token */
  quotaSecondary: string
  /** 搜索/筛选用全文 */
  searchBlob: string
  free: boolean
  unified: boolean
}

function fmtFiat(amount: number): string {
  if (!Number.isFinite(amount) || amount < 0) return ''
  if (amount === 0) return '0'
  if (amount >= 100) return amount.toFixed(2)
  if (amount >= 1) return amount.toFixed(3)
  if (amount >= 0.01) return amount.toFixed(4)
  return amount.toFixed(6).replace(/\.?0+$/, '')
}

/** 每千 token 额度 → 每百万 token 人民币 */
export function quotaPer1kToYuanPerMillion(quotaPer1k: number, quotaPerYuan: number): number {
  if (!(quotaPer1k >= 0) || !(quotaPerYuan > 0)) return 0
  return (quotaPer1k * 1000) / quotaPerYuan
}

/** 每千 token 额度 → 每百万 token 美元（与 billing 标价换算路径一致） */
export function quotaPer1kToUsdPerMillion(
  quotaPer1k: number,
  factors: SiteCurrencyFactors,
): number {
  if (!(quotaPer1k >= 0)) return 0
  if (factors.quotaPerUsd > 0) {
    return (quotaPer1k * 1000) / factors.quotaPerUsd
  }
  if (factors.quotaPerYuan > 0 && factors.cnyPerUsd > 0) {
    const yuan = quotaPer1kToYuanPerMillion(quotaPer1k, factors.quotaPerYuan)
    return yuan / factors.cnyPerUsd
  }
  return 0
}

function fiatPerMillion(
  quotaPer1k: number,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
): number {
  if (currency === 'cny') {
    return quotaPer1kToYuanPerMillion(quotaPer1k, factors.quotaPerYuan)
  }
  return quotaPer1kToUsdPerMillion(quotaPer1k, factors)
}

function canShowFiat(currency: PriceCurrency, factors: SiteCurrencyFactors): boolean {
  if (currency === 'cny') return factors.quotaPerYuan > 0
  return factors.quotaPerUsd > 0 || (factors.quotaPerYuan > 0 && factors.cnyPerUsd > 0)
}

function formatFiatLines(
  qIn: number,
  qOut: number,
  unified: boolean,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
  labels: {
    in: string
    out: string
    unified: string
    perMillion: string
    quotaPer1k: string
    quotaUnit: string
    symbolUsd: string
    symbolCny: string
  },
): { fiatPrimary: string; quotaSecondary: string } {
  const sym = currency === 'usd' ? labels.symbolUsd : labels.symbolCny
  const showFiat = canShowFiat(currency, factors)

  if (unified) {
    const amt = showFiat ? fiatPerMillion(qIn, currency, factors) : 0
    const fiatPrimary =
      showFiat
        ? `${labels.unified} ${sym}${fmtFiat(amt)} ${labels.perMillion}`
        : `${labels.unified} ${qIn} ${labels.quotaPer1k}`
    const quotaSecondary = showFiat ? `${labels.unified} ${qIn} ${labels.quotaUnit}` : ''
    return { fiatPrimary, quotaSecondary }
  }

  const amtIn = showFiat ? fiatPerMillion(qIn, currency, factors) : 0
  const amtOut = showFiat ? fiatPerMillion(qOut, currency, factors) : 0
  const fiatPrimary = showFiat
    ? `${labels.in} ${sym}${fmtFiat(amtIn)} / ${labels.out} ${sym}${fmtFiat(amtOut)} ${labels.perMillion}`
    : `${labels.in} ${qIn} / ${labels.out} ${qOut} ${labels.quotaPer1k}`
  const quotaSecondary = showFiat
    ? `${labels.in} ${qIn} / ${labels.out} ${qOut} ${labels.quotaUnit}`
    : ''
  return { fiatPrimary, quotaSecondary }
}

export function formatModelPricingDisplay(
  pricing: ModelPricing | undefined | null,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
  labels: {
    free: string
    in: string
    out: string
    unified: string
    perMillion: string
    quotaPer1k: string
    quotaUnit: string
    symbolUsd: string
    symbolCny: string
  },
): ModelPricingDisplay {
  if (!pricing) {
    return { fiatPrimary: '', quotaSecondary: '', searchBlob: '', free: false, unified: false }
  }
  if (!pricing.billable) {
    return {
      fiatPrimary: labels.free,
      quotaSecondary: '',
      searchBlob: labels.free,
      free: true,
      unified: false,
    }
  }
  if (pricing.pricing_unit === 'per_request' && (pricing.quota_per_request ?? 0) > 0) {
    const q = pricing.quota_per_request!
    const sec = `${q} ${labels.quotaUnit}/次`
    return {
      fiatPrimary: sec,
      quotaSecondary: '',
      searchBlob: sec,
      free: false,
      unified: true,
    }
  }
  const qIn = pricing.quota_in_per_1k ?? 0
  const qOut = pricing.quota_out_per_1k ?? 0
  const unified = qIn === qOut

  const { fiatPrimary, quotaSecondary } = formatFiatLines(qIn, qOut, unified, currency, factors, labels)
  const cnyLines = formatFiatLines(qIn, qOut, unified, 'cny', factors, labels)
  const usdLines = formatFiatLines(qIn, qOut, unified, 'usd', factors, labels)
  const searchBlob = [cnyLines.fiatPrimary, cnyLines.quotaSecondary, usdLines.fiatPrimary, usdLines.quotaSecondary]
    .filter(Boolean)
    .join(' ')

  return {
    fiatPrimary,
    quotaSecondary,
    searchBlob,
    free: false,
    unified,
  }
}

export function modelTitle(displayName: string | undefined, id: string): string {
  const dn = (displayName ?? '').trim()
  return dn || id
}

export function modelHasDisplayAlias(displayName: string | undefined): boolean {
  return (displayName ?? '').trim().length > 0
}
