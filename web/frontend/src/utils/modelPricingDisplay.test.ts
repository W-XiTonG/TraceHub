import { describe, expect, it } from 'vitest'
import { formatModelPricingDisplay, quotaPerSecToYuan, type SiteCurrencyFactors } from './modelPricingDisplay'

const factors: SiteCurrencyFactors = { quotaPerYuan: 10000, quotaPerUsd: 0, cnyPerUsd: 7.2 }

const labels = {
  free: '免费',
  in: '入',
  out: '出',
  unified: '统一',
  perMillion: '/ 百万 token',
  perSecond: '/ 秒',
  quotaPer1k: '额度/千 token',
  quotaPerSec: '额度/秒',
  quotaUnit: '额度/千 token',
  quotaPerRequest: '额度/次',
  perRequest: '/ 次',
  hybridFloor: '额度/次（底价）',
  symbolUsd: '$',
  symbolCny: '¥',
  unitHintToken: '按 Token',
  unitHintVideo: '按秒',
  unitHintPerRequest: '按次',
  unitHintHybrid: '混合',
}

describe('formatModelPricingDisplay', () => {
  it('video model uses per-second fiat and hint', () => {
    const d = formatModelPricingDisplay(
      { billable: true, quota_in_per_1k: 800, quota_out_per_1k: 0, pricing_unit: 'per_1k_tokens' },
      'cny',
      factors,
      labels,
      { labels: ['video'], modelId: 'sora-2' },
    )
    expect(d.unitKind).toBe('video_second')
    expect(d.unitHint).toBe('按秒')
    expect(d.fiatPrimary).toContain('/ 秒')
    expect(d.quotaSecondary).toContain('额度/秒')
    expect(quotaPerSecToYuan(800, 10000)).toBeCloseTo(0.08, 5)
  })

  it('per_second pricing_unit from API uses video display', () => {
    const d = formatModelPricingDisplay(
      { billable: true, quota_in_per_1k: 900, quota_out_per_1k: 0, pricing_unit: 'per_second' },
      'cny',
      factors,
      labels,
      { labels: ['chat'], modelId: 'doubao-seedance-1-5-pro-251215' },
    )
    expect(d.unitKind).toBe('video_second')
    expect(d.unitHint).toBe('按秒')
    expect(d.fiatPrimary).toContain('/ 秒')
  })

  it('chat model keeps per-million token display', () => {
    const d = formatModelPricingDisplay(
      { billable: true, quota_in_per_1k: 50, quota_out_per_1k: 200, pricing_unit: 'per_1k_tokens' },
      'cny',
      factors,
      labels,
      { labels: ['chat'], modelId: 'gpt-4o-mini' },
    )
    expect(d.unitKind).toBe('token')
    expect(d.fiatPrimary).toContain('/ 百万 token')
    expect(d.unitHint).toBe('按 Token')
  })
})
