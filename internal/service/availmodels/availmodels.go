package availmodels

import (
	"context"
	"sort"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/service/billing"
)

// Item 与 OpenAI /v1/models 单项字段对齐（控制台展示用）。
type Item struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

func inferOwnedBy(id string) string {
	owner := "tracehub"
	if strings.HasPrefix(id, "gpt") || strings.Contains(id, "gpt") {
		owner = "openai"
	} else if strings.HasPrefix(id, "claude") {
		owner = "anthropic"
	} else if strings.HasPrefix(id, "gemini") {
		owner = "google"
	} else if strings.HasPrefix(id, "deepseek") {
		owner = "deepseek"
	}
	return owner
}

// collectChannelModelKeys 从已筛选的渠道列表中汇总「声明的模型/别名」及其中健康渠道（health_status==1）覆盖的子集。
func collectChannelModelKeys(channels []*ent.Channel) (all map[string]struct{}, healthy map[string]struct{}) {
	all = make(map[string]struct{})
	healthy = make(map[string]struct{})
	for _, ch := range channels {
		hsOK := ch.HealthStatus == 1
		for _, m := range ch.Models {
			id := strings.TrimSpace(m)
			if id == "" {
				continue
			}
			all[id] = struct{}{}
			if hsOK {
				healthy[id] = struct{}{}
			}
		}
		if ch.ModelAlias != nil {
			for alias := range ch.ModelAlias {
				a := strings.TrimSpace(alias)
				if a == "" {
					continue
				}
				all[a] = struct{}{}
				if hsOK {
					healthy[a] = struct{}{}
				}
			}
		}
	}
	return all, healthy
}

func itemsFromModelIDSet(set map[string]struct{}) []Item {
	if len(set) == 0 {
		return []Item{}
	}
	ids := make([]string, 0, len(set))
	for m := range set {
		ids = append(ids, m)
	}
	sort.Strings(ids)
	ts := time.Now().Unix()
	out := make([]Item, 0, len(ids))
	for _, id := range ids {
		out = append(out, Item{
			ID:      id,
			Object:  "model",
			Created: ts,
			OwnedBy: inferOwnedBy(id),
		})
	}
	return out
}

// RouterHealthLabel 根据渠道声明与健康位，给出与 SelectChannel 一致语义的状态标签（供 /v1/models 扩展字段）。
// healthy：至少一条已启用、可调度且 health_status==1 的渠道支持该模型；unhealthy：仅有异常渠道声明；catalog_only：来自目录/默认列表，当前分组渠道未声明该模型。
func RouterHealthLabel(channelAll, channelHealthy map[string]struct{}, modelID string) string {
	if len(channelAll) == 0 {
		return "catalog_only"
	}
	if _, ok := channelHealthy[modelID]; ok {
		return "healthy"
	}
	if _, ok := channelAll[modelID]; ok {
		return "unhealthy"
	}
	return "catalog_only"
}

