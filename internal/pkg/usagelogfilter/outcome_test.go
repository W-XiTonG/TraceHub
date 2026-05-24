//go:build cgo

package usagelogfilter_test

import (
	"context"
	"testing"
	"time"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/pkg/usagelogfilter"

	_ "github.com/mattn/go-sqlite3"
)

func TestApplyClientOutcomeOnly_excludesFailoverIntermediateFirstAttempt(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:outcomefilter?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	u, err := client.User.Create().SetEmail("of@test.local").SetUsername("of").SetPasswordHash("x").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rid := "req-chain"
	now := time.Now()
	_, err = client.UsageLog.Create().
		SetUserID(u.ID).SetModel("gpt-4o").SetRequestID(rid).
		SetUpstreamAttempt(1).SetIsClientOutcome(false).SetIsFailoverAttempt(true).SetStatusCode(503).
		SetRequestTime(now).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UsageLog.Create().
		SetUserID(u.ID).SetModel("gpt-4o").SetRequestID(rid).
		SetUpstreamAttempt(2).SetIsClientOutcome(true).SetIsFailoverAttempt(false).SetStatusCode(200).
		SetRequestTime(now).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.UsageLog.Create().
		SetUserID(u.ID).SetModel("gpt-4o").SetRequestID("legacy-single").
		SetUpstreamAttempt(1).SetIsClientOutcome(false).SetStatusCode(200).
		SetRequestTime(now).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	n, err := usagelogfilter.ApplyClientOutcomeOnly(client.UsageLog.Query()).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 client outcomes, got %d", n)
	}
}
