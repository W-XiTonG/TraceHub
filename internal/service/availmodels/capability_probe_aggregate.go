package availmodels

import (
	"context"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/upstreamprobe"
)

const (
	ServiceStatusOperational = "operational"
	ServiceStatusUnavailable = "unavailable"
	ServiceStatusStale       = "stale"
	ServiceStatusDegraded      = "degraded"
	ServiceStatusCatalogOnly = "catalog_only"
)

// ListOpts 控制模型列表聚合与过滤。
type ListOpts struct {
	OmitStatuses          []string
	IncludeUnavailable    bool
	FailStaleHours        int
	SkipCapabilityOverlay bool // 控制台大列表：跳过 Redis 能力覆盖层，避免数百次 GET
}

func defaultListOpts(ctx context.Context, client *ent.Client) ListOpts {
	h := sitecfg.EffectiveUpstreamCapabilityFailStaleHours(ctx, client)
	return ListOpts{
		OmitStatuses:       []string{ServiceStatusUnavailable},
		IncludeUnavailable: false,
		FailStaleHours:     h,
	}
}

// ListOptsFromQuery 根据是否包含 unavailable 构建列表选项。
func ListOptsFromQuery(ctx context.Context, client *ent.Client, includeUnavailable bool) ListOpts {
	o := defaultListOpts(ctx, client)
	if includeUnavailable {
		o.IncludeUnavailable = true
		o.OmitStatuses = nil
	}
	return o
}

func shouldOmitStatus(opts ListOpts, status string) bool {
	if opts.IncludeUnavailable {
		return false
	}
	for _, s := range opts.OmitStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// AggregateServiceStatusForModel 跨渠道合并能力探测为单一 service_status。
func AggregateServiceStatusForModel(
	ctx context.Context,
	channels []*ent.Channel,
	modelID string,
	labels []string,
	failStaleHours int,
) string {
	return AggregateServiceStatusForModelOpts(ctx, channels, modelID, labels, failStaleHours, false)
}

// AggregateServiceStatusForModelOpts 同 AggregateServiceStatusForModel；skipOverlay 为 true 时不读 Redis 覆盖层（大列表性能）。
func AggregateServiceStatusForModelOpts(
	ctx context.Context,
	channels []*ent.Channel,
	modelID string,
	labels []string,
	failStaleHours int,
	skipOverlay bool,
) string {
	cap := upstreamprobe.PickPrimaryCapability(labels)
	if cap == "" {
		cap = guessCapability(modelID, labels)
	}
	if cap == "" {
		return ServiceStatusCatalogOnly
	}
	var hasOK, hasRecentFail, hasStaleFail, hasUnknown bool
	for _, ch := range channels {
		if !channelDeclaresModel(ch, modelID) {
			continue
		}
		var eff string
		if skipOverlay {
			_, eff = upstreamprobe.CapabilityFromProbeRowDBOnly(ch, modelID, cap, failStaleHours)
		} else {
			_, eff = upstreamprobe.CapabilityFromProbeRow(ctx, ch, modelID, cap, failStaleHours)
		}
		switch eff {
		case upstreamprobe.CapStatusOK:
			hasOK = true
		case upstreamprobe.CapStatusFail:
			hasRecentFail = true
		case "stale":
			hasStaleFail = true
		case upstreamprobe.CapStatusUnknown, "":
			hasUnknown = true
		}
	}
	if hasOK && hasRecentFail {
		return ServiceStatusDegraded
	}
	if hasOK {
		return ServiceStatusOperational
	}
	if hasRecentFail {
		return ServiceStatusUnavailable
	}
	if hasStaleFail {
		return ServiceStatusStale
	}
	if hasUnknown {
		return ServiceStatusCatalogOnly
	}
	return ServiceStatusCatalogOnly
}

func guessCapability(modelID string, labels []string) string {
	if c := upstreamprobe.PickPrimaryCapability(labels); c != "" {
		return c
	}
	lbs := modelkind.LabelsForModel(modelID, nil, nil)
	return upstreamprobe.PickPrimaryCapability(lbs)
}

func attachServiceStatusFromChannels(ctx context.Context, channels []*ent.Channel, byID map[string]*ConsoleModel, opts ListOpts) {
	for id, cm := range byID {
		if !cm.InGateway {
			continue
		}
		cm.ServiceStatus = AggregateServiceStatusForModelOpts(ctx, channels, id, cm.Labels, opts.FailStaleHours, opts.SkipCapabilityOverlay)
	}
}

func filterConsoleModels(items []ConsoleModel, opts ListOpts) []ConsoleModel {
	out := make([]ConsoleModel, 0, len(items))
	for _, it := range items {
		st := strings.TrimSpace(it.ServiceStatus)
		if st == "" {
			st = ServiceStatusCatalogOnly
		}
		if shouldOmitStatus(opts, st) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func filterItemsByServiceStatus(items []Item, statusByID map[string]string, opts ListOpts) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		st := statusByID[it.ID]
		if st == "" {
			st = ServiceStatusCatalogOnly
		}
		if shouldOmitStatus(opts, st) {
			continue
		}
		it.ServiceStatus = st
		out = append(out, it)
	}
	return out
}

func buildServiceStatusMap(ctx context.Context, channels []*ent.Channel, modelIDs []string, labelsByID map[string][]string, failStaleHours int) map[string]string {
	out := make(map[string]string, len(modelIDs))
	for _, id := range modelIDs {
		lbs := labelsByID[id]
		out[id] = AggregateServiceStatusForModel(ctx, channels, id, lbs, failStaleHours) // gateway /v1/models 仍合并覆盖层
	}
	return out
}
