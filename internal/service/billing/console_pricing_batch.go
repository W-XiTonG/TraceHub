package billing

import (
	"context"
	"strings"

	"tracehub/internal/ent"
)

// ConsolePricingBatch 控制台可用模型列表批量计价：一次请求预热全局/分组/渠道计价缓存，避免数百次 ModelBillRates 重复加锁与查库。
type ConsolePricingBatch struct {
	globalByModel map[string]*ent.GlobalModelPricing
	channelByID   map[int]map[string]*ent.ChannelModelPricing
	groupRates    map[string]*ent.GroupModelRate
	noBillSet     map[string]struct{}
	siteMarkup    float64
	groupMul      float64
	userMul       float64
}

// NewConsolePricingBatch 为控制台列表预热计价数据；channelIDs 为本次可能用到的渠道（分组下健康渠道即可）。
func (e *Engine) NewConsolePricingBatch(ctx context.Context, u *ent.User, channelIDs []int) *ConsolePricingBatch {
	b := &ConsolePricingBatch{
		globalByModel: e.cachedGlobalPricingByModel(ctx),
		channelByID:   make(map[int]map[string]*ent.ChannelModelPricing, len(channelIDs)),
		siteMarkup:    e.cachedDefaultPricingMarkupPercent(ctx),
	}
	if u != nil {
		b.groupMul, b.userMul = e.groupUserMultipliers(u)
		if u.Edges.Group != nil {
			b.groupRates = e.cachedGroupModelRatesByName(ctx, u.Edges.Group.ID)
		}
	}
	for _, cid := range channelIDs {
		if cid <= 0 {
			continue
		}
		if m := e.cachedChannelPricingByName(ctx, cid); len(m) > 0 {
			b.channelByID[cid] = m
		}
	}
	list := e.cachedNoBillModels(ctx)
	if len(list) > 0 {
		b.noBillSet = make(map[string]struct{}, len(list))
		for _, id := range list {
			id = strings.TrimSpace(id)
			if id != "" {
				b.noBillSet[id] = struct{}{}
			}
		}
	}
	return b
}

func (b *ConsolePricingBatch) lookupGlobal(model string) *ent.GlobalModelPricing {
	if b == nil || len(b.globalByModel) == 0 {
		return nil
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if row, ok := b.globalByModel[name]; ok {
			return row
		}
	}
	return nil
}

func (b *ConsolePricingBatch) lookupChannel(channelID int, model string) *ent.ChannelModelPricing {
	if b == nil || channelID <= 0 {
		return nil
	}
	byName := b.channelByID[channelID]
	if len(byName) == 0 {
		return nil
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if row, ok := byName[name]; ok {
			return row
		}
	}
	return nil
}

func (b *ConsolePricingBatch) lookupGroupRate(model string) *ent.GroupModelRate {
	if b == nil || len(b.groupRates) == 0 {
		return nil
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if row, ok := b.groupRates[name]; ok {
			return row
		}
	}
	return nil
}

func (b *ConsolePricingBatch) billable(model string) bool {
	if gmr := b.lookupGroupRate(model); gmr != nil {
		return gmr.Billable
	}
	if len(b.noBillSet) == 0 {
		return true
	}
	for _, name := range CandidateGroupModelRateNames(model) {
		if _, ok := b.noBillSet[name]; ok {
			return false
		}
	}
	return true
}

// Rates 计算单模型参考价因子（无额外 DB/锁，须先 NewConsolePricingBatch）。
func (b *ConsolePricingBatch) Rates(model string, channelID int) ModelBillRates {
	in1k, out1k := LookupPricing(model)
	glo := b.lookupGlobal(model)
	ch := b.lookupChannel(channelID, model)
	in1k, out1k = applyQuotaFromPricingRows(in1k, out1k, glo, ch)
	ir, or := 1.0, 1.0
	if gmr := b.lookupGroupRate(model); gmr != nil && gmr.Enabled {
		ir, or = gmr.InputRate, gmr.OutputRate
	} else if ch != nil && ch.Enabled {
		ir, or = ch.InputRate, ch.OutputRate
	} else if glo != nil && glo.Enabled {
		ir, or = glo.InputRate, glo.OutputRate
	}
	markup := ResolveMarkupPercent(b.siteMarkup, glo, ch)
	return ModelBillRates{
		groupMul: b.groupMul, userMul: b.userMul, inRate: ir, outRate: or,
		in1k: in1k, out1k: out1k, markupPercent: markup,
		Billable: b.billable(model),
	}
}
