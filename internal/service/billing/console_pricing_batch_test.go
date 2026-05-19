package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"

	_ "github.com/mattn/go-sqlite3"
)

func TestConsolePricingBatch_Rates(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:cpbatch?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	_, err := client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(500).
		SetQuotaOutPer1k(1500).
		SetInputRate(1).
		SetOutputRate(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	g, err := client.Group.Create().SetName("g").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	uid, err := client.User.Create().SetEmail("a@b.c").SetUsername("u").SetGroupID(g.ID).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	u, err := client.User.Query().Where(user.IDEQ(uid.ID)).WithGroup().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}

	eng := &Engine{Client: client}
	batch := eng.NewConsolePricingBatch(ctx, u, nil)
	br := batch.Rates("gpt-4o-mini", 0)
	in, out := br.EffectiveQuotaPer1k()
	if in != 500 || out != 1500 {
		t.Fatalf("quota in=%d out=%d", in, out)
	}
}
