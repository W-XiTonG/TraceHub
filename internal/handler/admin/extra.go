package adminhandler

import (
	"context"
	"encoding/csv"
	"math"
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
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/predicate"
	"tracehub/internal/ent/redeemcode"
	"tracehub/internal/ent/redeemledger"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/handler/health"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/alertconfig"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"
	"tracehub/internal/pkg/redeemutil"
	"tracehub/internal/pkg/sitecfg"

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
			Name           *string            `json:"name"`
			Description    *string            `json:"description"`
			RateMultiplier *float64           `json:"rate_multiplier"`
			RPMLimit       *int               `json:"rpm_limit"`
			ModelRouting   map[string][]int64 `json:"model_routing"`
			DefaultModel   *string            `json:"default_model"`
			Enabled        *bool              `json:"enabled"`
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
				"id":              row.ID,
				"user_id":         row.UserID,
				"user_email":      email,
				"redeem_code_id":  row.RedeemCodeID,
				"amount":          row.Amount,
				"balance_after":   row.BalanceAfter,
				"code_hint":       row.CodeHint,
				"created_at":      row.CreatedAt,
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

// GetAdminSiteSettings GET /api/admin/site-settings — 结构化站点配置（供系统配置页）；不含密钥。
func GetAdminSiteSettings(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		dbSet, dbVal := sitecfg.LoginCaptchaDBMeta(ctx, client)
		dbSetBal, dbValBal := sitecfg.DefaultNewUserBalanceDBMeta(ctx, client)
		// 管理页依赖最新库值；禁止缓存以免与「恢复默认」等操作不同步
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		httpx.OK(c, gin.H{
			"brand_name":                        sitecfg.EffectiveBrandName(ctx, client),
			"brand_icon_url":                    sitecfg.StoredBrandIconURL(ctx, client),
			"login_captcha_enabled":             sitecfg.EffectiveLoginCaptchaEnabled(ctx, client, cfg),
			"login_captcha_file":                cfg.Site.LoginCaptcha.Enabled,
			"login_captcha_db_set":              dbSet,
			"login_captcha_db_value":            strings.TrimSpace(dbVal),
			"operator_display_name":             sitecfg.EffectiveOperatorDisplayName(ctx, client, cfg),
			"operator_display_file":             strings.TrimSpace(cfg.Site.OperatorDisplayName),
			"default_new_user_balance":          sitecfg.EffectiveDefaultNewUserBalance(ctx, client, cfg),
			"default_new_user_balance_file":     cfg.Site.DefaultNewUserBalance,
			"default_new_user_balance_db_set":   dbSetBal,
			"default_new_user_balance_db_value": strings.TrimSpace(dbValBal),
			"api_public_base_url":               sitecfg.EffectiveAPIPublicBaseURL(ctx, client),
		})
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
			sitecfg.SystemConfigKeyLoginCaptchaEnabled,
			sitecfg.SystemConfigKeyOperatorDisplayName,
			sitecfg.SystemConfigKeyDefaultNewUserBalance,
			sitecfg.SystemConfigKeyAPIPublicBaseURL,
		}
		n, err := client.SystemConfig.Delete().Where(systemconfig.ConfigKeyIn(keys...)).Exec(ctx)
		if err != nil {
			httpx.ServerError(c, "清除配置失败")
			return
		}
		auditLog(client, adm.ID, "site.settings_reset_defaults", "system_config", "", map[string]any{"deleted_rows": n}, c)
		httpx.OK(c, gin.H{"deleted_rows": n})
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
			if err := sitecfg.ValidateSystemConfigPut(k, it.Value); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			val := strings.TrimSpace(it.Value)
			if k == sitecfg.SystemConfigKeyAPIPublicBaseURL {
				nv, err := sitecfg.NormalizeAPIPublicBaseURL(val)
				if err != nil {
					httpx.BadRequest(c, err.Error())
					return
				}
				val = nv
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
				"app_version":  health.Version,
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
