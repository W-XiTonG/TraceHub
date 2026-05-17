package availmodels

import "testing"

func TestInferOwnedBy(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"gpt-4o", "openai"},
		{"claude-3-5-sonnet", "anthropic"},
		{"gemini-2.5-flash", "google"},
		{"deepseek-chat", "deepseek"},
		{"Pro/BAAI/bge-m3", "baai"},
		{"BAAI/bge-large-zh-v1.5", "baai"},
		{"Qwen/Qwen2.5-72B-Instruct", "alibaba"},
		{"meta-llama/Llama-3.1-8B-Instruct", "meta"},
		{"mistralai/Mistral-7B-Instruct-v0.2", "mistral"},
		{"doubao-seedance-2-0-260128", "bytedance"},
		{"kimi-k2-thinking", "moonshot"},
		{"glm-4-flash", "zhipu"},
		{"text-embedding-3-large", "openai"},
		{"unknown-vendor-model-xyz", "tracehub"},
		{"google/kling-v1", "google"},
		{"ep-20250428-custom", "tracehub"},
	}
	for _, tt := range tests {
		got := inferOwnedBy(tt.id)
		if got != tt.want {
			t.Errorf("inferOwnedBy(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}
