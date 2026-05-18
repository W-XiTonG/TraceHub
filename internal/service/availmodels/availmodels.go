package availmodels

import (
	"context"
	"sort"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/channelmodelpricing"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/service/billing"
)

// Item 与 OpenAI /v1/models 单项字段对齐（控制台展示用）。
type Item struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	OwnedBy       string `json:"owned_by"`
	ServiceStatus string `json:"service_status,omitempty"`
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
	channels []*ent.Channel,
	err error,
) {
	if groupID <= 0 {
		return map[string]struct{}{}, nil, nil, nil, nil
	}
	var chErr error
	channels, chErr = client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HasGroupWith(group.IDEQ(groupID)),
		).
		All(ctx)
	err = chErr
	if err != nil {
		return nil, nil, nil, nil, err
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
			return nil, nil, nil, nil, err
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
	return set, channelAll, channelHealthy, channels, nil
}

// ListForGroup 汇总分组下可用渠道中的模型 ID（含别名），与网关 GET /v1/models 一致。
// 列表用于发现模型：不要求渠道 health=健康（路由时仍会过滤异常渠道）。
// 若渠道未配置任何模型，则合并「模型市场」可见条目，再回退到内置默认定价表模型 ID，避免 /v1/models 返回空数组导致下游工具无法拉模。
func ListForGroup(ctx context.Context, client *ent.Client, groupID int) ([]Item, error) {
	items, _, _, err := ListForGroupWithChannelHealthKeys(ctx, client, groupID, defaultListOpts(ctx, client))
	return items, err
}

// ListForGroupWithOpts 带 service_status 聚合与默认过滤（omit unavailable）。
func ListForGroupWithOpts(ctx context.Context, client *ent.Client, groupID int, opts ListOpts) ([]Item, error) {
	items, _, _, err := ListForGroupWithChannelHealthKeys(ctx, client, groupID, opts)
	return items, err
}

// ListForGroupWithChannelHealthKeys 返回与 ListForGroup 相同的列表，以及用于 RouterHealthLabel 的渠道模型键（全部 / 健康子集）。
func ListForGroupWithChannelHealthKeys(ctx context.Context, client *ent.Client, groupID int, opts ListOpts) ([]Item, map[string]struct{}, map[string]struct{}, error) {
	set, channelAll, channelHealthy, channels, err := listGroupModelSet(ctx, client, groupID)
	if err != nil {
		return nil, nil, nil, err
	}
	items := itemsFromModelIDSet(set)
	if groupID <= 0 || len(channels) == 0 {
		return items, channelAll, channelHealthy, nil
	}
	labelsByID := make(map[string][]string, len(set))
	for id := range set {
		labelsByID[id] = modelkind.LabelsForModel(id, nil, nil)
	}
	statusMap := buildServiceStatusMap(ctx, channels, itemsToIDs(items), labelsByID, opts.FailStaleHours)
	items = filterItemsByServiceStatus(items, statusMap, opts)
	return items, channelAll, channelHealthy, nil
}

func itemsToIDs(items []Item) []string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

// DeclaredModelDistinctCount 汇总多路渠道上「已声明」的模型键去重数量：
// 每条渠道的 models 列表与 model_alias 的键（请求侧逻辑名），语义与 collectChannelModelKeys 的 all 集一致。
func DeclaredModelDistinctCount(channels []*ent.Channel) int {
	all, _ := collectChannelModelKeys(channels)
	return len(all)
}

// DistinctDeclaredModelCountSiteWide 统计全站已启用渠道上的声明模型去重数（models ∪ model_alias 键）。
func DistinctDeclaredModelCountSiteWide(ctx context.Context, client *ent.Client) (int, error) {
	channels, err := client.Channel.Query().
		Where(channel.Enabled(true)).
		All(ctx)
	if err != nil {
		return 0, err
	}
	return DeclaredModelDistinctCount(channels), nil
}

// ModelPricing 用户侧展示的有效单价（与 billing.ModelBillRates 一致）。
type ModelPricing struct {
	Billable      bool   `json:"billable"`
	QuotaInPer1k  int64  `json:"quota_in_per_1k"`
	QuotaOutPer1k int64  `json:"quota_out_per_1k"`
	PricingUnit   string `json:"pricing_unit"` // per_1k_tokens
}

