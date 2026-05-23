package pricingsync

import (
	"context"
	"fmt"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channelmodelpricing"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/modelpricingsynctarget"
	"tracehub/internal/service/billing"
)

type applyPlan struct {
	Global  []billing.GlobalPricingUpsertItem
	Channel []billing.ChannelPricingUpsertItem
	Group   map[int][]billing.GroupModelRateUpsertItem
	Removed int
}

func resolveChannelID(t *ent.ModelPricingSyncTarget, block ChannelBlock) (int, error) {
	if block.ChannelID > 0 {
		return block.ChannelID, nil
	}
	ref := strings.TrimSpace(block.Ref)
	if ref == "" {
		return 0, fmt.Errorf("channel 缺少 channel_id 或 ref")
	}
	if t.ChannelRefMap == nil {
		return 0, fmt.Errorf("未知 channel ref: %s", ref)
	}
	id, ok := t.ChannelRefMap[ref]
	if !ok || id <= 0 {
		return 0, fmt.Errorf("未知 channel ref: %s", ref)
	}
	return id, nil
}

func resolveGroupID(t *ent.ModelPricingSyncTarget, block GroupBlock) (int, error) {
	if block.GroupID > 0 {
		return block.GroupID, nil
	}
	ref := strings.TrimSpace(block.Ref)
	if ref == "" {
		return 0, fmt.Errorf("group 缺少 group_id 或 ref")
	}
	if t.GroupRefMap == nil {
		return 0, fmt.Errorf("未知 group ref: %s", ref)
	}
	id, ok := t.GroupRefMap[ref]
	if !ok || id <= 0 {
		return 0, fmt.Errorf("未知 group ref: %s", ref)
	}
	return id, nil
}

func buildApplyPlan(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, p *Payload, opts FilterOpts) (applyPlan, []string, error) {
	var warnings []string
	plan := applyPlan{Group: map[int][]billing.GroupModelRateUpsertItem{}}
	if p == nil || p.Version != VersionTraceHubPricingV1 {
		return plan, warnings, errVersionMismatch
	}
	skipExisting := t.MergeStrategy == modelpricingsynctarget.MergeStrategySkipExisting

	for _, item := range p.Global {
		key := SelectedKey{Layer: ScopeGlobal, ModelName: item.ModelName}
		if !rowPasses(key, opts) {
			continue
		}
		if skipExisting {
			if _, ok, err := lookupGlobal(ctx, client, item.ModelName); err != nil {
				return plan, warnings, err
			} else if ok {
				continue
			}
		}
		plan.Global = append(plan.Global, billing.GlobalPricingUpsertItem{
			ModelName:       item.ModelName,
			Enabled:         item.Enabled,
			QuotaInPer1k:    item.QuotaInPer1k,
			QuotaOutPer1k:   item.QuotaOutPer1k,
			InputRate:       defaultRate(item.InputRate),
			OutputRate:      defaultRate(item.OutputRate),
			MarkupPercent:   item.MarkupPercent,
			BillingMode:     item.BillingMode,
			QuotaPerRequest: item.QuotaPerRequest,
		})
	}

	for _, block := range p.Channels {
		cid, err := resolveChannelID(t, block)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if _, err := client.Channel.Get(ctx, cid); err != nil {
			if ent.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("渠道 %d 不存在", cid))
				continue
			}
			return plan, warnings, err
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid}
			if !rowPasses(key, opts) {
				continue
			}
			if skipExisting {
				if _, ok, err := lookupChannel(ctx, client, cid, item.ModelName); err != nil {
					return plan, warnings, err
				} else if ok {
					continue
				}
			}
			bm := item.BillingMode
			var bmPtr *string
			if strings.TrimSpace(bm) != "" {
				bmPtr = &bm
			}
			plan.Channel = append(plan.Channel, billing.ChannelPricingUpsertItem{
				ChannelID:       cid,
				ModelName:       item.ModelName,
				Enabled:         item.Enabled,
				QuotaInPer1k:    item.QuotaInPer1k,
				QuotaOutPer1k:   item.QuotaOutPer1k,
				InputRate:       defaultRate(item.InputRate),
				OutputRate:      defaultRate(item.OutputRate),
				MarkupPercent:   item.MarkupPercent,
				BillingMode:     bmPtr,
				QuotaPerRequest: item.QuotaPerRequest,
			})
		}
	}

	for _, block := range p.Groups {
		gid, err := resolveGroupID(t, block)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		if _, err := client.Group.Get(ctx, gid); err != nil {
			if ent.IsNotFound(err) {
				warnings = append(warnings, fmt.Sprintf("分组 %d 不存在", gid))
				continue
			}
			return plan, warnings, err
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid}
			if !rowPasses(key, opts) {
				continue
			}
			if skipExisting {
				if _, ok, err := lookupGroupRate(ctx, client, gid, item.ModelName); err != nil {
					return plan, warnings, err
				} else if ok {
					continue
				}
			}
			plan.Group[gid] = append(plan.Group[gid], billing.GroupModelRateUpsertItem{
				ModelName:  item.ModelName,
				InputRate:  defaultRate(item.InputRate),
				OutputRate: defaultRate(item.OutputRate),
				Enabled:    item.Enabled,
				Billable:   item.Billable,
			})
		}
	}

	if t.MergeStrategy == modelpricingsynctarget.MergeStrategyReplaceScoped {
		removed, err := planReplaceScoped(ctx, client, t, p, opts)
		if err != nil {
			return plan, warnings, err
		}
		plan.Removed = removed
	}
	return plan, warnings, nil
}

