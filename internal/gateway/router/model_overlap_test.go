package router

import (
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestGroupModelChannelOverlap_twoChannels(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:overlap?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := t.Context()

	g, err := client.Group.Create().SetName("ov-g").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetChannelName("A").
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetChannelName("B").
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GroupModelChannelOverlap(ctx, client, g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Model != "gpt-4o" || len(items[0].Channels) != 2 {
		t.Fatalf("got %+v", items)
	}
}

func TestGroupModelChannelOverlap_singleChannelExcluded(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:overlap1?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := t.Context()

	g, err := client.Group.Create().SetName("ov1").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GroupModelChannelOverlap(ctx, client, g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no overlaps, got %+v", items)
	}
}

func TestGroupModelChannelOverlap_aliasKey(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:overlapalias?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := t.Context()

	g, err := client.Group.Create().SetName("ov-alias").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetModels([]string{"upstream-id"}).
		SetModelAlias(map[string]string{"gpt-4o": "upstream-id"}).
		SetEnabled(true).
		SetSchedulable(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	items, err := GroupModelChannelOverlap(ctx, client, g.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Model != "gpt-4o" {
		t.Fatalf("got %+v", items)
	}
}
