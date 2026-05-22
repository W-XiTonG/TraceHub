/** 用户侧模型价格展示（与 GET /api/user/available-models 的 pricing 字段对齐）。 */

import { modelLooksVideo } from '../lib/usageLedgerDetail'

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

/** 展示用计费粒度（与后台 quota_in 语义对齐：视频为每秒额度）。 */
export type PricingUnitKind = 'token' | 'video_second' | 'per_request' | 'hybrid_floor'

export type ModelPricingDisplay = {
  /** 主行：法币参考价 */
  fiatPrimary: string
  /** 副行：额度单位 */
  quotaSecondary: string
  /** 价格下方小字：计费单位说明 */
  unitHint: string
  /** 搜索/筛选用全文 */
  searchBlob: string
  free: boolean
  unified: boolean
  unitKind: PricingUnitKind
}

export type ModelPricingDisplayLabels = {
  free: string
  in: string
  out: string
  unified: string
  perMillion: string
  perSecond: string
  quotaPer1k: string
  quotaPerSec: string
  quotaUnit: string
  quotaPerRequest: string
  perRequest: string
  hybridFloor: string
  symbolUsd: string
  symbolCny: string
  unitHintToken: string
  unitHintVideo: string
  unitHintPerRequest: string
  unitHintHybrid: string
}

