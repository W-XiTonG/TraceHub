import { describe, expect, it } from 'vitest'
import { isVideoDurationBilling, modelLooksVideo, resolveUsageBillingMode } from './usageLedgerDetail'

describe('modelLooksVideo', () => {
  it('matches seedance models', () => {
    expect(modelLooksVideo('doubao-seedance-1-5-pro-251215')).toBe(true)
  })
  it('matches videos route placeholder', () => {
    expect(modelLooksVideo('route:/v1/videos/:video_id/content')).toBe(true)
  })
  it('does not match plain chat', () => {
    expect(modelLooksVideo('gpt-4o-mini')).toBe(false)
  })
})

describe('resolveUsageBillingMode', () => {
  it('video model with zero tokens still shows video billing', () => {
    expect(
      resolveUsageBillingMode({
        model: 'doubao-seedance-1-0-pro-fast-251015',
        billing_mode: 'token',
        prompt_tokens: 0,
        completion_tokens: 0,
      }),
    ).toBe('video_duration')
  })
})

describe('isVideoDurationBilling', () => {
  it('uses billing_mode from API', () => {
    expect(isVideoDurationBilling({ billing_mode: 'video_duration', prompt_tokens: 8 })).toBe(true)
  })
  it('infers from seconds in prompt_tokens', () => {
    expect(
      isVideoDurationBilling({
        model: 'doubao-seedance-1-5-pro-251215',
        prompt_tokens: 8,
        completion_tokens: 0,
      }),
    ).toBe(true)
  })
})
