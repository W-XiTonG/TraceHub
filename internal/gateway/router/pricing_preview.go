package router

import (
	"context"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/pkg/sitecfg"
)

// PricingPreview 控制台参考价：按分组生效选路策略确定性挑选渠道（请求内复用渠道列表与路由配置）。
type PricingPreview struct {
	cfg      RoutingConfig
	channels []*ent.Channel
}

// NewPricingPreview 加载分组下健康可调度渠道与合并后的路由配置（一次请求调用一次即可）。
func NewPricingPreview(ctx context.Context, client *ent.Client, groupID int) (*PricingPreview, error) {
	if groupID <= 0 {
		return &PricingPreview{}, nil
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HealthStatusEQ(1),
			channel.HasGroupWith(group.ID(groupID)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return newPricingPreview(ctx, client, groupID, channels)
}

// NewPricingPreviewWithChannels 复用请求内已加载的分组渠道（须为 enabled+schedulable；内部再筛 health_status==1，与 NewPricingPreview 查库语义一致）。
func NewPricingPreviewWithChannels(ctx context.Context, client *ent.Client, groupID int, groupChannels []*ent.Channel) (*PricingPreview, error) {
	if groupID <= 0 {
		return &PricingPreview{}, nil
	}
	return newPricingPreview(ctx, client, groupID, schedulableHealthyChannels(groupChannels))
}

func newPricingPreview(ctx context.Context, client *ent.Client, groupID int, channels []*ent.Channel) (*PricingPreview, error) {
	cfg, err := ResolveRoutingConfig(ctx, client, groupID)
	if err != nil {
		return nil, err
	}
	return &PricingPreview{cfg: cfg, channels: channels}, nil
}

// schedulableHealthyChannels 与 NewPricingPreview 的 channel 查询条件一致（enabled ∧ schedulable ∧ health_status==1）。
func schedulableHealthyChannels(channels []*ent.Channel) []*ent.Channel {
	if len(channels) == 0 {
		return nil
	}
	out := make([]*ent.Channel, 0, len(channels))
	for _, ch := range channels {
		if ch == nil || !ch.Enabled || !ch.Schedulable || ch.HealthStatus != 1 {
			continue
		}
		out = append(out, ch)
	}
	return out
}

// EffectiveStrategy 返回合并后的选路策略名（空则视为 weighted_random）。
func (p *PricingPreview) EffectiveStrategy() string {
	if p == nil {
		return sitecfg.RoutingStrategyWeightedRandom
	}
	return StrategyName(p.cfg)
}

// ChannelIDs 返回预览用到的全部渠道 ID（用于批量预热渠道计价缓存）。
func (p *PricingPreview) ChannelIDs() []int {
	if p == nil || len(p.channels) == 0 {
		return nil
	}
	ids := make([]int, 0, len(p.channels))
	for _, ch := range p.channels {
		ids = append(ids, ch.ID)
	}
	return ids
}

// ChannelIDForModel 按与网关相同的候选过滤 + 确定性策略返回用于 ModelBillRates 的 channel_id；无候选返回 0。
func (p *PricingPreview) ChannelIDForModel(model string) int {
	if p == nil || len(p.channels) == 0 {
		return 0
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return 0
	}
	candidates := candidatesForModel(p.channels, model, nil)
	if len(candidates) == 0 {
		return 0
	}
	filtered, err := filterCandidatesByModelRouting(candidates, p.cfg.ModelRouting, model)
	if err != nil || len(filtered) == 0 {
		return 0
	}
	return PickPricingDisplayChannel(p.cfg, filtered, model)
}

func candidatesForModel(channels []*ent.Channel, requestedModel string, exclude map[int]struct{}) []candidate {
	var out []candidate
	for _, ch := range channels {
		if exclude != nil {
			if _, skip := exclude[ch.ID]; skip {
				continue
			}
		}
		if up, ok := modelMatch(ch, requestedModel); ok {
			out = append(out, candidate{ch: ch, upstream: up})
		}
	}
	return out
}

// PickPricingDisplayChannel 在已过滤的候选集上按生效策略确定性选渠（供展示参考价，非真实随机选路）。
func PickPricingDisplayChannel(cfg RoutingConfig, candidates []candidate, requestedModel string) int {
	if len(candidates) == 0 {
		return 0
	}
	var chosen candidate
	switch cfg.Strategy {
	case sitecfg.RoutingStrategyPriority:
		chosen = pickByPriorityStable(candidates)
	case sitecfg.RoutingStrategyFixedOrder:
		chosen = pickByFixedOrderStable(candidates, cfg.ModelRouting, requestedModel)
	default:
		chosen = pickByMaxWeightStable(candidates)
	}
	return chosen.ch.ID
}

func pickByPriorityStable(candidates []candidate) candidate {
	maxP := candidates[0].ch.Priority
	for _, c := range candidates[1:] {
		if c.ch.Priority > maxP {
			maxP = c.ch.Priority
		}
	}
	var best *candidate
	for i := range candidates {
		c := &candidates[i]
		if c.ch.Priority != maxP {
			continue
		}
		if best == nil || c.ch.ID < best.ch.ID {
			best = c
		}
	}
	return *best
}

func pickByMaxWeightStable(candidates []candidate) candidate {
	pool := poolMinErrorCount(candidates)
	best := pool[0]
	bestW := channelWeight(best.ch)
	for _, c := range pool[1:] {
		w := channelWeight(c.ch)
		if w > bestW || (w == bestW && c.ch.ID < best.ch.ID) {
			best = c
			bestW = w
		}
	}
	return best
}

func channelWeight(ch *ent.Channel) int {
	w := ch.Weight
	if w <= 0 {
		return 1
	}
	return w
}

func pickByFixedOrderStable(candidates []candidate, modelRouting map[string][]int64, requestedModel string) candidate {
	if modelRouting != nil {
		if order, ok := modelRouting[strings.TrimSpace(requestedModel)]; ok && len(order) > 0 {
			byID := make(map[int]candidate, len(candidates))
			for _, c := range candidates {
				byID[c.ch.ID] = c
			}
			for _, id := range order {
				if c, ok := byID[int(id)]; ok {
					return c
				}
			}
		}
	}
	return pickByPriorityStable(candidates)
}
