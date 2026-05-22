package modelkind

import (
	"reflect"
	"testing"
)

func TestLabelsForModel_heuristics(t *testing.T) {
	tests := []struct {
		id   string
		want []string
	}{
		{"text-embedding-3-small", []string{"embedding"}},
		{"dall-e-3", []string{"image"}},
		{"tts-1", []string{"audio"}},
		{"whisper-1", []string{"audio"}},
		{"text-moderation-latest", []string{"moderation"}},
		{"rerank-english-v3.0", []string{"rerank"}},
		{"gpt-4o", []string{"chat"}},
		{"bge-large-en-v1.5", []string{"embedding"}},
		{"bge-reranker-v2-m3", []string{"rerank"}},
		{"e5-mistral-7b-instruct", []string{"embedding"}},
		{"Qwen/Qwen-Image", []string{"image"}},
		{"Qwen/Qwen-Image-Edit-2509", []string{"image"}},
		{"Tongyi-MAI/Z-Image-Turbo", []string{"image"}},
		{"baidu/ERNIE-Image-Turbo", []string{"image"}},
		{"voyage-3-large", []string{"embedding"}},
		{"nomic-embed-text-v1.5", []string{"embedding"}},
		{"google/kling-v1", []string{"video"}},
		{"openai/sora-2", []string{"video"}},
		{"doubao-seedance-1-5-pro-251215", []string{"video"}},
		{"suno-v3.5", []string{"audio"}},
	}
	for _, tt := range tests {
		got := LabelsForModel(tt.id, nil, nil)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("LabelsForModel(%q, nil, nil) = %v; want %v", tt.id, got, tt.want)
		}
	}
}

func TestLabelsForModel_metadata(t *testing.T) {
	meta := map[string]any{
		"kind":   "image",
		"labels": []any{"experimental"},
	}
	got := LabelsForModel("gpt-4o-mini", meta, nil)
	want := []string{"image", "experimental"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LabelsForModel with meta = %v; want %v", got, want)
	}

	got2 := LabelsForModel("gpt-4o-mini", map[string]any{"kind": "embedding"}, nil)
	if !reflect.DeepEqual(got2, []string{"embedding"}) {
		t.Errorf("kind only merge = %v", got2)
	}
}

func TestLabelsForModel_emptyID(t *testing.T) {
	if got := LabelsForModel("  ", nil, nil); !reflect.DeepEqual(got, []string{"chat"}) {
		t.Fatalf("empty id: %v", got)
	}
}