// ConsoleModel 控制台合并列表：网关渠道可调度模型 + 模型市场中对用户可见的目录条目。
type ConsoleModel struct {
	ID                    string        `json:"id"`
	Object                string        `json:"object"`
	Created               int64         `json:"created"`
	OwnedBy               string        `json:"owned_by"`
	DisplayName           string        `json:"display_name,omitempty"`
	InGateway             bool          `json:"in_gateway"`
	InCatalog             bool          `json:"in_catalog"`
	UpstreamListStatus    string        `json:"upstream_list_status,omitempty"`     // listed|not_listed|mixed
	UpstreamListCheckedAt string        `json:"upstream_list_checked_at,omitempty"` // RFC3339，多渠道路径取最晚探测时间
	Labels                []string      `json:"labels,omitempty"`                   // 类型标签（启发式 + 可选目录 metadata）
	ServiceStatus         string        `json:"service_status,omitempty"`           // operational|unavailable|stale|degraded|catalog_only
	Pricing               *ModelPricing `json:"pricing,omitempty"`
}

// ListConsoleMerged 合并渠道模型 ID 与模型市场 visible+enabled 条目；网关侧仍以 ListForGroup 为准。
func ListConsoleMerged(ctx context.Context, client *ent.Client, groupID int, opts ListOpts) ([]ConsoleModel, error) {
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
		attachServiceStatusFromChannels(ctx, channels, byID, opts)
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
		if !m.InGateway && m.ServiceStatus == "" {
			m.ServiceStatus = ServiceStatusCatalogOnly
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
	return filterConsoleModels(out, opts), nil
}

// GlobalPricingWorkspaceItem 管理端「全局计价」合并视图的一行：含代码基准、来源标签与可选的全局库记录。
type GlobalPricingWorkspaceItem struct {
	ModelName    string                  `json:"model_name"`
	CodeInPer1k  int64                   `json:"code_in_per_1k"`
	CodeOutPer1k int64                   `json:"code_out_per_1k"`
	Sources      []string                `json:"sources"`
	Global       *ent.GlobalModelPricing `json:"global,omitempty"`
}

// EnabledChannelDeclaredModelKeys 返回全站「已启用」渠道在 models 与 model_alias 中声明的键（trim、去重）。
func EnabledChannelDeclaredModelKeys(ctx context.Context, client *ent.Client) (map[string]struct{}, error) {
	channels, err := client.Channel.Query().Where(channel.Enabled(true)).All(ctx)
	if err != nil {
		return nil, err
	}
	all, _ := collectChannelModelKeys(channels)
	return all, nil
}

// VisibleCatalogModelIDs 返回模型市场中 visible 且 enabled 条目的 model_id 集合。
func VisibleCatalogModelIDs(ctx context.Context, client *ent.Client) (map[string]struct{}, error) {
	rows, err := client.ModelCatalogEntry.Query().
		Where(
			modelcatalogentry.Visible(true),
			modelcatalogentry.Enabled(true),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		id := strings.TrimSpace(row.ModelID)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	return out, nil
}

func buildGlobalPricingWorkspaceList(ctx context.Context, client *ent.Client) ([]GlobalPricingWorkspaceItem, error) {
	builtin := billing.BuiltinPricingModelIDSet()
	ch, err := EnabledChannelDeclaredModelKeys(ctx, client)
	if err != nil {
		return nil, err
	}
	cat, err := VisibleCatalogModelIDs(ctx, client)
	if err != nil {
		return nil, err
	}
	gRows, err := client.GlobalModelPricing.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*ent.GlobalModelPricing, len(gRows))
	for _, row := range gRows {
		n := strings.TrimSpace(row.ModelName)
		if n != "" {
			byName[n] = row
		}
	}
	set := make(map[string]struct{})
	for k := range builtin {
		set[k] = struct{}{}
	}
	for k := range ch {
		set[k] = struct{}{}
	}
	for k := range cat {
		set[k] = struct{}{}
	}
	for n := range byName {
		set[n] = struct{}{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]GlobalPricingWorkspaceItem, 0, len(ids))
	for _, name := range ids {
		cin, cout := billing.LookupPricing(name)
		src := make([]string, 0, 4)
		if _, ok := builtin[name]; ok {
			src = append(src, "builtin")
		}
		if _, ok := ch[name]; ok {
			src = append(src, "channel")
		}
		if _, ok := cat[name]; ok {
			src = append(src, "catalog")
		}
		if _, ok := byName[name]; ok {
			src = append(src, "global_db")
		}
		sort.Strings(src)
		item := GlobalPricingWorkspaceItem{
			ModelName:    name,
			CodeInPer1k:  cin,
			CodeOutPer1k: cout,
			Sources:      src,
		}
		if g, ok := byName[name]; ok {
			item.Global = g
		}
		out = append(out, item)
	}
	return out, nil
}

func filterWorkspaceByModelQuery(all []GlobalPricingWorkspaceItem, q string) []GlobalPricingWorkspaceItem {
	q = strings.TrimSpace(q)
	if q == "" {
		return all
	}
	ql := strings.ToLower(q)
	out := make([]GlobalPricingWorkspaceItem, 0)
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.ModelName), ql) {
			out = append(out, it)
		}
	}
	return out
}

