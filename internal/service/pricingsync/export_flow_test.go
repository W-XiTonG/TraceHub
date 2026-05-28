//go:build cgo

package pricingsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/ent/enttest"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

// 覆盖「服务端 Export + 入站 token 校验」对接路径（TraceHub 作价格源）。
func TestExportLocal_tracehubPricingV1(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:psexport?mode=memory&cache=shared&_fk=1")
	ctx := context.Background()
	plain := "thps_test_export_token_abc"
	sum := sha256.Sum256([]byte(plain))
	_, err := client.SystemConfig.Create().
		SetConfigKey(sitecfg.SystemConfigKeyPricingSyncInboundTokenSHA256).
		SetConfigValue(hex.EncodeToString(sum[:])).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !sitecfg.ValidateInboundSyncToken(ctx, client, plain) {
		t.Fatal("expected valid inbound token")
	}
	_, err = client.GlobalModelPricing.Create().
		SetModelName("gpt-export-test").
		SetEnabled(true).
		SetInputRate(1).
		SetOutputRate(1).
		SetBillingMode("token").
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tgt := &ent.ModelPricingSyncTarget{Scopes: []string{ScopeGlobal}}
	payload, err := ExportLocal(ctx, client, tgt)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Version != VersionTraceHubPricingV1 {
		t.Fatalf("version=%q", payload.Version)
	}
	if len(payload.Global) == 0 {
		t.Fatal("expected global pricing rows")
	}
	exportPath, _ := sitecfg.InboundPricingPaths()
	if exportPath != "/api/pricing/sync/export" {
		t.Fatalf("export path=%q", exportPath)
	}
}
