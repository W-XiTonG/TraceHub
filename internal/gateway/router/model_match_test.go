package router

import (
	"testing"

	"tracehub/internal/ent"
)

func TestModelMatch_aliasOverridesModelsList(t *testing.T) {
	ch := &ent.Channel{
		Models: []string{"doubao-seedance-1-0-lite-i2v-250428"},
		ModelAlias: map[string]string{
			"doubao-seedance-1-0-lite-i2v-250428": "ep-20250428123456-abcde",
		},
	}
	up, ok := modelMatch(ch, "doubao-seedance-1-0-lite-i2v-250428")
	if !ok {
		t.Fatal("expected match")
	}
	if up != "ep-20250428123456-abcde" {
		t.Fatalf("upstream = %q, want ep id", up)
	}
}

func TestModelMatch_modelsListWithoutAlias(t *testing.T) {
	ch := &ent.Channel{
		Models: []string{"gpt-4o-mini"},
	}
	up, ok := modelMatch(ch, "gpt-4o-mini")
	if !ok || up != "gpt-4o-mini" {
		t.Fatalf("got %q ok=%v", up, ok)
	}
}

func TestModelMatch_aliasOnly(t *testing.T) {
	ch := &ent.Channel{
		Models: []string{"other-model"},
		ModelAlias: map[string]string{
			"my-video": "ep-xyz",
		},
	}
	up, ok := modelMatch(ch, "my-video")
	if !ok || up != "ep-xyz" {
		t.Fatalf("got %q ok=%v", up, ok)
	}
}
