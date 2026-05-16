package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

func TestCachedDefaultPricingMarkupPercent_usesCache(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:markupcache?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	_, _ = client.SystemConfig.Create().
		SetConfigKey(sitecfg.SystemConfigKeyDefaultPricingMarkupPercent).
		SetConfigValue("10").
		Save(ctx)

	eng := &Engine{Client: client, Config: &config.Config{}}
	if got := eng.cachedDefaultPricingMarkupPercent(ctx); got != 10 {
		t.Fatalf("want 10 got %v", got)
	}

	_, _ = client.SystemConfig.Update().
		Where(systemconfig.ConfigKeyEQ(sitecfg.SystemConfigKeyDefaultPricingMarkupPercent)).
		SetConfigValue("25").
		Save(ctx)

	if got := eng.cachedDefaultPricingMarkupPercent(ctx); got != 10 {
		t.Fatalf("want cached 10 got %v", got)
	}

	eng.InvalidateDefaultPricingMarkupCache()
	if got := eng.cachedDefaultPricingMarkupPercent(ctx); got != 25 {
		t.Fatalf("want 25 after invalidate got %v", got)
	}
}

func TestCachedGroupModelRates_usesCache(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:grcache?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("gcache").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("gpt-4o-mini").
		SetInputRate(2).
		SetOutputRate(3).
		SetEnabled(true).
		SetBillable(true).
		Save(ctx)

	eng := &Engine{Client: client, Config: &config.Config{}}
	gmr1 := eng.lookupGroupModelRateRow(ctx, g.ID, "gpt-4o-mini")
	if gmr1 == nil || gmr1.InputRate != 2 {
		t.Fatalf("unexpected first lookup: %+v", gmr1)
	}

	gRow, _ := client.GroupModelRate.Query().Only(ctx)
	_, _ = client.GroupModelRate.UpdateOne(gRow).SetInputRate(9).Save(ctx)

	gmr2 := eng.lookupGroupModelRateRow(ctx, g.ID, "gpt-4o-mini")
	if gmr2 == nil || gmr2.InputRate != 2 {
		t.Fatalf("want cached input rate 2 got %+v", gmr2)
	}

	eng.InvalidateGroupModelRateCache(g.ID)
	gmr3 := eng.lookupGroupModelRateRow(ctx, g.ID, "gpt-4o-mini")
	if gmr3 == nil || gmr3.InputRate != 9 {
		t.Fatalf("want refreshed input rate 9 got %+v", gmr3)
	}
}

func TestModelBillRates_groupRateFromCache(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:grbill?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("gbill").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("gpt-4o-mini").
		SetInputRate(2).
		SetOutputRate(1).
		SetEnabled(true).
		SetBillable(true).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@grbill.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	eng := &Engine{Client: client, Config: &config.Config{}}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 2 {
		t.Fatalf("want inRate 2 from group cache path, got %v", br.inRate)
	}
}
