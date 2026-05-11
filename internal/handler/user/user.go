package userhandler

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/apikey"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/redeemcode"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/apikeyhash"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const maxAPIKeysPerUser = 20

func Register(client *ent.Client, cfg *config.Config, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Username string `json:"username"`
			Code     string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Email == "" || req.Password == "" {
			httpx.BadRequest(c, "邮箱与密码必填")
			return
		}
		if cfg.Email.Enabled {
			if rdb == nil {
				httpx.ServiceUnavailable(c, "已启用邮件验证但未连接 Redis，无法注册")
				return
			}
			if strings.TrimSpace(cfg.Email.SMTPHost) == "" || strings.TrimSpace(cfg.Email.From) == "" {
				httpx.ServiceUnavailable(c, "已启用邮件验证但 SMTP 未配置完整，无法注册")
				return
			}
			if strings.TrimSpace(req.Code) == "" {
				httpx.BadRequest(c, "请填写邮箱验证码")
				return
			}
			if !verifyAndConsumeRegisterCode(c, rdb, req.Email, req.Code) {
				return
			}
		}
		ctx := c.Request.Context()
		ok, err := client.User.Query().Where(user.EmailEQ(req.Email)).Exist(ctx)
		if err != nil {
			httpx.ServerError(c, "注册失败")
			return
		}
		if ok {
			httpx.BadRequest(c, "邮箱已注册")
			return
		}
		gname := cfg.Bootstrap.DefaultGroupName
		if gname == "" {
			gname = "基础分组"
		}
		g, err := client.Group.Query().Where(group.NameEQ(gname)).Only(ctx)
		if err != nil {
			httpx.ServerError(c, "系统分组未初始化")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			httpx.ServerError(c, "密码处理失败")
			return
		}
		cr := client.User.Create().
			SetEmail(strings.TrimSpace(req.Email)).
			SetPasswordHash(string(hash)).
			SetBalance(cfg.Site.DefaultNewUserBalance).
			SetStatus(1).
			SetRole("user").
			SetGroupID(g.ID)
		if req.Username != "" {
			cr.SetUsername(req.Username)
		}
		if err := cr.Exec(ctx); err != nil {
			httpx.ServerError(c, "创建用户失败")
			return
		}
		httpx.OKMsg(c, "注册成功，请登录", nil)
	}
}

func Login(client *ent.Client, cfg *config.Config, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		id := strings.TrimSpace(req.Email)
		if id == "" || req.Password == "" {
			httpx.BadRequest(c, "邮箱或用户名与密码必填")
			return
		}
		ctx := c.Request.Context()
		u, err := client.User.Query().
			Where(user.EmailEQ(id)).
			WithGroup().
			Only(ctx)
		if ent.IsNotFound(err) {
			u, err = client.User.Query().
				Where(user.UsernameEQ(id)).
				WithGroup().
				Only(ctx)
		}
		if err != nil || u.Status != 1 {
			httpx.Unauthorized(c, "账号或密码错误")
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
			httpx.Unauthorized(c, "账号或密码错误")
			return
		}
		writeUserSession(c, client, u, jwtSecret)
	}
}

func Refresh(client *ent.Client, jwtSecret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
			httpx.BadRequest(c, "缺少 refresh_token")
			return
		}
		uid, email, err := jwtx.ParseRefresh(jwtSecret, req.RefreshToken)
		if err != nil {
			httpx.Unauthorized(c, "刷新令牌无效")
			return
		}
		u, err := client.User.Get(c.Request.Context(), int(uid))
		if err != nil || u.Status != 1 {
			httpx.Unauthorized(c, "用户不可用")
			return
		}
		accessTTL := 72 * time.Hour
		at, err := jwtx.SignAccess(jwtSecret, int64(u.ID), email, u.Role, accessTTL)
		if err != nil {
			httpx.ServerError(c, "签发令牌失败")
			return
		}
		httpx.OK(c, gin.H{"access_token": at, "expires_in": int(accessTTL.Seconds())})
	}
}

