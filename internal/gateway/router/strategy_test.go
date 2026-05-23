package router

import (
	"testing"

	"tracehub/internal/ent"
)

func TestPickByPriority_prefersLowerErrorCount(t *testing.T) {
	candidates := []candidate{
		{ch: &ent.Channel{ID: 1, Priority: 10, Weight: 1, ErrorCount: 5}},
		{ch: &ent.Channel{ID: 2, Priority: 10, Weight: 1, ErrorCount: 0}},
	}
	rng := newRNG()
	chosen := pickByPriority(candidates, rng)
	if chosen.ch.ID != 2 {
		t.Fatalf("expected channel 2, got %d", chosen.ch.ID)
	}
}
