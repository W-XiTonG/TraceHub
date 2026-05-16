package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"

	_ "github.com/mattn/go-sqlite3"
)

func TestModelBillRates_globalQuotaOverridesCode(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:globq?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g1").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@gq.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	codeIn, codeOut := LookupPricing("gpt-4o-mini")
	qIn := int64(999)
	qOut := int64(888)
	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(qIn).
		SetQuotaOutPer1k(qOut).
		SetInputRate(1).
		SetOutputRate(1).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.in1k != qIn || br.out1k != qOut {
		t.Fatalf("want quota %d,%d got %d,%d", qIn, qOut, br.in1k, br.out1k)
	}
	if codeIn == qIn && codeOut == qOut {
		t.Fatal("test setup: code defaults should differ from forced quota")
	}
}

func TestModelBillRates_globalRatesWhenNoGroupRow(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:glor?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g2").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@gr.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(2).
		SetOutputRate(3).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 2 || br.outRate != 3 {
		t.Fatalf("want global rates 2,3 got %v,%v", br.inRate, br.outRate)
	}
}

func TestModelBillRates_groupRateOverridesGlobal(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:grgl?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g3").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("gpt-4o-mini").
		SetInputRate(5).
		SetOutputRate(7).
		SetEnabled(true).
		Save(ctx)
	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(2).
		SetOutputRate(3).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@ov2.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 5 || br.outRate != 7 {
		t.Fatalf("want group rates 5,7 got %v,%v", br.inRate, br.outRate)
	}
}

func TestModelBillRates_globalDisabledUsesCodeRates(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:gdis?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g4").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@gd.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(false).
		SetInputRate(99).
		SetOutputRate(99).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 1 || br.outRate != 1 {
		t.Fatalf("disabled global row should not apply rates, got %v,%v", br.inRate, br.outRate)
	}
}

func TestModelBillRates_globalRatesUserWithoutGroup(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:gnog?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	u, _ := client.User.Create().SetEmail("u@nogrp.test").Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).Only(ctx)

	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(1.5).
		SetOutputRate(1.25).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 1.5 || br.outRate != 1.25 {
		t.Fatalf("want global rates for user without group, got %v,%v", br.inRate, br.outRate)
	}
}
