package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"

	_ "github.com/mattn/go-sqlite3"
)

func TestModelBillRates_EffectiveQuotaPer1k_matchesCost(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:effq?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g-eff").SetRateMultiplier(2).Save(ctx)
	u, _ := client.User.Create().SetEmail("u@eff.test").SetGroupID(g.ID).SetRateMultiplier(1.5).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	qIn := int64(100)
	qOut := int64(400)
	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(qIn).
		SetQuotaOutPer1k(qOut).
		SetInputRate(2).
		SetOutputRate(3).
		SetMarkupPercent(10).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini")
	if err != nil {
		t.Fatal(err)
	}
	in, out := br.EffectiveQuotaPer1k()
	if in != br.Cost(1000, 0, 0) || out != br.Cost(0, 1000, 0) {
		t.Fatalf("EffectiveQuotaPer1k %d,%d != Cost slices %d,%d", in, out, br.Cost(1000, 0, 0), br.Cost(0, 1000, 0))
	}
	if in <= 0 || out <= 0 {
		t.Fatalf("expected positive effective quota, got %d,%d", in, out)
	}
}

func TestModelBillRates_EffectiveQuotaPer1k_notBillable(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:effnb?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g-nb").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("free-model").
		SetBillable(false).
		SetEnabled(true).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@nb.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "free-model")
	if err != nil {
		t.Fatal(err)
	}
	in, out := br.EffectiveQuotaPer1k()
	if in != 0 || out != 0 {
		t.Fatalf("not billable: want 0,0 got %d,%d", in, out)
	}
}
