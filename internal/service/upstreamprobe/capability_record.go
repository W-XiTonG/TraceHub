package upstreamprobe

import (
	"context"
	"strings"
	"time"

	"tracehub/internal/ent"
)

// MergeProbeRowWithOverlay 合并 DB 行与覆盖层（读路径）。
func MergeProbeRowWithOverlay(ctx context.Context, channelID int, logicalModel string, row map[string]interface{}) map[string]interface{} {
	if row == nil {
		row = make(map[string]interface{})
	}
	MergeCapabilityIntoModelProbe(ctx, channelID, logicalModel, row)
	return row
}

// CapabilityFromProbeRow 从模型 probe 行读取某能力状态（已合并覆盖层）。
func CapabilityFromProbeRow(ctx context.Context, ch *ent.Channel, logicalModel, capability string, failStaleHours int) (CapabilitySnapshot, string) {
	row := ch.ModelUpstreamProbe
	var modelRow map[string]interface{}
	if row != nil {
		if raw, ok := row[logicalModel]; ok {
			modelRow, _ = raw.(map[string]interface{})
		}
	}
	if modelRow == nil {
		modelRow = make(map[string]interface{})
	}
	modelRow = MergeProbeRowWithOverlay(ctx, ch.ID, logicalModel, modelRow)
	caps, _ := modelRow["capabilities"].(map[string]interface{})
	if caps == nil {
		return CapabilitySnapshot{}, ""
	}
	snap, ok := capRowFromMap(asMap(caps[capability]))
	if !ok {
		return CapabilitySnapshot{}, ""
	}
	eff := EffectiveCapStatus(snap, failStaleHours, time.Now())
	return snap, eff
}

// PickPrimaryCapability 按模型标签选择主要能力键。
func PickPrimaryCapability(labels []string) string {
	for _, l := range labels {
		switch strings.ToLower(strings.TrimSpace(l)) {
		case CapabilityVideo:
			return CapabilityVideo
		case CapabilityImage:
			return CapabilityImage
		case CapabilityAudio:
			return CapabilityAudio
		}
	}
	return ""
}
