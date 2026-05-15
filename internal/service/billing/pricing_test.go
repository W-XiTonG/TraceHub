package billing

import "testing"

func TestResolvePricingTableKey_DateSuffix(t *testing.T) {
	if got := ResolvePricingTableKey("gpt-4o-2024-08-06"); got != "gpt-4o" {
		t.Fatalf("got %q want gpt-4o", got)
	}
	if got := ResolvePricingTableKey("gpt-4o-mini-2024-07-18"); got != "gpt-4o-mini" {
		t.Fatalf("got %q want gpt-4o-mini", got)
	}
}

func TestResolvePricingTableKey_Prefix(t *testing.T) {
	if got := ResolvePricingTableKey("gpt-4o-mini"); got != "gpt-4o-mini" {
		t.Fatalf("got %q", got)
	}
	if got := ResolvePricingTableKey("deepseek-chat-v3"); got != "deepseek-chat" {
		t.Fatalf("got %q want deepseek-chat", got)
	}
	if got := ResolvePricingTableKey("o3-mini-2025-01-31"); got != "o3-mini" {
		t.Fatalf("got %q want o3-mini", got)
	}
	if got := ResolvePricingTableKey("o3-2025-04-16"); got != "o3" {
		t.Fatalf("got %q want o3", got)
	}
	if got := ResolvePricingTableKey("text-embedding-3-small"); got != "text-embedding-3-small" {
		t.Fatalf("got %q want text-embedding-3-small", got)
	}
	if got := ResolvePricingTableKey("text-embedding-3-large"); got != "text-embedding-3-large" {
		t.Fatalf("got %q want text-embedding-3-large", got)
	}
}

func TestLookupPricing_Unknown(t *testing.T) {
	in, out := LookupPricing("unknown-model-xyz")
	if in != 100 || out != 400 {
		t.Fatalf("fallback got %d,%d", in, out)
	}
}
