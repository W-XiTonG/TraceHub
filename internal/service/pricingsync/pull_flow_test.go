//go:build cgo

package pricingsync

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/modelpricingsynctarget"

	_ "github.com/mattn/go-sqlite3"
)

// 覆盖「下游拉取 → diff → apply」主路径（不发起 HTTP）。
func TestPullDiffAndApply_updatesQuota(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pspull?mode=memory&cache=shared&_fk=1")
	ctx := context.Background()
	_, err := client.GlobalModelPricing.Create().
		SetModelName("gpt-pull-test").
		SetEnabled(true).
		SetNillableQuotaInPer1k(ptrI64(100)).
		SetNillableQuotaOutPer1k(ptrI64(400)).
		SetInputRate(1).
		SetOutputRate(1).
		SetBillingMode("token").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target, err := client.ModelPricingSyncTarget.Create().
		SetName("upstream").
		SetBaseURL("https://upstream.example").
		SetPullPath("/api/pricing/sync/export").
		SetDirection(modelpricingsynctarget.DirectionPull).
		SetScopes([]string{ScopeGlobal}).
		SetMergeStrategy(modelpricingsynctarget.MergeStrategyUpsert).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote := &Payload{
		Version: VersionTraceHubPricingV1,
		Global: []GlobalItem{{
			ModelName:     "gpt-pull-test",
			Enabled:       true,
			QuotaInPer1k:  ptrI64(8),
			QuotaOutPer1k: ptrI64(8),
			InputRate:     1,
			OutputRate:    1,
			BillingMode:   "token",
		}},
	}
	filter := filterOptsFromTarget(target, nil, nil)
	rows, _, err := diffPullPayload(ctx, client, target, remote, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !rows[0].Selected || rows[0].Change != ChangeUpdated {
		t.Fatalf("rows=%+v", rows)
	}
	snap := &previewSnapshot{
		Payload:      remote,
		Rows:         rows,
		SelectedKeys: selectedKeysFromRows(rows),
	}
	res, err := runPull(ctx, ServiceOpts{Client: client}, target, snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 {
		t.Fatalf("updated=%d", res.Updated)
	}
	got, err := client.GlobalModelPricing.Query().Where().Only(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.QuotaInPer1k == nil || *got.QuotaInPer1k != 8 {
		t.Fatalf("quota_in=%v", got.QuotaInPer1k)
	}
}

func TestPullDiff_unchangedSkippedNoApply(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:pspullskip?mode=memory&cache=shared&_fk=1")
	ctx := context.Background()
	_, err := client.GlobalModelPricing.Create().
		SetModelName("same-model").
		SetEnabled(true).
		SetInputRate(1).
		SetOutputRate(1).
		SetBillingMode("token").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target, err := client.ModelPricingSyncTarget.Create().
		SetName("t").
		SetBaseURL("https://x.example").
		SetPullPath("/api/pricing/sync/export").
		SetDirection(modelpricingsynctarget.DirectionPull).
		SetScopes([]string{ScopeGlobal}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	remote := &Payload{
		Version: VersionTraceHubPricingV1,
		Global: []GlobalItem{{
			ModelName:   "same-model",
			Enabled:     true,
			InputRate:   1,
			OutputRate:  1,
			BillingMode: "token",
		}},
	}
	rows, _, err := diffPullPayload(ctx, client, target, remote, filterOptsFromTarget(target, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Selected || rows[0].Change != ChangeSkipped {
		t.Fatalf("rows=%+v", rows)
	}
	snap := &previewSnapshot{Payload: remote, Rows: rows, SelectedKeys: selectedKeysFromRows(rows)}
	res, err := runPull(ctx, ServiceOpts{Client: client}, target, snap)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added+res.Updated != 0 {
		t.Fatalf("unexpected writes: %+v", res)
	}
}
