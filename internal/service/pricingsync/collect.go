package pricingsync

import (
	"context"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channelmodelpricing"
	"tracehub/internal/ent/globalmodelpricing"
	"tracehub/internal/ent/groupmodelrate"
)

// CollectLocal 从 DB 收集定价快照。
func CollectLocal(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, opts FilterOpts) (*Payload, error) {
	out := &Payload{
		Version:     VersionTraceHubPricingV1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if scopeEnabled(opts.Scopes, ScopeGlobal) {
		rows, err := client.GlobalModelPricing.Query().All(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeGlobal, ModelName: r.ModelName}
			if !rowPasses(key, opts) {
				continue
			}
			out.Global = append(out.Global, globalFromEnt(r))
		}
	}
	if scopeEnabled(opts.Scopes, ScopeChannel) {
		q := client.ChannelModelPricing.Query()
		if len(opts.ChannelFilter) > 0 {
			q = q.Where(channelmodelpricing.ChannelIDIn(opts.ChannelFilter...))
		}
		rows, err := q.All(ctx)
		if err != nil {
			return nil, err
		}
		byChannel := map[int][]ChannelItem{}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeChannel, ModelName: r.ModelName, ChannelID: r.ChannelID}
			if !rowPasses(key, opts) {
				continue
			}
			byChannel[r.ChannelID] = append(byChannel[r.ChannelID], ChannelItem{GlobalItem: channelFromEnt(r)})
		}
		for cid, items := range byChannel {
			block := ChannelBlock{ChannelID: cid, Items: items}
			if ref := refForChannelID(t, cid); ref != "" {
				block.Ref = ref
				block.ChannelID = 0
			}
			out.Channels = append(out.Channels, block)
		}
	}
	if scopeEnabled(opts.Scopes, ScopeGroup) {
		q := client.GroupModelRate.Query()
		if len(opts.GroupFilter) > 0 {
			q = q.Where(groupmodelrate.GroupIDIn(opts.GroupFilter...))
		}
		rows, err := q.All(ctx)
		if err != nil {
			return nil, err
		}
		byGroup := map[int][]GroupItem{}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeGroup, ModelName: r.ModelName, GroupID: r.GroupID}
			if !rowPasses(key, opts) {
				continue
			}
			byGroup[r.GroupID] = append(byGroup[r.GroupID], GroupItem{
				ModelName:  r.ModelName,
				InputRate:  r.InputRate,
				OutputRate: r.OutputRate,
				Enabled:    r.Enabled,
				Billable:   r.Billable,
			})
		}
		for gid, items := range byGroup {
			block := GroupBlock{GroupID: gid, Items: items}
			if ref := refForGroupID(t, gid); ref != "" {
				block.Ref = ref
				block.GroupID = 0
			}
			out.Groups = append(out.Groups, block)
		}
	}
	return out, nil
}

func globalFromEnt(r *ent.GlobalModelPricing) GlobalItem {
	item := GlobalItem{
		ModelName:   r.ModelName,
		Enabled:     r.Enabled,
		InputRate:   r.InputRate,
		OutputRate:  r.OutputRate,
		BillingMode: r.BillingMode,
	}
	if r.QuotaInPer1k != nil {
		v := *r.QuotaInPer1k
		item.QuotaInPer1k = &v
	}
	if r.QuotaOutPer1k != nil {
		v := *r.QuotaOutPer1k
		item.QuotaOutPer1k = &v
	}
	if r.MarkupPercent != nil {
		v := *r.MarkupPercent
		item.MarkupPercent = &v
	}
	if r.QuotaPerRequest != nil {
		v := *r.QuotaPerRequest
		item.QuotaPerRequest = &v
	}
	return item
}

func channelFromEnt(r *ent.ChannelModelPricing) GlobalItem {
	item := GlobalItem{
		ModelName:  r.ModelName,
		Enabled:    r.Enabled,
		InputRate:  r.InputRate,
		OutputRate: r.OutputRate,
	}
	if r.QuotaInPer1k != nil {
		v := *r.QuotaInPer1k
		item.QuotaInPer1k = &v
	}
	if r.QuotaOutPer1k != nil {
		v := *r.QuotaOutPer1k
		item.QuotaOutPer1k = &v
	}
	if r.MarkupPercent != nil {
		v := *r.MarkupPercent
		item.MarkupPercent = &v
	}
	if r.BillingMode != nil && *r.BillingMode != "" {
		item.BillingMode = *r.BillingMode
	}
	if r.QuotaPerRequest != nil {
		v := *r.QuotaPerRequest
		item.QuotaPerRequest = &v
	}
	return item
}

func refForChannelID(t *ent.ModelPricingSyncTarget, id int) string {
	if t.ChannelRefMap == nil {
		return ""
	}
	for ref, cid := range t.ChannelRefMap {
		if cid == id {
			return ref
		}
	}
	return ""
}

func refForGroupID(t *ent.ModelPricingSyncTarget, id int) string {
	if t.GroupRefMap == nil {
		return ""
	}
	for ref, gid := range t.GroupRefMap {
		if gid == id {
			return ref
		}
	}
	return ""
}

func countPayloadRows(p *Payload) int {
	if p == nil {
		return 0
	}
	n := len(p.Global)
	for _, c := range p.Channels {
		n += len(c.Items)
	}
	for _, g := range p.Groups {
		n += len(g.Items)
	}
	return n
}

func lookupGlobal(ctx context.Context, client *ent.Client, name string) (*ent.GlobalModelPricing, bool, error) {
	r, err := client.GlobalModelPricing.Query().Where(globalmodelpricing.ModelNameEQ(name)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

func lookupChannel(ctx context.Context, client *ent.Client, channelID int, name string) (*ent.ChannelModelPricing, bool, error) {
	r, err := client.ChannelModelPricing.Query().
		Where(channelmodelpricing.ChannelIDEQ(channelID), channelmodelpricing.ModelNameEQ(name)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}

func lookupGroupRate(ctx context.Context, client *ent.Client, groupID int, name string) (*ent.GroupModelRate, bool, error) {
	r, err := client.GroupModelRate.Query().
		Where(groupmodelrate.GroupIDEQ(groupID), groupmodelrate.ModelNameEQ(name)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return r, true, nil
}
