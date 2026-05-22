import { describe, expect, it } from 'vitest'
import { isVideoDurationBilling, modelLooksVideo } from './usageLedgerDetail'

describe('modelLooksVideo', () => {
  it('matches seedance models', () => {
    expect(modelLooksVideo('doubao-seedance-1-5-pro-251215')).toBe(true)
  })
  it('does not match plain chat', () => {
    expect(modelLooksVideo('gpt-4o-mini')).toBe(false)
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
