package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"

	_ "github.com/mattn/go-sqlite3"
)

func TestModelBillRates_globalNoBill(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:billfree?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g-free").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@free.test").SetGroupID(g.ID).SetBalance(10000).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	cfg := &config.Config{}
	cfg.Site.NoBillModels = []string{"gpt-4o-mini"}

	eng := &Engine{Client: client, Config: cfg}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.Billable {
		t.Fatal("expected not billable")
	}
	if br.Cost(1000, 500, 0) != 0 {
		t.Fatal("expected zero cost")
	}
}

func TestModelBillRates_groupBillableOverridesGlobalFree(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:billgrp?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g-ov").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("gpt-4o-mini").
		SetInputRate(1).
		SetOutputRate(1).
		SetEnabled(true).
		SetBillable(true).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@ov.test").SetGroupID(g.ID).SetBalance(10000).Save(ctx)
	u, _ = client.User.Query().WithGroup().Only(ctx)

	cfg := &config.Config{}
	cfg.Site.NoBillModels = []string{"gpt-4o-mini"}

	eng := &Engine{Client: client, Config: cfg}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if !br.Billable {
		t.Fatal("group billable=true should override global free list")
	}
	if br.Cost(1000, 0, 0) == 0 {
		t.Fatal("expected positive cost")
	}
}

func TestModelBillRates_groupNotBillable(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:billgrp2?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g-nb").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("custom-model").
		SetBillable(false).
		SetEnabled(true).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@nb.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().WithGroup().Only(ctx)

	eng := &Engine{Client: client, Config: &config.Config{}}
	br, err := eng.ModelBillRates(ctx, u, "custom-model")
	if err != nil {
		t.Fatal(err)
	}
	if br.Billable || br.Cost(100, 100, 0) != 0 {
		t.Fatalf("billable=%v cost=%d", br.Billable, br.Cost(100, 100, 0))
	}
}
