package availmodels

import (
	"testing"

	"tracehub/internal/ent"
)

func TestRouterHealthLabel(t *testing.T) {
	all := map[string]struct{}{"a": {}, "b": {}}
	healthy := map[string]struct{}{"a": {}}
	if g := RouterHealthLabel(all, healthy, "a"); g != "healthy" {
		t.Fatalf("a want healthy got %q", g)
	}
	if g := RouterHealthLabel(all, healthy, "b"); g != "unhealthy" {
		t.Fatalf("b want unhealthy got %q", g)
	}
	if g := RouterHealthLabel(all, healthy, "c"); g != "catalog_only" {
		t.Fatalf("c want catalog_only got %q", g)
	}
	if g := RouterHealthLabel(map[string]struct{}{}, map[string]struct{}{}, "x"); g != "catalog_only" {
		t.Fatalf("empty channel sets want catalog_only got %q", g)
	}
}

func TestCollectChannelModelKeys(t *testing.T) {
	chs := []*ent.Channel{
		{Models: []string{"m1", " m2 "}, HealthStatus: 1, ModelAlias: map[string]string{"alias1": "m1"}},
		{Models: []string{"m3"}, HealthStatus: 0},
	}
	all, h := collectChannelModelKeys(chs)
	for _, id := range []string{"m1", "m2", "alias1", "m3"} {
		if _, ok := all[id]; !ok {
			t.Fatalf("missing in all: %q", id)
		}
	}
	for _, id := range []string{"m1", "m2", "alias1"} {
		if _, ok := h[id]; !ok {
			t.Fatalf("missing in healthy: %q", id)
		}
	}
	if _, ok := h["m3"]; ok {
		t.Fatal("m3 should not be healthy")
	}
}

func TestCollectChannelModelKeys_caseInsensitiveDedup(t *testing.T) {
	chs := []*ent.Channel{
		{Models: []string{"GPT-4o", "gpt-4o"}, HealthStatus: 1},
		{Models: []string{"claude-3"}, HealthStatus: 0},
	}
	all, h := collectChannelModelKeys(chs)
	if len(all) != 2 {
		t.Fatalf("want 2 distinct ids, got %d: %v", len(all), all)
	}
	if _, ok := all["GPT-4o"]; !ok {
		t.Fatal("expected first-seen canonical GPT-4o")
	}
	if _, ok := all["claude-3"]; !ok {
		t.Fatal("expected claude-3")
	}
	if _, ok := h["GPT-4o"]; !ok {
		t.Fatal("GPT-4o should be healthy from first channel")
	}
	if _, ok := h["claude-3"]; ok {
		t.Fatal("claude-3 should not be healthy")
	}
}

func TestOmitAliasUpstreamTargets(t *testing.T) {
	set := map[string]struct{}{
		"ep-202501011234": {},
		"doubao-pro":      {},
		"other":           {},
	}
	chs := []*ent.Channel{{
		Models:     []string{"ep-202501011234", "other"},
		ModelAlias: map[string]string{"doubao-pro": "ep-202501011234"},
	}}
	omitAliasUpstreamTargets(set, chs)
	if _, ok := set["ep-202501011234"]; ok {
		t.Fatal("upstream ep id should be hidden when aliased")
	}
	for _, want := range []string{"doubao-pro", "other"} {
		if _, ok := set[want]; !ok {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestDeclaredModelDistinctCount(t *testing.T) {
	chs := []*ent.Channel{
		{Models: []string{"a", "b"}, ModelAlias: map[string]string{"c": "upstream"}},
		{Models: []string{"b", "d"}, ModelAlias: map[string]string{"e": "x"}},
	}
	if n := DeclaredModelDistinctCount(chs); n != 5 {
		t.Fatalf("got %d want 5 (a,b,c,d,e)", n)
	}
	if n := DeclaredModelDistinctCount(nil); n != 0 {
		t.Fatalf("nil got %d", n)
	}
	if n := DeclaredModelDistinctCount([]*ent.Channel{{Models: []string{"  ", "x"}}}); n != 1 {
		t.Fatalf("trim/empty got %d", n)
	}
}
