package billing

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/user"

	_ "github.com/mattn/go-sqlite3"
)

func TestModelBillRates_channelQuotaOverridesGlobal(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:chq?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("gch").Save(ctx)
	chRow, _ := client.Channel.Create().SetChannelType("openai").SetModels([]string{"gpt-4o-mini"}).Save(ctx)
	u, _ := client.User.Create().SetEmail("u@chq.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(100).
		SetQuotaOutPer1k(100).
		Save(ctx)
	_, _ = client.ChannelModelPricing.Create().
		SetChannelID(chRow.ID).
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetQuotaInPer1k(777).
		SetQuotaOutPer1k(666).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini", chRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if br.in1k != 777 || br.out1k != 666 {
		t.Fatalf("want channel quota 777,666 got %d,%d", br.in1k, br.out1k)
	}
	br0, _ := eng.ModelBillRates(ctx, u, "gpt-4o-mini", 0)
	if br0.in1k != 100 || br0.out1k != 100 {
		t.Fatalf("without channel want global 100,100 got %d,%d", br0.in1k, br0.out1k)
	}
}

func TestModelBillRates_channelRatesWhenNoGroupRow(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:chr?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("gchr").Save(ctx)
	chRow, _ := client.Channel.Create().SetChannelType("openai").Save(ctx)
	u, _ := client.User.Create().SetEmail("u@chr.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	_, _ = client.GlobalModelPricing.Create().
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(2).
		SetOutputRate(3).
		Save(ctx)
	_, _ = client.ChannelModelPricing.Create().
		SetChannelID(chRow.ID).
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(4).
		SetOutputRate(6).
		Save(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini", chRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 4 || br.outRate != 6 {
		t.Fatalf("want channel rates 4,6 got %v,%v", br.inRate, br.outRate)
	}
}

func TestModelBillRates_groupRateOverridesChannel(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:chgr?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("gchgr").Save(ctx)
	chRow, _ := client.Channel.Create().SetChannelType("openai").Save(ctx)
	_, _ = client.GroupModelRate.Create().
		SetGroupID(g.ID).
		SetModelName("gpt-4o-mini").
		SetInputRate(9).
		SetOutputRate(8).
		SetEnabled(true).
		Save(ctx)
	_, _ = client.ChannelModelPricing.Create().
		SetChannelID(chRow.ID).
		SetModelName("gpt-4o-mini").
		SetEnabled(true).
		SetInputRate(4).
		SetOutputRate(6).
		Save(ctx)
	u, _ := client.User.Create().SetEmail("u@chgr.test").SetGroupID(g.ID).Save(ctx)
	u, _ = client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)

	eng := &Engine{Client: client}
	br, err := eng.ModelBillRates(ctx, u, "gpt-4o-mini", chRow.ID)
	if err != nil {
		t.Fatal(err)
	}
	if br.inRate != 9 || br.outRate != 8 {
		t.Fatalf("want group rates 9,8 got %v,%v", br.inRate, br.outRate)
	}
}
