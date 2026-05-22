package adminhandler

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/adminauditlog"
	"tracehub/internal/ent/announcement"
	"tracehub/internal/ent/apikey"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/globalmodelpricing"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/predicate"
	"tracehub/internal/ent/redeemcode"
	"tracehub/internal/ent/redeemledger"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/router"
	"tracehub/internal/handler/health"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/alertconfig"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/pkg/redeemutil"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/availmodels"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

func GetUser(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		u, err := client.User.Query().Where(user.IDEQ(id)).WithGroup().Only(ctx)
		if err != nil {
			httpx.BadRequest(c, "用户不存在")
			return
		}
		cl := c.MustGet(middleware.CtxClaimsKey).(*jwtx.Claims)
		if adminElevatedRole(u.Role) && cl.Role != "superadmin" {
			httpx.Forbidden(c, "仅超级管理员可查看管理员详情")
			return
		}
		totalReq, _ := client.UsageLog.Query().Where(usagelog.UserIDEQ(id)).Count(ctx)
		gn := ""
		if u.Edges.Group != nil {
			gn = u.Edges.Group.Name
		}
		httpx.OK(c, gin.H{
			"id":                      u.ID,
			"email":                   u.Email,
			"username":                u.Username,
			"group_id":                u.GroupID,
			"group_name":              gn,
			"balance":                 u.Balance,
			"status":                  u.Status,
			"role":                    u.Role,
			"concurrency":             u.Concurrency,
			"rate_multiplier":         u.RateMultiplier,
			"balance_alert_threshold": u.BalanceAlertThreshold,
			"custom_alert_config":     alertconfig.PublicView(u.CustomAlertConfig),
			"webhook_url":             u.WebhookURL,
			"ip_whitelist":            u.IPWhitelist,
			"ip_blacklist":            u.IPBlacklist,
			"totp_enabled":            u.TotpSecret != "",
			"total_requests":          totalReq,
			"created_at":              u.CreatedAt,
			"last_login_at":           u.LastLoginAt,
		})
	}
}

func BanUser(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.User.UpdateOneID(id).SetStatus(2).Exec(ctx); err != nil {
			httpx.ServerError(c, "操作失败")
			return
		}
		auditLog(client, adm.ID, "user.ban", "user", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已禁用", nil)
	}
}

func UnbanUser(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.User.UpdateOneID(id).SetStatus(1).Exec(ctx); err != nil {
			httpx.ServerError(c, "操作失败")
			return
		}
		auditLog(client, adm.ID, "user.unban", "user", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已启用", nil)
	}
}

func DeleteUser(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		cl := c.MustGet(middleware.CtxClaimsKey).(*jwtx.Claims)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		if id == adm.ID {
			httpx.BadRequest(c, "不能删除当前登录账号")
			return
		}
		ctx := c.Request.Context()
		target, err := client.User.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "用户不存在")
			return
		}
		if target.Role == "admin" || target.Role == "superadmin" {
			if cl.Role != "superadmin" {
				httpx.Forbidden(c, "仅超级管理员可删除管理员账号")
				return
			}
		}
		tx, err := client.Tx(ctx)
		if err != nil {
			httpx.ServerError(c, "操作失败")
			return
		}
		rollback := true
		defer func() {
			if rollback {
				_ = tx.Rollback()
			}
		}()
		if _, err := tx.AdminAuditLog.Delete().Where(adminauditlog.AdminIDEQ(id)).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		if _, err := tx.APIKey.Delete().Where(apikey.UserIDEQ(id)).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		if err := tx.User.DeleteOneID(id).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		rollback = false
		auditLog(client, adm.ID, "user.delete", "user", strconv.Itoa(id), map[string]any{"email": target.Email}, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func BatchAdjustBalance(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Items []struct {
				UserID int   `json:"user_id"`
				Delta  int64 `json:"delta"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
			httpx.BadRequest(c, "items 不能为空")
			return
		}
		if len(req.Items) > 500 {
			httpx.BadRequest(c, "单次最多 500 条")
			return
		}
		ctx := c.Request.Context()
		for _, it := range req.Items {
			if it.UserID == 0 {
				continue
			}
			if err := client.User.UpdateOneID(it.UserID).AddBalance(it.Delta).Exec(ctx); err != nil {
				httpx.ServerError(c, "调整失败: "+strconv.Itoa(it.UserID))
				return
			}
		}
		auditLog(client, adm.ID, "user.batch_balance", "user", "", map[string]any{"count": len(req.Items)}, c)
		httpx.OKMsg(c, "批量调整完成", nil)
	}
}

func UpdateGroup(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Name            *string            `json:"name"`
			Description     *string            `json:"description"`
			RateMultiplier  *float64           `json:"rate_multiplier"`
			RPMLimit        *int               `json:"rpm_limit"`
			ModelRouting    map[string][]int64 `json:"model_routing"`
			RoutingStrategy *string            `json:"routing_strategy"`
			RoutingOptions  map[string]any     `json:"routing_options"`
			DefaultModel    *string            `json:"default_model"`
			Enabled         *bool              `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		g, err := client.Group.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "分组不存在")
			return
		}
		up := client.Group.UpdateOne(g)
		if req.Name != nil && *req.Name != "" {
			up.SetName(*req.Name)
		}
		if req.Description != nil {
			up.SetDescription(*req.Description)
		}
		if req.RateMultiplier != nil {
			up.SetRateMultiplier(*req.RateMultiplier)
		}
		if req.RPMLimit != nil {
			up.SetRpmLimit(*req.RPMLimit)
		}
		if req.ModelRouting != nil {
			if err := validateGroupModelRouting(ctx, client, id, req.ModelRouting); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			up.SetModelRouting(req.ModelRouting)
		}
		if req.RoutingStrategy != nil {
			if err := validateGroupRoutingStrategy(*req.RoutingStrategy); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			if *req.RoutingStrategy == "" {
				up.ClearRoutingStrategy()
			} else {
				up.SetRoutingStrategy(*req.RoutingStrategy)
			}
		}
		if req.RoutingOptions != nil {
			if err := validateGroupRoutingOptions(req.RoutingOptions); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			if len(req.RoutingOptions) == 0 {
				up.ClearRoutingOptions()
			} else {
				up.SetRoutingOptions(req.RoutingOptions)
			}
		}
		if req.DefaultModel != nil {
			up.SetDefaultModel(*req.DefaultModel)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		router.InvalidateRoutingCache(id)
		auditLog(client, adm.ID, "group.update", "group", strconv.Itoa(id), map[string]any{"body": req}, c)
		httpx.OKMsg(c, "已更新", nil)
	}
}

func DeleteGroup(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		uc, _ := client.User.Query().Where(user.GroupIDEQ(id)).Count(ctx)
		cc, _ := client.Channel.Query().Where(channel.GroupIDEQ(id)).Count(ctx)
		if uc > 0 || cc > 0 {
			httpx.BadRequest(c, "分组下仍有用户或渠道，无法删除")
			return
		}
		if err := client.Group.DeleteOneID(id).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "group.delete", "group", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func ListGroupModelRates(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		gid, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if _, err := client.Group.Get(ctx, gid); err != nil {
			httpx.BadRequest(c, "分组不存在")
			return
		}
		rates, err := client.GroupModelRate.Query().Where(groupmodelrate.GroupIDEQ(gid)).All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": rates, "total": len(rates)})
	}
}

// GetGlobalModelPricingWorkspace GET /api/admin/global-model-pricing/workspace — 合并列表（内置表 ∪ 渠道 ∪ 目录 ∪ 全局库）与代码基准。
// 查询参数：q（可选，model_name 子串，不区分大小写）、page（默认 1）、page_size（默认 50，最大 200）。
func GetGlobalModelPricingWorkspace(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		q := strings.TrimSpace(c.Query("q"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
		items, total, pageOut, err := availmodels.GlobalModelPricingWorkspacePage(ctx, client, q, page, ps)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		outPS := ps
		if outPS <= 0 {
			outPS = 50
		}
		if outPS > 200 {
			outPS = 200
		}
		httpx.OK(c, gin.H{
			"items":     items,
			"total":     total,
			"page":      pageOut,
			"page_size": outPS,
			"q":         q,
		})
	}
}

// ListGlobalModelPricing GET /api/admin/global-model-pricing — 全站模型计价（含 disabled）。
func ListGlobalModelPricing(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		list, err := client.GlobalModelPricing.Query().
			Order(ent.Asc(globalmodelpricing.FieldModelName)).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": list, "total": len(list)})
	}
}

type globalPricingUpsertItem struct {
	ModelName       string
	Enabled         bool
	QuotaInPer1k    *int64
	QuotaOutPer1k   *int64
	InputRate       float64
	OutputRate      float64
	MarkupPercent   *float64
	BillingMode     string
	QuotaPerRequest *int64
}

// upsertGlobalModelPricingBatch 执行批量 upsert。badRequest 非空返回 400；err 非空返回 500。
func upsertGlobalModelPricingBatch(ctx context.Context, client *ent.Client, items []globalPricingUpsertItem) (badRequest string, err error) {
	for _, it := range items {
		name := strings.TrimSpace(it.ModelName)
		if name == "" {
			return "model_name 不能为空", nil
		}
		if it.QuotaInPer1k != nil && *it.QuotaInPer1k < 0 {
			return "quota_in_per_1k 不能为负", nil
		}
		if it.QuotaOutPer1k != nil && *it.QuotaOutPer1k < 0 {
			return "quota_out_per_1k 不能为负", nil
		}
		inR, outR := it.InputRate, it.OutputRate
		if inR <= 0 || math.IsNaN(inR) || math.IsInf(inR, 0) {
			return "input_rate 须大于 0", nil
		}
		if outR <= 0 || math.IsNaN(outR) || math.IsInf(outR, 0) {
			return "output_rate 须大于 0", nil
		}
		if it.MarkupPercent != nil && !billing.ValidMarkupPercent(*it.MarkupPercent) {
			return "markup_percent 须为大于 -100 的有效数字", nil
		}
		bm := strings.TrimSpace(it.BillingMode)
		if bm == "" {
			bm = billing.BillingModeToken
		}
		switch bm {
		case billing.BillingModeToken, billing.BillingModePerRequest, billing.BillingModeHybridFloor, billing.BillingModeHybridSettle:
		default:
			return "billing_mode 无效（token|per_request|hybrid_floor|hybrid_settle）", nil
		}
		if bm != billing.BillingModeToken {
			qpr := int64(0)
			if it.QuotaPerRequest != nil {
				qpr = *it.QuotaPerRequest
			}
			if qpr <= 0 {
				return "按次/混合模式须设置 quota_per_request > 0", nil
			}
		}
		if it.QuotaPerRequest != nil && *it.QuotaPerRequest < 0 {
			return "quota_per_request 不能为负", nil
		}
		existing, qerr := client.GlobalModelPricing.Query().
			Where(globalmodelpricing.ModelNameEQ(name)).
			Only(ctx)
		if ent.IsNotFound(qerr) {
			cr := client.GlobalModelPricing.Create().
				SetModelName(name).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR).
				SetBillingMode(bm)
			if it.QuotaInPer1k != nil {
				cr.SetQuotaInPer1k(*it.QuotaInPer1k)
			}
			if it.QuotaOutPer1k != nil {
				cr.SetQuotaOutPer1k(*it.QuotaOutPer1k)
			}
			if it.MarkupPercent != nil {
				cr.SetMarkupPercent(*it.MarkupPercent)
			}
			if it.QuotaPerRequest != nil {
				cr.SetQuotaPerRequest(*it.QuotaPerRequest)
			}
			if _, err := cr.Save(ctx); err != nil {
				return "", err
			}
		} else if qerr == nil {
			up := client.GlobalModelPricing.UpdateOne(existing).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR).
				SetBillingMode(bm)
			if it.QuotaInPer1k != nil {
				up.SetQuotaInPer1k(*it.QuotaInPer1k)
			} else {
				up.ClearQuotaInPer1k()
			}
			if it.QuotaOutPer1k != nil {
				up.SetQuotaOutPer1k(*it.QuotaOutPer1k)
			} else {
				up.ClearQuotaOutPer1k()
			}
			if it.MarkupPercent != nil {
				up.SetMarkupPercent(*it.MarkupPercent)
			} else {
				up.ClearMarkupPercent()
			}
			if it.QuotaPerRequest != nil {
				up.SetQuotaPerRequest(*it.QuotaPerRequest)
			} else {
				up.ClearQuotaPerRequest()
			}
			if err := up.Exec(ctx); err != nil {
				return "", err
			}
		} else {
			return "", qerr
		}
	}
	return "", nil
}

