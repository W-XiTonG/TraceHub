package gateway

import "testing"

func TestParseOpenAIUsageJSON_CachedDetails(t *testing.T) {
	raw := []byte(`{"usage":{"prompt_tokens":100,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":40}}}`)
	u := ParseOpenAIUsageJSON(raw)
	if u.PromptTokens != 100 || u.CompletionTokens != 10 || u.CachedPromptTokens != 40 {
		t.Fatalf("got %+v", u)
	}
}

func TestCompletionTokensFallback(t *testing.T) {
	if completionTokensFallback(0) != 0 {
		t.Fatalf("zero")
	}
	if completionTokensFallback(100_000) != billingMaxCompletionFallback {
		t.Fatalf("cap")
	}
	if completionTokensFallback(100) != 100 {
		t.Fatalf("passthrough")
	}
}