func pageGlobalPricingWorkspace(filtered []GlobalPricingWorkspaceItem, page, pageSize int) (items []GlobalPricingWorkspaceItem, total int, pageOut int) {
	total = len(filtered)
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	pageOut = page
	if pageOut < 1 {
		pageOut = 1
	}
	maxPage := 1
	if total > 0 {
		maxPage = (total + pageSize - 1) / pageSize
	}
	if pageOut > maxPage {
		pageOut = maxPage
	}
	offset := (pageOut - 1) * pageSize
	if offset >= len(filtered) {
		return []GlobalPricingWorkspaceItem{}, total, pageOut
	}
	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]GlobalPricingWorkspaceItem(nil), filtered[offset:end]...), total, pageOut
}

// GlobalModelPricingWorkspacePage 在合并全表上做 model_name 子串筛选（不区分大小写）与分页。
// 返回当前页数据、筛选后总数、校正后的页码（防止越界）。
func GlobalModelPricingWorkspacePage(ctx context.Context, client *ent.Client, q string, page, pageSize int) ([]GlobalPricingWorkspaceItem, int, int, error) {
	all, err := buildGlobalPricingWorkspaceList(ctx, client)
	if err != nil {
		return nil, 0, page, err
	}
	filtered := filterWorkspaceByModelQuery(all, q)
	items, total, pageOut := pageGlobalPricingWorkspace(filtered, page, pageSize)
	return items, total, pageOut, nil
}

// AllGlobalPricingWorkspaceItems 返回合并后的全量工作区行（无筛选分页），供 CSV 导出等使用。
func AllGlobalPricingWorkspaceItems(ctx context.Context, client *ent.Client) ([]GlobalPricingWorkspaceItem, error) {
	return buildGlobalPricingWorkspaceList(ctx, client)
}

// ChannelPricingWorkspaceItem 管理端「渠道模型计价」合并视图的一行。
type ChannelPricingWorkspaceItem struct {
	ModelName    string                      `json:"model_name"`
	CodeInPer1k  int64                       `json:"code_in_per_1k"`
	CodeOutPer1k int64                       `json:"code_out_per_1k"`
	Sources      []string                    `json:"sources"`
	Global       *ent.GlobalModelPricing     `json:"global,omitempty"`
	Channel      *ent.ChannelModelPricing    `json:"channel,omitempty"`
}

func channelDeclaredModelKeys(ch *ent.Channel) map[string]struct{} {
	out := make(map[string]struct{})
	if ch == nil {
		return out
	}
	for _, m := range ch.Models {
		id := strings.TrimSpace(m)
		if id != "" {
			out[id] = struct{}{}
		}
	}
	if ch.ModelAlias != nil {
		for alias := range ch.ModelAlias {
			a := strings.TrimSpace(alias)
			if a != "" {
				out[a] = struct{}{}
			}
		}
	}
	return out
}

