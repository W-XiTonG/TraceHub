package router

import (
	"context"
	"sort"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
)

// OverlapChannel 分组内声明同一逻辑模型的渠道摘要。
type OverlapChannel struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
}

// ModelOverlapItem 在多个渠道重复出现的逻辑模型。
type ModelOverlapItem struct {
	Model    string           `json:"model"`
	Channels []OverlapChannel `json:"channels"`
	Pinned   []int64          `json:"pinned,omitempty"`
}

func collectLogicalModelKeys(ch *ent.Channel) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, m := range ch.Models {
		id := strings.TrimSpace(m)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			keys = append(keys, id)
		}
	}
	if ch.ModelAlias != nil {
		for alias := range ch.ModelAlias {
			a := strings.TrimSpace(alias)
			if a == "" {
				continue
			}
			if _, ok := seen[a]; !ok {
				seen[a] = struct{}{}
				keys = append(keys, a)
			}
		}
	}
	return keys
}

// GroupModelChannelOverlap 扫描分组内 enabled+schedulable 渠道，返回在 2+ 渠道重复的逻辑模型。
func GroupModelChannelOverlap(ctx context.Context, client *ent.Client, groupID int, modelRouting map[string][]int64) ([]ModelOverlapItem, error) {
	if groupID <= 0 {
		return nil, nil
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HasGroupWith(group.ID(groupID)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	type chRef struct {
		ch OverlapChannel
	}
	modelToChs := make(map[string]map[int]chRef)
	for _, ch := range channels {
		ref := OverlapChannel{
			ID:      ch.ID,
			Name:    strings.TrimSpace(ch.ChannelName),
			Healthy: ch.HealthStatus == 1,
		}
		if ref.Name == "" {
			ref.Name = fmtChannelLabel(ch)
		}
		for _, model := range collectLogicalModelKeys(ch) {
			if modelToChs[model] == nil {
				modelToChs[model] = make(map[int]chRef)
			}
			modelToChs[model][ch.ID] = chRef{ch: ref}
		}
	}
	var out []ModelOverlapItem
	for model, chMap := range modelToChs {
		if len(chMap) < 2 {
			continue
		}
		item := ModelOverlapItem{Model: model}
		for _, ref := range chMap {
			item.Channels = append(item.Channels, ref.ch)
		}
		sort.Slice(item.Channels, func(i, j int) bool {
			if item.Channels[i].ID != item.Channels[j].ID {
				return item.Channels[i].ID < item.Channels[j].ID
			}
			return item.Channels[i].Name < item.Channels[j].Name
		})
		if modelRouting != nil {
			if pinned, ok := modelRouting[model]; ok && len(pinned) > 0 {
				item.Pinned = append([]int64(nil), pinned...)
			}
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out, nil
}

func fmtChannelLabel(ch *ent.Channel) string {
	if n := strings.TrimSpace(ch.ChannelName); n != "" {
		return n
	}
	return strings.TrimSpace(ch.ChannelType)
}
