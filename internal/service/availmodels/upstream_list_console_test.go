package availmodels

import (
	"testing"
	"time"

	"tracehub/internal/ent"
)

func TestChannelDeclaresModel(t *testing.T) {
	ch := &ent.Channel{
		Models:     []string{"a", " b "},
		ModelAlias: map[string]string{"alias1": "a"},
	}
	if !channelDeclaresModel(ch, "a") || !channelDeclaresModel(ch, "b") || !channelDeclaresModel(ch, "alias1") {
		t.Fatal("expected true")
	}
	if channelDeclaresModel(ch, "c") {
		t.Fatal("expected false")
	}
}

func TestAttachUpstreamListProbeFromChannels(t *testing.T) {
	ts := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	ch1 := &ent.Channel{
		Models: []string{"m1", "m2"},
		ModelUpstreamProbe: map[string]interface{}{
			"m1": map[string]interface{}{"in_upstream_list": true, "checked_at": ts},
			"m2": map[string]interface{}{"in_upstream_list": false, "checked_at": ts},
		},
	}
	ch2 := &ent.Channel{
		Models: []string{"m2"},
		ModelUpstreamProbe: map[string]interface{}{
			"m2": map[string]interface{}{"in_upstream_list": true, "checked_at": ts},
		},
	}
	byID := map[string]*ConsoleModel{
		"m1": {ID: "m1", InGateway: true},
		"m2": {ID: "m2", InGateway: true},
		"m3": {ID: "m3", InGateway: true},
		"c1": {ID: "c1", InGateway: false},
	}
	attachUpstreamListProbeFromChannels([]*ent.Channel{ch1, ch2}, byID)
	if byID["m1"].UpstreamListStatus != "listed" {
		t.Fatalf("m1 got %q", byID["m1"].UpstreamListStatus)
	}
	if byID["m2"].UpstreamListStatus != "mixed" {
		t.Fatalf("m2 got %q", byID["m2"].UpstreamListStatus)
	}
	if byID["m3"].UpstreamListStatus != "" {
		t.Fatalf("m3 got %q", byID["m3"].UpstreamListStatus)
	}
	if byID["c1"].UpstreamListStatus != "" {
		t.Fatalf("catalog-only got %q", byID["c1"].UpstreamListStatus)
	}
}