func Balance(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		uu, err := client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		gname := ""
		rm := 1.0
		if uu.Edges.Group != nil {
			gname = uu.Edges.Group.Name
			rm = uu.Edges.Group.RateMultiplier
		}
		unit := cfg.Site.CurrencyQuotaPerYuan
		if unit <= 0 {
			unit = 10000
		}
		httpx.OK(c, gin.H{
			"balance":           uu.Balance,
			"currency_unit":     unit,
			"balance_yuan":      float64(uu.Balance) / float64(unit),
			"group_name":        gname,
			"rate_multiplier":   rm,
			"user_rate_overlay": uu.RateMultiplier,
		})
	}
}

func Usage(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		period := c.DefaultQuery("period", "7d")
		ctx := c.Request.Context()
		since := time.Now().Add(-7 * 24 * time.Hour)
		switch period {
		case "today":
			y, m, d := time.Now().Date()
			since = time.Date(y, m, d, 0, 0, 0, 0, time.Local)
		case "30d":
			since = time.Now().Add(-30 * 24 * time.Hour)
		case "all":
			since = time.Time{}
		}
		q := client.UsageLog.Query().Where(usagelog.UserIDEQ(u.ID))
		if !since.IsZero() {
			q = q.Where(usagelog.RequestTimeGTE(since))
		}
		if m := strings.TrimSpace(c.Query("model")); m != "" {
			q = q.Where(usagelog.ModelEQ(m))
		}
		logs, err := q.All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var totalReq, totalTok int
		var totalCost int64
		byDay := map[string]struct{ reqs int; cost int64 }{}
		byModel := map[string]struct{ reqs int; cost int64 }{}
		for _, l := range logs {
			totalReq++
			totalTok += l.TotalTokens
			totalCost += l.TotalCost
			d := l.RequestTime.In(time.Local).Format("2006-01-02")
			bd := byDay[d]
			bd.reqs++
			bd.cost += l.TotalCost
			byDay[d] = bd
			bm := byModel[l.Model]
			bm.reqs++
			bm.cost += l.TotalCost
			byModel[l.Model] = bm
		}
		unit := cfg.Site.CurrencyQuotaPerYuan
		if unit <= 0 {
			unit = 10000
		}
		var dailyBreakdown []gin.H
		for d, v := range byDay {
			dailyBreakdown = append(dailyBreakdown, gin.H{
				"date":     d,
				"requests": v.reqs,
				"cost":     float64(v.cost) / float64(unit),
			})
		}
		sort.Slice(dailyBreakdown, func(i, j int) bool {
			return dailyBreakdown[i]["date"].(string) < dailyBreakdown[j]["date"].(string)
		})
		var modelBreakdown []gin.H
		for m, v := range byModel {
			modelBreakdown = append(modelBreakdown, gin.H{
				"model":    m,
				"requests": v.reqs,
				"cost":     float64(v.cost) / float64(unit),
			})
		}
		sort.Slice(modelBreakdown, func(i, j int) bool {
			return modelBreakdown[i]["cost"].(float64) > modelBreakdown[j]["cost"].(float64)
		})
		httpx.OK(c, gin.H{
			"period":            period,
			"total_requests":    totalReq,
			"total_tokens":      totalTok,
			"total_cost":        float64(totalCost) / float64(unit),
			"daily_breakdown":   dailyBreakdown,
			"model_breakdown":   modelBreakdown,
		})
	}
}

func UsageExport(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		format := c.DefaultQuery("format", "csv")
		logs, err := client.UsageLog.Query().
			Where(usagelog.UserIDEQ(u.ID)).
			Order(ent.Desc(usagelog.FieldRequestTime)).
			Limit(5000).
			All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		if format == "json" {
			c.JSON(200, logs)
			return
		}
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=usage.csv")
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"time", "model", "prompt", "completion", "cost", "status"})
		for _, l := range logs {
			_ = w.Write([]string{
				l.RequestTime.Format(time.RFC3339),
				l.Model,
				strconv.Itoa(l.PromptTokens),
				strconv.Itoa(l.CompletionTokens),
				strconv.FormatInt(l.TotalCost, 10),
				strconv.Itoa(l.StatusCode),
			})
		}
		w.Flush()
	}
}

