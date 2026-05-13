package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"tracehub/internal/service/billing"
)

func TestEstimateChatPromptTokensFromRequestBody_visionDataURLNotInflated(t *testing.T) {
	hugeB64 := "data:image/png;base64," + strings.Repeat("A", 500_000)
	body := map[string]any{
		"model": "gpt-4o",
		"messages": []any{
			map[string]any{
				"role": "user",
				"content": []any{
					map[string]any{"type": "text", "text": "hello"},
					map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url":    hugeB64,
							"detail": "high",
						},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	eng := &billing.Engine{}
	toks, err := EstimateChatPromptTokensFromRequestBody(raw, "gpt-4o", eng)
	if err != nil {
		t.Fatal(err)
	}
	if toks > 50_000 {
		t.Fatalf("expected precheck tokens bounded without counting base64; got %d", toks)
	}
	if toks < 10 {
		t.Fatalf("expected some text + vision floor; got %d", toks)
	}
}

func TestEstimateChatPromptTokensFromRequestBody_textOnly(t *testing.T) {
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"short"}]}`)
	eng := &billing.Engine{}
	toks, err := EstimateChatPromptTokensFromRequestBody(body, "gpt-4o", eng)
	if err != nil {
		t.Fatal(err)
	}
	if toks < 1 || toks > 100 {
		t.Fatalf("unexpected token count %d for short text", toks)
	}
}

func TestEstimatePromptTokensFromOpenAIBody_fallbackRoutingModel(t *testing.T) {
	body := []byte(`{"model":"gpt-4o-mini"}`)
	eng := &billing.Engine{}
	toks, err := EstimatePromptTokensFromOpenAIBody(body, "gpt-4o-mini", "gpt-4o-mini", eng)
	if err != nil {
		t.Fatal(err)
	}
	if toks < 1 {
		t.Fatalf("got %d", toks)
	}
}
