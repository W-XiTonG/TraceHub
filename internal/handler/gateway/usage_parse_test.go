package gateway

import (
	"testing"
)

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

func TestParseUsageFlexible_SecondsSuppressesCompletionFallback(t *testing.T) {
	raw := []byte(`{"text":"hi","usage":{"seconds":12.5}}`)
	u := parseUsageFlexible(raw)
	if u.PromptTokens != 1250 {
		t.Fatalf("prompt got %d", u.PromptTokens)
	}
	if !u.SuppressCompletionFallback {
		t.Fatal("expected SuppressCompletionFallback")
	}
	applyCompletionFallback(&u, 2048, true)
	if u.CompletionTokens != 0 {
		t.Fatalf("completion got %d", u.CompletionTokens)
	}
}

func TestParseUsageFlexible_TotalTokensSuppressesCompletionFallback(t *testing.T) {
	raw := []byte(`{"usage":{"total_tokens":500}}`)
	u := parseUsageFlexible(raw)
	if u.PromptTokens != 500 || u.CompletionTokens != 0 || !u.SuppressCompletionFallback {
		t.Fatalf("got %+v", u)
	}
	applyCompletionFallback(&u, 2048, true)
	if u.CompletionTokens != 0 {
		t.Fatalf("completion got %d", u.CompletionTokens)
	}
}

func TestParseUsageFlexible_StandardStillAllowsCompletionFallback(t *testing.T) {
	raw := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":0}}`)
	u := parseUsageFlexible(raw)
	if u.SuppressCompletionFallback {
		t.Fatal("unexpected suppress for standard shape")
	}
	applyCompletionFallback(&u, 2048, true)
	if u.CompletionTokens != 2048 {
		t.Fatalf("completion got %d", u.CompletionTokens)
	}
}

func TestParseUsageFlexible_InputOutputNoSuppress(t *testing.T) {
	raw := []byte(`{"usage":{"input_tokens":100,"output_tokens":0}}`)
	u := parseUsageFlexible(raw)
	if u.PromptTokens != 100 || u.CompletionTokens != 0 {
		t.Fatalf("got %+v", u)
	}
	if u.SuppressCompletionFallback {
		t.Fatal("input/output path should not set suppress")
	}
}