func buildChannelPricingWorkspaceList(ctx context.Context, client *ent.Client, channelID int) ([]ChannelPricingWorkspaceItem, error) {
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil {
		return nil, err
	}
	declared := channelDeclaredModelKeys(ch)
	chRows, err := client.ChannelModelPricing.Query().
		Where(channelmodelpricing.ChannelIDEQ(channelID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]*ent.ChannelModelPricing, len(chRows))
	for _, row := range chRows {
		n := strings.TrimSpace(row.ModelName)
		if n != "" {
			byName[n] = row
		}
	}
	gRows, err := client.GlobalModelPricing.Query().All(ctx)
	if err != nil {
		return nil, err
	}
	globalByName := make(map[string]*ent.GlobalModelPricing, len(gRows))
	for _, row := range gRows {
		n := strings.TrimSpace(row.ModelName)
		if n != "" {
			globalByName[n] = row
		}
	}
	set := make(map[string]struct{})
	for k := range declared {
		set[k] = struct{}{}
	}
	for n := range byName {
		set[n] = struct{}{}
	}
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ChannelPricingWorkspaceItem, 0, len(ids))
	for _, name := range ids {
		cin, cout := billing.LookupPricing(name)
		src := make([]string, 0, 2)
		if _, ok := declared[name]; ok {
			src = append(src, "channel_declared")
		}
		if _, ok := byName[name]; ok {
			src = append(src, "channel_db")
		}
		sort.Strings(src)
		item := ChannelPricingWorkspaceItem{
			ModelName:    name,
			CodeInPer1k:  cin,
			CodeOutPer1k: cout,
			Sources:      src,
		}
		if g, ok := globalByName[name]; ok {
			item.Global = g
		}
		if row, ok := byName[name]; ok {
			item.Channel = row
		}
		out = append(out, item)
	}
	return out, nil
}

func filterChannelWorkspaceByModelQuery(all []ChannelPricingWorkspaceItem, q string) []ChannelPricingWorkspaceItem {
	q = strings.TrimSpace(q)
	if q == "" {
		return all
	}
	ql := strings.ToLower(q)
	out := make([]ChannelPricingWorkspaceItem, 0)
	for _, it := range all {
		if strings.Contains(strings.ToLower(it.ModelName), ql) {
			out = append(out, it)
		}
	}
	return out
}

func pageChannelPricingWorkspace(filtered []ChannelPricingWorkspaceItem, page, pageSize int) (items []ChannelPricingWorkspaceItem, total int, pageOut int) {
	total = len(filtered)
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	pageOut = page
	if pageOut < 1 {
		pageOut = 1
	}
	maxPage := 1
	if total > 0 {
		maxPage = (total + pageSize - 1) / pageSize
	}
	if pageOut > maxPage {
		pageOut = maxPage
	}
	offset := (pageOut - 1) * pageSize
	if offset >= len(filtered) {
		return []ChannelPricingWorkspaceItem{}, total, pageOut
	}
	end := offset + pageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return append([]ChannelPricingWorkspaceItem(nil), filtered[offset:end]...), total, pageOut
}

// ChannelModelPricingWorkspacePage 渠道工作区列表：该渠道声明模型 ∪ 已有渠道计价行。
func ChannelModelPricingWorkspacePage(ctx context.Context, client *ent.Client, channelID int, q string, page, pageSize int) ([]ChannelPricingWorkspaceItem, int, int, error) {
	all, err := buildChannelPricingWorkspaceList(ctx, client, channelID)
	if err != nil {
		return nil, 0, page, err
	}
	filtered := filterChannelWorkspaceByModelQuery(all, q)
	items, total, pageOut := pageChannelPricingWorkspace(filtered, page, pageSize)
	return items, total, pageOut, nil
}

// AllChannelPricingWorkspaceItems 返回指定渠道的全量工作区行（无筛选分页）。
func AllChannelPricingWorkspaceItems(ctx context.Context, client *ent.Client, channelID int) ([]ChannelPricingWorkspaceItem, error) {
	return buildChannelPricingWorkspaceList(ctx, client, channelID)
}