export type ModelPricingDisplayOpts = {
  /** 可用模型 labels（含 video 时优先） */
  labels?: string[]
  /** 模型 ID，用于无 labels 时的视频启发式 */
  modelId?: string
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

/** 每秒额度 → 每秒人民币 */
export function quotaPerSecToYuan(quotaPerSec: number, quotaPerYuan: number): number {
  if (!(quotaPerSec >= 0) || !(quotaPerYuan > 0)) return 0
  return quotaPerSec / quotaPerYuan
}

/** 每秒额度 → 每秒美元 */
export function quotaPerSecToUsd(quotaPerSec: number, factors: SiteCurrencyFactors): number {
  if (!(quotaPerSec >= 0)) return 0
  if (factors.quotaPerUsd > 0) {
    return quotaPerSec / factors.quotaPerUsd
  }
  if (factors.quotaPerYuan > 0 && factors.cnyPerUsd > 0) {
    return quotaPerSecToYuan(quotaPerSec, factors.quotaPerYuan) / factors.cnyPerUsd
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

function fiatPerSecond(
  quotaPerSec: number,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
): number {
  if (currency === 'cny') {
    return quotaPerSecToYuan(quotaPerSec, factors.quotaPerYuan)
  }
  return quotaPerSecToUsd(quotaPerSec, factors)
}

function quotaPerRequestToFiat(
  quota: number,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
): number {
  if (!(quota >= 0)) return 0
  if (currency === 'cny' && factors.quotaPerYuan > 0) {
    return quota / factors.quotaPerYuan
  }
  if (currency === 'usd' && factors.quotaPerUsd > 0) {
    return quota / factors.quotaPerUsd
  }
  if (currency === 'usd' && factors.quotaPerYuan > 0 && factors.cnyPerUsd > 0) {
    return quota / factors.quotaPerYuan / factors.cnyPerUsd
  }
  return 0
}

function canShowFiat(currency: PriceCurrency, factors: SiteCurrencyFactors): boolean {
  if (currency === 'cny') return factors.quotaPerYuan > 0
  return factors.quotaPerUsd > 0 || (factors.quotaPerYuan > 0 && factors.cnyPerUsd > 0)
}

export function resolvePricingUnitKind(
  pricing: ModelPricing | undefined | null,
  opts?: ModelPricingDisplayOpts,
): PricingUnitKind {
  if (!pricing?.billable) return 'token'
  if (pricing.pricing_unit === 'per_request') {
    return pricing.billing_mode === 'hybrid_floor' ? 'hybrid_floor' : 'per_request'
  }
  if (pricing.pricing_unit === 'per_second') return 'video_second'
  const labels = opts?.labels ?? []
  if (labels.includes('video')) return 'video_second'
  if (modelLooksVideo(opts?.modelId ?? '')) return 'video_second'
  return 'token'
}

function unitHintForKind(kind: PricingUnitKind, labels: ModelPricingDisplayLabels): string {
  switch (kind) {
    case 'video_second':
      return labels.unitHintVideo
    case 'per_request':
      return labels.unitHintPerRequest
    case 'hybrid_floor':
      return labels.unitHintHybrid
    default:
      return labels.unitHintToken
  }
}

function formatTokenFiatLines(
  qIn: number,
  qOut: number,
  unified: boolean,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
  labels: ModelPricingDisplayLabels,
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

function formatVideoFiatLines(
  qIn: number,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
  labels: ModelPricingDisplayLabels,
): { fiatPrimary: string; quotaSecondary: string } {
  const sym = currency === 'usd' ? labels.symbolUsd : labels.symbolCny
  const showFiat = canShowFiat(currency, factors)
  const amt = showFiat ? fiatPerSecond(qIn, currency, factors) : 0
  const fiatPrimary = showFiat
    ? `${labels.in} ${sym}${fmtFiat(amt)} ${labels.perSecond}`
    : `${labels.in} ${qIn} ${labels.quotaPerSec}`
  const quotaSecondary = showFiat ? `${qIn} ${labels.quotaPerSec}` : ''
  return { fiatPrimary, quotaSecondary }
}

export function formatModelPricingDisplay(
  pricing: ModelPricing | undefined | null,
  currency: PriceCurrency,
  factors: SiteCurrencyFactors,
  labels: ModelPricingDisplayLabels,
  opts?: ModelPricingDisplayOpts,
): ModelPricingDisplay {
  const unitKind = resolvePricingUnitKind(pricing, opts)
  const unitHint = unitHintForKind(unitKind, labels)

  if (!pricing) {
    return {
      fiatPrimary: '',
      quotaSecondary: '',
      unitHint: '',
      searchBlob: '',
      free: false,
      unified: false,
      unitKind,
    }
  }
  if (!pricing.billable) {
    return {
      fiatPrimary: labels.free,
      quotaSecondary: '',
      unitHint: '',
      searchBlob: labels.free,
      free: true,
      unified: false,
      unitKind,
    }
  }
  if (pricing.pricing_unit === 'per_request' && (pricing.quota_per_request ?? 0) > 0) {
    const q = pricing.quota_per_request!
    const isHybrid = pricing.billing_mode === 'hybrid_floor'
    const unitLabel = isHybrid ? labels.hybridFloor : labels.quotaPerRequest
    const quotaLine = `${q} ${unitLabel}`
    const showFiat = canShowFiat(currency, factors)
    const sym = currency === 'usd' ? labels.symbolUsd : labels.symbolCny
    const fiatAmt = showFiat ? quotaPerRequestToFiat(q, currency, factors) : 0
    const fiatPrimary =
      showFiat && fiatAmt > 0
        ? `${sym}${fmtFiat(fiatAmt)}${labels.perRequest}`
        : quotaLine
    const quotaSecondary = showFiat && fiatAmt > 0 ? quotaLine : ''
    const searchBlob = [fiatPrimary, quotaSecondary, unitHint].filter(Boolean).join(' ')
    return {
      fiatPrimary,
      quotaSecondary,
      unitHint,
      searchBlob,
      free: false,
      unified: true,
      unitKind,
    }
  }

  const qIn = pricing.quota_in_per_1k ?? 0
  const qOut = pricing.quota_out_per_1k ?? 0
  const unified = qIn === qOut

  let fiatPrimary: string
  let quotaSecondary: string
  if (unitKind === 'video_second') {
    const lines = formatVideoFiatLines(qIn, currency, factors, labels)
    fiatPrimary = lines.fiatPrimary
    quotaSecondary = lines.quotaSecondary
  } else {
    const lines = formatTokenFiatLines(qIn, qOut, unified, currency, factors, labels)
    fiatPrimary = lines.fiatPrimary
    quotaSecondary = lines.quotaSecondary
  }

  const cnyLines =
    unitKind === 'video_second'
      ? formatVideoFiatLines(qIn, 'cny', factors, labels)
      : formatTokenFiatLines(qIn, qOut, unified, 'cny', factors, labels)
  const usdLines =
    unitKind === 'video_second'
      ? formatVideoFiatLines(qIn, 'usd', factors, labels)
      : formatTokenFiatLines(qIn, qOut, unified, 'usd', factors, labels)
  const searchBlob = [
    cnyLines.fiatPrimary,
    cnyLines.quotaSecondary,
    usdLines.fiatPrimary,
    usdLines.quotaSecondary,
    unitHint,
  ]
    .filter(Boolean)
    .join(' ')

  return {
    fiatPrimary,
    quotaSecondary,
    unitHint,
    searchBlob,
    free: false,
    unified: unitKind === 'video_second' ? true : unified,
    unitKind,
  }
}

export function modelTitle(displayName: string | undefined, id: string): string {
  const dn = (displayName ?? '').trim()
  return dn || id
}

export function modelHasDisplayAlias(displayName: string | undefined): boolean {
  return (displayName ?? '').trim().length > 0
}
