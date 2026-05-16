package payment

import (
	"context"
	"testing"

	"time"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/pkg/config"

	_ "github.com/mattn/go-sqlite3"
)

func TestSettleOrderIdempotent(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.Site.CurrencyQuotaPerYuan = 10000
	svc := NewService(client, cfg, nil)

	u, err := client.User.Create().SetEmail("pay@test.com").SetBalance(0).Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PaymentOrder.Create().
		SetOrderNo("THTEST001").
		SetUserID(u.ID).
		SetProvider(ProviderMock).
		SetAmountCents(100).
		SetCurrency("CNY").
		SetQuotaAmount(10000).
		SetStatus(StatusPending).
		SetExpireAt(time.Now().Add(30 * time.Minute)).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SettleOrder(ctx, "THTEST001", "txn-1", 100); err != nil {
		t.Fatal(err)
	}
	if err := svc.SettleOrder(ctx, "THTEST001", "txn-1", 100); err != nil {
		t.Fatal(err)
	}
	u2, _ := client.User.Get(ctx, u.ID)
	if u2.Balance != 10000 {
		t.Fatalf("balance want 10000 got %d", u2.Balance)
	}
	n, _ := client.PaymentLedger.Query().Count(ctx)
	if n != 1 {
		t.Fatalf("ledger count want 1 got %d", n)
	}
}
