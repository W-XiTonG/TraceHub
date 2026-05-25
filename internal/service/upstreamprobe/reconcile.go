package upstreamprobe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

const (
	// ProbeUpstreamSnapshotKey 上次成功拉取的上游模型 ID 列表快照。
	ProbeUpstreamSnapshotKey = "_upstream_snapshot"
	// ProbeReconcileMetaKey 联动去重与通知指纹元数据。
	ProbeReconcileMetaKey = "_reconcile_meta"
)

// reconcileDiff 一次探测相对上次快照的上游侧与本地同步候选。
type reconcileDiff struct {
	AddedUpstream   []string
	RemovedUpstream []string
	ToAddModels     []string
	ToRemoveLocal   []string
}

func maybeReconcileAfterProbe(
	ctx context.Context,
	client *ent.Client,
	ch *ent.Channel,
	upstream map[string]struct{},
	probeOut map[string]interface{},
	oldProbe map[string]interface{},
	now string,
) error {
	cfg := sitecfg.EffectiveUpstreamModelProbeReconcile(ctx, client)
	if !cfg.AnyAction() {
		return nil
	}

	currentIDs := upstreamIDSlice(upstream)
	diff := computeReconcileDiff(ch, upstream, oldProbe, currentIDs, cfg.Scope)

	synced := false
	models := append([]string(nil), ch.Models...)
	alias := cloneAliasMap(ch.ModelAlias)

	if cfg.Enabled && cfg.SyncOnIncrease && len(diff.ToAddModels) > 0 {
		models = appendModelsDedup(models, diff.ToAddModels)
		synced = true
	}
	if cfg.Enabled && cfg.SyncOnDecrease && len(diff.ToRemoveLocal) > 0 {
		models, alias = removeLocalModels(models, alias, diff.ToRemoveLocal, cfg.Scope)
		synced = true
	}

	mergedProbe := mergeProbeWithReconcileMeta(probeOut, oldProbe, currentIDs, now, diff)
	localKeys := collectLocalModelKeys(models, alias)
	for _, k := range localKeys {
		inList := modelInUpstreamList(k, upstream, alias)
		mergedProbe[k] = map[string]interface{}{
			"in_upstream_list": inList,
			"checked_at":       now,
		}
	}

	up := client.Channel.UpdateOneID(ch.ID).SetModelUpstreamProbe(mergedProbe)
	if synced {
		up = up.SetModels(models)
		if alias != nil {
			up = up.SetModelAlias(alias)
		} else if ch.ModelAlias != nil {
			up = up.ClearModelAlias()
		}
	}
	if err := up.Exec(ctx); err != nil {
		return err
	}

	notifyAdded := cfg.NotifyOnIncrease && len(diff.AddedUpstream) > 0
	notifyRemoved := cfg.NotifyOnDecrease && len(diff.RemovedUpstream) > 0
	if notifyAdded || notifyRemoved {
		maybeNotifyReconcile(ctx, client, ch, diff, cfg, synced, notifyAdded, notifyRemoved, mergedProbe)
	}
	return nil
}

