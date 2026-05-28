package pricingsync

import "testing"

func TestCountPayloadRows_scheduledLimitGuard(t *testing.T) {
	p := &Payload{
		Version: VersionTraceHubPricingV1,
		Global: []GlobalItem{
			{ModelName: "a"},
			{ModelName: "b"},
			{ModelName: "c"},
		},
	}
	if n := countPayloadRows(p); n != 3 {
		t.Fatalf("count=%d", n)
	}
	const maxRows = 2
	if countPayloadRows(p) <= maxRows {
		t.Fatal("fixture should exceed limit")
	}
}
