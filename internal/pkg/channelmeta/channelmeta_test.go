package channelmeta

import "testing"

func TestDefaultBaseURL_volcengine(t *testing.T) {
	got := DefaultBaseURL("volcengine")
	want := "https://ark.cn-beijing.volces.com/api/v3"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultBaseURL_deepseek(t *testing.T) {
	got := DefaultBaseURL("deepseek")
	if got != "https://api.deepseek.com/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestBuiltInPresetModels_volcengine(t *testing.T) {
	models := BuiltInPresetModels("volcengine")
	if len(models) < 8 {
		t.Fatalf("expected seedance presets, got %d", len(models))
	}
}

func TestLookup_unknown(t *testing.T) {
	_, ok := Lookup("custom_openai")
	if ok {
		t.Fatal("expected false for unknown type")
	}
}

func TestDefaultBaseURL_kling(t *testing.T) {
	got := DefaultBaseURL("kling")
	want := "https://api-beijing.klingai.com"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestBuiltInPresetModels_kling(t *testing.T) {
	models := BuiltInPresetModels("kling")
	if len(models) < 6 {
		t.Fatalf("expected kling presets, got %d", len(models))
	}
}

func TestModelMatchesChannelType_kling(t *testing.T) {
	if !ModelMatchesChannelType("kling-v2.6-pro", "kling") {
		t.Fatal("expected kling model match")
	}
	if ModelMatchesChannelType("gpt-4o", "kling") {
		t.Fatal("expected no match for gpt on kling channel")
	}
}
