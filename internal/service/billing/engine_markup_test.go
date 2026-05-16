package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

func TestModelBillRates_markupAtBilling(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:markup?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	_, _ = client.SystemConfig.Create().
		SetConfigKey(sitecfg.SystemConfigKeyDefaultPricingMarkupPercent).
		SetConfigValue("10").
		Save(ctx)

	g, _ := client.Group.Create().SetName("gm").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@markup.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	qIn := int64(1000)
	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(qIn).
		SetQuotaOutPer1k(1000).
		SetInputRate(1).
		SetOutputRate(1).
		Save(ctx)

	eng := &Engine{Client: client, Config: &config.Config{}}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if got := br.Cost(1000, 0, 0); got != 1100 {
		t.Fatalf("want cost 1100 with 10%% site markup, got %d", got)
	}

	row, _ := client.GlobalModelPricing.Query().Only(ctx)
	_, _ = client.GlobalModelPricing.UpdateOne(row).SetMarkupPercent(20).Save(ctx)
	eng.InvalidateGlobalModelPricingCache()

	br2, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	if got := br2.Cost(1000, 0, 0); got != 1200 {
		t.Fatalf("want cost 1200 with 20%% row markup, got %d", got)
	}
}
