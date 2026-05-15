package adminhandler

import (
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelcatalogentry"
	"tracehub/internal/ent/modelcatalogsource"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/cryptokey"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/service/modelcatalog"

	"github.com/gin-gonic/gin"
)

type ModelCatalogDeps struct {
	Client *ent.Client
	AESKey []byte
}

func ListModelCatalogSources(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := client.ModelCatalogSource.Query().WithChannel().Order(ent.Desc(modelcatalogsource.FieldUpdatedAt)).All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, s := range list {
			items = append(items, gin.H{
				"id":                  s.ID,
				"name":                s.Name,
				"provider_slug":       s.ProviderSlug,
				"base_url":            s.BaseURL,
				"models_path":         s.ModelsPath,
				"credential_mode":     s.CredentialMode,
				"has_static_secret":   s.APIKeyEncrypted != "",
				"channel_id":          s.ChannelID,
				"extra_headers":       s.ExtraHeaders,
				"parse_mode":          s.ParseMode,
				"enabled":             s.Enabled,
				"last_sync_at":        s.LastSyncAt,
				"last_sync_ok":        s.LastSyncOk,
				"last_sync_message":   s.LastSyncMessage,
				"created_at":          s.CreatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": len(items)})
	}
}

func CreateModelCatalogSource(d ModelCatalogDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Name            string            `json:"name"`
			ProviderSlug    string            `json:"provider_slug"`
			BaseURL         string            `json:"base_url"`
			ModelsPath      string            `json:"models_path"`
			CredentialMode  string            `json:"credential_mode"`
			APIKey          string            `json:"api_key"`
			ChannelID       int               `json:"channel_id"`
			ExtraHeaders    map[string]string `json:"extra_headers"`
			ParseMode       string            `json:"parse_mode"`
			Enabled         *bool             `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.BaseURL) == "" {
			httpx.BadRequest(c, "name 与 base_url 必填")
			return
		}
		cm := modelcatalogsource.CredentialMode(req.CredentialMode)
		if req.CredentialMode == "" {
			cm = modelcatalogsource.CredentialModeStatic
		}
		if err := modelcatalogsource.CredentialModeValidator(cm); err != nil {
			httpx.BadRequest(c, "credential_mode 无效")
			return
		}
		pm := modelcatalogsource.ParseMode(req.ParseMode)
		if req.ParseMode == "" {
			pm = modelcatalogsource.ParseModeOpenaiJSON
		}
		if err := modelcatalogsource.ParseModeValidator(pm); err != nil {
			httpx.BadRequest(c, "parse_mode 无效")
			return
		}
		ctx := c.Request.Context()
		cr := d.Client.ModelCatalogSource.Create().
			SetName(strings.TrimSpace(req.Name)).
			SetProviderSlug(strings.TrimSpace(req.ProviderSlug)).
			SetBaseURL(strings.TrimSpace(req.BaseURL)).
			SetCredentialMode(cm).
			SetParseMode(pm)
		if req.ModelsPath != "" {
			cr.SetModelsPath(req.ModelsPath)
		}
		if len(req.ExtraHeaders) > 0 {
			cr.SetExtraHeaders(req.ExtraHeaders)
		}
		if req.Enabled != nil {
			cr.SetEnabled(*req.Enabled)
		}
		switch cm {
		case modelcatalogsource.CredentialModeStatic:
			if req.APIKey != "" {
				enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(req.APIKey))
				if err != nil {
					httpx.ServerError(c, "加密失败")
					return
				}
				cr.SetAPIKeyEncrypted(enc)
			}
		case modelcatalogsource.CredentialModeChannel:
			if req.ChannelID <= 0 {
				httpx.BadRequest(c, "channel 模式需填写 channel_id")
				return
			}
			if _, err := d.Client.Channel.Get(ctx, req.ChannelID); err != nil {
				httpx.BadRequest(c, "渠道不存在")
				return
			}
			cr.SetChannelID(req.ChannelID)
		}
		s, err := cr.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(d.Client, adm.ID, "model_catalog_source.create", "model_catalog_source", strconv.Itoa(s.ID), nil, c)
		httpx.OKMsg(c, "已创建", gin.H{"id": s.ID})
	}
}

func UpdateModelCatalogSource(d ModelCatalogDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Name           *string            `json:"name"`
			ProviderSlug   *string            `json:"provider_slug"`
			BaseURL        *string            `json:"base_url"`
			ModelsPath     *string            `json:"models_path"`
			CredentialMode *string            `json:"credential_mode"`
			APIKey         *string            `json:"api_key"`
			ChannelID      *int               `json:"channel_id"`
			ExtraHeaders   map[string]string  `json:"extra_headers"`
			ParseMode      *string            `json:"parse_mode"`
			Enabled        *bool              `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		s, err := d.Client.ModelCatalogSource.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		up := d.Client.ModelCatalogSource.UpdateOne(s)
		if req.Name != nil {
			up.SetName(strings.TrimSpace(*req.Name))
		}
		if req.ProviderSlug != nil {
			up.SetProviderSlug(strings.TrimSpace(*req.ProviderSlug))
		}
		if req.BaseURL != nil {
			up.SetBaseURL(strings.TrimSpace(*req.BaseURL))
		}
		if req.ModelsPath != nil {
			up.SetModelsPath(*req.ModelsPath)
		}
		if req.CredentialMode != nil {
			cm := modelcatalogsource.CredentialMode(*req.CredentialMode)
			if err := modelcatalogsource.CredentialModeValidator(cm); err != nil {
				httpx.BadRequest(c, "credential_mode 无效")
				return
			}
			up.SetCredentialMode(cm)
		}
		if req.ParseMode != nil {
			pm := modelcatalogsource.ParseMode(*req.ParseMode)
			if err := modelcatalogsource.ParseModeValidator(pm); err != nil {
				httpx.BadRequest(c, "parse_mode 无效")
				return
			}
			up.SetParseMode(pm)
		}
		if req.ExtraHeaders != nil {
			up.SetExtraHeaders(req.ExtraHeaders)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		if req.APIKey != nil && *req.APIKey != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(*req.APIKey))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			up.SetAPIKeyEncrypted(enc)
		}
		if req.ChannelID != nil {
			if *req.ChannelID > 0 {
				if _, err := d.Client.Channel.Get(ctx, *req.ChannelID); err != nil {
					httpx.BadRequest(c, "渠道不存在")
					return
				}
				up.SetChannelID(*req.ChannelID)
			} else {
				up.ClearChannelID()
			}
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(d.Client, adm.ID, "model_catalog_source.update", "model_catalog_source", strconv.Itoa(id), map[string]any{"body": req}, c)
		httpx.OKMsg(c, "已更新", nil)
	}
}

func DeleteModelCatalogSource(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if _, err := client.ModelCatalogEntry.Delete().Where(modelcatalogentry.SourceIDEQ(id)).Exec(ctx); err != nil {
			httpx.ServerError(c, "清理目录项失败")
			return
		}
		if err := client.ModelCatalogSource.DeleteOneID(id).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "model_catalog_source.delete", "model_catalog_source", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func SyncModelCatalogSource(d ModelCatalogDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ins, upd, err := modelcatalog.SyncSource(c.Request.Context(), d.Client, d.AESKey, id)
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		auditLog(d.Client, adm.ID, "model_catalog_source.sync", "model_catalog_source", strconv.Itoa(id), map[string]any{"inserted": ins, "updated": upd}, c)
		httpx.OK(c, gin.H{"inserted": ins, "updated": upd})
	}
}

func ListModelCatalogEntries(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 200 {
			ps = 50
		}
		ctx := c.Request.Context()
		q := client.ModelCatalogEntry.Query().WithSource()
		if qstr := strings.TrimSpace(c.Query("q")); qstr != "" {
			q = q.Where(
				modelcatalogentry.Or(
					modelcatalogentry.ModelIDContainsFold(qstr),
					modelcatalogentry.DisplayNameContainsFold(qstr),
				),
			)
		}
		if src := c.Query("source_id"); src != "" {
			if sid, err := strconv.Atoi(src); err == nil && sid > 0 {
				q = q.Where(modelcatalogentry.SourceIDEQ(sid))
			}
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		list, err := q.Order(ent.Desc(modelcatalogentry.FieldUpdatedAt)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, e := range list {
			items = append(items, gin.H{
				"id":            e.ID,
				"model_id":      e.ModelID,
				"display_name":  e.DisplayName,
				"provider_slug": e.ProviderSlug,
				"owned_by":      e.OwnedBy,
				"description":   e.Description,
				"metadata":      e.Metadata,
				"admin_labels": e.AdminLabels,
				"source_id":     e.SourceID,
				"visible":       e.Visible,
				"enabled":       e.Enabled,
				"synced_at":     e.SyncedAt,
				"updated_at":    e.UpdatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func UpdateModelCatalogEntry(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			DisplayName *string   `json:"display_name"`
			Description *string   `json:"description"`
			Visible     *bool     `json:"visible"`
			Enabled     *bool     `json:"enabled"`
			AdminLabels *[]string `json:"admin_labels"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		e, err := client.ModelCatalogEntry.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		up := client.ModelCatalogEntry.UpdateOne(e)
		if req.DisplayName != nil {
			up.SetDisplayName(strings.TrimSpace(*req.DisplayName))
		}
		if req.Description != nil {
			up.SetDescription(*req.Description)
		}
		if req.Visible != nil {
			up.SetVisible(*req.Visible)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		if req.AdminLabels != nil {
			norm, err := modelkind.NormalizeAdminLabels(*req.AdminLabels)
			if err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			if len(norm) == 0 {
				up.ClearAdminLabels()
			} else {
				up.SetAdminLabels(norm)
			}
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(client, adm.ID, "model_catalog_entry.update", "model_catalog_entry", strconv.Itoa(id), map[string]any{"body": req}, c)
		httpx.OKMsg(c, "已更新", nil)
	}
}

func DeleteModelCatalogEntry(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.ModelCatalogEntry.DeleteOneID(id).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "model_catalog_entry.delete", "model_catalog_entry", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}
