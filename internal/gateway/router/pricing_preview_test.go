package router

import (
	"context"
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/group"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

func TestPickPricingDisplayChannel_priorityStable(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 3, Priority: 5, Weight: 200}, upstream: "m"},
		{ch: &ent.Channel{ID: 2, Priority: 10, Weight: 1}, upstream: "m"},
		{ch: &ent.Channel{ID: 1, Priority: 10, Weight: 100}, upstream: "m"},
	}
	cfg := RoutingConfig{Strategy: sitecfg.RoutingStrategyPriority}
	if got := PickPricingDisplayChannel(cfg, cands, "gpt-4o"); got != 1 {
		t.Fatalf("channel_id=%d want 1 (min id among max priority)", got)
	}
}

func TestPickPricingDisplayChannel_weightedStable(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 1, Weight: 10, ErrorCount: 0}, upstream: "m"},
		{ch: &ent.Channel{ID: 2, Weight: 100, ErrorCount: 0}, upstream: "m"},
		{ch: &ent.Channel{ID: 3, Weight: 50, ErrorCount: 5}, upstream: "m"},
	}
	cfg := RoutingConfig{Strategy: sitecfg.RoutingStrategyWeightedRandom}
	if got := PickPricingDisplayChannel(cfg, cands, "gpt-4o"); got != 2 {
		t.Fatalf("channel_id=%d want 2 (max weight in min error pool)", got)
	}
}

func TestPickPricingDisplayChannel_fixedOrder(t *testing.T) {
	cands := []candidate{
		{ch: &ent.Channel{ID: 10, Priority: 1}, upstream: "m"},
		{ch: &ent.Channel{ID: 20, Priority: 99}, upstream: "m"},
	}
	cfg := RoutingConfig{
		Strategy:     sitecfg.RoutingStrategyFixedOrder,
		ModelRouting: map[string][]int64{"gpt-4o": {20, 10}},
	}
	if got := PickPricingDisplayChannel(cfg, cands, "gpt-4o"); got != 20 {
		t.Fatalf("channel_id=%d want 20", got)
	}
}

func TestPricingPreview_ChannelIDForModel(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pricingpreview?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("pp-g").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chHigh, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(100).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pp, err := NewPricingPreview(ctx, client, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pp.EffectiveStrategy() != sitecfg.RoutingStrategyPriority {
		t.Fatalf("strategy=%q", pp.EffectiveStrategy())
	}
	if got := pp.ChannelIDForModel("gpt-4o"); got != chHigh.ID {
		t.Fatalf("channel_id=%d want %d", got, chHigh.ID)
	}
	if pp.ChannelIDForModel("unknown-model") != 0 {
		t.Fatal("expected 0 for unknown model")
	}
}

func TestPricingPreview_inheritGlobalPriority(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pricingpreview2?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	_, err := client.SystemConfig.Create().
		SetConfigKey(sitecfg.SystemConfigKeyRoutingStrategyDefault).
		SetConfigValue(sitecfg.RoutingStrategyPriority).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	g, err := client.Group.Create().SetName("pp-inherit").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	ch, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(50).
		SetModels([]string{"gpt-4o-mini"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	pp, err := NewPricingPreview(ctx, client, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pp.EffectiveStrategy() != sitecfg.RoutingStrategyPriority {
		t.Fatalf("strategy=%q want priority", pp.EffectiveStrategy())
	}
	if got := pp.ChannelIDForModel("gpt-4o-mini"); got != ch.ID {
		t.Fatalf("channel_id=%d want %d", got, ch.ID)
	}
}

func TestNewPricingPreviewWithChannels_matchesDBQuery(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pricingpreview3?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("pp-reuse").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	healthy, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(10).
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
		SetPriority(99).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(0).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	all, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HasGroupWith(group.IDEQ(g.ID)),
		).
		All(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ppDB, err := NewPricingPreview(ctx, client, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	ppReuse, err := NewPricingPreviewWithChannels(ctx, client, g.ID, all)
	if err != nil {
		t.Fatal(err)
	}
	if got := ppReuse.ChannelIDForModel("gpt-4o"); got != healthy.ID {
		t.Fatalf("reuse channel_id=%d want healthy %d", got, healthy.ID)
	}
	if got := ppDB.ChannelIDForModel("gpt-4o"); got != ppReuse.ChannelIDForModel("gpt-4o") {
		t.Fatalf("db=%d reuse=%d", got, ppReuse.ChannelIDForModel("gpt-4o"))
	}
}
