//go:build cgo

package gateway

import (
	"testing"
	"time"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/usagelog"

	_ "github.com/mattn/go-sqlite3"
)

func TestUsageRecorder_SaveFailoverAttempt_doesNotMarkLogged(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failoverrec?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	u, err := client.User.Create().SetEmail("fo@test.local").SetUsername("fo").SetPasswordHash("x").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	k, err := client.APIKey.Create().SetUserID(u.ID).SetKeyHash("kh").SetKeyPrefix("sk-f").SetName("k").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ch, err := client.Channel.Create().SetName("ch").SetChannelType("openai").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	rec := &UsageRecorder{
		client:    client,
		userID:    u.ID,
		apiKeyID:  k.ID,
		requestID: "req-fo",
		model:     "gpt-4o",
		start:     time.Now(),
	}
	rec.SaveFailoverAttempt(ch.ID, "gpt-4o", 503, nil, 1, 0)
	if rec.IsLogged() {
		t.Fatal("SaveFailoverAttempt should not MarkLogged")
	}
	rec.SetOutcomeAttempt(2)
	rec.SaveFail(502, "transport", 0, 0, 0)
	if !rec.IsLogged() {
		t.Fatal("SaveFail should MarkLogged")
	}

	waitUsageLogs(t, client, 2)
	logs, err := client.UsageLog.Query().
		Where(usagelog.RequestIDEQ("req-fo")).
		Order(usagelog.ByUpstreamAttempt()).
		All(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(logs))
	}
	if logs[0].IsFailoverAttempt != true || logs[0].IsClientOutcome || logs[0].UpstreamAttempt != 1 || logs[0].TotalCost != 0 {
		t.Fatalf("intermediate log: failover=%v outcome=%v attempt=%d cost=%d", logs[0].IsFailoverAttempt, logs[0].IsClientOutcome, logs[0].UpstreamAttempt, logs[0].TotalCost)
	}
	if logs[1].IsFailoverAttempt || !logs[1].IsClientOutcome || logs[1].UpstreamAttempt != 2 {
		t.Fatalf("outcome log: failover=%v outcome=%v attempt=%d", logs[1].IsFailoverAttempt, logs[1].IsClientOutcome, logs[1].UpstreamAttempt)
	}
}