func planReplaceScoped(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, p *Payload, opts FilterOpts) (int, error) {
	// replace_scoped: delete scoped rows not present in payload keys
	keep := map[string]struct{}{}
	for _, item := range p.Global {
		key := SelectedKey{Layer: ScopeGlobal, ModelName: item.ModelName}
		if rowPasses(key, opts) {
			keep[key.String()] = struct{}{}
		}
	}
	for _, block := range p.Channels {
		cid, err := resolveChannelID(t, block)
		if err != nil {
			continue
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid}
			if rowPasses(key, opts) {
				keep[key.String()] = struct{}{}
			}
		}
	}
	for _, block := range p.Groups {
		gid, err := resolveGroupID(t, block)
		if err != nil {
			continue
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid}
			if rowPasses(key, opts) {
				keep[key.String()] = struct{}{}
			}
		}
	}
	removed := 0
	if scopeEnabled(opts.Scopes, ScopeGlobal) {
		rows, err := client.GlobalModelPricing.Query().All(ctx)
		if err != nil {
			return 0, err
		}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeGlobal, ModelName: r.ModelName}
			if !rowPasses(key, opts) {
				continue
			}
			if _, ok := keep[key.String()]; !ok {
				if err := client.GlobalModelPricing.DeleteOneID(r.ID).Exec(ctx); err != nil {
					return removed, err
				}
				removed++
			}
		}
	}
	if scopeEnabled(opts.Scopes, ScopeChannel) {
		q := client.ChannelModelPricing.Query()
		if len(opts.ChannelFilter) > 0 {
			q = q.Where(channelmodelpricing.ChannelIDIn(opts.ChannelFilter...))
		}
		rows, err := q.All(ctx)
		if err != nil {
			return removed, err
		}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeChannel, ModelName: r.ModelName, ChannelID: r.ChannelID}
			if !rowPasses(key, opts) {
				continue
			}
			if _, ok := keep[key.String()]; !ok {
				if err := client.ChannelModelPricing.DeleteOneID(r.ID).Exec(ctx); err != nil {
					return removed, err
				}
				removed++
			}
		}
	}
	if scopeEnabled(opts.Scopes, ScopeGroup) {
		q := client.GroupModelRate.Query()
		if len(opts.GroupFilter) > 0 {
			q = q.Where(groupmodelrate.GroupIDIn(opts.GroupFilter...))
		}
		rows, err := q.All(ctx)
		if err != nil {
			return removed, err
		}
		for _, r := range rows {
			key := SelectedKey{Layer: ScopeGroup, ModelName: r.ModelName, GroupID: r.GroupID}
			if !rowPasses(key, opts) {
				continue
			}
			if _, ok := keep[key.String()]; !ok {
				if err := client.GroupModelRate.DeleteOneID(r.ID).Exec(ctx); err != nil {
					return removed, err
				}
				removed++
			}
		}
	}
	return removed, nil
}

