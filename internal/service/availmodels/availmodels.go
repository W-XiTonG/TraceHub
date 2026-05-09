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

// ListForGroup 汇总分组下可用渠道中的模型 ID（含别名），与网关 GET /v1/models 一致。
func ListForGroup(ctx context.Context, client *ent.Client, groupID int) ([]Item, error) {
	if groupID <= 0 {
		return []Item{}, nil
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HealthStatusEQ(1),
			channel.HasGroupWith(group.IDEQ(groupID)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, ch := range channels {
		for _, m := range ch.Models {
			set[m] = struct{}{}
		}
		for alias := range ch.ModelAlias {
			set[alias] = struct{}{}
		}
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
	return out, nil
}

// ConsoleModel 控制台合并列表：网关渠道可调度模型 + 模型市场中对用户可见的目录条目。
type ConsoleModel struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created"`
	OwnedBy     string `json:"owned_by"`
	DisplayName string `json:"display_name,omitempty"`
	InGateway   bool   `json:"in_gateway"`
	InCatalog   bool   `json:"in_catalog"`
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
