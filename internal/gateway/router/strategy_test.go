package router

import (
	"testing"

	"tracehub/internal/ent"
)

func TestPickByPriority_highestWins(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 1, Priority: 1, Weight: 100}, upstream: "m"},
		{ch: &ent.Channel{ID: 2, Priority: 5, Weight: 1}, upstream: "m"},
	}
	chosen := pickByPriority(cands, newRNG())
	if chosen.ch.ID != 2 {
		t.Fatalf("got channel %d", chosen.ch.ID)
	}
}

func TestPickByFixedOrder_respectsModelRouting(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 10, Priority: 99}, upstream: "m"},
		{ch: &ent.Channel{ID: 20, Priority: 1}, upstream: "m"},
	}
	order := map[string][]int64{"gpt-4o": {20, 10}}
	chosen := pickByFixedOrder(cands, order, "gpt-4o", newRNG())
	if chosen.ch.ID != 20 {
		t.Fatalf("got channel %d", chosen.ch.ID)
	}
}

func TestIsRetryableUpstream(t *testing.T) {
	if !IsRetryableUpstream(500, nil) {
		t.Fatal("500 should retry")
	}
	if IsRetryableUpstream(400, nil) {
		t.Fatal("400 should not retry")
	}
	if !IsRetryableUpstream(0, errTestTransport) {
		t.Fatal("transport err should retry")
	}
}

var errTestTransport = errTest("transport")

type errTest string

func (e errTest) Error() string { return string(e) }