func executeApplyPlan(ctx context.Context, client *ent.Client, plan applyPlan) (added, updated int, err error) {
	for _, item := range plan.Global {
		_, existed, err := lookupGlobal(ctx, client, item.ModelName)
		if err != nil {
			return added, updated, err
		}
		if br, err := billing.UpsertGlobalModelPricingBatch(ctx, client, []billing.GlobalPricingUpsertItem{item}); err != nil {
			return added, updated, err
		} else if br != "" {
			return added, updated, fmt.Errorf("%s", br)
		}
		if existed {
			updated++
		} else {
			added++
		}
	}
	for _, item := range plan.Channel {
		_, existed, err := lookupChannel(ctx, client, item.ChannelID, item.ModelName)
		if err != nil {
			return added, updated, err
		}
		if br, err := billing.UpsertChannelModelPricingBatch(ctx, client, []billing.ChannelPricingUpsertItem{item}); err != nil {
			return added, updated, err
		} else if br != "" {
			return added, updated, fmt.Errorf("%s", br)
		}
		if existed {
			updated++
		} else {
			added++
		}
	}
	for gid, items := range plan.Group {
		for _, item := range items {
			_, existed, err := lookupGroupRate(ctx, client, gid, item.ModelName)
			if err != nil {
				return added, updated, err
			}
			if err := billing.UpsertGroupModelRatesBatch(ctx, client, gid, []billing.GroupModelRateUpsertItem{item}); err != nil {
				return added, updated, err
			}
			if existed {
				updated++
			} else {
				added++
			}
		}
	}
	return added, updated, nil
}

func invalidateCachesAfterApply(plan applyPlan) {
	if len(plan.Global) > 0 || plan.Removed > 0 {
		billing.InvalidateGlobalModelPricingCacheAdmin()
	}
	channelIDs := map[int]struct{}{}
	for _, it := range plan.Channel {
		channelIDs[it.ChannelID] = struct{}{}
	}
	for cid := range channelIDs {
		billing.InvalidateChannelModelPricingCacheAdmin(cid)
	}
	if len(channelIDs) > 0 || (plan.Removed > 0 && len(plan.Channel) == 0) {
		billing.InvalidateChannelModelPricingCacheAdmin(0)
	}
	for gid := range plan.Group {
		billing.InvalidateGroupModelRateCacheAdmin(gid)
	}
}

func defaultRate(v float64) float64 {
	if v <= 0 {
		return 1
	}
	return v
}

// ApplyInboundPayload 公开入站 Push：按 upsert 合并 global/channel/group。
func ApplyInboundPayload(ctx context.Context, client *ent.Client, p *Payload) (added, updated int, warnings []string, err error) {
	t := &ent.ModelPricingSyncTarget{
		Scopes:        []string{ScopeGlobal, ScopeChannel, ScopeGroup},
		MergeStrategy: modelpricingsynctarget.MergeStrategyUpsert,
	}
	opts := FilterOpts{Scopes: append([]string(nil), t.Scopes...)}
	plan, warnings, err := buildApplyPlan(ctx, client, t, p, opts)
	if err != nil {
		return 0, 0, warnings, err
	}
	added, updated, err = executeApplyPlan(ctx, client, plan)
	if err != nil {
		return 0, 0, warnings, err
	}
	invalidateCachesAfterApply(plan)
	return added, updated, warnings, nil
}
