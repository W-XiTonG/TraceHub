package userhandler

import (
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/redeemledger"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/alertconfig"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/redeemutil"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/pkg/modelkind"
	"tracehub/internal/pkg/usagelogfilter"
	"tracehub/internal/pkg/usagelogpublic"
	"tracehub/internal/gateway/router"
	"tracehub/internal/service/availmodels"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

// Me 返回当前用户资料（不含敏感字段），供控制台账户设置等使用。
func Me(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		fu, err := client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		gn := ""
		if fu.Edges.Group != nil {
			gn = fu.Edges.Group.Name
		}
		unit := cfg.Site.CurrencyQuotaPerYuan
		if unit <= 0 {
			unit = 10000
		}
		httpx.OK(c, gin.H{
			"id":                      fu.ID,
			"email":                   fu.Email,
			"username":                fu.Username,
			"group_id":                fu.GroupID,
			"group_name":              gn,
			"webhook_url":             fu.WebhookURL,
			"ip_whitelist":            fu.IPWhitelist,
			"balance_alert_threshold": fu.BalanceAlertThreshold,
			"custom_alert_config":     alertconfig.PublicView(fu.CustomAlertConfig),
			"rate_multiplier":         fu.RateMultiplier,
			"balance":                 fu.Balance,
			"currency_quota_per_yuan": unit,
		})
	}
}

