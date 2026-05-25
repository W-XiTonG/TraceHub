package pricingsync

import (
	"context"
	"fmt"
	"math"
	"strings"

	"tracehub/internal/ent"
)

const floatTol = 1e-9

const (
	labelEnabled     = "启用"
	labelQuotaIn     = "输入额度/1k"
	labelQuotaOut    = "输出额度/1k"
	labelInputRate   = "输入倍率"
	labelOutputRate  = "输出倍率"
	labelMarkup      = "成本加成%"
	labelBilling     = "计费模式"
	labelQuotaReq    = "按次额度"
	labelBillable    = "可计费"
)

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) <= floatTol
}

func int64PtrEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return floatEqual(*a, *b)
}

func fmtBool(v bool) string {
	if v {
		return "开"
	}
	return "关"
}

func fmtInt64Ptr(p *int64) string {
	if p == nil {
		return "未设置"
	}
	return fmt.Sprintf("%d", *p)
}

func fmtFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}

func fmtFloatPtr(p *float64) string {
	if p == nil {
		return "未设置"
	}
	return fmtFloat(*p)
}

func fmtBillingMode(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "未设置"
	}
	return v
}

func fmtBillingModePtr(p *string) string {
	if p == nil {
		return "未设置"
	}
	return fmtBillingMode(*p)
}

func appendFieldChange(changes []FieldChange, label, old, new string) []FieldChange {
	if old == new {
		return changes
	}
	return append(changes, FieldChange{Label: label, Old: old, New: new})
}

func appendNewField(changes []FieldChange, label, val string) []FieldChange {
	return append(changes, FieldChange{Label: label, New: val})
}

func summarizeChanges(changes []FieldChange) string {
	if len(changes) == 0 {
		return "无变更"
	}
	if len(changes) == 1 {
		c := changes[0]
		if c.Old == "" {
			return fmt.Sprintf("%s → %s", c.Label, c.New)
		}
		return fmt.Sprintf("%s: %s → %s", c.Label, c.Old, c.New)
	}
	return fmt.Sprintf("%d 项变更", len(changes))
}

func newGlobalFields(item GlobalItem) []FieldChange {
	inRate := defaultRate(item.InputRate)
	outRate := defaultRate(item.OutputRate)
	bm := fmtBillingMode(item.BillingMode)
	if bm == "未设置" {
		bm = "token"
	}
	var changes []FieldChange
	changes = appendNewField(changes, labelEnabled, fmtBool(item.Enabled))
	if item.QuotaInPer1k != nil {
		changes = appendNewField(changes, labelQuotaIn, fmtInt64Ptr(item.QuotaInPer1k))
	}
	if item.QuotaOutPer1k != nil {
		changes = appendNewField(changes, labelQuotaOut, fmtInt64Ptr(item.QuotaOutPer1k))
	}
	if !floatEqual(inRate, 1) {
		changes = appendNewField(changes, labelInputRate, fmtFloat(inRate))
	}
	if !floatEqual(outRate, 1) {
		changes = appendNewField(changes, labelOutputRate, fmtFloat(outRate))
	}
	if item.MarkupPercent != nil {
		changes = appendNewField(changes, labelMarkup, fmtFloatPtr(item.MarkupPercent))
	}
	if bm != "token" {
		changes = appendNewField(changes, labelBilling, bm)
	}
	if item.QuotaPerRequest != nil {
		changes = appendNewField(changes, labelQuotaReq, fmtInt64Ptr(item.QuotaPerRequest))
	}
	return changes
}