func ListAPIKeys(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		ctx := c.Request.Context()
		keys, err := client.APIKey.Query().
			Where(apikey.HasUserWith(user.IDEQ(u.ID))).
			WithGroup().
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		since24 := time.Now().Add(-24 * time.Hour)
		logs24, err := client.UsageLog.Query().
			Where(usagelog.UserIDEQ(u.ID), usagelog.RequestTimeGTE(since24), usagelog.APIKeyIDNotNil()).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		tok24h := map[int]int{}
		for _, l := range logs24 {
			tok24h[l.APIKeyID] += l.TotalTokens
		}
		var items []gin.H
		for _, k := range keys {
			st := "enabled"
			if !k.Enabled {
				st = "disabled"
			}
			gn := ""
			if k.Edges.Group != nil {
				gn = k.Edges.Group.Name
			}
			row := gin.H{
				"id":               k.ID,
				"name":             k.Name,
				"key_prefix":       k.KeyPrefix,
				"group_name":       gn,
				"status":           st,
				"last_used_at":     k.LastUsedAt,
				"created_at":       k.CreatedAt,
				"usage_24h":        tok24h[k.ID],
				"daily_quota":      k.DailyQuota,
				"ip_whitelist":     k.IPWhitelist,
				"log_full_request": k.LogFullRequest,
			}
			if !k.ExpiresAt.IsZero() {
				row["expires_at"] = k.ExpiresAt.UTC().Format(time.RFC3339)
			}
			items = append(items, row)
		}
		httpx.OK(c, gin.H{"items": items, "total": len(items)})
	}
}

func CreateAPIKey(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Name         string   `json:"name"`
			GroupID      *int     `json:"group_id"`
			IPWhitelist  []string `json:"ip_whitelist"`
			DailyQuota   *int64   `json:"daily_quota"`
			ExpiresAt    *string  `json:"expires_at"`
			LogFullReq   *bool    `json:"log_full_request"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		cnt, err := client.APIKey.Query().Where(apikey.HasUserWith(user.IDEQ(u.ID))).Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		if cnt >= maxAPIKeysPerUser {
			httpx.BadRequest(c, fmt.Sprintf("最多创建 %d 个 API Key", maxAPIKeysPerUser))
			return
		}
		rawRand := make([]byte, 24)
		if _, err := rand.Read(rawRand); err != nil {
			httpx.ServerError(c, "生成失败")
			return
		}
		prefix := cfg.Site.APIKeyPrefix
		if prefix == "" {
			prefix = "sk-th-"
		}
		raw := prefix + hex.EncodeToString(rawRand)
		hash := apikeyhash.Hash(raw)
		short := raw
		if len(short) > 16 {
			short = short[:16] + "..."
		}
		cr := client.APIKey.Create().
			SetUserID(u.ID).
			SetKeyHash(hash).
			SetKeyPrefix(short).
			SetName(req.Name)
		if len(req.IPWhitelist) > 0 {
			cr.SetIPWhitelist(req.IPWhitelist)
		}
		if req.GroupID != nil {
			cr.SetGroupID(*req.GroupID)
		}
		if req.DailyQuota != nil {
			cr.SetDailyQuota(*req.DailyQuota)
		}
		if req.LogFullReq != nil {
			cr.SetLogFullRequest(*req.LogFullReq)
		}
		if req.ExpiresAt != nil && *req.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err == nil {
				cr.SetExpiresAt(t)
			}
		}
		saved, err := cr.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		httpx.OKMsg(c, "API Key 创建成功", gin.H{
			"id":         saved.ID,
			"name":       saved.Name,
			"key":        raw,
			"key_prefix": short,
			"created_at": saved.CreatedAt,
		})
	}
}

func DeleteAPIKey(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		k, err := client.APIKey.Query().
			Where(apikey.IDEQ(id), apikey.HasUserWith(user.IDEQ(u.ID))).
			Only(ctx)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		if err := client.APIKey.DeleteOne(k).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		httpx.OKMsg(c, "已删除", nil)
	}
}

func UpdateAPIKey(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Name        *string  `json:"name"`
			IPWhitelist []string `json:"ip_whitelist"`
			DailyQuota  *int64   `json:"daily_quota"`
			Enabled     *bool    `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		k, err := client.APIKey.Query().
			Where(apikey.IDEQ(id), apikey.HasUserWith(user.IDEQ(u.ID))).
			Only(ctx)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		up := client.APIKey.UpdateOne(k)
		if req.Name != nil {
			up.SetName(*req.Name)
		}
		if req.IPWhitelist != nil {
			up.SetIPWhitelist(req.IPWhitelist)
		}
		if req.DailyQuota != nil {
			up.SetDailyQuota(*req.DailyQuota)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		httpx.OKMsg(c, "已更新", nil)
	}
}

