package router

import (
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestCountModelCandidates_respectsModelRouting(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:countcandidates?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := t.Context()

	g, err := client.Group.Create().
		SetName("count-g").
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
		SetModels([]string{"sora-2"}).
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
		SetModels([]string{"sora-2"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Group.UpdateOneID(g.ID).
		SetModelRouting(map[string][]int64{"sora-2": {int64(chPin.ID)}}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	// 未配置 routing 时 Count 会数 2 个；配置 pin 后应与 Select 一致为 1。
	if n := CountModelCandidates(ctx, client, g.ID, "sora-2", nil); n != 1 {
		t.Fatalf("CountModelCandidates want 1, got %d", n)
	}
	if n := CountModelCandidates(ctx, client, g.ID, "sora-2", []int{chPin.ID}); n != 0 {
		t.Fatalf("CountModelCandidates after exclude want 0, got %d", n)
	}
}
