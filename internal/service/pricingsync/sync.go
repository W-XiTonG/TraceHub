package pricingsync

import (
	"context"
	"fmt"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelpricingsynctarget"
	"tracehub/internal/pkg/sitecfg"

	"go.uber.org/zap"
)

// ServiceOpts 同步服务依赖。
type ServiceOpts struct {
	Client            *ent.Client
	AESKey            []byte
	AllowInsecureHTTP bool
	Log               *zap.Logger
}

// PreviewRequest 预览请求。
type PreviewRequest struct {
	Direction      string
	SelectedModels []string
	SelectedKeys   []string
}

// ConfirmRequest 确认执行请求。
type ConfirmRequest struct {
	PreviewID string
	Direction string
}

func normalizeDirection(d string) (string, error) {
	switch d {
	case DirectionPull, DirectionPush, DirectionBoth:
		return d, nil
	default:
		return "", errDirection
	}
}

func directionAllowed(t *ent.ModelPricingSyncTarget, d string) bool {
	switch t.Direction {
	case modelpricingsynctarget.DirectionPull:
		return d == DirectionPull
	case modelpricingsynctarget.DirectionPush:
		return d == DirectionPush
	case modelpricingsynctarget.DirectionBoth:
		return d == DirectionPull || d == DirectionPush || d == DirectionBoth
	}
	return false
}

func checkFeatureEnabled(ctx context.Context, client *ent.Client) error {
	if !sitecfg.EffectivePricingSyncEnabled(ctx, client) {
		return errFeatureDisabled
	}
	return nil
}

// Preview 生成同步预览并返回 preview_id。
func Preview(ctx context.Context, opts ServiceOpts, targetID int, req PreviewRequest) (PreviewResult, error) {
	if err := checkFeatureEnabled(ctx, opts.Client); err != nil {
		return PreviewResult{}, err
	}
	direction, err := normalizeDirection(req.Direction)
	if err != nil {
		return PreviewResult{}, err
	}
	t, err := opts.Client.ModelPricingSyncTarget.Get(ctx, targetID)
	if err != nil {
		return PreviewResult{}, err
	}
	if !t.Enabled {
		return PreviewResult{}, errTargetDisabled
	}
	if !directionAllowed(t, direction) {
		return PreviewResult{}, errDirection
	}
	// SelectedKeys 仅用于勾选状态，不参与 diff 过滤（否则刷新预览会只剩子集行）。
	filter := filterOptsFromTarget(t, req.SelectedModels, nil)
	var rows []DiffRow
	var warnings []string
	var payload *Payload

	switch direction {
	case DirectionPull:
		payload, err = FetchRemotePull(ctx, opts.Client, t, RemoteOpts{AESKey: opts.AESKey, AllowInsecureHTTP: opts.AllowInsecureHTTP})
		if err != nil {
			return PreviewResult{}, err
		}
		maxRows := sitecfg.EffectivePricingSyncMaxRows(ctx, opts.Client)
		if countPayloadRows(payload) > maxRows {
			return PreviewResult{}, errTooManyRows
		}
		rows, warnings, err = diffPullPayload(ctx, opts.Client, t, payload, filter)
		if err != nil {
			return PreviewResult{}, err
		}
	case DirectionPush:
		payload, err = CollectLocal(ctx, opts.Client, t, filter)
		if err != nil {
			return PreviewResult{}, err
		}
		maxRows := sitecfg.EffectivePricingSyncMaxRows(ctx, opts.Client)
		if countPayloadRows(payload) > maxRows {
			return PreviewResult{}, errTooManyRows
		}
		rows = diffPushPayload(payload, filter)
	default:
		return PreviewResult{}, errDirection
	}
	if len(req.SelectedKeys) > 0 {
		rows = applyRowSelection(rows, req.SelectedKeys)
	}
	expires := time.Now().Add(previewTTL)
	id := storePreview(&previewSnapshot{
		TargetID:     targetID,
		Direction:    direction,
		ExpiresAt:    expires,
		SelectedKeys: selectedKeysFromRows(rows),
		Payload:      payload,
		Rows:         rows,
		Warnings:     warnings,
	})
	return buildPreviewResult(id, direction, expires.UTC().Format(time.RFC3339), rows, warnings), nil
}

// Confirm 根据 preview_id 执行同步。
func Confirm(ctx context.Context, svc ServiceOpts, targetID int, req ConfirmRequest, manual bool) (SyncResult, error) {
	if err := checkFeatureEnabled(ctx, svc.Client); err != nil {
		return SyncResult{}, err
	}
	direction, err := normalizeDirection(req.Direction)
	if err != nil {
		return SyncResult{}, err
	}
	t, err := svc.Client.ModelPricingSyncTarget.Get(ctx, targetID)
	if err != nil {
		return SyncResult{}, err
	}
	if !t.Enabled {
		return SyncResult{}, errTargetDisabled
	}
	if !directionAllowed(t, direction) {
		return SyncResult{}, errDirection
	}
	snap, err := takePreview(req.PreviewID, targetID, direction)
	if err != nil {
		return SyncResult{}, err
	}
	var result SyncResult
	err = withTargetLock(targetID, func() error {
		if manual {
			cooldown := sitecfg.EffectivePricingSyncManualCooldownSec(ctx, svc.Client)
			if err := checkManualCooldown(targetID, cooldown); err != nil {
				return err
			}
		}
		switch direction {
		case DirectionPull:
			r, e := runPull(ctx, svc, t, snap)
			result = r
			return e
		case DirectionPush:
			r, e := runPush(ctx, svc, t, snap)
			result = r
			return e
		default:
			return errDirection
		}
	})
	if err != nil {
		return SyncResult{}, err
	}
	if manual {
		setLastManualSync(targetID)
	}
	return result, nil
}

