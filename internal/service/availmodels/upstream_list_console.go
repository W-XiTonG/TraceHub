package availmodels

import (
	"strings"
	"time"

	"tracehub/internal/ent"
)

func channelDeclaresModel(ch *ent.Channel, modelID string) bool {
	m := strings.TrimSpace(modelID)
	if m == "" {
		return false
	}
	for _, x := range ch.Models {
		if strings.TrimSpace(x) == m {
			return true
		}
	}
	if ch.ModelAlias != nil {
		for k := range ch.ModelAlias {
			if strings.TrimSpace(k) == m {
				return true
			}
		}
	}
	return false
}

func asBoolFromProbe(v interface{}) (val bool, ok bool) {
	switch x := v.(type) {
	case bool:
		return x, true
	case float64:
		return x != 0, true
	default:
		return false, false
	}
}

// attachUpstreamListProbeFromChannels 将各渠道 model_upstream_probe 聚合到控制台模型行（仅 InGateway）。
// 状态：listed=至少一条声明渠道探测为在列表；not_listed=有探测且均为不在列表；mixed=有在列表与不在列表；空=无有效探测数据。
func attachUpstreamListProbeFromChannels(channels []*ent.Channel, byID map[string]*ConsoleModel) {
	for _, cm := range byID {
		if !cm.InGateway {
			continue
		}
		var anyTrue, anyFalse bool
		var latest time.Time
		var latestSet bool
		for _, ch := range channels {
			if !channelDeclaresModel(ch, cm.ID) {
				continue
			}
			probe := ch.ModelUpstreamProbe
			if probe == nil {
				continue
			}
			raw, ok := probe[cm.ID]
			if !ok {
				continue
			}
			row, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if e, _ := row["error"].(string); strings.TrimSpace(e) != "" {
				continue
			}
			inList, okb := asBoolFromProbe(row["in_upstream_list"])
			if !okb {
				continue
			}
			if s, _ := row["checked_at"].(string); strings.TrimSpace(s) != "" {
				if ts, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
					if !latestSet || ts.After(latest) {
						latest = ts
						latestSet = true
					}
				}
			}
			if inList {
				anyTrue = true
			} else {
				anyFalse = true
			}
		}
		switch {
		case anyTrue && anyFalse:
			cm.UpstreamListStatus = "mixed"
		case anyTrue:
			cm.UpstreamListStatus = "listed"
		case anyFalse:
			cm.UpstreamListStatus = "not_listed"
		}
		if latestSet {
			cm.UpstreamListCheckedAt = latest.UTC().Format(time.RFC3339)
		}
	}
}
