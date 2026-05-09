package billing

// DefaultQuotaPer1K 默认每千 Token 计费（额度单位），可按模型覆盖；管理员可通过后续版本写入 DB。
var DefaultQuotaPer1K = map[string]struct {
	In  int64
	Out int64
}{
	"gpt-4o":                     {300, 1200},
	"gpt-4o-mini":                {50, 200},
	"gpt-4-turbo":                {500, 1500},
	"gpt-3.5-turbo":              {50, 150},
	"claude-3-5-sonnet-20241022": {300, 1500},
	"claude-3-opus-20240229":     {1500, 7500},
	"gemini-2.0-flash":           {50, 200},
	"gemini-1.5-pro":             {250, 1000},
	"deepseek-chat":              {40, 120},
	"deepseek-reasoner":          {120, 360},
}

func LookupPricing(model string) (inPer1k, outPer1k int64) {
	if p, ok := DefaultQuotaPer1K[model]; ok {
		return p.In, p.Out
	}
	return 100, 400
}
