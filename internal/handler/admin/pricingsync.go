package adminhandler

import (
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelpricingsynctarget"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/cryptokey"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/pricingsync"

	"github.com/gin-gonic/gin"
)

type PricingSyncDeps struct {
	Client *ent.Client
	AESKey []byte
	Config *config.Config
}

func pricingSyncSvc(d PricingSyncDeps) pricingsync.ServiceOpts {
	return pricingsync.ServiceOpts{
		Client:            d.Client,
		AESKey:            d.AESKey,
		AllowInsecureHTTP: true,
	}
}

func isSuperadmin(c *gin.Context) bool {
	cl := c.MustGet(middleware.CtxUserKey).(*ent.User)
	return cl.Role == "superadmin"
}

func requireSyncExecute(c *gin.Context, client *ent.Client) bool {
	if sitecfg.EffectivePricingSyncRequireSuperadmin(c.Request.Context(), client) && !isSuperadmin(c) {
		httpx.Forbidden(c, "仅 superadmin 可执行同步")
		return false
	}
	return true
}

func targetToJSON(t *ent.ModelPricingSyncTarget) gin.H {
	return gin.H{
		"id":                     t.ID,
		"name":                   t.Name,
		"direction":              t.Direction,
		"base_url":               t.BaseURL,
		"pull_path":              t.PullPath,
		"push_path":              t.PushPath,
		"credential_mode":        t.CredentialMode,
		"has_static_secret":      t.APIKeyEncrypted != "",
		"channel_id":             t.ChannelID,
		"extra_headers":          t.ExtraHeaders,
		"parse_mode":             t.ParseMode,
		"scopes":                 t.Scopes,
		"channel_filter":         t.ChannelFilter,
		"group_filter":           t.GroupFilter,
		"model_include_mode":     t.ModelIncludeMode,
		"model_allowlist":        t.ModelAllowlist,
		"model_denylist":         t.ModelDenylist,
		"model_include_patterns": t.ModelIncludePatterns,
		"channel_ref_map":        t.ChannelRefMap,
		"group_ref_map":          t.GroupRefMap,
		"merge_strategy":         t.MergeStrategy,
		"schedule_enabled":       t.ScheduleEnabled,
		"schedule_interval_min":  t.ScheduleIntervalMin,
		"enabled":                t.Enabled,
		"last_pull_at":           t.LastPullAt,
		"last_pull_ok":           t.LastPullOk,
		"last_pull_message":      t.LastPullMessage,
		"last_push_at":           t.LastPushAt,
		"last_push_ok":           t.LastPushOk,
		"last_push_message":      t.LastPushMessage,
		"created_at":             t.CreatedAt,
		"updated_at":             t.UpdatedAt,
	}
}

func ListPricingSyncTargets(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := client.ModelPricingSyncTarget.Query().Order(ent.Desc(modelpricingsynctarget.FieldUpdatedAt)).All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, t := range list {
			items = append(items, targetToJSON(t))
		}
		enabled := sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), client)
		meta, _ := sitecfg.PricingSyncSettingsMeta(c.Request.Context(), client)
		httpx.OK(c, gin.H{"items": items, "total": len(items), "feature_enabled": enabled, "settings": meta})
	}
}

func CreatePricingSyncTarget(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可创建同步目标")
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req pricingSyncTargetReq
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.BaseURL) == "" {
			httpx.BadRequest(c, "name 与 base_url 必填")
			return
		}
		ctx := c.Request.Context()
		cr, err := buildTargetCreate(d.Client, req, d.AESKey)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		t, err := cr.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(d.Client, adm.ID, "pricing_sync.target.create", "pricing_sync_target", strconv.Itoa(t.ID), nil, c)
		httpx.OKMsg(c, "已创建", targetToJSON(t))
	}
}

func UpdatePricingSyncTarget(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可修改同步目标")
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req pricingSyncTargetReq
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		up, err := buildTargetUpdate(d.Client, id, req, d.AESKey)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		t, err := up.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(d.Client, adm.ID, "pricing_sync.target.update", "pricing_sync_target", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已更新", targetToJSON(t))
	}
}