// UsageLogs 当前用户调用日志（分页、筛选），对应产品文档「调用日志」。
func UsageLogs(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.UsageLog.Query().Where(usagelog.UserIDEQ(u.ID))
		if m := strings.TrimSpace(c.Query("model")); m != "" {
			q = q.Where(usagelog.ModelEQ(m))
		}
		if sc := c.Query("status_code"); sc != "" {
			if v, err := strconv.Atoi(sc); err == nil {
				q = q.Where(usagelog.StatusCodeEQ(v))
			}
		}
		q = usagelogfilter.ApplyIPAddress(q, c.Query("ip"))
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
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		logs, err := q.Order(ent.Desc(usagelog.FieldRequestTime)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, l := range logs {
			items = append(items, gin.H{
				"id":                l.ID,
				"request_id":        l.RequestID,
				"model":             l.Model,
				"api_key_id":        l.APIKeyID,
				"billing_mode":      modelkind.UsageBillingMode(l.Model, l.PromptTokens, l.CompletionTokens),
				"prompt_tokens":     l.PromptTokens,
				"completion_tokens": l.CompletionTokens,
				"total_tokens":      l.TotalTokens,
				"total_cost":        l.TotalCost,
				"request_time":      l.RequestTime,
				"response_time_ms":  l.ResponseTimeMs,
				"status_code":       l.StatusCode,
				"error_message":     usagelogpublic.UserFacingFields(l.ErrorMessage, l.StatusCode),
				"ip_address":        l.IPAddress,
				"is_cached":         l.IsCached,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

// MyGroup 当前用户所在分组及模型倍率（产品文档「我的分组」）。
func MyGroup(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		fu, err := client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		if fu.GroupID <= 0 || fu.Edges.Group == nil {
			httpx.OK(c, gin.H{"group": nil, "model_rates": []any{}})
			return
		}
		g := fu.Edges.Group
		routing, _ := router.ResolveRoutingConfig(ctx, client, g.ID)
		rates, err := client.GroupModelRate.Query().
			Where(groupmodelrate.GroupIDEQ(g.ID)).
			Order(ent.Asc(groupmodelrate.FieldModelName)).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{
			"group": gin.H{
				"id":                      g.ID,
				"name":                    g.Name,
				"description":             g.Description,
				"rate_multiplier":         g.RateMultiplier,
				"rpm_limit":               g.RpmLimit,
				"default_model":           g.DefaultModel,
				"subscription_type":       g.SubscriptionType,
				"routing_strategy":        routing.Strategy,
				"failover_enabled":        routing.FailoverEnabled,
				"failover_max_attempts":   routing.FailoverMaxAttempts,
				"failover_4xx_mode":       routing.Failover4xxMode,
				"failover_4xx_codes":      sitecfg.EffectiveFailover4xxCodes(routing.Failover4xxMode, routing.Failover4xxCodes),
				"sticky_enabled":          routing.StickyEnabled,
			},
			"model_rates": rates,
		})
	}
}

// AvailableModels 控制台模型列表：合并「分组下渠道可调度的模型」与「模型市场中对用户可见的目录」。
func AvailableModels(client *ent.Client, eng *billing.Engine, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		gid := u.GroupID
		ctx := c.Request.Context()
		includeUnavail := strings.EqualFold(strings.TrimSpace(c.Query("tracehub_include_unavailable")), "1") ||
			strings.EqualFold(strings.TrimSpace(c.Query("tracehub_include_unavailable")), "true")
		opts := availmodels.ListOptsFromQuery(ctx, client, includeUnavail)
		opts.SkipCapabilityOverlay = true // 大列表避免按模型×渠道触发 Redis 能力覆盖层查询
		items, groupChannels, err := availmodels.ListConsoleMerged(ctx, client, gid, opts)
		if err != nil {
			httpx.ServerError(c, "读取模型列表失败")
			return
		}
		billUser := u
		if eng != nil {
			fu, qerr := client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(ctx)
			if qerr == nil {
				billUser = fu
			}
			var pricingPreview *router.PricingPreview
			pricingStrategy := ""
			var pricingBatch *billing.ConsolePricingBatch
			if gid > 0 {
				pp, ppErr := router.NewPricingPreviewWithChannels(ctx, client, gid, groupChannels)
				if ppErr == nil {
					pricingPreview = pp
					pricingStrategy = pp.EffectiveStrategy()
					pricingBatch = eng.NewConsolePricingBatch(ctx, billUser, pp.ChannelIDs())
				}
			}
			if pricingBatch == nil {
				pricingBatch = eng.NewConsolePricingBatch(ctx, billUser, nil)
			}
			for i := range items {
				channelID := 0
				if pricingPreview != nil {
					channelID = pricingPreview.ChannelIDForModel(items[i].ID)
				}
				br := pricingBatch.Rates(items[i].ID, channelID)
				mp := &availmodels.ModelPricing{Billable: br.Billable}
				mp.BillingMode = br.BillingMode()
				if br.IsPerRequest() || br.IsHybridFloor() {
					mp.PricingUnit = "per_request"
					mp.QuotaPerRequest = br.RequestCost()
				} else if modelkind.IsVideoBillingModel(items[i].ID) {
					in, _ := br.EffectiveQuotaPer1k()
					mp.QuotaInPer1k = in
					mp.QuotaOutPer1k = 0
					mp.PricingUnit = "per_second"
				} else {
					in, out := br.EffectiveQuotaPer1k()
					mp.QuotaInPer1k = in
					mp.QuotaOutPer1k = out
					mp.PricingUnit = "per_1k_tokens"
				}
				items[i].Pricing = mp
				items[i].PricingChannelID = channelID
				if pricingStrategy != "" {
					items[i].PricingStrategy = pricingStrategy
				}
			}
		}
		hint := ""
		switch {
		case gid <= 0 && len(items) == 0:
			hint = "账户尚未分配分组；模型市场也无可见条目。分配分组并配置渠道后，方可通过 API 调用模型。"
		case gid <= 0:
			hint = "账户尚未分配分组：调用 GET /v1/models 需分组与渠道。下列含「目录」条目来自模型市场，实际可调用的模型以渠道配置为准。"
		case len(items) > 0 && countGateway(items) == 0:
			hint = "当前分组下暂无渠道可用模型；下列均为模型市场目录，请在管理后台为该分组配置启用且健康的渠道。"
		}
		cp := sitecfg.EffectiveCurrencyPricing(ctx, client, cfg)
		unit := cp.QuotaPerYuan
		if unit <= 0 {
			unit = 10000
		}
		httpx.OK(c, gin.H{
			"items":                   items,
			"total":                   len(items),
			"group_id":                gid,
			"group_assigned":          gid > 0,
			"hint":                    hint,
			"currency_quota_per_yuan": unit,
			"currency_quota_per_usd":  cp.QuotaPerUsd,
			"pricing_cny_per_usd":     cp.CnyPerUsd,
		})
	}
}

func countGateway(items []availmodels.ConsoleModel) int {
	n := 0
	for _, it := range items {
		if it.InGateway {
			n++
		}
	}
	return n
}

// ListRedeemLedger 当前用户兑换入账流水（分页，按时间倒序）。
// 可选查询：start_time、end_time（RFC3339，按 created_at 含界过滤）、redeem_code（脱敏片段子串或完整兑换码精确匹配）。
func ListRedeemLedger(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.RedeemLedger.Query().Where(redeemledger.UserIDEQ(u.ID))
		if st := strings.TrimSpace(c.Query("start_time")); st != "" {
			if t0, err := time.Parse(time.RFC3339, st); err == nil {
				q = q.Where(redeemledger.CreatedAtGTE(t0))
			}
		}
		if et := strings.TrimSpace(c.Query("end_time")); et != "" {
			if t0, err := time.Parse(time.RFC3339, et); err == nil {
				q = q.Where(redeemledger.CreatedAtLTE(t0))
			}
		}
		if codeStr := strings.TrimSpace(c.Query("redeem_code")); codeStr != "" {
			q = q.Where(redeemledger.Or(
				redeemledger.CodeHintContainsFold(codeStr),
				redeemledger.HasRedeemCodeWith(redeemutil.CodeMatchPredicate(codeStr)),
			))
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		list, err := q.Order(ent.Desc(redeemledger.FieldCreatedAt)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, row := range list {
			items = append(items, gin.H{
				"id":             row.ID,
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
