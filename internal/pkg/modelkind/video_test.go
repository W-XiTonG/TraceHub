package modelkind

import "testing"

func TestIsVideoBillingModel_seedance(t *testing.T) {
	if !IsVideoBillingModel("doubao-seedance-1-5-pro-251215") {
		t.Fatal("expected video billing model")
	}
}

func TestLabelsForModel_seedanceOverridesCatalogChat(t *testing.T) {
	meta := map[string]any{"kind": "chat"}
	got := LabelsForModel("doubao-seedance-1-5-pro-251215", meta, nil)
	if len(got) != 1 || got[0] != "video" {
		t.Fatalf("got %v want [video]", got)
	}
}

func TestLabelsForModel_seedanceOverridesAdminChat(t *testing.T) {
	got := LabelsForModel("doubao-seedance-1-5-pro-251215", nil, []string{"chat"})
	if len(got) != 1 || got[0] != "video" {
		t.Fatalf("got %v want [video]", got)
	}
}
