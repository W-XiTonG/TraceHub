package userhandler

import (
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// Me 返回当前用户资料（不含敏感字段），供控制台账户设置等使用。
func Me(client *ent.Client) gin.HandlerFunc {
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
		httpx.OK(c, gin.H{
			"id":                        fu.ID,
			"email":                     fu.Email,
			"username":                  fu.Username,
			"group_id":                  fu.GroupID,
			"group_name":                gn,
			"webhook_url":               fu.WebhookURL,
			"ip_whitelist":              fu.IPWhitelist,
			"balance_alert_threshold":   fu.BalanceAlertThreshold,
			"rate_multiplier":           fu.RateMultiplier,
			"balance":                   fu.Balance,
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
				"id":                 l.ID,
				"request_id":         l.RequestID,
				"model":              l.Model,
				"model_upstream":     l.ModelUpstream,
				"channel_id":         l.ChannelID,
				"api_key_id":         l.APIKeyID,
				"prompt_tokens":      l.PromptTokens,
				"completion_tokens":  l.CompletionTokens,
				"total_tokens":       l.TotalTokens,
				"total_cost":         l.TotalCost,
				"request_time":       l.RequestTime,
				"response_time_ms":   l.ResponseTimeMs,
				"status_code":        l.StatusCode,
				"error_message":      l.ErrorMessage,
				"ip_address":         l.IPAddress,
				"is_cached":          l.IsCached,
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
				"id":               g.ID,
				"name":             g.Name,
				"description":      g.Description,
				"rate_multiplier":  g.RateMultiplier,
				"rpm_limit":        g.RpmLimit,
				"default_model":    g.DefaultModel,
				"subscription_type": g.SubscriptionType,
			},
			"model_rates": rates,
		})
	}
}
