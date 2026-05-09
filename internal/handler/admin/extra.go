package adminhandler

import (
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/redeemcode"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
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
		totalReq, _ := client.UsageLog.Query().Where(usagelog.UserIDEQ(id)).Count(ctx)
		gn := ""
		if u.Edges.Group != nil {
			gn = u.Edges.Group.Name
		}
		httpx.OK(c, gin.H{
			"id":               u.ID,
			"email":            u.Email,
			"username":         u.Username,
			"group_name":       gn,
			"balance":          u.Balance,
			"status":           u.Status,
			"total_requests":   totalReq,
			"created_at":       u.CreatedAt,
			"last_login_at":    u.LastLoginAt,
			"ip_whitelist":     u.IPWhitelist,
			"rate_multiplier":  u.RateMultiplier,
			"webhook_url":      u.WebhookURL,
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
			Name           *string             `json:"name"`
			Description    *string             `json:"description"`
			RateMultiplier *float64            `json:"rate_multiplier"`
			RPMLimit       *int                `json:"rpm_limit"`
			ModelRouting   map[string][]int64  `json:"model_routing"`
			DefaultModel   *string             `json:"default_model"`
			Enabled        *bool               `json:"enabled"`
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
			up.SetModelRouting(req.ModelRouting)
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
			masked := rc.Code
			if len(masked) > 10 {
				masked = masked[:6] + "****" + masked[len(masked)-4:]
			}
			items = append(items, gin.H{
				"id":         rc.ID,
				"code_hint":  masked,
				"code_type":  rc.CodeType,
				"amount":     rc.Amount,
				"status":     rc.Status,
				"max_uses":   rc.MaxUses,
				"used_count": rc.UsedCount,
				"valid_until": rc.ValidUntil,
				"created_at": rc.CreatedAt,
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
		_ = w.Write([]string{"id", "user_id", "model", "channel_id", "prompt", "completion", "cost", "status", "time"})
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
			_ = w.Write([]string{
				strconv.Itoa(l.ID),
				uid,
				l.Model,
				ch,
				strconv.Itoa(l.PromptTokens),
				strconv.Itoa(l.CompletionTokens),
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

func PutSystemConfig(client *ent.Client) gin.HandlerFunc {
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
		for _, it := range req.Items {
			k := strings.TrimSpace(it.Key)
			if k == "" {
				continue
			}
			ex, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(k)).Only(ctx)
			if ent.IsNotFound(err) {
				cr := client.SystemConfig.Create().SetConfigKey(k).SetConfigValue(it.Value)
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
				up := client.SystemConfig.UpdateOne(ex).SetConfigValue(it.Value)
				if it.Description != "" {
					up.SetDescription(it.Description)
				}
				if err := up.Exec(ctx); err != nil {
					httpx.ServerError(c, "更新失败")
					return
				}
			}
		}
		auditLog(client, adm.ID, "config.update", "system_config", "", map[string]any{"count": len(req.Items)}, c)
		httpx.OKMsg(c, "已保存", nil)
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
		todayLogs, err := client.UsageLog.Query().Where(usagelog.RequestTimeGTE(start)).All(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		var todayTokens int64
		var todayCost int64
		for _, l := range todayLogs {
			todayTokens += int64(l.TotalTokens)
			todayCost += l.TotalCost
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
			"today_requests":    len(todayLogs),
			"today_tokens":      todayTokens,
			"today_cost_quota":  todayCost,
			"active_users_24h":  len(activeIDs),
			"total_users":       totalUsers,
			"channel_health": gin.H{
				"healthy":    healthy,
				"unhealthy":  unhealthy,
				"disabled":   disabled,
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
				"id":             ch.ID,
				"channel_name":   ch.ChannelName,
				"channel_type":   ch.ChannelType,
				"group_name":     gn,
				"enabled":        ch.Enabled,
				"health_status":  ch.HealthStatus,
				"schedulable":    ch.Schedulable,
			})
		}
		httpx.OK(c, gin.H{"items": items})
	}
}
