package availmodels

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestChannelModelPricingWorkspacePage_declaredOnly(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:chws?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	ch, _ := client.Channel.Create().
		SetChannelType("openai").
		SetModels([]string{"gpt-4o-mini", "custom-x"}).
		Save(ctx)

	items, total, _, err := ChannelModelPricingWorkspacePage(ctx, client, ch.ID, "", 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("want 2 items, got total=%d len=%d", total, len(items))
	}
	found := false
	for _, it := range items {
		if it.ModelName == "custom-x" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected custom-x in workspace")
	}
}
