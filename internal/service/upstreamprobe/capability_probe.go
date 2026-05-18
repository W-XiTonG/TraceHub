package upstreamprobe

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/pkg/modelkind"
)

// CapabilityProbeMaxPerRound 每渠道每轮能力冒烟上限。
const CapabilityProbeMaxPerRound = 10

// ProbeChannelCapabilities 对单渠道执行视频能力冒烟（直连上游，不经网关 handler）。
func ProbeChannelCapabilities(ctx context.Context, client *ent.Client, aesKey []byte, ch *ent.Channel, maxModels int) error {
	if maxModels <= 0 {
		maxModels = CapabilityProbeMaxPerRound
	}
	bearer, err := bearerForChannel(aesKey, ch)
	if err != nil {
		return err
	}
	baseURL := strings.TrimSpace(ch.APIBaseURL)
	localKeys := collectLocalModelKeys(ch.Models, ch.ModelAlias)
	cutoff := time.Now().Add(-24 * time.Hour)
	var picked []string
	for _, modelID := range localKeys {
		if len(picked) >= maxModels {
			break
		}
		labels := modelkind.LabelsForModel(modelID, nil, nil)
		if PickPrimaryCapability(labels) != CapabilityVideo {
			continue
		}
		snap, eff := CapabilityFromProbeRow(ctx, ch, modelID, CapabilityVideo, 0)
		if eff == CapStatusOK && !snap.CheckedAt.IsZero() && snap.CheckedAt.After(cutoff) {
			continue
		}
		picked = append(picked, modelID)
	}
	for _, modelID := range picked {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		upstream := modelID
		if ch.ModelAlias != nil {
			if t, ok := ch.ModelAlias[modelID]; ok && strings.TrimSpace(t) != "" {
				upstream = strings.TrimSpace(t)
			}
		}
		body, _ := json.Marshal(map[string]string{
			"model":  upstream,
			"prompt": "tracehub capability probe",
		})
		resp, ferr := relay.ForwardOpenAIVideoCreate(ctx, baseURL, bearer, body)
		var status int
		var raw []byte
		if ferr != nil {
			status = 0
		} else if resp != nil {
			status = resp.StatusCode
			raw, _ = io.ReadAll(resp.Body)
			relay.DrainBody(resp.Body)
		}
		NotifyCapabilityOutcome(ch.ID, modelID, CapabilityVideo, status, raw, CapSourceProbe)
		select {
		case <-time.After(400 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
