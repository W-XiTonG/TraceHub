package upstreamprobe

import (
	"time"

	"tracehub/internal/ent"
)

// CapabilityFromProbeRowDBOnly 仅从渠道 DB 中的 model_upstream_probe 读取能力状态（不访问 Redis 覆盖层）。
func CapabilityFromProbeRowDBOnly(ch *ent.Channel, logicalModel, capability string, failStaleHours int) (CapabilitySnapshot, string) {
	row := ch.ModelUpstreamProbe
	var modelRow map[string]interface{}
	if row != nil {
		if raw, ok := row[logicalModel]; ok {
			modelRow, _ = raw.(map[string]interface{})
		}
	}
	if modelRow == nil {
		return CapabilitySnapshot{}, ""
	}
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