func DeletePricingSyncTarget(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可删除同步目标")
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		if err := client.ModelPricingSyncTarget.DeleteOneID(id).Exec(c.Request.Context()); err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "记录不存在")
				return
			}
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "pricing_sync.target.delete", "pricing_sync_target", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func PreviewPricingSync(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), d.Client) {
			httpx.Forbidden(c, "价格同步功能未开启")
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Direction      string   `json:"direction"`
			SelectedModels []string `json:"selected_models"`
			SelectedKeys   []string `json:"selected_keys"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Direction == "" {
			httpx.BadRequest(c, "direction 必填")
			return
		}
		if req.Direction != pricingsync.DirectionPull {
			httpx.BadRequest(c, "仅支持 pull 拉取同步")
			return
		}
		res, err := pricingsync.Preview(c.Request.Context(), pricingSyncSvc(d), id, pricingsync.PreviewRequest{
			Direction:      req.Direction,
			SelectedModels: req.SelectedModels,
			SelectedKeys:   req.SelectedKeys,
		})
		if err != nil {
			mapPricingSyncErr(c, err)
			return
		}
		auditLog(d.Client, adm.ID, "pricing_sync.preview", "pricing_sync_target", strconv.Itoa(id), gin.H{"preview_id": res.PreviewID, "direction": req.Direction}, c)
		httpx.OK(c, res)
	}
}

func ConfirmPricingSync(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), d.Client) {
			httpx.Forbidden(c, "价格同步功能未开启")
			return
		}
		if !requireSyncExecute(c, d.Client) {
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			PreviewID string `json:"preview_id"`
			Direction string `json:"direction"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.PreviewID == "" || req.Direction == "" {
			httpx.BadRequest(c, "preview_id 与 direction 必填")
			return
		}
		if req.Direction != pricingsync.DirectionPull {
			httpx.BadRequest(c, "仅支持 pull 拉取同步")
			return
		}
		res, err := pricingsync.Confirm(c.Request.Context(), pricingSyncSvc(d), id, pricingsync.ConfirmRequest{
			PreviewID: req.PreviewID,
			Direction: req.Direction,
		}, true)
		if err != nil {
			mapPricingSyncErr(c, err)
			return
		}
		auditLog(d.Client, adm.ID, "pricing_sync.confirm", "pricing_sync_target", strconv.Itoa(id), gin.H{"preview_id": req.PreviewID, "result": res}, c)
		httpx.OK(c, res)
	}
}

func GetPricingSyncInboundInfo(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		configured, createdAt := sitecfg.InboundSyncTokenMeta(ctx, client)
		exportURL, importURL, base := sitecfg.BuildInboundSyncURLs(ctx, client, c.Request.URL.Scheme, c.Request.Host)
		exportPath, importPath := sitecfg.InboundPricingPaths()
		httpx.OK(c, gin.H{
			"configured":        configured,
			"token_created_at":  createdAt,
			"export_url":        exportURL,
			"import_url":        importURL,
			"export_path":       exportPath,
			"import_path":       importPath,
			"public_base":       base,
			"feature_enabled":   sitecfg.EffectivePricingSyncEnabled(ctx, client),
			"default_pull_path": exportPath,
			"default_push_path": importPath,
		})
	}
}

func RegeneratePricingSyncInboundToken(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可轮换入站密钥")
			return
		}
		if !sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), client) {
			httpx.Forbidden(c, "请先开启价格同步功能")
			return
		}
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		plain, err := sitecfg.GenerateInboundSyncToken(c.Request.Context(), client)
		if err != nil {
			httpx.ServerError(c, "生成失败")
			return
		}
		exportURL, importURL, _ := sitecfg.BuildInboundSyncURLs(c.Request.Context(), client, c.Request.URL.Scheme, c.Request.Host)
		auditLog(client, adm.ID, "pricing_sync.inbound_token.regenerate", "system_config", "pricing_sync_inbound", nil, c)
		httpx.OK(c, gin.H{
			"token":       plain,
			"export_url":  exportURL,
			"import_url":  importURL,
			"message":     "密钥已生成，请立即复制保存（仅显示一次）",
		})
	}
}

func GeneratePricingSyncOutboundKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可生成出站密钥")
			return
		}
		key, err := sitecfg.GenerateOutboundSyncAPIKey()
		if err != nil {
			httpx.ServerError(c, "生成失败")
			return
		}
		httpx.OK(c, gin.H{"api_key": key})
	}
}

