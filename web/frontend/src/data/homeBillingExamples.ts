/** 首页参考表：与 internal/service/billing/pricing.go 中 DefaultQuotaPer1K 保持一致；任一侧变更时请同步另一侧。 */
export type HomeBillingExampleRow = {
  model: string
  inPer1k: number
  outPer1k: number
}

export const homeBillingExampleRows: HomeBillingExampleRow[] = [
  { model: 'gpt-4o', inPer1k: 300, outPer1k: 1200 },
  { model: 'gpt-4o-mini', inPer1k: 50, outPer1k: 200 },
  { model: 'gpt-4-turbo', inPer1k: 500, outPer1k: 1500 },
  { model: 'gpt-3.5-turbo', inPer1k: 50, outPer1k: 150 },
  { model: 'claude-3-5-sonnet-20241022', inPer1k: 300, outPer1k: 1500 },
  { model: 'claude-3-opus-20240229', inPer1k: 1500, outPer1k: 7500 },
  { model: 'gemini-2.0-flash', inPer1k: 50, outPer1k: 200 },
  { model: 'gemini-1.5-pro', inPer1k: 250, outPer1k: 1000 },
  { model: 'deepseek-chat', inPer1k: 40, outPer1k: 120 },
  { model: 'deepseek-reasoner', inPer1k: 120, outPer1k: 360 },
].sort((a, b) => a.inPer1k + a.outPer1k - (b.inPer1k + b.outPer1k))
