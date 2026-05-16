package availmodels

import (
	"fmt"
	"testing"
)

func TestFilterWorkspaceByModelQuery(t *testing.T) {
	all := []GlobalPricingWorkspaceItem{
		{ModelName: "gpt-4o-mini"},
		{ModelName: "GPT-5"},
		{ModelName: "claude-3-haiku"},
	}
	if got := filterWorkspaceByModelQuery(all, ""); len(got) != 3 {
		t.Fatalf("empty q: want 3, got %d", len(got))
	}
	got := filterWorkspaceByModelQuery(all, "gpt")
	if len(got) != 2 {
		t.Fatalf("gpt: want 2, got %d", len(got))
	}
	got = filterWorkspaceByModelQuery(all, "HAIKU")
	if len(got) != 1 || got[0].ModelName != "claude-3-haiku" {
		t.Fatalf("HAIKU: got %#v", got)
	}
}

func TestPageGlobalPricingWorkspace(t *testing.T) {
	mk := func(n int) []GlobalPricingWorkspaceItem {
		out := make([]GlobalPricingWorkspaceItem, n)
		for i := 0; i < n; i++ {
			out[i] = GlobalPricingWorkspaceItem{ModelName: fmt.Sprintf("id-%d", i)}
		}
		return out
	}
	t.Run("empty", func(t *testing.T) {
		items, total, p := pageGlobalPricingWorkspace(nil, 1, 50)
		if total != 0 || p != 1 || len(items) != 0 {
			t.Fatalf("got items=%d total=%d page=%d", len(items), total, p)
		}
	})
	t.Run("clamp page", func(t *testing.T) {
		all := mk(5)
		items, total, p := pageGlobalPricingWorkspace(all, 99, 2)
		if total != 5 || p != 3 || len(items) != 1 {
			t.Fatalf("got len=%d total=%d page=%d", len(items), total, p)
		}
	})
	t.Run("page1", func(t *testing.T) {
		all := mk(5)
		items, total, p := pageGlobalPricingWorkspace(all, 1, 2)
		if total != 5 || p != 1 || len(items) != 2 {
			t.Fatalf("got len=%d total=%d page=%d", len(items), total, p)
		}
	})
}