// PutGlobalModelPricing PUT /api/admin/global-model-pricing — 按 model_name 批量 upsert。
func PutGlobalModelPricing(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Items []struct {
				ModelName       string   `json:"model_name"`
				Enabled         bool     `json:"enabled"`
				QuotaInPer1k    *int64   `json:"quota_in_per_1k"`
				QuotaOutPer1k   *int64   `json:"quota_out_per_1k"`
				InputRate       float64  `json:"input_rate"`
				OutputRate      float64  `json:"output_rate"`
				MarkupPercent   *float64 `json:"markup_percent"`
				BillingMode     string   `json:"billing_mode"`
				QuotaPerRequest *int64   `json:"quota_per_request"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		items := make([]globalPricingUpsertItem, 0, len(req.Items))
		for _, it := range req.Items {
			items = append(items, globalPricingUpsertItem{
				ModelName:       it.ModelName,
				Enabled:         it.Enabled,
				QuotaInPer1k:    it.QuotaInPer1k,
				QuotaOutPer1k:   it.QuotaOutPer1k,
				InputRate:       it.InputRate,
				OutputRate:      it.OutputRate,
				MarkupPercent:   it.MarkupPercent,
				BillingMode:     it.BillingMode,
				QuotaPerRequest: it.QuotaPerRequest,
			})
		}
		if br, err := upsertGlobalModelPricingBatch(ctx, client, items); br != "" {
			httpx.BadRequest(c, br)
			return
		} else if err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		billing.InvalidateGlobalModelPricingCacheAdmin()
		auditLog(client, adm.ID, "global_model_pricing.put", "global_model_pricing", "", map[string]any{"count": len(req.Items)}, c)
		httpx.OKMsg(c, "已保存", nil)
	}
}

// DeleteGlobalModelPricing DELETE /api/admin/global-model-pricing?model_name= — 删除一条全站模型计价配置。
func DeleteGlobalModelPricing(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		name := strings.TrimSpace(c.Query("model_name"))
		if name == "" {
			httpx.BadRequest(c, "缺少 model_name")
			return
		}
		ctx := c.Request.Context()
		n, err := client.GlobalModelPricing.Delete().Where(globalmodelpricing.ModelNameEQ(name)).Exec(ctx)
		if err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		if n == 0 {
			httpx.BadRequest(c, "记录不存在")
			return
		}
		billing.InvalidateGlobalModelPricingCacheAdmin()
		auditLog(client, adm.ID, "global_model_pricing.delete", "global_model_pricing", name, nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func adminSitePricingFactors(ctx context.Context, client *ent.Client, cfg *config.Config) billing.SitePricingFactors {
	cp := sitecfg.EffectiveCurrencyPricing(ctx, client, cfg)
	return billing.SitePricingFactors{
		QuotaPerYuan: cp.QuotaPerYuan,
		QuotaPerUsd:  cp.QuotaPerUsd,
		CnyPerUsd:    cp.CnyPerUsd,
	}
}

// PostDeriveGlobalModelPricingQuota POST /api/admin/global-model-pricing/derive-quota — 由上游人民币/美元标价换算 quota_*_per_1k（与站点 site.currency_quota_per_yuan 等配置一致）。
func PostDeriveGlobalModelPricingQuota(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil {
			httpx.ServerError(c, "配置无效")
			return
		}
		var req struct {
			Currency          string   `json:"currency"`
			DenominatorTokens int64    `json:"denominator_tokens"`
			PriceIn           float64  `json:"price_in"`
			PriceOut          *float64 `json:"price_out"`
			MarginPercent     float64  `json:"margin_percent"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		cur := strings.TrimSpace(strings.ToLower(req.Currency))
		po := 0.0
		if req.PriceOut != nil {
			po = *req.PriceOut
		}
		factors := adminSitePricingFactors(c.Request.Context(), client, cfg)
		qIn, qOut, ri, ro, err := billing.DeriveGlobalQuotaPer1KPair(factors, cur, req.DenominatorTokens, req.PriceIn, po, req.MarginPercent)
		if err != nil {
			httpx.BadRequest(c, mapGlobalPricingDeriveError(err))
			return
		}
		httpx.OK(c, gin.H{
			"quota_in_per_1k":  qIn,
			"quota_out_per_1k": qOut,
			"resolved": gin.H{
				"input":    ri,
				"output":   ro,
				"rounding": "ceil_to_int64",
			},
		})
	}
}

func mapGlobalPricingDeriveError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	switch {
	case strings.Contains(s, "currency_quota_per_yuan must be > 0"):
		return "站点 currency_quota_per_yuan 须大于 0（人民币标价或美元经汇率换算时需要）"
	case strings.Contains(s, "for USD upstream prices"):
		return "美元标价须配置 currency_quota_per_usd > 0，或同时配置 pricing_cny_per_usd 与 currency_quota_per_yuan"
	case strings.Contains(s, "denom_tokens"):
		return "denominator_tokens 须为 1、1000 或 1000000"
	case strings.Contains(s, "currency must be cny or usd"):
		return "currency 须为 cny 或 usd"
	case strings.Contains(s, "price must be"):
		return "price 须为非负有限数字"
	case strings.Contains(s, "margin_percent must be greater than -100"):
		return "margin_percent 须大于 -100"
	case strings.Contains(s, "margin_percent must be finite"):
		return "margin_percent 无效"
	default:
		return s
	}
}

func normGlobalPricingCSVHeader(s string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "\ufeff"))
}

func parseGlobalPricingCSVHeaderIndex(header []string) map[string]int {
	idx := make(map[string]int)
	for i, h := range header {
		key := normGlobalPricingCSVHeader(h)
		if key != "" {
			idx[key] = i
		}
	}
	return idx
}

func parseGlobalPricingEnabledCell(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "1", "true", "yes", "y", "t", "是", "启用", "on":
		return true, nil
	case "0", "false", "no", "n", "f", "否", "禁用", "off":
		return false, nil
	default:
		return false, fmt.Errorf("enabled 须为 true/false、1/0、是/否 等，当前为 %q", s)
	}
}

func parseGlobalPricingOptionalFloat64Cell(s string) (*float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("须为数字: %q", s)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil, fmt.Errorf("须为有限数字: %q", s)
	}
	return &n, nil
}

func parseGlobalPricingOptionalInt64Cell(s string) (*int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("须为整数: %q", s)
	}
	if n < 0 {
		return nil, fmt.Errorf("不能为负: %d", n)
	}
	return &n, nil
}

func globalPricingInt64Ptr(n int64) *int64 {
	p := n
	return &p
}

// applyUpstreamQuotaDerivationCSV 当 upstream_currency 非空时，在未手填的 quota 侧用站点配置换算（额度为成本基数，加成固定为 0）；
// upstream_margin_percent 解析为行级 markup_percent（不写入额度换算）。双侧均已手填时报错。
func applyUpstreamQuotaDerivationCSV(ctx context.Context, client *ent.Client, line int, get func(string) string, cfg *config.Config, qInCell, qOutCell *int64) (qIn, qOut *int64, rowMarkup *float64, errMsg string) {
	qIn, qOut = qInCell, qOutCell
	if cfg == nil {
		return qIn, qOut, nil, ""
	}
	cur := strings.TrimSpace(strings.ToLower(get("upstream_currency")))
	if cur == "" {
		has := strings.TrimSpace(get("upstream_denominator_tokens")) != "" ||
			strings.TrimSpace(get("upstream_price_in")) != "" ||
			strings.TrimSpace(get("upstream_price_out")) != "" ||
			strings.TrimSpace(get("upstream_margin_percent")) != ""
		if has {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: 填写了上游单价列但未填 upstream_currency", line)
		}
		return qIn, qOut, nil, ""
	}
	if qIn != nil && qOut != nil {
		return nil, nil, nil, fmt.Sprintf("第 %d 行: 已填写两侧 quota_*_per_1k 时不应再填 upstream_currency", line)
	}

	denomStr := strings.TrimSpace(get("upstream_denominator_tokens"))
	if denomStr == "" {
		return nil, nil, nil, fmt.Sprintf("第 %d 行: 使用上游换算时须填 upstream_denominator_tokens（1、1000 或 1000000）", line)
	}
	denom, err := strconv.ParseInt(denomStr, 10, 64)
	if err != nil || (denom != 1 && denom != 1000 && denom != 1_000_000) {
		return nil, nil, nil, fmt.Sprintf("第 %d 行: upstream_denominator_tokens 须为 1、1000 或 1000000", line)
	}
	pinS := strings.TrimSpace(get("upstream_price_in"))
	if pinS == "" {
		return nil, nil, nil, fmt.Sprintf("第 %d 行: 使用上游换算时须填 upstream_price_in", line)
	}
	pin, err := strconv.ParseFloat(pinS, 64)
	if err != nil || math.IsNaN(pin) || math.IsInf(pin, 0) || pin < 0 {
		return nil, nil, nil, fmt.Sprintf("第 %d 行: upstream_price_in 须为非负有限数字", line)
	}
	poutS := strings.TrimSpace(get("upstream_price_out"))
	var pout float64
	if poutS != "" {
		pout, err = strconv.ParseFloat(poutS, 64)
		if err != nil || math.IsNaN(pout) || math.IsInf(pout, 0) || pout < 0 {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: upstream_price_out 须为非负有限数字", line)
		}
	}

	if ms := strings.TrimSpace(get("upstream_margin_percent")); ms != "" {
		m, perr := strconv.ParseFloat(ms, 64)
		if perr != nil || math.IsNaN(m) || math.IsInf(m, 0) {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: upstream_margin_percent 无效", line)
		}
		if !billing.ValidMarkupPercent(m) {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: upstream_margin_percent 须大于 -100", line)
		}
		rowMarkup = &m
	}

	factors := adminSitePricingFactors(ctx, client, cfg)
	if factors.QuotaPerYuan <= 0 {
		return nil, nil, nil, "站点 currency_quota_per_yuan 须大于 0，方能从上游单价换算额度"
	}

	if qIn == nil {
		di, _, err := billing.QuotaPer1KFromUpstreamPrice(factors, cur, denom, pin, 0)
		if err != nil {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: %s", line, mapGlobalPricingDeriveError(err))
		}
		qIn = globalPricingInt64Ptr(di)
	}
	if qOut == nil {
		pForOut := pout
		if pForOut <= 0 {
			pForOut = pin
		}
		do, _, err := billing.QuotaPer1KFromUpstreamPrice(factors, cur, denom, pForOut, 0)
		if err != nil {
			return nil, nil, nil, fmt.Sprintf("第 %d 行: %s", line, mapGlobalPricingDeriveError(err))
		}
		qOut = globalPricingInt64Ptr(do)
	}
	return qIn, qOut, rowMarkup, ""
}

