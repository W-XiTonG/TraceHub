//go:build cgo

package pricingsyncmigrate

import (
	"context"
	"database/sql"
	"testing"

	"tracehub/internal/ent/enttest"

	_ "github.com/mattn/go-sqlite3"
)

func TestFixLegacyPullPaths(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:psmigrate?mode=memory&cache=shared&_fk=1")
	ctx := context.Background()
	_, err := client.ModelPricingSyncTarget.Create().
		SetName("legacy").
		SetBaseURL("https://example.com").
		SetPullPath("/api/pricing/export").
		SetPushPath("/api/pricing/import").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "file:psmigrate?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := FixLegacyPullPaths(ctx, db); err != nil {
		t.Fatal(err)
	}
	got, err := client.ModelPricingSyncTarget.Query().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.PullPath != correctPullPath {
		t.Fatalf("pull_path=%q want %q", got.PullPath, correctPullPath)
	}
	if got.PushPath != correctPushPath {
		t.Fatalf("push_path=%q want %q", got.PushPath, correctPushPath)
	}
}