// listGroupModelSet 与 ListForGroup 相同的模型 ID 集合，并返回用于 RouterHealthLabel 的渠道键集合。
func listGroupModelSet(ctx context.Context, client *ent.Client, groupID int) (
	set map[string]struct{},
	channelAll map[string]struct{},
	channelHealthy map[string]struct{},
	err error,
) {
	if groupID <= 0 {
		return map[string]struct{}{}, nil, nil, nil
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HasGroupWith(group.IDEQ(groupID)),
		).
		All(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	channelAll, channelHealthy = collectChannelModelKeys(channels)
	set = make(map[string]struct{})
	for id := range channelAll {
		set[id] = struct{}{}
	}
	if len(set) == 0 {
		rows, err := client.ModelCatalogEntry.Query().
			Where(
				modelcatalogentry.Visible(true),
				modelcatalogentry.Enabled(true),
			).
			All(ctx)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, row := range rows {
			id := strings.TrimSpace(row.ModelID)
			if id != "" {
				set[id] = struct{}{}
			}
		}
	}
	if len(set) == 0 {
		for _, id := range billing.DefaultModelIDs() {
			set[id] = struct{}{}
		}
	}
	return set, channelAll, channelHealthy, nil
}

// ListForGroup 汇总分组下可用渠道中的模型 ID（含别名），与网关 GET /v1/models 一致。
// 列表用于发现模型：不要求渠道 health=健康（路由时仍会过滤异常渠道）。
// 若渠道未配置任何模型，则合并「模型市场」可见条目，再回退到内置默认定价表模型 ID，避免 /v1/models 返回空数组导致下游工具无法拉模。
func ListForGroup(ctx context.Context, client *ent.Client, groupID int) ([]Item, error) {
	set, _, _, err := listGroupModelSet(ctx, client, groupID)
	if err != nil {
		return nil, err
	}
	return itemsFromModelIDSet(set), nil
}

// ListForGroupWithChannelHealthKeys 返回与 ListForGroup 相同的列表，以及用于 RouterHealthLabel 的渠道模型键（全部 / 健康子集）。
func ListForGroupWithChannelHealthKeys(ctx context.Context, client *ent.Client, groupID int) ([]Item, map[string]struct{}, map[string]struct{}, error) {
	set, channelAll, channelHealthy, err := listGroupModelSet(ctx, client, groupID)
	if err != nil {
		return nil, nil, nil, err
	}
	return itemsFromModelIDSet(set), channelAll, channelHealthy, nil
}

// ConsoleModel 控制台合并列表：网关渠道可调度模型 + 模型市场中对用户可见的目录条目。
type ConsoleModel struct {
	ID                    string `json:"id"`
	Object                string `json:"object"`
	Created               int64  `json:"created"`
	OwnedBy               string `json:"owned_by"`
	DisplayName           string `json:"display_name,omitempty"`
	InGateway             bool   `json:"in_gateway"`
	InCatalog             bool   `json:"in_catalog"`
	UpstreamListStatus    string `json:"upstream_list_status,omitempty"`    // listed|not_listed|mixed
	UpstreamListCheckedAt string `json:"upstream_list_checked_at,omitempty"` // RFC3339，多渠道路径取最晚探测时间
	Labels                []string `json:"labels,omitempty"`                 // 类型标签（启发式 + 可选目录 metadata）
}

// ListConsoleMerged 合并渠道模型 ID 与模型市场 visible+enabled 条目；网关侧仍以 ListForGroup 为准。
func ListConsoleMerged(ctx context.Context, client *ent.Client, groupID int) ([]ConsoleModel, error) {
	gatewayItems, err := ListForGroup(ctx, client, groupID)
	if err != nil {
		return nil, err
	}
	catalogRows, err := client.ModelCatalogEntry.Query().
		Where(
			modelcatalogentry.Visible(true),
			modelcatalogentry.Enabled(true),
		).
		Order(ent.Asc(modelcatalogentry.FieldModelID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ts := time.Now().Unix()
	byID := make(map[string]*ConsoleModel)
	for _, it := range gatewayItems {
		byID[it.ID] = &ConsoleModel{
			ID: it.ID, Object: "model", Created: ts, OwnedBy: it.OwnedBy,
			InGateway: true, InCatalog: false,
		}
	}
	for _, row := range catalogRows {
		id := strings.TrimSpace(row.ModelID)
		if id == "" {
			continue
		}
		ob := strings.TrimSpace(row.OwnedBy)
		if ob == "" {
			ob = inferOwnedBy(id)
		}
		dn := strings.TrimSpace(row.DisplayName)
		if ex, ok := byID[id]; ok {
			ex.InCatalog = true
			if dn != "" {
				ex.DisplayName = dn
			}
			if strings.TrimSpace(row.OwnedBy) != "" {
				ex.OwnedBy = strings.TrimSpace(row.OwnedBy)
			}
			continue
		}
		byID[id] = &ConsoleModel{
			ID: id, Object: "model", Created: ts, OwnedBy: ob,
			DisplayName: dn,
			InGateway:   false,
			InCatalog:   true,
		}
	}
	if groupID > 0 {
		channels, err := client.Channel.Query().
			Where(
				channel.Enabled(true),
				channel.Schedulable(true),
				channel.HasGroupWith(group.IDEQ(groupID)),
			).
			All(ctx)
		if err != nil {
			return nil, err
		}
		attachUpstreamListProbeFromChannels(channels, byID)
	}
	catalogMetaByID := make(map[string]map[string]any, len(catalogRows))
	adminLabelsByID := make(map[string][]string, len(catalogRows))
	for _, row := range catalogRows {
		id := strings.TrimSpace(row.ModelID)
		if id == "" {
			continue
		}
		if len(row.AdminLabels) > 0 {
			adminLabelsByID[id] = row.AdminLabels
		}
		if row.Metadata != nil && len(row.Metadata) > 0 {
			catalogMetaByID[id] = row.Metadata
		}
	}
	for id, m := range byID {
		m.Labels = modelkind.LabelsForModel(id, catalogMetaByID[id], adminLabelsByID[id])
	}
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ConsoleModel, 0, len(ids))
	for _, id := range ids {
		out = append(out, *byID[id])
	}
	return out, nil
}