func globalPricingMarkupCSVCell(g *ent.GlobalModelPricing) string {
	if g == nil || g.MarkupPercent == nil {
		return ""
	}
	return strconv.FormatFloat(*g.MarkupPercent, 'f', -1, 64)
}

func capabilityHintForModel(modelName string) string {
	for _, l := range modelkind.LabelsForModel(modelName, nil, nil) {
		if l == "video" {
			return "video"
		}
	}
	return ""
}

func globalPricingBillingCSVCells(billingMode string, quotaPerRequest *int64) (bm, qpr string) {
	bm = strings.TrimSpace(billingMode)
	if bm == "" {
		bm = billing.BillingModeToken
	}
	if quotaPerRequest != nil {
		qpr = strconv.FormatInt(*quotaPerRequest, 10)
	}
	return bm, qpr
}

func appendGlobalPricingImportTemplateExtras(cells []string, modelName, billingMode string, quotaPerRequest *int64) []string {
	bm, qpr := globalPricingBillingCSVCells(billingMode, quotaPerRequest)
	return append(cells, bm, qpr, capabilityHintForModel(modelName))
}

func parseGlobalPricingOptionalBillingCSV(line int, col map[string]int, get func(string) string) (bm string, qpr *int64, errMsg string) {
	bm = billing.BillingModeToken
	if _, ok := col["billing_mode"]; ok {
		s := strings.TrimSpace(strings.ToLower(get("billing_mode")))
		if s != "" {
			switch s {
			case billing.BillingModeToken, billing.BillingModePerRequest, billing.BillingModeHybridFloor, billing.BillingModeHybridSettle:
				bm = s
			default:
				return "", nil, fmt.Sprintf("第 %d 行: billing_mode 无效（token|per_request|hybrid_floor|hybrid_settle）", line)
			}
		}
	}
	if _, ok := col["quota_per_request"]; ok {
		var err error
		qpr, err = parseGlobalPricingOptionalInt64Cell(get("quota_per_request"))
		if err != nil {
			return "", nil, fmt.Sprintf("第 %d 行 quota_per_request: %v", line, err)
		}
	}
	return bm, qpr, ""
}

var (
	globalPricingDataExportHeader = []string{
		"model_name", "enabled", "quota_in_per_1k", "quota_out_per_1k", "input_rate", "output_rate", "markup_percent",
	}
	globalPricingImportTemplateHeader = []string{
		"model_name", "enabled", "quota_in_per_1k", "quota_out_per_1k", "input_rate", "output_rate",
		"upstream_currency", "upstream_denominator_tokens", "upstream_price_in", "upstream_price_out", "upstream_margin_percent",
		"billing_mode", "quota_per_request", "capability_hint",
	}
)

func writeGlobalPricingCSVBOM(w io.Writer) {
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
}

func globalPricingDenomFilenameSuffix(denom int64) string {
	switch denom {
	case 1:
		return "per-1"
	case 1000:
		return "per-1k"
	case 1_000_000:
		return "per-1m"
	default:
		return fmt.Sprintf("per-%d", denom)
	}
}

func formatGlobalPricingUpstreamCSV(price float64) string {
	if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return ""
	}
	abs := math.Abs(price)
	switch {
	case abs >= 100:
		return strconv.FormatFloat(price, 'f', 2, 64)
	case abs >= 1:
		return strconv.FormatFloat(price, 'f', 4, 64)
	case abs >= 0.0001:
		return strconv.FormatFloat(price, 'f', 6, 64)
	default:
		return strconv.FormatFloat(price, 'g', 3, 64)
	}
}

func upstreamPriceCellFromQuota(factors billing.SitePricingFactors, currency string, denom int64, quota *int64) string {
	if quota == nil {
		return ""
	}
	p, err := billing.UpstreamPriceFromQuotaPer1K(factors, currency, denom, *quota, 0)
	if err != nil {
		return ""
	}
	return formatGlobalPricingUpstreamCSV(p)
}

func parseGlobalPricingImportTemplateQuery(c *gin.Context) (currency string, denom int64, errMsg string) {
	cur := strings.ToLower(strings.TrimSpace(c.Query("currency")))
	switch cur {
	case "cny", "rmb":
		currency = "cny"
	case "usd":
		currency = "usd"
	default:
		return "", 0, "currency 须为 cny 或 usd"
	}
	denomStr := strings.TrimSpace(c.Query("denom"))
	if denomStr == "" {
		denomStr = strings.TrimSpace(c.Query("denominator_tokens"))
	}
	if denomStr == "" {
		return "", 0, "denom 须为 1、1000 或 1000000"
	}
	d, err := strconv.ParseInt(denomStr, 10, 64)
	if err != nil || (d != 1 && d != 1000 && d != 1_000_000) {
		return "", 0, "denom 须为 1、1000 或 1000000"
	}
	return currency, d, ""
}

// parseGlobalPricingExportQuery 解析导出 Query：未传 currency/denom 时返回空 currency（仅额度列）；否则与导入模板相同。
func parseGlobalPricingExportQuery(c *gin.Context) (currency string, denom int64, errMsg string) {
	cur := strings.ToLower(strings.TrimSpace(c.Query("currency")))
	denomStr := strings.TrimSpace(c.Query("denom"))
	if denomStr == "" {
		denomStr = strings.TrimSpace(c.Query("denominator_tokens"))
	}
	if cur == "" && denomStr == "" {
		return "", 0, ""
	}
	return parseGlobalPricingImportTemplateQuery(c)
}

// ExportGlobalModelPricingCSV GET /api/admin/global-model-pricing/export — UTF-8 CSV。
// 无 currency/denom：仅可回导的数据列；有 currency/denom：含 upstream_* 且预填额度与反算上游价（与导入模板列一致）。
func ExportGlobalModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currency, denom, errMsg := parseGlobalPricingExportQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		withUpstream := currency != ""
		ctx := c.Request.Context()
		list, err := availmodels.AllGlobalPricingWorkspaceItems(ctx, client)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		ts := time.Now().Format("20060102-150405")
		var fn string
		if withUpstream {
			fn = fmt.Sprintf("global-model-pricing-data-%s-%s-%s.csv",
				currency, globalPricingDenomFilenameSuffix(denom), ts)
		} else {
			fn = fmt.Sprintf("global-model-pricing-data-%s.csv", ts)
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		var factors billing.SitePricingFactors
		if withUpstream && cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		if withUpstream {
			denomStr := strconv.FormatInt(denom, 10)
			_ = w.Write(globalPricingImportTemplateHeader)
			for _, it := range list {
				g := it.Global
				enabled := true
				qIn, qOut := "", ""
				inR, outR := 1.0, 1.0
				var qInPtr, qOutPtr *int64
				if g != nil {
					enabled = g.Enabled
					qInPtr, qOutPtr = g.QuotaInPer1k, g.QuotaOutPer1k
					if qInPtr != nil {
						qIn = strconv.FormatInt(*qInPtr, 10)
					}
					if qOutPtr != nil {
						qOut = strconv.FormatInt(*qOutPtr, 10)
					}
					inR, outR = g.InputRate, g.OutputRate
				}
				pIn := upstreamPriceCellFromQuota(factors, currency, denom, qInPtr)
				pOut := upstreamPriceCellFromQuota(factors, currency, denom, qOutPtr)
				if pOut == "" && pIn != "" {
					pOut = pIn
				}
				bm, qpr := "", (*int64)(nil)
				if g != nil {
					bm, qpr = g.BillingMode, g.QuotaPerRequest
				}
				row := appendGlobalPricingImportTemplateExtras([]string{
					it.ModelName,
					strconv.FormatBool(enabled),
					qIn,
					qOut,
					strconv.FormatFloat(inR, 'f', -1, 64),
					strconv.FormatFloat(outR, 'f', -1, 64),
					currency,
					denomStr,
					pIn,
					pOut,
					globalPricingMarkupCSVCell(g),
				}, it.ModelName, bm, qpr)
				_ = w.Write(row)
			}
		} else {
			_ = w.Write(globalPricingDataExportHeader)
			for _, it := range list {
				g := it.Global
				enabled := true
				qIn, qOut := "", ""
				inR, outR := 1.0, 1.0
				if g != nil {
					enabled = g.Enabled
					if g.QuotaInPer1k != nil {
						qIn = strconv.FormatInt(*g.QuotaInPer1k, 10)
					}
					if g.QuotaOutPer1k != nil {
						qOut = strconv.FormatInt(*g.QuotaOutPer1k, 10)
					}
					inR, outR = g.InputRate, g.OutputRate
				}
				_ = w.Write([]string{
					it.ModelName,
					strconv.FormatBool(enabled),
					qIn,
					qOut,
					strconv.FormatFloat(inR, 'f', -1, 64),
					strconv.FormatFloat(outR, 'f', -1, 64),
					globalPricingMarkupCSVCell(g),
				})
			}
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		audit := map[string]any{"rows": len(list)}
		if withUpstream {
			audit["currency"] = currency
			audit["denom"] = denom
		}
		auditLog(client, adm.ID, "global_model_pricing.export_csv", "global_model_pricing", "", audit, c)
	}
}

