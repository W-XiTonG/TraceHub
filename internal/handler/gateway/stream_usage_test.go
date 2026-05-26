package gateway

import (
	"strings"
	"testing"

	"tracehub/internal/service/billing"
)

func TestResolveOpenAIStreamUsage_upstreamUsage(t *testing.T) {
	cap := StreamUsageCapture{
		PromptTokens:     10,
		CompletionTokens: 20,
		HasUpstreamUsage: true,
	}
	got := ResolveOpenAIStreamUsage(cap, nil, "gpt-4o", 5, 512, false)
	if got.CompletionTokens != 20 || got.PromptTokens != 10 {
		t.Fatalf("got pt=%d ct=%d", got.PromptTokens, got.CompletionTokens)
	}
	if got.Note != usageNotePrefix+"upstream" {
		t.Fatalf("note=%q", got.Note)
	}
}

func TestResolveOpenAIStreamUsage_streamContent(t *testing.T) {
	eng := &billing.Engine{}
	cap := StreamUsageCapture{DeltaContent: "hello world"}
	got := ResolveOpenAIStreamUsage(cap, eng, "gpt-4o", 12, 512, false)
	if got.CompletionTokens <= 0 {
		t.Fatalf("expected positive completion from content, got %d", got.CompletionTokens)
	}
	if got.Note != usageNotePrefix+"stream_content" {
		t.Fatalf("note=%q", got.Note)
	}
}

func TestResolveOpenAIStreamUsage_noDefault512WithoutMaxTokens(t *testing.T) {
	cap := StreamUsageCapture{}
	got := ResolveOpenAIStreamUsage(cap, nil, "gpt-4o", 10, 0, false)
	if got.CompletionTokens != 0 {
		t.Fatalf("expected 0 completion without max_tokens, got %d", got.CompletionTokens)
	}
}

func TestResolveOpenAIStreamUsage_explicitMaxTokensFallback(t *testing.T) {
	cap := StreamUsageCapture{}
	got := ResolveOpenAIStreamUsage(cap, nil, "gpt-4o", 10, 128, true)
	if got.CompletionTokens != 128 {
		t.Fatalf("got %d want 128", got.CompletionTokens)
	}
	if got.Note != usageNotePrefix+"max_tokens" {
		t.Fatalf("note=%q", got.Note)
	}
}

func TestAppendOpenAIStreamDelta(t *testing.T) {
	var b strings.Builder
	appendOpenAIStreamDelta(&b, []byte(`{"choices":[{"delta":{"content":"你","reasoning_content":"思"}}]}`))
	appendOpenAIStreamDelta(&b, []byte(`{"choices":[{"delta":{"content":"好"}}]}`))
	if b.String() != "你思好" {
		t.Fatalf("got %q", b.String())
	}
}
