package pricingsync

import (
	"context"
	"fmt"

	"tracehub/internal/ent"
)

func diffPullPayload(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, p *Payload, opts FilterOpts) ([]DiffRow, []string, error) {
	var rows []DiffRow
	var warnings []string
	if p == nil || p.Version != VersionTraceHubPricingV1 {
		return nil, warnings, errVersionMismatch
	}
	for _, item := range p.Global {
		key := SelectedKey{Layer: ScopeGlobal, ModelName: item.ModelName}
		if !rowPasses(key, opts) {
			continue
		}
		row, err := pullRowFromGlobal(ctx, client, t, item)
		if err != nil {
			return nil, warnings, err
		}
		rows = append(rows, row)
	}
	for _, block := range p.Channels {
		cid, err := resolveChannelID(t, block)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid}
			if !rowPasses(key, opts) {
				continue
			}
			row, err := pullRowFromChannel(ctx, client, t, cid, item.GlobalItem)
			if err != nil {
				return nil, warnings, err
			}
			rows = append(rows, row)
		}
	}
	for _, block := range p.Groups {
		gid, err := resolveGroupID(t, block)
		if err != nil {
			warnings = append(warnings, err.Error())
			continue
		}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid}
			if !rowPasses(key, opts) {
				continue
			}
			row, err := pullRowFromGroup(ctx, client, t, gid, item)
			if err != nil {
				return nil, warnings, err
			}
			rows = append(rows, row)
		}
	}
	return rows, warnings, nil
}

func diffPushPayload(p *Payload, opts FilterOpts) []DiffRow {
	var rows []DiffRow
	if p == nil {
		return rows
	}
	for _, item := range p.Global {
		key := SelectedKey{Layer: ScopeGlobal, ModelName: item.ModelName}
		if !rowPasses(key, opts) {
			continue
		}
		rows = append(rows, DiffRow{Layer: ScopeGlobal, ModelName: item.ModelName, Change: ChangeUpdated, Selected: true, Detail: "将推送至外部"})
	}
	for _, block := range p.Channels {
		cid := block.ChannelID
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid}
			if !rowPasses(key, opts) {
				continue
			}
			rows = append(rows, DiffRow{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid, Change: ChangeUpdated, Selected: true, Detail: "将推送至外部"})
		}
	}
	for _, block := range p.Groups {
		gid := block.GroupID
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid}
			if !rowPasses(key, opts) {
				continue
			}
			rows = append(rows, DiffRow{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid, Change: ChangeUpdated, Selected: true, Detail: "将推送至外部"})
		}
	}
	return rows
}

func summarizeRows(rows []DiffRow) (added, updated, skipped, removed int) {
	for _, r := range rows {
		if !r.Selected {
			skipped++
			continue
		}
		switch r.Change {
		case ChangeAdded:
			added++
		case ChangeUpdated:
			updated++
		case ChangeSkipped:
			skipped++
		case ChangeRemoved:
			removed++
		}
	}
	return
}

func applyRowSelection(rows []DiffRow, selectedKeys []string) []DiffRow {
	if len(selectedKeys) == 0 {
		return rows
	}
	set := make(map[string]struct{}, len(selectedKeys))
	for _, k := range selectedKeys {
		set[k] = struct{}{}
	}
	out := make([]DiffRow, len(rows))
	for i, r := range rows {
		key := SelectedKey{Layer: r.Layer, ModelName: r.ModelName, ChannelID: r.ChannelID, GroupID: r.GroupID}
		_, ok := set[key.String()]
		out[i] = r
		out[i].Selected = ok
	}
	return out
}

func filterPayloadBySelectedKeys(p *Payload, keys map[string]struct{}) *Payload {
	if p == nil || len(keys) == 0 {
		return p
	}
	out := &Payload{Version: p.Version, GeneratedAt: p.GeneratedAt}
	for _, item := range p.Global {
		key := SelectedKey{Layer: ScopeGlobal, ModelName: item.ModelName}
		if _, ok := keys[key.String()]; ok {
			out.Global = append(out.Global, item)
		}
	}
	for _, block := range p.Channels {
		cid := block.ChannelID
		if cid == 0 && block.Ref != "" {
			continue
		}
		nb := ChannelBlock{Ref: block.Ref, ChannelID: block.ChannelID}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeChannel, ModelName: item.ModelName, ChannelID: cid}
			if _, ok := keys[key.String()]; ok {
				nb.Items = append(nb.Items, item)
			}
		}
		if len(nb.Items) > 0 {
			out.Channels = append(out.Channels, nb)
		}
	}
	for _, block := range p.Groups {
		gid := block.GroupID
		nb := GroupBlock{Ref: block.Ref, GroupID: block.GroupID}
		for _, item := range block.Items {
			key := SelectedKey{Layer: ScopeGroup, ModelName: item.ModelName, GroupID: gid}
			if _, ok := keys[key.String()]; ok {
				nb.Items = append(nb.Items, item)
			}
		}
		if len(nb.Items) > 0 {
			out.Groups = append(out.Groups, nb)
		}
	}
	return out
}

func selectedKeysFromRows(rows []DiffRow) []string {
	var keys []string
	for _, r := range rows {
		if !r.Selected {
			continue
		}
		key := SelectedKey{Layer: r.Layer, ModelName: r.ModelName, ChannelID: r.ChannelID, GroupID: r.GroupID}
		keys = append(keys, key.String())
	}
	return keys
}

func selectedKeySet(rows []DiffRow) map[string]struct{} {
	m := map[string]struct{}{}
	for _, r := range rows {
		if !r.Selected {
			continue
		}
		key := SelectedKey{Layer: r.Layer, ModelName: r.ModelName, ChannelID: r.ChannelID, GroupID: r.GroupID}
		m[key.String()] = struct{}{}
	}
	return m
}

func buildPreviewResult(id, direction string, expiresAt string, rows []DiffRow, warnings []string) PreviewResult {
	added, updated, skipped, removed := summarizeRows(rows)
	sample := rows
	if len(sample) > 20 {
		sample = sample[:20]
	}
	return PreviewResult{
		PreviewID:  id,
		ExpiresAt:  expiresAt,
		Direction:  direction,
		Added:      added,
		Updated:    updated,
		Skipped:    skipped,
		Removed:    removed,
		Warnings:   warnings,
		Rows:       rows,
		SampleDiff: sample,
	}
}

func formatSyncMessage(res SyncResult) string {
	return fmt.Sprintf("同步完成: 新增 %d, 更新 %d, 跳过 %d, 删除 %d", res.Added, res.Updated, res.Skipped, res.Removed)
}
