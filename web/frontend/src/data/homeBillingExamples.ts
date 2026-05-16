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
  { model: 'gpt-4', inPer1k: 600, outPer1k: 1800 },
  { model: 'gpt-3.5-turbo', inPer1k: 50, outPer1k: 150 },
  { model: 'gpt-5.5', inPer1k: 525, outPer1k: 3360 },
  { model: 'gpt-5', inPer1k: 450, outPer1k: 1800 },
  { model: 'o3-mini', inPer1k: 200, outPer1k: 800 },
  { model: 'o3', inPer1k: 500, outPer1k: 2000 },
  { model: 'o1-mini', inPer1k: 400, outPer1k: 1600 },
  { model: 'o1', inPer1k: 2500, outPer1k: 10000 },
  { model: 'claude-3-5-sonnet-20241022', inPer1k: 300, outPer1k: 1500 },
  { model: 'claude-3-5-haiku-20241022', inPer1k: 25, outPer1k: 125 },
  { model: 'claude-3-opus-20240229', inPer1k: 1500, outPer1k: 7500 },
  { model: 'gemini-2.5-pro', inPer1k: 300, outPer1k: 1200 },
  { model: 'gemini-2.5-flash', inPer1k: 50, outPer1k: 200 },
  { model: 'gemini-2.0-flash', inPer1k: 50, outPer1k: 200 },
  { model: 'gemini-1.5-pro', inPer1k: 250, outPer1k: 1000 },
  { model: 'deepseek-chat', inPer1k: 40, outPer1k: 120 },
  { model: 'deepseek-reasoner', inPer1k: 120, outPer1k: 360 },
  { model: 'text-embedding-3-large', inPer1k: 65, outPer1k: 65 },
  { model: 'text-embedding-3-small', inPer1k: 15, outPer1k: 15 },
  { model: 'text-embedding-ada-002', inPer1k: 10, outPer1k: 10 },
  { model: 'gpt-image-2', inPer1k: 480, outPer1k: 2880 },
].sort((a, b) => a.inPer1k + a.outPer1k - (b.inPer1k + b.outPer1k))
