package router

import (
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestFilterCandidatesByModelRouting_noBinding(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 1}, upstream: "m"},
		{ch: &ent.Channel{ID: 2}, upstream: "m"},
	}
	out, err := filterCandidatesByModelRouting(cands, nil, "gpt-4o")
	if err != nil || len(out) != 2 {
		t.Fatalf("got %d err=%v", len(out), err)
	}
}

func TestFilterCandidatesByModelRouting_singlePin(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 10, Weight: 1}, upstream: "m"},
		{ch: &ent.Channel{ID: 20, Weight: 100}, upstream: "m"},
	}
	routing := map[string][]int64{"gpt-4o": {20}}
	out, err := filterCandidatesByModelRouting(cands, routing, "gpt-4o")
	if err != nil || len(out) != 1 || out[0].ch.ID != 20 {
		t.Fatalf("got %+v err=%v", out, err)
	}
}

func TestFilterCandidatesByModelRouting_pinnedUnavailable(t *testing.T) {
	cands := []candidate{{ch: &ent.Channel{ID: 10}, upstream: "m"}}
	routing := map[string][]int64{"gpt-4o": {99}}
	_, err := filterCandidatesByModelRouting(cands, routing, "gpt-4o")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFilterCandidatesByModelRouting_preservesOrder(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 10}, upstream: "m"},
		{ch: &ent.Channel{ID: 20}, upstream: "m"},
		{ch: &ent.Channel{ID: 30}, upstream: "m"},
	}
	routing := map[string][]int64{"gpt-4o": {30, 10}}
	out, err := filterCandidatesByModelRouting(cands, routing, "gpt-4o")
	if err != nil || len(out) != 2 {
		t.Fatalf("len=%d err=%v", len(out), err)
	}
	if out[0].ch.ID != 30 || out[1].ch.ID != 10 {
		t.Fatalf("order: %d %d", out[0].ch.ID, out[1].ch.ID)
	}
}

func TestSelectChannelExcluding_respectsModelRoutingAllStrategies(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pinrouting?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := t.Context()

	g, err := client.Group.Create().
		SetName("pin-g").
		SetRoutingStrategy("weighted_random").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	chPin, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetWeight(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetWeight(1000).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Group.UpdateOneID(g.ID).
		SetModelRouting(map[string][]int64{"gpt-4o": {int64(chPin.ID)}}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	sel, err := SelectChannel(ctx, nil, client, 1, g.ID, "gpt-4o", "")
	if err != nil {
		t.Fatal(err)
	}
	if sel.Channel.ID != chPin.ID {
		t.Fatalf("expected pinned channel %d, got %d", chPin.ID, sel.Channel.ID)
	}
}
