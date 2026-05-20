package router

import "testing"

func TestUsesOpenAICompatRelay(t *testing.T) {
	extras := []string{"siliconflow"}
	tests := []struct {
		ct   string
		want bool
	}{
		{"openai", true},
		{"deepseek", true},
		{"volcengine", true},
		{"VolcEngine", true},
		{"claude", false},
		{"gemini", false},
		{"siliconflow", true},
		{"unknown", false},
	}
	for _, tt := range tests {
		got := UsesOpenAICompatRelay(tt.ct, extras)
		if got != tt.want {
			t.Errorf("UsesOpenAICompatRelay(%q, extras) = %v; want %v", tt.ct, got, tt.want)
		}
	}
}

func TestIsOpenAICompatChannelType_volcengineExcluded(t *testing.T) {
	if IsOpenAICompatChannelType("volcengine", nil) {
		t.Fatal("volcengine must not be in IsOpenAICompatChannelType alone; use UsesOpenAICompatRelay")
	}
}
