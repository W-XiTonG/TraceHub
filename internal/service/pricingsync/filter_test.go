package pricingsync

import "testing"

func TestSimpleGlob(t *testing.T) {
	if !simpleGlob("gpt-*", "gpt-4o") {
		t.Fatal("expected match")
	}
	if simpleGlob("gpt-*", "claude-3") {
		t.Fatal("expected no match")
	}
	if !simpleGlob("*-mini", "gpt-4o-mini") {
		t.Fatal("expected suffix match")
	}
}

func TestModelAllowedDenylist(t *testing.T) {
	opts := FilterOpts{IncludeMode: "denylist", Denylist: []string{"test-model"}}
	if !modelAllowed("gpt-4o", opts) {
		t.Fatal("expected allowed")
	}
	if modelAllowed("test-model", opts) {
		t.Fatal("expected denied")
	}
}

func TestModelAllowedAllowlist(t *testing.T) {
	opts := FilterOpts{IncludeMode: "allowlist", Allowlist: []string{"gpt-4o"}}
	if !modelAllowed("gpt-4o", opts) {
		t.Fatal("expected allowed")
	}
	if modelAllowed("other", opts) {
		t.Fatal("expected denied")
	}
}

func TestValidateOutboundURLBlocksLoopback(t *testing.T) {
	if err := ValidateOutboundURL("http://127.0.0.1/pricing", true); err == nil {
		t.Fatal("expected SSRF block")
	}
}

func TestCountPayloadRows(t *testing.T) {
	p := &Payload{
		Global: []GlobalItem{{ModelName: "a"}},
		Channels: []ChannelBlock{{Items: []ChannelItem{{GlobalItem: GlobalItem{ModelName: "b"}}}}},
	}
	if n := countPayloadRows(p); n != 2 {
		t.Fatalf("got %d", n)
	}
}

func TestSelectedKeyString(t *testing.T) {
	k := SelectedKey{Layer: ScopeChannel, ModelName: "m", ChannelID: 3}
	if k.String() != "channel:3:m" {
		t.Fatalf("got %s", k.String())
	}
}

func TestApplyRowSelectionKeepsAllRows(t *testing.T) {
	rows := []DiffRow{
		{Layer: ScopeGlobal, ModelName: "a", Change: ChangeSkipped, Selected: false},
		{Layer: ScopeChannel, ModelName: "b", ChannelID: 1, Change: ChangeAdded, Selected: true},
	}
	out := applyRowSelection(rows, []string{"channel:1:b"})
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].Selected || !out[1].Selected {
		t.Fatalf("selection=%v %v", out[0].Selected, out[1].Selected)
	}
}
