package gateway

import "encoding/json"

// billingMaxCompletionFallback caps debited completion tokens when upstream omits usage,
// so a huge max_tokens does not map 1:1 into a worst-case bill.
const billingMaxCompletionFallback = 8192

// completionTokensFallback returns a conservative completion count when usage.completion_tokens is missing.
func completionTokensFallback(maxRequested int) int {
	if maxRequested < 1 {
		return 0
	}
	if maxRequested > billingMaxCompletionFallback {
		return billingMaxCompletionFallback
	}
	return maxRequested
}

// ParsedOpenAIUsage is usage fields parsed from OpenAI-compatible JSON bodies.
type ParsedOpenAIUsage struct {
	PromptTokens       int
	CompletionTokens   int
	CachedPromptTokens int
	// SuppressCompletionFallback is set when prompt-like usage came from non-chat shapes
	// (e.g. usage.seconds or usage.total_tokens alone) so we do not fabricate completion_tokens.
	SuppressCompletionFallback bool
}

// ParseOpenAIUsageJSON reads usage from a full response object or a stream chunk.
func ParseOpenAIUsageJSON(raw []byte) ParsedOpenAIUsage {
	var wrap struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			CachedTokens     int `json:"cached_tokens"`
			PromptTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return ParsedOpenAIUsage{}
	}
	u := ParsedOpenAIUsage{
		PromptTokens:     wrap.Usage.PromptTokens,
		CompletionTokens: wrap.Usage.CompletionTokens,
	}
	if wrap.Usage.PromptTokensDetails != nil {
		u.CachedPromptTokens = wrap.Usage.PromptTokensDetails.CachedTokens
	}
	if u.CachedPromptTokens == 0 && wrap.Usage.CachedTokens > 0 {
		u.CachedPromptTokens = wrap.Usage.CachedTokens
	}
	if u.CachedPromptTokens > u.PromptTokens {
		u.CachedPromptTokens = u.PromptTokens
	}
	if u.CachedPromptTokens < 0 {
		u.CachedPromptTokens = 0
	}
	return u
}

func parseUsageFromOpenAIResponse(raw []byte) (promptTokens, completionTokens int) {
	u := ParseOpenAIUsageJSON(raw)
	return u.PromptTokens, u.CompletionTokens
}

// parseUsageFlexible parses OpenAI usage first, then common alternate shapes (audio, etc.).
func parseUsageFlexible(raw []byte) ParsedOpenAIUsage {
	u := ParseOpenAIUsageJSON(raw)
	if u.PromptTokens > 0 || u.CompletionTokens > 0 {
		return u
	}
	var wrap struct {
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			InputTokens      int     `json:"input_tokens"`
			OutputTokens     int     `json:"output_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			Seconds          float64 `json:"seconds"`
			PromptTokensDetails *struct {
				CachedTokens int `json:"cached_tokens"`
			} `json:"prompt_tokens_details"`
			CachedTokens int `json:"cached_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return ParsedOpenAIUsage{}
	}
	out := ParsedOpenAIUsage{}
	if wrap.Usage.TotalTokens > 0 {
		out.PromptTokens = wrap.Usage.TotalTokens
		out.SuppressCompletionFallback = true
		return out
	}
	if wrap.Usage.InputTokens > 0 || wrap.Usage.OutputTokens > 0 {
		out.PromptTokens = wrap.Usage.InputTokens
		out.CompletionTokens = wrap.Usage.OutputTokens
		return out
	}
	if wrap.Usage.Seconds > 0 {
		out.PromptTokens = int(wrap.Usage.Seconds * 100)
		out.SuppressCompletionFallback = true
		return out
	}
	// Rare path: same struct as ParseOpenAIUsageJSON but zero prompt/completion — still attach cache if present
	out.PromptTokens = wrap.Usage.PromptTokens
	out.CompletionTokens = wrap.Usage.CompletionTokens
	if wrap.Usage.PromptTokensDetails != nil {
		out.CachedPromptTokens = wrap.Usage.PromptTokensDetails.CachedTokens
	}
	if out.CachedPromptTokens == 0 && wrap.Usage.CachedTokens > 0 {
		out.CachedPromptTokens = wrap.Usage.CachedTokens
	}
	if out.CachedPromptTokens > out.PromptTokens {
		out.CachedPromptTokens = out.PromptTokens
	}
	return out
}

// applyCompletionFallback sets CompletionTokens when zero, using capped max requested tokens.
func applyCompletionFallback(u *ParsedOpenAIUsage, maxRequested int) {
	if u == nil || u.CompletionTokens > 0 {
		return
	}
	if u.SuppressCompletionFallback {
		return
	}
	if maxRequested <= 0 {
		return
	}
	u.CompletionTokens = completionTokensFallback(maxRequested)
}

// applyPromptFallback fills PromptTokens when zero.
func applyPromptFallback(u *ParsedOpenAIUsage, estimatedPrompt int) {
	if u == nil || u.PromptTokens > 0 {
		return
	}
	if estimatedPrompt < 1 {
		estimatedPrompt = 1
	}
	u.PromptTokens = estimatedPrompt
}
