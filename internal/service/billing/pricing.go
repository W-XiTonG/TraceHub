package billing

import (
	"regexp"
	"sort"
	"strings"
)

// DefaultQuotaPer1K 默认每千 Token 计费（额度单位），可按模型覆盖；管理员可通过后续版本写入 DB。
// 若调整本表，请同步更新前端首页参考表：web/frontend/src/data/homeBillingExamples.ts
var DefaultQuotaPer1K = map[string]struct {
	In  int64
	Out int64
}{
	"gpt-4o":        {300, 1200},
	"gpt-4o-mini":   {50, 200},
	"gpt-4-turbo":   {500, 1500},
	"gpt-4":         {600, 1800},
	"gpt-3.5-turbo": {50, 150},
	// GPT-5 系列：较长键须保留以便 longestPrefixPricingKey 将 gpt-5.5 与 gpt-5 区分（勿仅配 gpt-5）。
	// gpt-5.5：按两路上游「每百万美元」最贵一档 max($4,$5)/max($30,$32)≈$5/$32，额度与成本同比例；数值整体 ×1.05 作约 5% 毛利（分组/模型倍率仍为乘子）。
	"gpt-5.5":                    {525, 3360},
	"gpt-5":                      {450, 1800},
	"o3-mini":                    {200, 800},
	"o3":                         {500, 2000},
	"o1-mini":                    {400, 1600},
	"o1":                         {2500, 10000},
	"claude-3-5-sonnet-20241022": {300, 1500},
	"claude-3-5-haiku-20241022":  {25, 125},
	"claude-3-opus-20240229":     {1500, 7500},
	"gemini-2.5-pro":             {300, 1200},
	"gemini-2.5-flash":           {50, 200},
	"gemini-2.0-flash":           {50, 200},
	"gemini-1.5-pro":             {250, 1000},
	"deepseek-chat":              {40, 120},
	"deepseek-reasoner":          {120, 360},
	"text-embedding-3-large":     {65, 65},
	"text-embedding-3-small":     {15, 15},
	"text-embedding-ada-002":     {10, 10},
	// Audio: built-in per-1k quota hints; operators may override. Longer keys should sort before shorter ones for prefix matching (longestPrefixPricingKey).
	"tts-1-hd":        {80, 600},
	"gpt-4o-mini-tts": {50, 200},
	"tts-1":           {50, 400},
	"whisper-1":       {25, 100},
	// gpt-image-2：上游常见标价为输入 $4 / 输出 $24 每百万 token（与 gpt-4o 表内比例一致：×120）；前缀命中 gpt-image-2-all、gpt-image-2-vip 等。
	// 官方 gpt-image-1 为文本/图像 token 分项计价且数值不同，若直连 OpenAI 原价请按需改表或走分组模型倍率。
	"gpt-image-2": {480, 2880},
}

// openAIModelDateSuffix matches trailing snapshot dates on OpenAI model IDs, e.g. gpt-4o-2024-08-06.
var openAIModelDateSuffix = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}$`)

// CachedPromptInputFactor scales input per-1k price for prompt-cached tokens (subset of prompt_tokens).
// 0.25 approximates common “cached read” vs full input discount tiers; tune to match your upstream.
const CachedPromptInputFactor = 0.25

func stripOpenAIModelDateSuffix(model string) string {
	s := model
	for {
		ns := openAIModelDateSuffix.ReplaceAllString(s, "")
		if ns == s {
			break
		}
		s = ns
	}
	return s
}

func longestPrefixPricingKey(model string) string {
	keys := make([]string, 0, len(DefaultQuotaPer1K))
	for k := range DefaultQuotaPer1K {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		if strings.HasPrefix(model, k) {
			return k
		}
	}
	return ""
}

// ResolvePricingTableKey returns the built-in pricing table key used for model, or empty if only the generic fallback applies.
func ResolvePricingTableKey(model string) string {
	m := strings.TrimSpace(model)
	if m == "" {
		return ""
	}
	if _, ok := DefaultQuotaPer1K[m]; ok {
		return m
	}
	stripped := stripOpenAIModelDateSuffix(m)
	if stripped != m {
		if _, ok := DefaultQuotaPer1K[stripped]; ok {
			return stripped
		}
	}
	if k := longestPrefixPricingKey(m); k != "" {
		return k
	}
	if stripped != m {
		if k := longestPrefixPricingKey(stripped); k != "" {
			return k
		}
	}
	return ""
}

func LookupPricing(model string) (inPer1k, outPer1k int64) {
	key := ResolvePricingTableKey(model)
	if key != "" {
		p := DefaultQuotaPer1K[key]
		return p.In, p.Out
	}
	return 100, 400
}

// CandidateGroupModelRateNames returns model name variants to match GroupModelRate rows (exact admin key first).
func CandidateGroupModelRateNames(model string) []string {
	m := strings.TrimSpace(model)
	if m == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 4)
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(m)
	add(stripOpenAIModelDateSuffix(m))
	if k := ResolvePricingTableKey(model); k != "" {
		add(k)
	}
	return out
}

// BuiltinPricingModelIDSet 返回 DefaultQuotaPer1K 中的键（trim 后），供管理端合并列表等使用。
func BuiltinPricingModelIDSet() map[string]struct{} {
	out := make(map[string]struct{}, len(DefaultQuotaPer1K))
	for k := range DefaultQuotaPer1K {
		k = strings.TrimSpace(k)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

// DefaultModelIDs 返回内置默认定价表中的模型 ID（排序），供 /v1/models 在无渠道配置时兜底展示。
func DefaultModelIDs() []string {
	ids := make([]string, 0, len(DefaultQuotaPer1K))
	for id := range DefaultQuotaPer1K {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