func Redeem(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Code string `json:"code"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
			httpx.BadRequest(c, "请输入兑换码")
			return
		}
		ctx := c.Request.Context()
		code := strings.TrimSpace(req.Code)
		rc, err := client.RedeemCode.Query().
			Where(redeemcode.CodeEQ(code)).
			Only(ctx)
		if err != nil {
			httpx.BadRequest(c, "兑换码无效")
			return
		}
		if rc.Status != 1 {
			httpx.BadRequest(c, "兑换码不可用")
			return
		}
		now := time.Now()
		if !rc.ValidFrom.IsZero() && now.Before(rc.ValidFrom) {
			httpx.BadRequest(c, "兑换码尚未生效")
			return
		}
		if !rc.ValidUntil.IsZero() && now.After(rc.ValidUntil) {
			httpx.BadRequest(c, "兑换码已过期")
			return
		}
		if rc.UsedCount >= rc.MaxUses {
			httpx.BadRequest(c, "兑换码次数已用尽")
			return
		}
		var add int64
		switch rc.CodeType {
		case "fixed":
			add = rc.Amount
		default:
			add = rc.Amount
		}
		tx, err := client.Tx(ctx)
		if err != nil {
			httpx.ServerError(c, "兑换失败")
			return
		}
		if err := tx.User.UpdateOneID(u.ID).AddBalance(add).Exec(ctx); err != nil {
			_ = tx.Rollback()
			httpx.ServerError(c, "兑换失败")
			return
		}
		up := tx.RedeemCode.UpdateOne(rc).
			AddUsedCount(1).
			SetUsedAt(now).
			SetUsedByUserID(u.ID)
		if rc.UsedCount+1 >= rc.MaxUses {
			up.SetStatus(2)
		}
		if err := up.Exec(ctx); err != nil {
			_ = tx.Rollback()
			httpx.ServerError(c, "兑换失败")
			return
		}
		if err := tx.Commit(); err != nil {
			httpx.ServerError(c, "兑换失败")
			return
		}
		nu, _ := client.User.Get(ctx, u.ID)
		httpx.OKMsg(c, "兑换成功", gin.H{"amount_added": add, "new_balance": nu.Balance})
	}
}

func BindProfile(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Username   *string `json:"username"`
			WebhookURL *string `json:"webhook_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		up := client.User.UpdateOneID(u.ID)
		changed := false
		if req.Username != nil {
			up.SetUsername(strings.TrimSpace(*req.Username))
			changed = true
		}
		if req.WebhookURL != nil {
			v := strings.TrimSpace(*req.WebhookURL)
			if v == "" {
				up.ClearWebhookURL()
			} else {
				up.SetWebhookURL(v)
			}
			changed = true
		}
		if !changed {
			httpx.BadRequest(c, "请提供 username 或 webhook_url")
			return
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		httpx.OKMsg(c, "已更新", nil)
	}
}

func SetIPWhitelist(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			IPWhitelist []string `json:"ip_whitelist"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		if err := client.User.UpdateOneID(u.ID).SetIPWhitelist(req.IPWhitelist).Exec(ctx); err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		httpx.OKMsg(c, "已更新", nil)
	}
}

func ChangePassword(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Old string `json:"old_password"`
			New string `json:"new_password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.New == "" {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		fu, err := client.User.Get(ctx, u.ID)
		if err != nil {
			httpx.ServerError(c, "用户不存在")
			return
		}
		if fu.PasswordHash != "" && bcrypt.CompareHashAndPassword([]byte(fu.PasswordHash), []byte(req.Old)) != nil {
			httpx.BadRequest(c, "原密码错误")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.New), bcrypt.DefaultCost)
		if err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		if err := client.User.UpdateOneID(u.ID).SetPasswordHash(string(hash)).Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		httpx.OKMsg(c, "密码已更新", nil)
	}
}