func runPull(ctx context.Context, svc ServiceOpts, t *ent.ModelPricingSyncTarget, snap *previewSnapshot) (SyncResult, error) {
	filter := filterOptsFromTarget(t, nil, snap.SelectedKeys)
	payload := filterPayloadBySelectedKeys(snap.Payload, selectedKeySet(snap.Rows), t)
	plan, warnings, err := buildApplyPlan(ctx, svc.Client, t, payload, filter)
	if err != nil {
		updatePullStatus(ctx, svc, t.ID, false, err.Error())
		return SyncResult{}, err
	}
	added, updated, err := executeApplyPlan(ctx, svc.Client, plan)
	if err != nil {
		updatePullStatus(ctx, svc, t.ID, false, err.Error())
		return SyncResult{}, err
	}
	invalidateCachesAfterApply(plan)
	_, _, skippedCount, _ := summarizeRows(snap.Rows)
	res := SyncResult{
		Added:    added,
		Updated:  updated,
		Skipped:  skippedCount,
		Removed:  plan.Removed,
		Warnings: append(snap.Warnings, warnings...),
		Message:  "",
	}
	res.Message = formatSyncMessage(res)
	updatePullStatus(ctx, svc, t.ID, true, res.Message)
	return res, nil
}

func runPush(ctx context.Context, svc ServiceOpts, t *ent.ModelPricingSyncTarget, snap *previewSnapshot) (SyncResult, error) {
	payload := filterPayloadBySelectedKeys(snap.Payload, selectedKeySet(snap.Rows), t)
	if payload == nil {
		payload = snap.Payload
	}
	if err := PostRemotePush(ctx, svc.Client, t, payload, RemoteOpts{AESKey: svc.AESKey, AllowInsecureHTTP: svc.AllowInsecureHTTP}); err != nil {
		updatePushStatus(ctx, svc, t.ID, false, err.Error())
		return SyncResult{}, err
	}
	added, updated, skipped, _ := summarizeRows(snap.Rows)
	res := SyncResult{
		Added:    added,
		Updated:  updated,
		Skipped:  skipped,
		Warnings: snap.Warnings,
		Message:  fmt.Sprintf("推送完成: %d 条记录", added+updated),
	}
	updatePushStatus(ctx, svc, t.ID, true, res.Message)
	return res, nil
}

func updatePullStatus(ctx context.Context, svc ServiceOpts, id int, ok bool, msg string) {
	if err := svc.Client.ModelPricingSyncTarget.UpdateOneID(id).
		SetLastPullAt(time.Now()).
		SetLastPullOk(ok).
		SetLastPullMessage(msg).
		Exec(ctx); err != nil && svc.Log != nil {
		svc.Log.Warn("pricing_sync update pull status failed",
			zap.Int("target_id", id),
			zap.Bool("ok", ok),
			zap.Error(err),
		)
	}
}

func updatePushStatus(ctx context.Context, svc ServiceOpts, id int, ok bool, msg string) {
	if err := svc.Client.ModelPricingSyncTarget.UpdateOneID(id).
		SetLastPushAt(time.Now()).
		SetLastPushOk(ok).
		SetLastPushMessage(msg).
		Exec(ctx); err != nil && svc.Log != nil {
		svc.Log.Warn("pricing_sync update push status failed",
			zap.Int("target_id", id),
			zap.Bool("ok", ok),
			zap.Error(err),
		)
	}
}

// ExportLocal 导出本地定价快照。
func ExportLocal(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget) (*Payload, error) {
	opts := filterOptsFromTarget(t, nil, nil)
	return CollectLocal(ctx, client, t, opts)
}

// RunScheduledTarget Worker 定时执行（无 preview，直接 pull/push）。
func RunScheduledTarget(ctx context.Context, svc ServiceOpts, targetID int) error {
	if !sitecfg.EffectivePricingSyncEnabled(ctx, svc.Client) {
		return errFeatureDisabled
	}
	if !sitecfg.EffectivePricingSyncWorkerEnabled(ctx, svc.Client) {
		return nil
	}
	t, err := svc.Client.ModelPricingSyncTarget.Get(ctx, targetID)
	if err != nil || !t.Enabled || !t.ScheduleEnabled {
		return err
	}
	return withTargetLock(targetID, func() error {
		filter := filterOptsFromTarget(t, nil, nil)
		switch t.Direction {
		case modelpricingsynctarget.DirectionPull, modelpricingsynctarget.DirectionBoth:
			p, err := FetchRemotePull(ctx, svc.Client, t, RemoteOpts{AESKey: svc.AESKey, AllowInsecureHTTP: svc.AllowInsecureHTTP})
			if err != nil {
				updatePullStatus(ctx, svc, t.ID, false, err.Error())
				return err
			}
			maxRows := sitecfg.EffectivePricingSyncMaxRows(ctx, svc.Client)
			if countPayloadRows(p) > maxRows {
				updatePullStatus(ctx, svc, t.ID, false, errTooManyRows.Error())
				return errTooManyRows
			}
			rows, _, err := diffPullPayload(ctx, svc.Client, t, p, filter)
			if err != nil {
				updatePullStatus(ctx, svc, t.ID, false, err.Error())
				return err
			}
			snap := &previewSnapshot{Payload: p, Rows: rows, SelectedKeys: selectedKeysFromRows(rows)}
			_, err = runPull(ctx, svc, t, snap)
			return err
		case modelpricingsynctarget.DirectionPush:
			return nil
		default:
			return errDirection
		}
	})
}