func diffGlobalItem(existing *ent.GlobalModelPricing, item GlobalItem) (changed bool, detail string, changes []FieldChange) {
	inRate := defaultRate(item.InputRate)
	outRate := defaultRate(item.OutputRate)
	bm := fmtBillingMode(item.BillingMode)
	if bm == "未设置" {
		bm = "token"
	}

	if existing == nil {
		changes = newGlobalFields(item)
		return true, summarizeChanges(changes), changes
	}

	if existing.Enabled != item.Enabled {
		changes = appendFieldChange(changes, labelEnabled, fmtBool(existing.Enabled), fmtBool(item.Enabled))
	}
	if !int64PtrEqual(existing.QuotaInPer1k, item.QuotaInPer1k) {
		changes = appendFieldChange(changes, labelQuotaIn, fmtInt64Ptr(existing.QuotaInPer1k), fmtInt64Ptr(item.QuotaInPer1k))
	}
	if !int64PtrEqual(existing.QuotaOutPer1k, item.QuotaOutPer1k) {
		changes = appendFieldChange(changes, labelQuotaOut, fmtInt64Ptr(existing.QuotaOutPer1k), fmtInt64Ptr(item.QuotaOutPer1k))
	}
	if !floatEqual(existing.InputRate, inRate) {
		changes = appendFieldChange(changes, labelInputRate, fmtFloat(existing.InputRate), fmtFloat(inRate))
	}
	if !floatEqual(existing.OutputRate, outRate) {
		changes = appendFieldChange(changes, labelOutputRate, fmtFloat(existing.OutputRate), fmtFloat(outRate))
	}
	if !floatPtrEqual(existing.MarkupPercent, item.MarkupPercent) {
		changes = appendFieldChange(changes, labelMarkup, fmtFloatPtr(existing.MarkupPercent), fmtFloatPtr(item.MarkupPercent))
	}
	if existing.BillingMode != bm {
		changes = appendFieldChange(changes, labelBilling, fmtBillingMode(existing.BillingMode), bm)
	}
	if !int64PtrEqual(existing.QuotaPerRequest, item.QuotaPerRequest) {
		changes = appendFieldChange(changes, labelQuotaReq, fmtInt64Ptr(existing.QuotaPerRequest), fmtInt64Ptr(item.QuotaPerRequest))
	}
	if len(changes) == 0 {
		return false, "无变更", nil
	}
	return true, summarizeChanges(changes), changes
}

func diffChannelItem(existing *ent.ChannelModelPricing, item GlobalItem) (changed bool, detail string, changes []FieldChange) {
	inRate := defaultRate(item.InputRate)
	outRate := defaultRate(item.OutputRate)
	var incomingBM *string
	if s := strings.TrimSpace(item.BillingMode); s != "" {
		incomingBM = &s
	}

	if existing == nil {
		changes = newGlobalFields(item)
		return true, summarizeChanges(changes), changes
	}

	if existing.Enabled != item.Enabled {
		changes = appendFieldChange(changes, labelEnabled, fmtBool(existing.Enabled), fmtBool(item.Enabled))
	}
	if !int64PtrEqual(existing.QuotaInPer1k, item.QuotaInPer1k) {
		changes = appendFieldChange(changes, labelQuotaIn, fmtInt64Ptr(existing.QuotaInPer1k), fmtInt64Ptr(item.QuotaInPer1k))
	}
	if !int64PtrEqual(existing.QuotaOutPer1k, item.QuotaOutPer1k) {
		changes = appendFieldChange(changes, labelQuotaOut, fmtInt64Ptr(existing.QuotaOutPer1k), fmtInt64Ptr(item.QuotaOutPer1k))
	}
	if !floatEqual(existing.InputRate, inRate) {
		changes = appendFieldChange(changes, labelInputRate, fmtFloat(existing.InputRate), fmtFloat(inRate))
	}
	if !floatEqual(existing.OutputRate, outRate) {
		changes = appendFieldChange(changes, labelOutputRate, fmtFloat(existing.OutputRate), fmtFloat(outRate))
	}
	if !floatPtrEqual(existing.MarkupPercent, item.MarkupPercent) {
		changes = appendFieldChange(changes, labelMarkup, fmtFloatPtr(existing.MarkupPercent), fmtFloatPtr(item.MarkupPercent))
	}
	if !billingModePtrEqual(existing.BillingMode, incomingBM) {
		changes = appendFieldChange(changes, labelBilling, fmtBillingModePtr(existing.BillingMode), fmtBillingModePtr(incomingBM))
	}
	if !int64PtrEqual(existing.QuotaPerRequest, item.QuotaPerRequest) {
		changes = appendFieldChange(changes, labelQuotaReq, fmtInt64Ptr(existing.QuotaPerRequest), fmtInt64Ptr(item.QuotaPerRequest))
	}
	if len(changes) == 0 {
		return false, "无变更", nil
	}
	return true, summarizeChanges(changes), changes
}