// ExportGlobalModelPricingImportTemplate GET /api/admin/global-model-pricing/export/template?currency=cny|usd&denom=1|1000|1000000
// 按币种与标价粒度导出导入模板：预填 upstream_currency/denominator；额度列留空；若全局库已有额度则反算填入上游价（margin 按 0）。
func ExportGlobalModelPricingImportTemplate(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currency, denom, errMsg := parseGlobalPricingImportTemplateQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		ctx := c.Request.Context()
		list, err := availmodels.AllGlobalPricingWorkspaceItems(ctx, client)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		var factors billing.SitePricingFactors
		if cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		denomStr := strconv.FormatInt(denom, 10)
		fn := fmt.Sprintf("global-model-pricing-import-%s-%s-%s.csv",
			currency, globalPricingDenomFilenameSuffix(denom), time.Now().Format("20060102-150405"))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		_ = w.Write(globalPricingImportTemplateHeader)
		for _, it := range list {
			g := it.Global
			enabled := true
			inR, outR := 1.0, 1.0
			var qInPtr, qOutPtr *int64
			if g != nil {
				enabled = g.Enabled
				qInPtr, qOutPtr = g.QuotaInPer1k, g.QuotaOutPer1k
				inR, outR = g.InputRate, g.OutputRate
			}
			pIn := upstreamPriceCellFromQuota(factors, currency, denom, qInPtr)
			pOut := upstreamPriceCellFromQuota(factors, currency, denom, qOutPtr)
			if pOut == "" && pIn != "" {
				pOut = pIn
			}
			bm, qpr := "", (*int64)(nil)
			if g != nil {
				bm, qpr = g.BillingMode, g.QuotaPerRequest
			}
			row := appendGlobalPricingImportTemplateExtras([]string{
				it.ModelName,
				strconv.FormatBool(enabled),
				"", "",
				strconv.FormatFloat(inR, 'f', -1, 64),
				strconv.FormatFloat(outR, 'f', -1, 64),
				currency,
				denomStr,
				pIn,
				pOut,
				globalPricingMarkupCSVCell(g),
			}, it.ModelName, bm, qpr)
			_ = w.Write(row)
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		auditLog(client, adm.ID, "global_model_pricing.export_import_template", "global_model_pricing", "", map[string]any{
			"rows": len(list), "currency": currency, "denom": denom,
		}, c)
	}
}

// readGlobalPricingUpload reads multipart field "file" (max 8MB). errMsg is non-empty on failure.
func readGlobalPricingUpload(c *gin.Context) ([]byte, string) {
	fh, err := c.FormFile("file")
	if err != nil {
		return nil, "请上传 file 字段（CSV）"
	}
	if fh.Size > 8<<20 {
		return nil, "文件过大（最大 8MB）"
	}
	src, err := fh.Open()
	if err != nil {
		return nil, "无法读取文件"
	}
	defer src.Close()
	body, err := io.ReadAll(src)
	if err != nil {
		return nil, "读取失败"
	}
	return body, ""
}

// parseGlobalPricingCSVBytes parses CSV (UTF-8 BOM optional). errMsg is non-empty on validation/parse failure (Chinese).
// cfg 非空时支持可选列 upstream_* 从上游标价换算额度（与 derive-quota 规则一致）。
func parseGlobalPricingCSVBytes(ctx context.Context, client *ent.Client, body []byte, cfg *config.Config) ([]globalPricingUpsertItem, string) {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, "CSV 表头无效"
	}
	col := parseGlobalPricingCSVHeaderIndex(header)
	reqCols := []string{"model_name", "enabled", "input_rate", "output_rate", "quota_in_per_1k", "quota_out_per_1k"}
	for _, k := range reqCols {
		if _, ok := col[k]; !ok {
			return nil, "缺少列 " + k + "（请使用「导出」生成的表头；额度列可留空表示清空覆盖）"
		}
	}
	var items []globalPricingUpsertItem
	line := 1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行解析失败", line+1)
		}
		line++
		if line > 5001 {
			return nil, "超过 5000 行数据上限"
		}
		get := func(key string) string {
			i, ok := col[key]
			if !ok || i >= len(rec) {
				return ""
			}
			return rec[i]
		}
		name := strings.TrimSpace(get("model_name"))
		if name == "" {
			continue
		}
		en, err := parseGlobalPricingEnabledCell(get("enabled"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行: %v", line, err)
		}
		inS := strings.TrimSpace(get("input_rate"))
		outS := strings.TrimSpace(get("output_rate"))
		inR, err := strconv.ParseFloat(inS, 64)
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 input_rate 无效", line)
		}
		outR, err := strconv.ParseFloat(outS, 64)
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 output_rate 无效", line)
		}
		var qIn, qOut *int64
		vIn, err := parseGlobalPricingOptionalInt64Cell(get("quota_in_per_1k"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 quota_in_per_1k: %v", line, err)
		}
		vOut, err := parseGlobalPricingOptionalInt64Cell(get("quota_out_per_1k"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 quota_out_per_1k: %v", line, err)
		}
		qIn, qOut, rowMarkup, upErr := applyUpstreamQuotaDerivationCSV(ctx, client, line, get, cfg, vIn, vOut)
		if upErr != "" {
			return nil, upErr
		}
		var markup *float64
		if _, ok := col["markup_percent"]; ok {
			markup, err = parseGlobalPricingOptionalFloat64Cell(get("markup_percent"))
			if err != nil {
				return nil, fmt.Sprintf("第 %d 行 markup_percent: %v", line, err)
			}
			if markup != nil && !billing.ValidMarkupPercent(*markup) {
				return nil, fmt.Sprintf("第 %d 行: markup_percent 须大于 -100", line)
			}
		}
		if rowMarkup != nil {
			markup = rowMarkup
		}
		bm, qpr, billErr := parseGlobalPricingOptionalBillingCSV(line, col, get)
		if billErr != "" {
			return nil, billErr
		}
		items = append(items, globalPricingUpsertItem{
			ModelName:       name,
			Enabled:         en,
			QuotaInPer1k:    qIn,
			QuotaOutPer1k:   qOut,
			InputRate:       inR,
			OutputRate:      outR,
			MarkupPercent:   markup,
			BillingMode:     bm,
			QuotaPerRequest: qpr,
		})
	}
	if len(items) == 0 {
		return nil, "没有有效数据行"
	}
	return items, ""
}

// PreviewGlobalModelPricingCSV POST /api/admin/global-model-pricing/import/preview — 仅解析校验，不落库。
func PreviewGlobalModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, errMsg := readGlobalPricingUpload(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		items, perr := parseGlobalPricingCSVBytes(c.Request.Context(), client, body, cfg)
		if perr != "" {
			httpx.BadRequest(c, perr)
			return
		}
		type itemOut struct {
			ModelName     string   `json:"model_name"`
			Enabled       bool     `json:"enabled"`
			InputRate     float64  `json:"input_rate"`
			OutputRate    float64  `json:"output_rate"`
			QuotaInPer1k  *int64   `json:"quota_in_per_1k,omitempty"`
			QuotaOutPer1k *int64   `json:"quota_out_per_1k,omitempty"`
			MarkupPercent *float64 `json:"markup_percent,omitempty"`
		}
		out := make([]itemOut, len(items))
		for i := range items {
			it := items[i]
			out[i] = itemOut{
				ModelName:     it.ModelName,
				Enabled:       it.Enabled,
				InputRate:     it.InputRate,
				OutputRate:    it.OutputRate,
				QuotaInPer1k:  it.QuotaInPer1k,
				QuotaOutPer1k: it.QuotaOutPer1k,
				MarkupPercent: it.MarkupPercent,
			}
		}
		httpx.OK(c, gin.H{"count": len(out), "items": out})
	}
}

// ImportGlobalModelPricingCSV POST /api/admin/global-model-pricing/import — multipart 字段 `file`，UTF-8 CSV；表头须含 model_name、enabled、input_rate、output_rate、quota_in_per_1k、quota_out_per_1k（额度列可留空表示清空覆盖）。最多 5000 行数据。
func ImportGlobalModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		body, errMsg := readGlobalPricingUpload(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		ctx := c.Request.Context()
		items, perr := parseGlobalPricingCSVBytes(ctx, client, body, cfg)
		if perr != "" {
			httpx.BadRequest(c, perr)
			return
		}
		if br, err := upsertGlobalModelPricingBatch(ctx, client, items); br != "" {
			httpx.BadRequest(c, br)
			return
		} else if err != nil {
			httpx.ServerError(c, "导入保存失败")
			return
		}
		billing.InvalidateGlobalModelPricingCacheAdmin()
		auditLog(client, adm.ID, "global_model_pricing.import_csv", "global_model_pricing", "", map[string]any{"count": len(items)}, c)
		httpx.OK(c, gin.H{"count": len(items)})
	}
}

