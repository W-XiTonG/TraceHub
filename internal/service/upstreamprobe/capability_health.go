package upstreamprobe

import (
	"context"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/channelhealth"
)

// MaybeApplyCapabilityProbeHealth 落库后评估渠道视频能力是否全线 fail（非热路径）。
func MaybeApplyCapabilityProbeHealth(ctx context.Context, client *ent.Client, channelID int) {
	if client == nil {
		return
	}
	defaults := sitecfg.EffectiveRoutingDefaults(ctx, client)
	if !defaults.AutoHealthFromProbe {
		return
	}
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil || ch == nil {
		return
	}
	keys := collectLocalModelKeys(ch.Models, ch.ModelAlias)
	if len(keys) == 0 {
		return
	}
	videoModels := 0
	recentFail := 0
	for _, k := range keys {
		_, eff := CapabilityFromProbeRow(ctx, ch, k, CapabilityVideo, 0)
		if eff == CapStatusFail || eff == "stale" {
			recentFail++
		}
		if eff != "" && eff != CapStatusUnknown {
			videoModels++
		}
	}
	if videoModels == 0 {
		return
	}
	probeOK := recentFail < videoModels
	_ = channelhealth.ApplyProbeHealth(ctx, client, channelID, probeOK, defaults.ProbeFailThreshold)
}
