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
	if len(models) < 5 {
		t.Fatalf("expected seedance presets, got %d", len(models))
	}
}

func TestLookup_unknown(t *testing.T) {
	_, ok := Lookup("custom_openai")
	if ok {
		t.Fatal("expected false for unknown type")
	}
}