func ListRedeemCodes(client *ent.Client) gin.HandlerFunc {
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
		q := client.RedeemCode.Query()
		if st := c.Query("status"); st != "" {
			if v, err := strconv.Atoi(st); err == nil {
				q = q.Where(redeemcode.StatusEQ(v))
			}
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		list, err := q.Order(ent.Desc(redeemcode.FieldCreatedAt)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, rc := range list {
			items = append(items, gin.H{
				"id":          rc.ID,
				"code_hint":   redeemutil.CodeHint(rc.Code),
				"code_type":   rc.CodeType,
				"amount":      rc.Amount,
				"status":      rc.Status,
				"max_uses":    rc.MaxUses,
				"used_count":  rc.UsedCount,
				"valid_until": rc.ValidUntil,
				"created_at":  rc.CreatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

// ListRedeemLedger 管理员查看用户兑换入账流水（可选 user_id、redeem_code_id、redeem_code 筛选，含用户邮箱便于对账）。
// redeem_code_match：exact（默认，整码精确）、prefix（前缀）、contains（子串，大小写不敏感）。
func ListRedeemLedger(client *ent.Client) gin.HandlerFunc {
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
		q := client.RedeemLedger.Query()
		if uid := strings.TrimSpace(c.Query("user_id")); uid != "" {
			if id, err := strconv.Atoi(uid); err == nil && id > 0 {
				q = q.Where(redeemledger.UserIDEQ(id))
			}
		}
		if ridStr := strings.TrimSpace(c.Query("redeem_code_id")); ridStr != "" {
			if rid, err := strconv.Atoi(ridStr); err == nil && rid > 0 {
				q = q.Where(redeemledger.RedeemCodeIDEQ(rid))
			}
		}
		if codeStr := strings.TrimSpace(c.Query("redeem_code")); codeStr != "" {
			mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("redeem_code_match", "exact")))
			var codePred predicate.RedeemCode
			switch mode {
			case "exact":
				codePred = redeemutil.CodeMatchPredicate(codeStr)
			case "prefix":
				codePred = redeemutil.CodePrefixPredicate(codeStr)
			case "contains":
				codePred = redeemcode.CodeContainsFold(codeStr)
			default:
				httpx.BadRequest(c, "redeem_code_match 无效，可选 exact、prefix、contains")
				return
			}
			q = q.Where(redeemledger.HasRedeemCodeWith(codePred))
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		list, err := q.WithUser().
			Order(ent.Desc(redeemledger.FieldCreatedAt)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, row := range list {
			email := ""
			if row.Edges.User != nil {
				email = row.Edges.User.Email
			}
			items = append(items, gin.H{
				"id":             row.ID,
				"user_id":        row.UserID,
				"user_email":     email,
				"redeem_code_id": row.RedeemCodeID,
				"amount":         row.Amount,
				"balance_after":  row.BalanceAfter,
				"code_hint":      row.CodeHint,
				"created_at":     row.CreatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func RevokeRedeemCode(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.RedeemCode.UpdateOneID(id).SetStatus(0).Exec(ctx); err != nil {
			httpx.ServerError(c, "作废失败")
			return
		}
		auditLog(client, adm.ID, "redeem.revoke", "redeem", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已作废", nil)
	}
}

func ExportLogs(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		format := c.DefaultQuery("format", "csv")
		ctx := c.Request.Context()
		q := client.UsageLog.Query()
		if uid := c.Query("user_id"); uid != "" {
			id, _ := strconv.Atoi(uid)
			q = q.Where(usagelog.UserIDEQ(id))
		}
		if model := c.Query("model"); model != "" {
			q = q.Where(usagelog.ModelEQ(model))
		}
		if ch := c.Query("channel_id"); ch != "" {
			id, _ := strconv.Atoi(ch)
			q = q.Where(usagelog.ChannelIDEQ(id))
		}
		if st := strings.TrimSpace(c.Query("start_time")); st != "" {
			if t, err := time.Parse(time.RFC3339, st); err == nil {
				q = q.Where(usagelog.RequestTimeGTE(t))
			}
		}
		if et := strings.TrimSpace(c.Query("end_time")); et != "" {
			if t, err := time.Parse(time.RFC3339, et); err == nil {
				q = q.Where(usagelog.RequestTimeLTE(t))
			}
		}
		logs, err := q.Order(ent.Desc(usagelog.FieldRequestTime)).Limit(10000).All(ctx)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		if format == "json" {
			c.JSON(200, logs)
			return
		}
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=admin-logs.csv")
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"id", "user_id", "model", "channel_id", "billing_mode", "usage", "prompt_tokens", "completion_tokens", "total_tokens", "cost", "status", "time"})
		for _, l := range logs {
			ch := ""
			if l.ChannelID != 0 {
				ch = strconv.Itoa(l.ChannelID)
			}
			uid := ""
			if l.UserID != 0 {
				uid = strconv.Itoa(l.UserID)
			}
			sc := strconv.Itoa(l.StatusCode)
			mode := modelkind.UsageBillingMode(l.Model, l.PromptTokens, l.CompletionTokens)
			usage := strconv.Itoa(l.PromptTokens) + " in / " + strconv.Itoa(l.CompletionTokens) + " out"
			if mode == modelkind.UsageBillingVideoDuration {
				usage = strconv.Itoa(l.PromptTokens) + "s"
			}
			_ = w.Write([]string{
				strconv.Itoa(l.ID),
				uid,
				l.Model,
				ch,
				mode,
				usage,
				strconv.Itoa(l.PromptTokens),
				strconv.Itoa(l.CompletionTokens),
				strconv.Itoa(l.TotalTokens),
				strconv.FormatInt(l.TotalCost, 10),
				sc,
				l.RequestTime.Format(time.RFC3339),
			})
		}
		w.Flush()
	}
}

func LogStats(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		since := time.Now().Add(-7 * 24 * time.Hour)
		if p := c.DefaultQuery("period", "7d"); p == "30d" {
			since = time.Now().Add(-30 * 24 * time.Hour)
		}
		ctx := c.Request.Context()
		q := client.UsageLog.Query().Where(usagelog.RequestTimeGTE(since))
		if uid := c.Query("user_id"); uid != "" {
			id, _ := strconv.Atoi(uid)
			q = q.Where(usagelog.UserIDEQ(id))
		}
		logs, err := q.Limit(50000).All(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		byModel := map[string]gin.H{}
		var totalReq int
		var totalTok int64
		var totalCost int64
		for _, l := range logs {
			totalReq++
			totalTok += int64(l.TotalTokens)
			totalCost += l.TotalCost
			m := l.Model
			cur, ok := byModel[m]
			if !ok {
				cur = gin.H{"model": m, "requests": 0, "tokens": int64(0), "cost": int64(0)}
			}
			cur["requests"] = cur["requests"].(int) + 1
			cur["tokens"] = cur["tokens"].(int64) + int64(l.TotalTokens)
			cur["cost"] = cur["cost"].(int64) + l.TotalCost
			byModel[m] = cur
		}
		var modelRows []gin.H
		for _, v := range byModel {
			modelRows = append(modelRows, v)
		}
		httpx.OK(c, gin.H{
			"total_requests": totalReq,
			"total_tokens":   totalTok,
			"total_cost":     totalCost,
			"by_model":       modelRows,
		})
	}
}

func GetSystemConfig(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		rows, err := client.SystemConfig.Query().All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "读取失败")
			return
		}
		var items []gin.H
		for _, r := range rows {
			items = append(items, gin.H{
				"key":         r.ConfigKey,
				"value":       r.ConfigValue,
				"description": r.Description,
				"updated_at":  r.UpdatedAt,
			})
		}
		httpx.OK(c, gin.H{"items": items})
	}
}

// GetAdminSiteSettings GET /api/admin/site-settings — 结构化站点配置（供系统配置页）；不含密钥。
func GetAdminSiteSettings(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		dbSet, dbVal := sitecfg.LoginCaptchaDBMeta(ctx, client)
		dbSetBal, dbValBal := sitecfg.DefaultNewUserBalanceDBMeta(ctx, client)
		dbSetYuan, dbValYuan := sitecfg.CurrencyQuotaPerYuanDBMeta(ctx, client)
		dbSetUsd, dbValUsd := sitecfg.CurrencyQuotaPerUsdDBMeta(ctx, client)
		dbSetRate, dbValRate := sitecfg.PricingCnyPerUsdDBMeta(ctx, client)
		dbSetMarkup, dbValMarkup := sitecfg.DefaultPricingMarkupPercentDBMeta(ctx, client)
		cp := sitecfg.EffectiveCurrencyPricing(ctx, client, cfg)
		dbSetPromo, dbValPromo := sitecfg.RegisterPromoTextDBMeta(ctx, client)
		dbSetBeian, dbValBeian := sitecfg.BeianFooterTextDBMeta(ctx, client)
		httpsPolicy := sitecfg.CachedHTTPSPolicy(ctx, client, cfg)
		dbSetHTTPSFR, dbValHTTPSFR := sitecfg.HTTPSForceRedirectDBMeta(ctx, client)
		dbSetHTTPSHSTS, dbValHTTPSHSTS := sitecfg.HTTPSHSTSEnabledDBMeta(ctx, client)
		dbSetHTTPSMax, dbValHTTPSMax := sitecfg.HTTPSHSTSMaxAgeDBMeta(ctx, client)
		dbSetTLS, dbValTLS := sitecfg.TLSEnabledDBMeta(ctx, client)
		dbSetTLSCert, dbValTLSCert := sitecfg.TLSCertFileDBMeta(ctx, client)
		dbSetTLSKey, dbValTLSKey := sitecfg.TLSKeyFileDBMeta(ctx, client)
		tlsPaths := sitecfg.EffectiveTLSCertKeyPaths(ctx, client, cfg)
		tlsCert := tlsPaths.CertFile
		tlsKey := tlsPaths.KeyFile
		tlsAddr := strings.TrimSpace(cfg.Server.TLS.Addr)
		if tlsAddr == "" {
			tlsAddr = ":443"
		}
		tlsMat := sitecfg.EvaluateTLSMaterial(ctx, client, cfg)
		_, certErr := os.Stat(tlsCert)
		_, keyErr := os.Stat(tlsKey)
		dbSetNoBill, dbValNoBill := sitecfg.NoBillModelsDBMeta(ctx, client)
		probeEnSet, probeEnVal, probeIntSet, probeIntVal, probeMetaErr := sitecfg.UpstreamModelProbeDBMeta(ctx, client)
		if probeMetaErr != nil {
			httpx.ServerError(c, "读取探测配置失败")
			return
		}
		capStaleSet, capStaleVal, capStaleMetaErr := sitecfg.UpstreamCapabilityFailStaleHoursDBMeta(ctx, client)
		if capStaleMetaErr != nil {
			httpx.ServerError(c, "读取能力探测配置失败")
			return
		}
		routingMeta, routingMetaErr := sitecfg.RoutingDBMeta(ctx, client)
		if routingMetaErr != nil {
			httpx.ServerError(c, "读取路由配置失败")
			return
		}
		gwIPSet, gwIPRaw, gwIPList := sitecfg.GatewayIPBlacklistDBMeta(ctx, client)
		routingDefaults := sitecfg.EffectiveRoutingDefaults(ctx, client)
		// 管理页依赖最新库值；禁止缓存以免与「恢复默认」等操作不同步
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		httpx.OK(c, gin.H{
			"brand_name":                                    sitecfg.EffectiveBrandName(ctx, client),
			"brand_icon_url":                                sitecfg.StoredBrandIconURL(ctx, client),
			"brand_favicon_url":                             sitecfg.StoredBrandFaviconURL(ctx, client),
			"brand_og_image_url":                            sitecfg.StoredBrandOGImageURL(ctx, client),
			"brand_contact_card_url":                        sitecfg.StoredBrandContactCardURL(ctx, client),
			"login_captcha_enabled":                         sitecfg.EffectiveLoginCaptchaEnabled(ctx, client, cfg),
			"login_captcha_file":                            cfg.Site.LoginCaptcha.Enabled,
			"login_captcha_db_set":                          dbSet,
			"login_captcha_db_value":                        strings.TrimSpace(dbVal),
			"operator_display_name":                         sitecfg.EffectiveOperatorDisplayName(ctx, client, cfg),
			"operator_display_file":                         strings.TrimSpace(cfg.Site.OperatorDisplayName),
			"default_new_user_balance":                      sitecfg.EffectiveDefaultNewUserBalance(ctx, client, cfg),
			"default_new_user_balance_file":                 cfg.Site.DefaultNewUserBalance,
			"default_new_user_balance_db_set":               dbSetBal,
			"default_new_user_balance_db_value":             strings.TrimSpace(dbValBal),
			"register_promo_text":                           sitecfg.EffectiveRegisterPromoText(ctx, client, cfg),
			"register_promo_text_file":                      strings.TrimSpace(cfg.Site.RegisterPromoText),
			"register_promo_text_db_set":                    dbSetPromo,
			"register_promo_text_db_value":                  strings.TrimSpace(dbValPromo),
			"beian_footer_text":                             sitecfg.EffectiveBeianFooterText(ctx, client, cfg),
			"beian_footer_text_file":                        strings.TrimSpace(cfg.Site.BeianFooterText),
			"beian_footer_text_db_set":                      dbSetBeian,
			"beian_footer_text_db_value":                    strings.TrimSpace(dbValBeian),
			"no_bill_models":                                sitecfg.EffectiveNoBillModels(ctx, client, cfg),
			"no_bill_models_file":                           cfg.Site.NoBillModels,
			"no_bill_models_db_set":                         dbSetNoBill,
			"no_bill_models_db_value":                       strings.TrimSpace(dbValNoBill),
			"api_public_base_url":                           sitecfg.EffectiveAPIPublicBaseURL(ctx, client),
			"public_contact":                                sitecfg.EffectivePublicContactData(ctx, client),
			"upstream_model_probe_enabled":                  sitecfg.EffectiveUpstreamModelProbeEnabled(ctx, client),
			"upstream_model_probe_interval_minutes":         sitecfg.EffectiveUpstreamModelProbeIntervalMinutes(ctx, client),
			"upstream_model_probe_enabled_db_set":           probeEnSet,
			"upstream_model_probe_enabled_db_value":         strings.TrimSpace(probeEnVal),
			"upstream_model_probe_interval_db_set":          probeIntSet,
			"upstream_model_probe_interval_db_value":        strings.TrimSpace(probeIntVal),
			"upstream_capability_fail_stale_hours":          sitecfg.EffectiveUpstreamCapabilityFailStaleHours(ctx, client),
			"upstream_capability_fail_stale_hours_db_set":   capStaleSet,
			"upstream_capability_fail_stale_hours_db_value": strings.TrimSpace(capStaleVal),
			"upstream_capability_probe_enabled":             sitecfg.EffectiveUpstreamCapabilityProbeEnabled(ctx, client),
			"routing_strategy_default":                      routingDefaults.Strategy,
			"routing_failover_enabled_default":              routingDefaults.FailoverEnabled,
			"routing_failover_max_attempts_default":         routingDefaults.FailoverMaxAttempts,
			"routing_sticky_enabled_default":                routingDefaults.StickyEnabled,
			"routing_auto_health_from_probe":                routingDefaults.AutoHealthFromProbe,
			"routing_probe_fail_threshold":                  routingDefaults.ProbeFailThreshold,
			"routing_db_meta":                               routingMeta,
			"gateway_ip_blacklist":                          gwIPList,
			"gateway_ip_blacklist_db_set":                   gwIPSet,
			"gateway_ip_blacklist_db_value":                 strings.TrimSpace(gwIPRaw),
			"currency_quota_per_yuan":                       cp.QuotaPerYuan,
			"currency_quota_per_yuan_file":                  cfg.Site.CurrencyQuotaPerYuan,
			"currency_quota_per_yuan_db_set":                dbSetYuan,
			"currency_quota_per_yuan_db_value":              strings.TrimSpace(dbValYuan),
			"currency_quota_per_usd":                        cp.QuotaPerUsd,
			"currency_quota_per_usd_file":                   cfg.Site.CurrencyQuotaPerUsd,
			"currency_quota_per_usd_db_set":                 dbSetUsd,
			"currency_quota_per_usd_db_value":               strings.TrimSpace(dbValUsd),
			"pricing_cny_per_usd":                           cp.CnyPerUsd,
			"pricing_cny_per_usd_file":                      cfg.Site.PricingCnyPerUsd,
			"pricing_cny_per_usd_db_set":                    dbSetRate,
			"pricing_cny_per_usd_db_value":                  strings.TrimSpace(dbValRate),
			"default_pricing_markup_percent":                sitecfg.EffectiveDefaultPricingMarkupPercent(ctx, client, cfg),
			"default_pricing_markup_percent_file":           cfg.Site.DefaultPricingMarkupPercent,
			"default_pricing_markup_percent_db_set":         dbSetMarkup,
			"default_pricing_markup_percent_db_value":       strings.TrimSpace(dbValMarkup),
			"https_force_redirect":                          httpsPolicy.ForceRedirect,
			"https_force_redirect_file":                     cfg.Site.HTTPS.ForceRedirect,
			"https_force_redirect_db_set":                   dbSetHTTPSFR,
			"https_force_redirect_db_value":                 strings.TrimSpace(dbValHTTPSFR),
			"https_hsts_enabled":                            httpsPolicy.HSTSEnabled,
			"https_hsts_enabled_file":                       cfg.Site.HTTPS.HSTSEnabled,
			"https_hsts_enabled_db_set":                     dbSetHTTPSHSTS,
			"https_hsts_enabled_db_value":                   strings.TrimSpace(dbValHTTPSHSTS),
			"https_hsts_max_age_seconds":                    httpsPolicy.HSTSMaxAgeSeconds,
			"https_hsts_max_age_seconds_file":               cfg.Site.HTTPS.HSTSMaxAgeSeconds,
			"https_hsts_max_age_seconds_db_set":             dbSetHTTPSMax,
			"https_hsts_max_age_seconds_db_value":           strings.TrimSpace(dbValHTTPSMax),
			"tls_enabled":                                   sitecfg.TLSEnabledAtStartup(ctx, client, cfg),
			"tls_enabled_file":                              cfg.Server.TLS.Enabled,
			"tls_enabled_db_set":                            dbSetTLS,
			"tls_enabled_db_value":                          strings.TrimSpace(dbValTLS),
			"tls_cert_file":                                 tlsCert,
			"tls_cert_file_yaml":                            strings.TrimSpace(cfg.Server.TLS.CertFile),
			"tls_cert_file_db_set":                          dbSetTLSCert,
			"tls_cert_file_db_value":                        strings.TrimSpace(dbValTLSCert),
			"tls_key_file":                                  tlsKey,
			"tls_key_file_yaml":                             strings.TrimSpace(cfg.Server.TLS.KeyFile),
			"tls_key_file_db_set":                           dbSetTLSKey,
			"tls_key_file_db_value":                         strings.TrimSpace(dbValTLSKey),
			"tls_addr":                                      tlsAddr,
			"tls_cert_present":                              tlsCert != "" && certErr == nil,
			"tls_key_present":                               tlsKey != "" && keyErr == nil,
			"tls_material_ready":                            tlsMat.MaterialReady,
			"tls_pair_ok":                                   tlsMat.PairOK,
		})
	}
}

// ResetAdminCurrencyPricingDefaults POST /api/admin/site-settings/reset-currency-pricing — 清除全站标价→额度换算与默认成本加成四项，回退 YAML。
func ResetAdminCurrencyPricingDefaults(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		keys := []string{
			sitecfg.SystemConfigKeyCurrencyQuotaPerYuan,
			sitecfg.SystemConfigKeyCurrencyQuotaPerUsd,
			sitecfg.SystemConfigKeyPricingCnyPerUsd,
			sitecfg.SystemConfigKeyDefaultPricingMarkupPercent,
		}
		n, err := client.SystemConfig.Delete().Where(systemconfig.ConfigKeyIn(keys...)).Exec(ctx)
		if err != nil {
			httpx.ServerError(c, "清除配置失败")
			return
		}
		billing.InvalidateDefaultPricingMarkupCacheAdmin()
		auditLog(client, adm.ID, "site.reset_currency_pricing", "system_config", "", map[string]any{"deleted_rows": n}, c)
		httpx.OK(c, gin.H{"deleted_rows": n})
	}
}

// ResetAdminSiteSettingsDefaults POST /api/admin/site-settings/reset-defaults — 删除本页相关 system_configs 行，运行时回退到 YAML / 内置默认；不删除本地上传的图标文件。
func ResetAdminSiteSettingsDefaults(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		keys := []string{
			sitecfg.SystemConfigKeyBrandName,
			sitecfg.SystemConfigKeyBrandIconURL,
			sitecfg.SystemConfigKeyBrandFaviconURL,
			sitecfg.SystemConfigKeyBrandOGImageURL,
			sitecfg.SystemConfigKeyBrandContactCardURL,
			sitecfg.SystemConfigKeyLoginCaptchaEnabled,
			sitecfg.SystemConfigKeyOperatorDisplayName,
			sitecfg.SystemConfigKeyDefaultNewUserBalance,
			sitecfg.SystemConfigKeyRegisterPromoText,
			sitecfg.SystemConfigKeyNoBillModels,
			sitecfg.SystemConfigKeyAPIPublicBaseURL,
			sitecfg.SystemConfigKeyPublicContactJSON,
			sitecfg.SystemConfigKeyBeianFooterText,
			sitecfg.SystemConfigKeyRoutingStrategyDefault,
			sitecfg.SystemConfigKeyRoutingFailoverEnabledDefault,
			sitecfg.SystemConfigKeyRoutingFailoverMaxAttempts,
			sitecfg.SystemConfigKeyRoutingStickyEnabledDefault,
			sitecfg.SystemConfigKeyRoutingAutoHealthFromProbe,
			sitecfg.SystemConfigKeyRoutingProbeFailThreshold,
			sitecfg.SystemConfigKeyGatewayIPBlacklist,
			sitecfg.SystemConfigKeyGatewayIPWhitelist,
			sitecfg.SystemConfigKeyHTTPSForceRedirect,
			sitecfg.SystemConfigKeyHTTPSHSTSEnabled,
			sitecfg.SystemConfigKeyHTTPSHSTSMaxAge,
			sitecfg.SystemConfigKeyTLSEnabled,
			sitecfg.SystemConfigKeyTLSCertFile,
			sitecfg.SystemConfigKeyTLSKeyFile,
			sitecfg.SystemConfigKeyCurrencyQuotaPerYuan,
			sitecfg.SystemConfigKeyCurrencyQuotaPerUsd,
			sitecfg.SystemConfigKeyPricingCnyPerUsd,
		}
		n, err := client.SystemConfig.Delete().Where(systemconfig.ConfigKeyIn(keys...)).Exec(ctx)
		if err != nil {
			httpx.ServerError(c, "清除配置失败")
			return
		}
		sitecfg.InvalidateGatewayIPBlacklistCache()
		sitecfg.InvalidateHTTPSPolicyCache()
		billing.InvalidateNoBillModelsCacheAdmin()
		auditLog(client, adm.ID, "site.settings_reset_defaults", "system_config", "", map[string]any{"deleted_rows": n}, c)
		httpx.OK(c, gin.H{"deleted_rows": n})
	}
}

func PutSystemConfig(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Items []struct {
				Key         string `json:"key"`
				Value       string `json:"value"`
				Description string `json:"description"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
			httpx.BadRequest(c, "items 不能为空")
			return
		}
		ctx := c.Request.Context()
		pending := make(map[string]string, len(req.Items))
		for _, it := range req.Items {
			k := strings.TrimSpace(it.Key)
			if k == "" {
				continue
			}
			pending[k] = strings.TrimSpace(it.Value)
		}
		if v, ok := pending[sitecfg.SystemConfigKeyTLSEnabled]; ok {
			if b, err := strconv.ParseBool(v); err == nil && b {
				st := sitecfg.TLSMaterialStatusWithOverrides(ctx, client, cfg,
					pending[sitecfg.SystemConfigKeyTLSCertFile],
					pending[sitecfg.SystemConfigKeyTLSKeyFile],
				)
				if err := sitecfg.ValidateTLSMaterialStatus(st); err != nil {
					httpx.BadRequest(c, err.Error())
					return
				}
			}
		}
		invalidateNoBill := false
		invalidateGatewayBlacklist := false
		invalidateDefaultMarkup := false
		invalidateHTTPSPolicy := false
		tlsEnabledTouched := false
		tlsMaterialTouched := false
		for _, it := range req.Items {
			k := strings.TrimSpace(it.Key)
			if k == "" {
				continue
			}
			if k == sitecfg.SystemConfigKeyGatewayIPBlacklist {
				invalidateGatewayBlacklist = true
			}
			if k == sitecfg.SystemConfigKeyNoBillModels {
				invalidateNoBill = true
			}
			if k == sitecfg.SystemConfigKeyDefaultPricingMarkupPercent {
				invalidateDefaultMarkup = true
			}
			switch k {
			case sitecfg.SystemConfigKeyHTTPSForceRedirect,
				sitecfg.SystemConfigKeyHTTPSHSTSEnabled,
				sitecfg.SystemConfigKeyHTTPSHSTSMaxAge:
				invalidateHTTPSPolicy = true
			case sitecfg.SystemConfigKeyTLSEnabled:
				tlsEnabledTouched = true
			case sitecfg.SystemConfigKeyTLSCertFile, sitecfg.SystemConfigKeyTLSKeyFile:
				tlsMaterialTouched = true
			}
			if err := sitecfg.ValidateSystemConfigPut(k, it.Value); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			val := strings.TrimSpace(it.Value)
			if k == sitecfg.SystemConfigKeyGatewayIPBlacklist && val == "" {
				val = "[]"
			}
			if k == sitecfg.SystemConfigKeyAPIPublicBaseURL {
				nv, err := sitecfg.NormalizeAPIPublicBaseURL(val)
				if err != nil {
					httpx.BadRequest(c, err.Error())
					return
				}
				val = nv
			}
			if k == sitecfg.SystemConfigKeyPublicContactJSON {
				canon, err := sitecfg.CanonicalPublicContactJSON(it.Value)
				if err != nil {
					httpx.BadRequest(c, err.Error())
					return
				}
				val = canon
			}
			ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(k)).Only(ctx)
			if ent.IsNotFound(err) {
				cr := client.SystemConfig.Create().SetConfigKey(k).SetConfigValue(val)
				if it.Description != "" {
					cr.SetDescription(it.Description)
				}
				if _, err := cr.Save(ctx); err != nil {
					httpx.ServerError(c, "保存失败")
					return
				}
			} else if err != nil {
				httpx.ServerError(c, "查询失败")
				return
			} else {
				up := client.SystemConfig.UpdateOne(ex).SetConfigValue(val)
				if it.Description != "" {
					up.SetDescription(it.Description)
				}
				if err := up.Exec(ctx); err != nil {
					httpx.ServerError(c, "更新失败")
					return
				}
			}
		}
		router.InvalidateAllRoutingCache()
		if invalidateGatewayBlacklist {
			sitecfg.InvalidateGatewayIPBlacklistCache()
		}
		if invalidateNoBill {
			billing.InvalidateNoBillModelsCacheAdmin()
		}
		if invalidateDefaultMarkup {
			billing.InvalidateDefaultPricingMarkupCacheAdmin()
		}
		if invalidateHTTPSPolicy {
			sitecfg.InvalidateHTTPSPolicyCache()
		}
		msg := "已保存"
		if tlsEnabledTouched || tlsMaterialTouched {
			msg = "已保存；内置 TLS 或证书路径变更需重启 TraceHub 服务后生效"
		}
		auditLog(client, adm.ID, "config.update", "system_config", "", map[string]any{"count": len(req.Items)}, c)
		httpx.OKMsg(c, msg, nil)
	}
}

func ReloadSystemConfig() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpx.OKMsg(c, "已触发重新加载（配置项在请求时从数据库读取）", nil)
	}
}

func DashboardOverview(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		totalUsers, err := client.User.Query().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		start := time.Now().Truncate(24 * time.Hour)
		tomorrow := start.Add(24 * time.Hour)
		todayReq, todayTokens, todayCost, err := usageStatsRange(ctx, client, start, tomorrow)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		since24h := time.Now().Add(-24 * time.Hour)
		activeIDs := map[int]struct{}{}
		activeLogs, _ := client.UsageLog.Query().Where(usagelog.RequestTimeGTE(since24h)).All(ctx)
		for _, l := range activeLogs {
			if l.UserID != 0 {
				activeIDs[l.UserID] = struct{}{}
			}
		}
		chAll, err := client.Channel.Query().All(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		healthy, unhealthy, disabled := 0, 0, 0
		for _, ch := range chAll {
			if !ch.Enabled {
				disabled++
				continue
			}
			if ch.HealthStatus == 1 {
				healthy++
			} else {
				unhealthy++
			}
		}
		var hourly []int
		now := time.Now()
		for h := 23; h >= 0; h-- {
			bucketStart := now.Add(-time.Duration(h) * time.Hour).Truncate(time.Hour)
			bucketEnd := bucketStart.Add(time.Hour)
			n, _ := client.UsageLog.Query().
				Where(usagelog.RequestTimeGTE(bucketStart), usagelog.RequestTimeLT(bucketEnd)).
				Count(ctx)
			hourly = append(hourly, n)
		}
		httpx.OK(c, gin.H{
			"today_requests":   todayReq,
			"today_tokens":     todayTokens,
			"today_cost_quota": todayCost,
			"active_users_24h": len(activeIDs),
			"total_users":      totalUsers,
			"channel_health": gin.H{
				"healthy":   healthy,
				"unhealthy": unhealthy,
				"disabled":  disabled,
			},
			"recent_requests_trend": hourly,
		})
	}
}

func DashboardTrends(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		gran := c.DefaultQuery("granularity", "day")
		days := 14
		if gran == "week" {
			days = 56
		}
		ctx := c.Request.Context()
		start := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Truncate(24 * time.Hour)
		logs, err := client.UsageLog.Query().Where(usagelog.RequestTimeGTE(start)).All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		byDay := map[string]gin.H{}
		for _, l := range logs {
			d := l.RequestTime.Format("2006-01-02")
			cur, ok := byDay[d]
			if !ok {
				cur = gin.H{"date": d, "requests": 0, "tokens": int64(0)}
			}
			cur["requests"] = cur["requests"].(int) + 1
			cur["tokens"] = cur["tokens"].(int64) + int64(l.TotalTokens)
			byDay[d] = cur
		}
		var series []gin.H
		for _, v := range byDay {
			series = append(series, v)
		}
		httpx.OK(c, gin.H{"granularity": gran, "series": series})
	}
}

func DashboardChannels(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		chs, err := client.Channel.Query().WithGroup().All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, ch := range chs {
			gn := ""
			if ch.Edges.Group != nil {
				gn = ch.Edges.Group.Name
			}
			items = append(items, gin.H{
				"id":            ch.ID,
				"channel_name":  ch.ChannelName,
				"channel_type":  ch.ChannelType,
				"group_name":    gn,
				"enabled":       ch.Enabled,
				"health_status": ch.HealthStatus,
				"schedulable":   ch.Schedulable,
			})
		}
		httpx.OK(c, gin.H{"items": items})
	}
}

func topModelsInRange(ctx context.Context, client *ent.Client, start, end time.Time, limit int) ([]gin.H, error) {
	var rows []struct {
		Model          string `json:"model"`
		Count          int    `json:"count"`
		SumTotalTokens *int64 `json:"sum_total_tokens"`
	}
	err := client.UsageLog.Query().
		Where(usagelog.RequestTimeGTE(start), usagelog.RequestTimeLT(end)).
		GroupBy(usagelog.FieldModel).
		Aggregate(
			ent.As(ent.Count(), "count"),
			ent.As(ent.Sum(usagelog.FieldTotalTokens), "sum_total_tokens"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Model < rows[j].Model
	})
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		tok := int64(0)
		if r.SumTotalTokens != nil {
			tok = *r.SumTotalTokens
		}
		out = append(out, gin.H{
			"model":    r.Model,
			"requests": r.Count,
			"tokens":   tok,
		})
	}
	return out, nil
}

func channelUsageInRange(ctx context.Context, client *ent.Client, start, end time.Time) ([]gin.H, int64, error) {
	var rows []struct {
		ChannelID int `json:"channel_id"`
		Count     int `json:"count"`
	}
	err := client.UsageLog.Query().
		Where(
			usagelog.RequestTimeGTE(start),
			usagelog.RequestTimeLT(end),
			usagelog.ChannelIDNotNil(),
		).
		GroupBy(usagelog.FieldChannelID).
		Aggregate(ent.As(ent.Count(), "count")).
		Scan(ctx, &rows)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	for _, r := range rows {
		total += int64(r.Count)
	}
	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ChannelID)
	}
	nameByID := map[int]string{}
	if len(ids) > 0 {
		chs, err := client.Channel.Query().Where(channel.IDIn(ids...)).All(ctx)
		if err != nil {
			return nil, 0, err
		}
		for _, ch := range chs {
			nm := strings.TrimSpace(ch.ChannelName)
			if nm == "" {
				nm = ch.ChannelType
			}
			nameByID[ch.ID] = nm
		}
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		pct := 0.0
		if total > 0 {
			pct = math.Round(1000*float64(r.Count)/float64(total)) / 10
		}
		nm := nameByID[r.ChannelID]
		if nm == "" {
			nm = "#" + strconv.Itoa(r.ChannelID)
		}
		out = append(out, gin.H{
			"channel_id":   r.ChannelID,
			"channel_name": nm,
			"requests":     r.Count,
			"pct":          pct,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		ai, _ := out[i]["requests"].(int)
		aj, _ := out[j]["requests"].(int)
		return ai > aj
	})
	return out, total, nil
}

// DashboardInsight 运营看板 P2：按日聚合时间序列、Top 模型、近 7 日渠道请求占比与渠道状态摘要。
func DashboardInsight(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
		if days != 1 && days != 7 && days != 30 {
			days = 7
		}
		ctx := c.Request.Context()
		unit := int64(10000)
		if cfg != nil && cfg.Site.CurrencyQuotaPerYuan > 0 {
			unit = cfg.Site.CurrencyQuotaPerYuan
		}
		now := time.Now()
		todayStart := now.Truncate(24 * time.Hour)
		tomorrowStart := todayStart.Add(24 * time.Hour)
		sevenStart := todayStart.AddDate(0, 0, -6)

		var series []gin.H
		timeseriesGranularity := "day"
		if days == 1 {
			timeseriesGranularity = "hour"
			for h := 0; h < 24; h++ {
				hourStart := todayStart.Add(time.Duration(h) * time.Hour)
				hourEnd := hourStart.Add(time.Hour)
				req, tok, cost, err := usageStatsRange(ctx, client, hourStart, hourEnd)
				if err != nil {
					httpx.ServerError(c, "统计失败")
					return
				}
				yuan := math.Round(float64(cost)/float64(unit)*100) / 100
				series = append(series, gin.H{
					"date":       hourStart.Format(time.RFC3339),
					"requests":   req,
					"tokens":     tok,
					"cost_quota": cost,
					"cost_yuan":  yuan,
				})
			}
		} else {
			for i := days - 1; i >= 0; i-- {
				dayStart := todayStart.AddDate(0, 0, -i)
				dayEnd := dayStart.Add(24 * time.Hour)
				req, tok, cost, err := usageStatsRange(ctx, client, dayStart, dayEnd)
				if err != nil {
					httpx.ServerError(c, "统计失败")
					return
				}
				yuan := math.Round(float64(cost)/float64(unit)*100) / 100
				series = append(series, gin.H{
					"date":       dayStart.Format("2006-01-02"),
					"requests":   req,
					"tokens":     tok,
					"cost_quota": cost,
					"cost_yuan":  yuan,
				})
			}
		}

		topToday, err := topModelsInRange(ctx, client, todayStart, tomorrowStart, 10)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		top7d, err := topModelsInRange(ctx, client, sevenStart, tomorrowStart, 10)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		chUsage, _, err := channelUsageInRange(ctx, client, sevenStart, tomorrowStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}

		chAll, err := client.Channel.Query().All(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		var enabledN, disabledN, unhealthyN, schedOffN int
		for _, ch := range chAll {
			if !ch.Enabled {
				disabledN++
				continue
			}
			enabledN++
			if ch.HealthStatus != 1 {
				unhealthyN++
			}
			if !ch.Schedulable {
				schedOffN++
			}
		}

		httpx.OK(c, gin.H{
			"window_days":            days,
			"timeseries_granularity": timeseriesGranularity,
			"timeseries":             series,
			"top_models_today": gin.H{
				"window": "today",
				"items":  topToday,
			},
			"top_models_7d": gin.H{
				"window": "last_7d",
				"items":  top7d,
			},
			"channels": gin.H{
				"enabled":         enabledN,
				"disabled":        disabledN,
				"unhealthy":       unhealthyN,
				"schedulable_off": schedOffN,
				"usage_7d":        chUsage,
			},
		})
	}
}

func httpErrorCounts(ctx context.Context, client *ent.Client, start, end time.Time) (total, n4xx, n5xx int, err error) {
	total, err = client.UsageLog.Query().
		Where(usagelog.RequestTimeGTE(start), usagelog.RequestTimeLT(end)).
		Count(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	n4xx, err = client.UsageLog.Query().
		Where(
			usagelog.RequestTimeGTE(start), usagelog.RequestTimeLT(end),
			usagelog.StatusCodeNotNil(),
			usagelog.StatusCodeGTE(400), usagelog.StatusCodeLT(500),
		).
		Count(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	n5xx, err = client.UsageLog.Query().
		Where(
			usagelog.RequestTimeGTE(start), usagelog.RequestTimeLT(end),
			usagelog.StatusCodeNotNil(),
			usagelog.StatusCodeGTE(500),
		).
		Count(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	return total, n4xx, n5xx, nil
}

func failPct(total, n4xx, n5xx int) float64 {
	if total <= 0 {
		return 0
	}
	return math.Round(1000*float64(n4xx+n5xx)/float64(total)) / 10
}

// DashboardSnapshot 运营看板 P3：用户/分组摘要、HTTP 失败占比、最近用量与渠道/公告草稿。
func DashboardSnapshot(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		now := time.Now()
		todayStart := now.Truncate(24 * time.Hour)
		tomorrowStart := todayStart.Add(24 * time.Hour)
		sevenStart := todayStart.AddDate(0, 0, -6)

		var dauRows []struct {
			UserID int `json:"user_id"`
			Cnt    int `json:"count"`
		}
		if err := client.UsageLog.Query().
			Where(
				usagelog.RequestTimeGTE(todayStart),
				usagelog.RequestTimeLT(tomorrowStart),
				usagelog.UserIDNotNil(),
				usagelog.UserIDNEQ(0),
			).
			GroupBy(usagelog.FieldUserID).
			Aggregate(ent.As(ent.Count(), "count")).
			Scan(ctx, &dauRows); err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		dauToday := len(dauRows)

		newToday, err := client.User.Query().
			Where(user.CreatedAtGTE(todayStart), user.CreatedAtLT(tomorrowStart)).
			Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		new7d, err := client.User.Query().
			Where(user.CreatedAtGTE(sevenStart), user.CreatedAtLT(tomorrowStart)).
			Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		groupsCount, err := client.Group.Query().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		negBal, err := client.User.Query().Where(user.BalanceLT(0)).Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		zeroBal, err := client.User.Query().Where(user.BalanceEQ(0)).Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}

		tTot, t4, t5, err := httpErrorCounts(ctx, client, todayStart, tomorrowStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		wTot, w4, w5, err := httpErrorCounts(ctx, client, sevenStart, tomorrowStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}

		logs, err := client.UsageLog.Query().
			Order(ent.Desc(usagelog.FieldRequestTime)).
			Limit(12).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var logItems []gin.H
		for _, l := range logs {
			logItems = append(logItems, gin.H{
				"id":           l.ID,
				"user_id":      l.UserID,
				"model":        l.Model,
				"status_code":  l.StatusCode,
				"request_time": l.RequestTime,
				"total_tokens": l.TotalTokens,
			})
		}

		chs, err := client.Channel.Query().
			Order(ent.Desc(channel.FieldUpdatedAt)).
			Limit(6).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var chItems []gin.H
		for _, ch := range chs {
			nm := strings.TrimSpace(ch.ChannelName)
			if nm == "" {
				nm = ch.ChannelType
			}
			chItems = append(chItems, gin.H{
				"id":           ch.ID,
				"channel_name": nm,
				"channel_type": ch.ChannelType,
				"enabled":      ch.Enabled,
				"updated_at":   ch.UpdatedAt,
			})
		}

		drafts, err := client.Announcement.Query().
			Where(announcement.PublishedEQ(false)).
			Order(ent.Desc(announcement.FieldID)).
			Limit(5).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var draftItems []gin.H
		for _, a := range drafts {
			draftItems = append(draftItems, announcementJSON(a))
		}

		httpx.OK(c, gin.H{
			"users": gin.H{
				"dau_today":        dauToday,
				"new_users_today":  newToday,
				"new_users_7d":     new7d,
				"groups_count":     groupsCount,
				"negative_balance": negBal,
				"zero_balance":     zeroBal,
			},
			"http": gin.H{
				"today": gin.H{
					"total_requests": tTot,
					"n_4xx":          t4,
					"n_5xx":          t5,
					"fail_pct":       failPct(tTot, t4, t5),
				},
				"last_7d": gin.H{
					"total_requests": wTot,
					"n_4xx":          w4,
					"n_5xx":          w5,
					"fail_pct":       failPct(wTot, w4, w5),
				},
			},
			"recent_logs":         logItems,
			"recent_channels":     chItems,
			"draft_announcements": draftItems,
		})
	}
}

// DashboardOps 运营看板 P4：管理审计摘要、系统健康探测、非敏感配置摘要、外部告警占位说明。
func DashboardOps(client *ent.Client, rdb *redis.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		logs, err := client.AdminAuditLog.Query().
			WithAdmin().
			Order(ent.Desc(adminauditlog.FieldCreatedAt)).
			Limit(15).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "审计日志查询失败")
			return
		}
		var auditItems []gin.H
		for _, l := range logs {
			email := ""
			if l.Edges.Admin != nil {
				email = l.Edges.Admin.Email
			}
			auditItems = append(auditItems, gin.H{
				"id":          l.ID,
				"created_at":  l.CreatedAt,
				"admin_id":    l.AdminID,
				"admin_email": email,
				"action":      l.Action,
				"resource":    l.Resource,
				"resource_id": l.ResourceID,
				"ip_address":  l.IPAddress,
			})
		}

		checks := gin.H{}
		if _, err := client.User.Query().Limit(1).Count(ctx); err != nil {
			checks["database"] = "error: " + err.Error()
		} else {
			checks["database"] = "ok"
		}
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				checks["redis"] = "error: " + err.Error()
			} else {
				checks["redis"] = "ok"
			}
		} else {
			checks["redis"] = "skipped"
		}

		dbCaptchaSet, dbCaptchaVal := sitecfg.LoginCaptchaDBMeta(ctx, client)
		effectiveCaptcha := sitecfg.EffectiveLoginCaptchaEnabled(ctx, client, cfg)
		outline := gin.H{
			"server_mode":             cfg.Server.Mode,
			"currency_quota_per_yuan": cfg.Site.CurrencyQuotaPerYuan,
			"currency_quota_per_usd":  cfg.Site.CurrencyQuotaPerUsd,
			"pricing_cny_per_usd":     cfg.Site.PricingCnyPerUsd,
			"cors_origins_count":      len(cfg.Site.CORSOrigins),
			"realtime_call_mapping":   cfg.Site.RealtimeCallMappingEnabled,
			"email_login_enabled":     cfg.Email.Enabled,
			"brand_name":              sitecfg.EffectiveBrandName(ctx, client),
			"login_captcha_enabled":   effectiveCaptcha,
			"login_captcha_file":      cfg.Site.LoginCaptcha.Enabled,
			"login_captcha_db_set":    dbCaptchaSet,
			"login_captcha_db_value":  strings.TrimSpace(dbCaptchaVal),
			"liveness_path":           "/health",
			"readiness_path":          "/ready",
		}

		httpx.OK(c, gin.H{
			"recent_audit": auditItems,
			"health": gin.H{
				"checks":       checks,
				"app_version":  cfg.AppVersion(),
				"go_version":   runtime.Version(),
				"uptime_sec":   health.UptimeSeconds(),
				"public_paths": gin.H{"liveness": "/health", "readiness": "/ready"},
			},
			"config_outline": outline,
			"external_alerts": gin.H{
				"status": "not_connected",
				"scope":  []string{"disk", "queue", "upstream_account_balance"},
			},
		})
	}
}
