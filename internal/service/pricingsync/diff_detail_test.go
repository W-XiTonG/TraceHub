package pricingsync

import (
	"testing"

	"tracehub/internal/ent"
)

func ptrI64(v int64) *int64 { return &v }
func ptrF64(v float64) *float64 { return &v }

func TestDiffGlobalItem_updatedHasFieldChanges(t *testing.T) {
	existing := &ent.GlobalModelPricing{
		ModelName:     "gpt-test",
		Enabled:       true,
		QuotaInPer1k:  ptrI64(100),
		QuotaOutPer1k: ptrI64(400),
		InputRate:     1,
		OutputRate:    1,
		BillingMode:   "token",
	}
	item := GlobalItem{
		ModelName:     "gpt-test",
		Enabled:       true,
		QuotaInPer1k:  ptrI64(8),
		QuotaOutPer1k: ptrI64(8),
		InputRate:     1,
		OutputRate:    1,
		MarkupPercent: ptrF64(5),
		BillingMode:   "token",
	}
	changed, detail, changes := diffGlobalItem(existing, item)
	if !changed {
		t.Fatal("expected changed")
	}
	if detail == "" {
		t.Fatal("expected non-empty detail")
	}
	if len(changes) == 0 {
		t.Fatal("expected field changes")
	}
	row := buildPullRow(&ent.ModelPricingSyncTarget{}, ScopeGlobal, item.ModelName, 0, 0, true, func() (bool, string, []FieldChange) {
		return changed, detail, changes
	})
	if row.Change != ChangeUpdated {
		t.Fatalf("change=%q", row.Change)
	}
	if len(row.FieldChanges) == 0 {
		t.Fatal("row missing field_changes")
	}
	if row.Detail == "" {
		t.Fatal("row missing detail")
	}
}

func TestDiffGlobalItem_unchangedSkipped(t *testing.T) {
	existing := &ent.GlobalModelPricing{
		ModelName:   "same",
		Enabled:     true,
		InputRate:   1,
		OutputRate:  1,
		BillingMode: "token",
	}
	item := GlobalItem{
		ModelName:  "same",
		Enabled:    true,
		InputRate:  1,
		OutputRate: 1,
		BillingMode: "token",
	}
	changed, detail, changes := diffGlobalItem(existing, item)
	if changed {
		t.Fatalf("expected unchanged, detail=%q changes=%d", detail, len(changes))
	}
	row := buildPullRow(&ent.ModelPricingSyncTarget{}, ScopeGlobal, item.ModelName, 0, 0, true, func() (bool, string, []FieldChange) {
		return changed, detail, changes
	})
	if row.Change != ChangeSkipped {
		t.Fatalf("change=%q want skipped", row.Change)
	}
}
