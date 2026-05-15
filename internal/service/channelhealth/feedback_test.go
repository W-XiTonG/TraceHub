package channelhealth

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestRecordUpstreamFailureAndSuccess(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:health?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, _ := client.Group.Create().SetName("g").Save(ctx)
	ch, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetModels([]string{"gpt-4o"}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := RecordUpstreamFailure(ctx, client, ch.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	fresh, _ := client.Channel.Get(ctx, ch.ID)
	if fresh.ErrorCount != 1 {
		t.Fatalf("error_count=%d", fresh.ErrorCount)
	}
	if err := RecordUpstreamSuccess(ctx, client, ch.ID); err != nil {
		t.Fatal(err)
	}
	fresh, _ = client.Channel.Get(ctx, ch.ID)
	if fresh.ErrorCount != 0 {
		t.Fatalf("error_count=%d", fresh.ErrorCount)
	}
}