func upstreamIDSlice(upstream map[string]struct{}) []string {
	ids := make([]string, 0, len(upstream))
	for id := range upstream {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func previousUpstreamSnapshot(oldProbe map[string]interface{}) []string {
	if oldProbe == nil {
		return nil
	}
	raw, ok := oldProbe[ProbeUpstreamSnapshotKey]
	if !ok {
		return nil
	}
	row, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	idsRaw, ok := row["ids"]
	if !ok {
		return nil
	}
	switch arr := idsRaw.(type) {
	case []interface{}:
		var out []string
		for _, x := range arr {
			if s, ok := x.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		sort.Strings(out)
		return out
	case []string:
		out := make([]string, 0, len(arr))
		for _, s := range arr {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	default:
		return nil
	}
}

func diffIDSets(prev, current []string) (added, removed []string) {
	prevSet := make(map[string]struct{}, len(prev))
	for _, id := range prev {
		prevSet[id] = struct{}{}
	}
	curSet := make(map[string]struct{}, len(current))
	for _, id := range current {
		curSet[id] = struct{}{}
	}
	for id := range curSet {
		if _, ok := prevSet[id]; !ok {
			added = append(added, id)
		}
	}
	for id := range prevSet {
		if _, ok := curSet[id]; !ok {
			removed = append(removed, id)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

func localManagedSet(ch *ent.Channel, scope string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, m := range ch.Models {
		if id := strings.TrimSpace(m); id != "" {
			set[id] = struct{}{}
		}
	}
	if scope == sitecfg.UpstreamModelProbeReconcileScopeModelsAndAlias && ch.ModelAlias != nil {
		for k := range ch.ModelAlias {
			if id := strings.TrimSpace(k); id != "" {
				set[id] = struct{}{}
			}
		}
	}
	return set
}

func computeReconcileDiff(ch *ent.Channel, upstream map[string]struct{}, oldProbe map[string]interface{}, currentIDs []string, scope string) reconcileDiff {
	prev := previousUpstreamSnapshot(oldProbe)
	addedUpstream, removedUpstream := diffIDSets(prev, currentIDs)
	if len(prev) == 0 {
		// 首次建立快照，不将全量上游列表视为「新增/移除」以免刷屏通知。
		addedUpstream = nil
		removedUpstream = nil
	}

	localSet := localManagedSet(ch, scope)
	var toAdd []string
	for id := range upstream {
		if _, ok := localSet[id]; !ok {
			toAdd = append(toAdd, id)
		}
	}
	sort.Strings(toAdd)

	alias := ch.ModelAlias
	if alias == nil {
		alias = map[string]string{}
	}
	var toRemove []string
	if scope == sitecfg.UpstreamModelProbeReconcileScopeModelsOnly {
		for _, m := range ch.Models {
			id := strings.TrimSpace(m)
			if id == "" {
				continue
			}
			if !modelInUpstreamList(id, upstream, alias) {
				toRemove = append(toRemove, id)
			}
		}
	} else {
		for id := range localSet {
			if !modelInUpstreamList(id, upstream, alias) {
				toRemove = append(toRemove, id)
			}
		}
	}
	sort.Strings(toRemove)

	return reconcileDiff{
		AddedUpstream:   addedUpstream,
		RemovedUpstream: removedUpstream,
		ToAddModels:     toAdd,
		ToRemoveLocal:   toRemove,
	}
}

func appendModelsDedup(models, add []string) []string {
	seen := make(map[string]struct{}, len(models))
	for _, m := range models {
		if id := strings.TrimSpace(m); id != "" {
			seen[id] = struct{}{}
		}
	}
	out := append([]string(nil), models...)
	for _, id := range add {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func removeLocalModels(models []string, alias map[string]string, remove []string, scope string) ([]string, map[string]string) {
	rm := make(map[string]struct{}, len(remove))
	for _, id := range remove {
		rm[id] = struct{}{}
	}
	var outModels []string
	for _, m := range models {
		id := strings.TrimSpace(m)
		if id == "" {
			continue
		}
		if _, drop := rm[id]; drop {
			continue
		}
		outModels = append(outModels, id)
	}
	outAlias := cloneAliasMap(alias)
	if scope == sitecfg.UpstreamModelProbeReconcileScopeModelsAndAlias && outAlias != nil {
		for k := range outAlias {
			if _, drop := rm[strings.TrimSpace(k)]; drop {
				delete(outAlias, k)
			}
		}
		if len(outAlias) == 0 {
			outAlias = nil
		}
	}
	return outModels, outAlias
}

func cloneAliasMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeProbeWithReconcileMeta(
	probeOut map[string]interface{},
	oldProbe map[string]interface{},
	currentIDs []string,
	now string,
	diff reconcileDiff,
) map[string]interface{} {
	merged := make(map[string]interface{}, len(probeOut)+2)
	for k, v := range probeOut {
		merged[k] = v
	}
	idsCopy := append([]string(nil), currentIDs...)
	merged[ProbeUpstreamSnapshotKey] = map[string]interface{}{
		"ids":        idsCopy,
		"checked_at": now,
	}
	meta := reconcileMetaFromOld(oldProbe)
	merged[ProbeReconcileMetaKey] = meta
	return merged
}

func reconcileMetaFromOld(oldProbe map[string]interface{}) map[string]interface{} {
	meta := map[string]interface{}{}
	if oldProbe == nil {
		return meta
	}
	raw, ok := oldProbe[ProbeReconcileMetaKey]
	if !ok {
		return meta
	}
	row, ok := raw.(map[string]interface{})
	if !ok {
		return meta
	}
	for k, v := range row {
		meta[k] = v
	}
	return meta
}

func idsFingerprint(ids []string) string {
	cp := append([]string(nil), ids...)
	sort.Strings(cp)
	h := sha256.Sum256([]byte(strings.Join(cp, "\n")))
	return hex.EncodeToString(h[:8])
}

func shouldNotifyReconcile(oldProbe map[string]interface{}, ids []string, fpKey string) bool {
	if len(ids) == 0 {
		return false
	}
	fp := idsFingerprint(ids)
	meta := reconcileMetaFromOld(oldProbe)
	if prev, ok := meta[fpKey].(string); ok && prev == fp {
		return false
	}
	return true
}

// formatReconcileNotifyTitle 供测试与通知复用。
func formatReconcileNotifyTitle(kind string, channelName string, n int) string {
	switch kind {
	case "upstream_model_added":
		return fmt.Sprintf("上游模型新增 · 渠道 %s（%d 个）", channelName, n)
	case "upstream_model_removed":
		return fmt.Sprintf("上游模型移除 · 渠道 %s（%d 个）", channelName, n)
	default:
		return fmt.Sprintf("上游模型变更 · 渠道 %s", channelName)
	}
}
