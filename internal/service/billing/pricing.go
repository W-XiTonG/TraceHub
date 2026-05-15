package billing

import (
	"regexp"
	"sort"
	"strings"
)

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
	// Audio: built-in per-1k quota hints; operators may override. Longer keys should sort before shorter ones for prefix matching (longestPrefixPricingKey).
	"tts-1-hd":      {80, 600},
	"gpt-4o-mini-tts": {50, 200},
	"tts-1":         {50, 400},
	"whisper-1":     {25, 100},
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

// DefaultModelIDs 返回内置默认定价表中的模型 ID（排序），供 /v1/models 在无渠道配置时兜底展示。
func DefaultModelIDs() []string {
	ids := make([]string, 0, len(DefaultQuotaPer1K))
	for id := range DefaultQuotaPer1K {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
