package adminhandler

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestValidateGroupModelRouting_rejectsForeignChannel(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:valmr?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g1, _ := client.Group.Create().SetName("g1").Save(ctx)
	g2, _ := client.Group.Create().SetName("g2").Save(ctx)
	ch, _ := client.Channel.Create().SetGroupID(g2.ID).SetChannelType("openai").SetModels([]string{"m"}).Save(ctx)

	err := validateGroupModelRouting(ctx, client, g1.ID, map[string][]int64{
		"gpt-4o": {int64(ch.ID)},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateGroupModelRouting_rejectsUndeclaredModel(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:valmr2?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g1").Save(ctx)
	ch, _ := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetModels([]string{"other-model"}).
		Save(ctx)

	err := validateGroupModelRouting(ctx, client, g.ID, map[string][]int64{
		"gpt-4o": {int64(ch.ID)},
	})
	if err == nil {
		t.Fatal("expected validation error for undeclared model")
	}
}