func TestPricingSyncConnection(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isSuperadmin(c) {
			httpx.Forbidden(c, "仅 superadmin 可测试连接")
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Direction string `json:"direction"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		if req.Direction != "" && req.Direction != pricingsync.DirectionPull {
			httpx.BadRequest(c, "仅支持 pull 拉取测试")
			return
		}
		res, err := pricingsync.TestTargetConnection(c.Request.Context(), d.Client, id, pricingsync.DirectionPull, pricingsync.RemoteOpts{
			AESKey:            d.AESKey,
			AllowInsecureHTTP: true,
		})
		if err != nil {
			mapPricingSyncErr(c, err)
			return
		}
		httpx.OK(c, res)
	}
}

func ExportLocalPricing(d PricingSyncDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sitecfg.EffectivePricingSyncEnabled(c.Request.Context(), d.Client) {
			httpx.Forbidden(c, "价格同步功能未开启")
			return
		}
		tid, _ := strconv.Atoi(c.Query("target_id"))
		ctx := c.Request.Context()
		var payload *pricingsync.Payload
		var err error
		if tid > 0 {
			t, e := d.Client.ModelPricingSyncTarget.Get(ctx, tid)
			if e != nil {
				httpx.BadRequest(c, "target 不存在")
				return
			}
			payload, err = pricingsync.ExportLocal(ctx, d.Client, t)
		} else {
			t := &ent.ModelPricingSyncTarget{Scopes: []string{"global", "channel", "group"}}
			payload, err = pricingsync.ExportLocal(ctx, d.Client, t)
		}
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		httpx.OK(c, payload)
	}
}

func mapPricingSyncErr(c *gin.Context, err error) {
	switch err {
	case pricingsync.ErrFeatureDisabled:
		httpx.Forbidden(c, err.Error())
	case pricingsync.ErrPreviewExpired, pricingsync.ErrPreviewMismatch:
		httpx.BadRequest(c, err.Error())
	case pricingsync.ErrCooldown:
		httpx.TooManyRequests(c, err.Error())
	case pricingsync.ErrSyncInProgress:
		httpx.Err(c, 409, 409, err.Error())
	default:
		if err == pricingsync.ErrFeatureDisabled {
			httpx.Forbidden(c, err.Error())
			return
		}
		httpx.BadRequest(c, err.Error())
	}
}

type pricingSyncTargetReq struct {
	Name                 string            `json:"name"`
	Direction            string            `json:"direction"`
	BaseURL              string            `json:"base_url"`
	PullPath             string            `json:"pull_path"`
	PushPath             string            `json:"push_path"`
	CredentialMode       string            `json:"credential_mode"`
	APIKey               string            `json:"api_key"`
	ChannelID            int               `json:"channel_id"`
	ExtraHeaders         map[string]string `json:"extra_headers"`
	Scopes               []string          `json:"scopes"`
	ChannelFilter        []int             `json:"channel_filter"`
	GroupFilter          []int             `json:"group_filter"`
	ModelIncludeMode     string            `json:"model_include_mode"`
	ModelAllowlist       []string          `json:"model_allowlist"`
	ModelDenylist        []string          `json:"model_denylist"`
	ModelIncludePatterns []string          `json:"model_include_patterns"`
	ChannelRefMap        map[string]int    `json:"channel_ref_map"`
	GroupRefMap          map[string]int    `json:"group_ref_map"`
	MergeStrategy        string            `json:"merge_strategy"`
	ScheduleEnabled      *bool             `json:"schedule_enabled"`
	ScheduleIntervalMin  int               `json:"schedule_interval_min"`
	Enabled              *bool             `json:"enabled"`
}

func buildTargetCreate(client *ent.Client, req pricingSyncTargetReq, aesKey []byte) (*ent.ModelPricingSyncTargetCreate, error) {
	cr := client.ModelPricingSyncTarget.Create().
		SetName(strings.TrimSpace(req.Name)).
		SetBaseURL(strings.TrimSpace(req.BaseURL)).
		SetDirection(modelpricingsynctarget.DirectionPull)
	if req.PullPath != "" {
		cr.SetPullPath(req.PullPath)
	}
	if req.CredentialMode != "" {
		cr.SetCredentialMode(modelpricingsynctarget.CredentialMode(req.CredentialMode))
	}
	if len(req.ExtraHeaders) > 0 {
		cr.SetExtraHeaders(req.ExtraHeaders)
	}
	if len(req.Scopes) > 0 {
		cr.SetScopes(req.Scopes)
	}
	if len(req.ChannelFilter) > 0 {
		cr.SetChannelFilter(req.ChannelFilter)
	}
	if len(req.GroupFilter) > 0 {
		cr.SetGroupFilter(req.GroupFilter)
	}
	if req.ModelIncludeMode != "" {
		cr.SetModelIncludeMode(modelpricingsynctarget.ModelIncludeMode(req.ModelIncludeMode))
	}
	if len(req.ModelAllowlist) > 0 {
		cr.SetModelAllowlist(req.ModelAllowlist)
	}
	if len(req.ModelDenylist) > 0 {
		cr.SetModelDenylist(req.ModelDenylist)
	}
	if len(req.ModelIncludePatterns) > 0 {
		cr.SetModelIncludePatterns(req.ModelIncludePatterns)
	}
	if len(req.ChannelRefMap) > 0 {
		cr.SetChannelRefMap(req.ChannelRefMap)
	}
	if len(req.GroupRefMap) > 0 {
		cr.SetGroupRefMap(req.GroupRefMap)
	}
	if req.MergeStrategy != "" {
		cr.SetMergeStrategy(modelpricingsynctarget.MergeStrategy(req.MergeStrategy))
	}
	if req.ScheduleEnabled != nil {
		cr.SetScheduleEnabled(*req.ScheduleEnabled)
	}
	cr.SetScheduleIntervalMin(pricingsync.NormalizeScheduleIntervalMin(req.ScheduleIntervalMin))
	if req.Enabled != nil {
		cr.SetEnabled(*req.Enabled)
	}
	if req.ChannelID > 0 {
		cr.SetChannelID(req.ChannelID)
	}
	if req.APIKey != "" {
		enc, err := cryptokey.EncryptGCM(aesKey, []byte(req.APIKey))
		if err != nil {
			return nil, err
		}
		cr.SetAPIKeyEncrypted(enc)
	}
	return cr, nil
}

func buildTargetUpdate(client *ent.Client, id int, req pricingSyncTargetReq, aesKey []byte) (*ent.ModelPricingSyncTargetUpdateOne, error) {
	up := client.ModelPricingSyncTarget.UpdateOneID(id)
	if strings.TrimSpace(req.Name) != "" {
		up.SetName(strings.TrimSpace(req.Name))
	}
	if strings.TrimSpace(req.BaseURL) != "" {
		up.SetBaseURL(strings.TrimSpace(req.BaseURL))
	}
	up.SetDirection(modelpricingsynctarget.DirectionPull)
	if req.PullPath != "" {
		up.SetPullPath(req.PullPath)
	}
	if req.CredentialMode != "" {
		up.SetCredentialMode(modelpricingsynctarget.CredentialMode(req.CredentialMode))
	}
	if req.ExtraHeaders != nil {
		up.SetExtraHeaders(req.ExtraHeaders)
	}
	if req.Scopes != nil {
		up.SetScopes(req.Scopes)
	}
	if req.ChannelFilter != nil {
		up.SetChannelFilter(req.ChannelFilter)
	}
	if req.GroupFilter != nil {
		up.SetGroupFilter(req.GroupFilter)
	}
	if req.ModelIncludeMode != "" {
		up.SetModelIncludeMode(modelpricingsynctarget.ModelIncludeMode(req.ModelIncludeMode))
	}
	if req.ModelAllowlist != nil {
		up.SetModelAllowlist(req.ModelAllowlist)
	}
	if req.ModelDenylist != nil {
		up.SetModelDenylist(req.ModelDenylist)
	}
	if req.ModelIncludePatterns != nil {
		up.SetModelIncludePatterns(req.ModelIncludePatterns)
	}
	if req.ChannelRefMap != nil {
		up.SetChannelRefMap(req.ChannelRefMap)
	}
	if req.GroupRefMap != nil {
		up.SetGroupRefMap(req.GroupRefMap)
	}
	if req.MergeStrategy != "" {
		up.SetMergeStrategy(modelpricingsynctarget.MergeStrategy(req.MergeStrategy))
	}
	if req.ScheduleEnabled != nil {
		up.SetScheduleEnabled(*req.ScheduleEnabled)
	}
	if req.ScheduleIntervalMin > 0 {
		up.SetScheduleIntervalMin(pricingsync.NormalizeScheduleIntervalMin(req.ScheduleIntervalMin))
	}
	if req.Enabled != nil {
		up.SetEnabled(*req.Enabled)
	}
	if req.ChannelID > 0 {
		up.SetChannelID(req.ChannelID)
	}
	if req.APIKey != "" {
		enc, err := cryptokey.EncryptGCM(aesKey, []byte(req.APIKey))
		if err != nil {
			return nil, err
		}
		up.SetAPIKeyEncrypted(enc)
	}
	return up, nil
}