func billingModePtrEqual(a, b *string) bool {
	return fmtBillingModePtr(a) == fmtBillingModePtr(b)
}

func newGroupFields(item GroupItem) []FieldChange {
	inRate := defaultRate(item.InputRate)
	outRate := defaultRate(item.OutputRate)
	var changes []FieldChange
	changes = appendNewField(changes, labelEnabled, fmtBool(item.Enabled))
	changes = appendNewField(changes, labelBillable, fmtBool(item.Billable))
	if !floatEqual(inRate, 1) {
		changes = appendNewField(changes, labelInputRate, fmtFloat(inRate))
	}
	if !floatEqual(outRate, 1) {
		changes = appendNewField(changes, labelOutputRate, fmtFloat(outRate))
	}
	return changes
}

func diffGroupItem(existing *ent.GroupModelRate, item GroupItem) (changed bool, detail string, changes []FieldChange) {
	inRate := defaultRate(item.InputRate)
	outRate := defaultRate(item.OutputRate)

	if existing == nil {
		changes = newGroupFields(item)
		return true, summarizeChanges(changes), changes
	}

	if existing.Enabled != item.Enabled {
		changes = appendFieldChange(changes, labelEnabled, fmtBool(existing.Enabled), fmtBool(item.Enabled))
	}
	if existing.Billable != item.Billable {
		changes = appendFieldChange(changes, labelBillable, fmtBool(existing.Billable), fmtBool(item.Billable))
	}
	if !floatEqual(existing.InputRate, inRate) {
		changes = appendFieldChange(changes, labelInputRate, fmtFloat(existing.InputRate), fmtFloat(inRate))
	}
	if !floatEqual(existing.OutputRate, outRate) {
		changes = appendFieldChange(changes, labelOutputRate, fmtFloat(existing.OutputRate), fmtFloat(outRate))
	}
	if len(changes) == 0 {
		return false, "无变更", nil
	}
	return true, summarizeChanges(changes), changes
}

func pullRowFromGlobal(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, item GlobalItem) (DiffRow, error) {
	existing, ok, err := lookupGlobal(ctx, client, item.ModelName)
	if err != nil {
		return DiffRow{}, err
	}
	return buildPullRow(t, ScopeGlobal, item.ModelName, 0, 0, ok, func() (bool, string, []FieldChange) {
		return diffGlobalItem(existing, item)
	}), nil
}

func pullRowFromChannel(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, cid int, item GlobalItem) (DiffRow, error) {
	existing, ok, err := lookupChannel(ctx, client, cid, item.ModelName)
	if err != nil {
		return DiffRow{}, err
	}
	return buildPullRow(t, ScopeChannel, item.ModelName, cid, 0, ok, func() (bool, string, []FieldChange) {
		return diffChannelItem(existing, item)
	}), nil
}

func pullRowFromGroup(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, gid int, item GroupItem) (DiffRow, error) {
	existing, ok, err := lookupGroupRate(ctx, client, gid, item.ModelName)
	if err != nil {
		return DiffRow{}, err
	}
	return buildPullRow(t, ScopeGroup, item.ModelName, 0, gid, ok, func() (bool, string, []FieldChange) {
		return diffGroupItem(existing, item)
	}), nil
}

func buildPullRow(t *ent.ModelPricingSyncTarget, layer, modelName string, channelID, groupID int, exists bool, diffFn func() (bool, string, []FieldChange)) DiffRow {
	changed, detail, changes := diffFn()
	ch := ChangeAdded
	selected := true
	if exists {
		if changed {
			ch = ChangeUpdated
		} else {
			ch = ChangeSkipped
			selected = false
		}
	}
	if string(t.MergeStrategy) == "skip_existing" && exists {
		ch = ChangeSkipped
		detail = "已存在，跳过"
		changes = nil
		selected = false
	}
	return DiffRow{
		Layer:        layer,
		ModelName:    modelName,
		ChannelID:    channelID,
		GroupID:      groupID,
		Change:       ch,
		Selected:     selected,
		Detail:       detail,
		FieldChanges: changes,
	}
}
