package adminhandler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/groupmodelrate"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/ent/user"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/cryptokey"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"
	"tracehub/internal/pkg/usagelogfilter"
	"tracehub/internal/service/billing"
	"tracehub/internal/service/channelhealth"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func auditLog(client *ent.Client, adminID int, action, resource, rid string, detail map[string]any, c *gin.Context) {
	ip := c.ClientIP()
	ua := c.GetHeader("User-Agent")
	cr := client.AdminAuditLog.Create().
		SetAdminID(adminID).
		SetAction(action).
		SetResource(resource).
		SetIPAddress(ip).
		SetUserAgent(ua)
	if rid != "" {
		cr.SetResourceID(rid)
	}
	if detail != nil {
		cr.SetDetail(detail)
	}
	_, _ = cr.Save(context.Background())
}

func ListUsers(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 200 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.User.Query().WithGroup()
		if em := strings.TrimSpace(c.Query("email")); em != "" {
			q = q.Where(user.EmailContainsFold(em))
		}
		total, err := q.Clone().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		us, err := q.
			Offset((page - 1) * ps).
			Limit(ps).
			Order(ent.Desc(user.FieldCreatedAt)).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		reqCounts := map[int]int{}
		if len(us) > 0 {
			ids := make([]int, 0, len(us))
			for _, u := range us {
				ids = append(ids, u.ID)
			}
			var rows []struct {
				UserID int `json:"user_id"`
				Count  int `json:"count"`
			}
			_ = client.UsageLog.Query().
				Where(usagelog.UserIDIn(ids...)).
				GroupBy(usagelog.FieldUserID).
				Aggregate(ent.As(ent.Count(), "count")).
				Scan(ctx, &rows)
			for _, r := range rows {
				reqCounts[r.UserID] = r.Count
			}
		}
		var items []gin.H
		for _, u := range us {
			gn := ""
			if u.Edges.Group != nil {
				gn = u.Edges.Group.Name
			}
			items = append(items, gin.H{
				"id":             u.ID,
				"email":          u.Email,
				"username":       u.Username,
				"role":           u.Role,
				"group_name":     gn,
				"balance":        u.Balance,
				"status":         u.Status,
				"total_requests": reqCounts[u.ID],
				"created_at":     u.CreatedAt,
				"last_login_at":  u.LastLoginAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func adminElevatedRole(r string) bool {
	return r == "admin" || r == "superadmin"
}

func UpdateUser(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		cl := c.MustGet(middleware.CtxClaimsKey).(*jwtx.Claims)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Status         *int      `json:"status"`
			GroupID        *int      `json:"group_id"`
			NoGroup        *bool     `json:"no_group"`
			RateMultiplier *float64  `json:"rate_multiplier"`
			BalanceAlert   *int64    `json:"balance_alert_threshold"`
			IPWhitelist    *[]string `json:"ip_whitelist"`
			IPBlacklist    *[]string `json:"ip_blacklist"`
			Email          *string   `json:"email"`
			Username       *string   `json:"username"`
			NewPassword    *string   `json:"new_password"`
			ClearTotp      *bool     `json:"clear_totp"`
			Balance        *int64    `json:"balance"`
			Role           *string   `json:"role"`
			Concurrency    *int      `json:"concurrency"`
			WebhookURL     *string   `json:"webhook_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		u, err := client.User.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "用户不存在")
			return
		}
		if adminElevatedRole(u.Role) && cl.Role != "superadmin" {
			httpx.Forbidden(c, "仅超级管理员可修改管理员账号")
			return
		}
		up := client.User.UpdateOne(u)
		changed := false

		if req.Email != nil {
			em := strings.TrimSpace(*req.Email)
			if em == "" || !strings.Contains(em, "@") {
				httpx.BadRequest(c, "邮箱无效")
				return
			}
			if em != u.Email {
				taken, err := client.User.Query().Where(user.EmailEQ(em), user.IDNEQ(id)).Exist(ctx)
				if err != nil {
					httpx.ServerError(c, "查询失败")
					return
				}
				if taken {
					httpx.BadRequest(c, "邮箱已被占用")
					return
				}
			}
			up.SetEmail(em)
			changed = true
		}
		if req.Username != nil {
			un := strings.TrimSpace(*req.Username)
			if un == "" {
				up.ClearUsername()
			} else {
				up.SetUsername(un)
			}
			changed = true
		}
		if req.NewPassword != nil {
			pw := strings.TrimSpace(*req.NewPassword)
			if pw != "" {
				if len(pw) < 6 {
					httpx.BadRequest(c, "新密码至少 6 位")
					return
				}
				hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
				if err != nil {
					httpx.ServerError(c, "密码处理失败")
					return
				}
				up.SetPasswordHash(string(hash))
				changed = true
			}
		}
		if req.ClearTotp != nil && *req.ClearTotp {
			up.ClearTotpSecret()
			changed = true
		}
		if req.Balance != nil {
			up.SetBalance(*req.Balance)
			changed = true
		}
		if req.Status != nil {
			if *req.Status != 1 && *req.Status != 2 {
				httpx.BadRequest(c, "状态无效")
				return
			}
			up.SetStatus(*req.Status)
			changed = true
		}
		if req.Role != nil {
			r := strings.TrimSpace(*req.Role)
			if r != "user" && r != "admin" && r != "superadmin" {
				httpx.BadRequest(c, "角色无效")
				return
			}
			if adminElevatedRole(r) || adminElevatedRole(u.Role) {
				if cl.Role != "superadmin" {
					httpx.Forbidden(c, "仅超级管理员可调整管理员角色")
					return
				}
			}
			if id == adm.ID && adminElevatedRole(u.Role) && !adminElevatedRole(r) {
				httpx.BadRequest(c, "不可将当前登录账号降级为普通用户")
				return
			}
			up.SetRole(r)
			changed = true
		}
		if req.NoGroup != nil && *req.NoGroup {
			up.ClearGroupID()
			changed = true
		} else if req.GroupID != nil {
			if *req.GroupID <= 0 {
				httpx.BadRequest(c, "分组 id 无效")
				return
			}
			_, err := client.Group.Query().Where(group.IDEQ(*req.GroupID)).Only(ctx)
			if err != nil {
				if ent.IsNotFound(err) {
					httpx.BadRequest(c, "分组不存在")
					return
				}
				httpx.ServerError(c, "查询失败")
				return
			}
			up.SetGroupID(*req.GroupID)
			changed = true
		}
		if req.RateMultiplier != nil {
			if *req.RateMultiplier <= 0 {
				httpx.BadRequest(c, "倍率须大于 0")
				return
			}
			up.SetRateMultiplier(*req.RateMultiplier)
			changed = true
		}
		if req.BalanceAlert != nil {
			up.SetBalanceAlertThreshold(*req.BalanceAlert)
			changed = true
		}
		if req.Concurrency != nil {
			if *req.Concurrency < 0 {
				httpx.BadRequest(c, "并发不能为负")
				return
			}
			up.SetConcurrency(*req.Concurrency)
			changed = true
		}
		if req.WebhookURL != nil {
			w := strings.TrimSpace(*req.WebhookURL)
			if w == "" {
				up.ClearWebhookURL()
			} else {
				up.SetWebhookURL(w)
			}
			changed = true
		}
		if req.IPWhitelist != nil {
			up.SetIPWhitelist(*req.IPWhitelist)
			changed = true
		}
		if req.IPBlacklist != nil {
			up.SetIPBlacklist(*req.IPBlacklist)
			changed = true
		}
		if !changed {
			httpx.BadRequest(c, "无更新字段")
			return
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		pwSet := req.NewPassword != nil && strings.TrimSpace(*req.NewPassword) != ""
		auditDetail := map[string]any{
			"status": req.Status, "group_id": req.GroupID, "no_group": req.NoGroup,
			"rate_multiplier": req.RateMultiplier, "balance_alert_threshold": req.BalanceAlert,
			"ip_whitelist": req.IPWhitelist != nil, "ip_blacklist": req.IPBlacklist != nil,
			"email": req.Email, "username": req.Username, "password_reset": pwSet,
			"clear_totp": req.ClearTotp, "balance": req.Balance, "role": req.Role, "concurrency": req.Concurrency, "webhook_touched": req.WebhookURL != nil,
		}
		auditLog(client, adm.ID, "user.update", "user", strconv.Itoa(id), auditDetail, c)
		httpx.OKMsg(c, "已更新", nil)
	}
}

func AdjustBalance(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Delta int64 `json:"delta"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		if err := client.User.UpdateOneID(id).AddBalance(req.Delta).Exec(ctx); err != nil {
			httpx.ServerError(c, "调整失败")
			return
		}
		auditLog(client, adm.ID, "user.balance", "user", strconv.Itoa(id), map[string]any{"delta": req.Delta}, c)
		httpx.OKMsg(c, "已调整", nil)
	}
}

func ListChannels(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		chs, err := client.Channel.Query().WithGroup().All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		var items []gin.H
		for _, ch := range chs {
			items = append(items, gin.H{
				"id":                    ch.ID,
				"group_id":              ch.GroupID,
				"channel_type":          ch.ChannelType,
				"channel_name":          ch.ChannelName,
				"auth_type":             ch.AuthType,
				"api_base_url":          ch.APIBaseURL,
				"models":                ch.Models,
				"model_alias":           ch.ModelAlias,
				"model_upstream_probe":  ch.ModelUpstreamProbe,
				"priority":              ch.Priority,
				"weight":                ch.Weight,
				"concurrency":           ch.Concurrency,
				"enabled":               ch.Enabled,
				"health_status":         ch.HealthStatus,
				"schedulable":           ch.Schedulable,
				"error_count":           ch.ErrorCount,
				"last_error_msg":        ch.LastErrorMsg,
				"has_api_secret":        ch.APIKey != "" || ch.OauthToken != "",
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": len(items)})
	}
}

type ChannelDeps struct {
	Client *ent.Client
	AESKey []byte
}

func CreateChannel(d ChannelDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			GroupID      int               `json:"group_id"`
			ChannelType  string            `json:"channel_type"`
			ChannelName  string            `json:"channel_name"`
			AuthType     channel.AuthType  `json:"auth_type"`
			APIKey       string            `json:"api_key"`
			OauthToken   string            `json:"oauth_token"`
			OauthRefresh string            `json:"oauth_refresh_token"`
			SetupToken   string            `json:"setup_token"`
			APIBaseURL   string            `json:"api_base_url"`
			Models       []string          `json:"models"`
			ModelAlias   map[string]string `json:"model_alias"`
			Priority     int               `json:"priority"`
			Weight       int               `json:"weight"`
			Concurrency  int               `json:"concurrency"`
			Enabled      bool              `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.ChannelType == "" || req.GroupID == 0 {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		cr := d.Client.Channel.Create().
			SetGroupID(req.GroupID).
			SetChannelType(req.ChannelType).
			SetChannelName(req.ChannelName).
			SetAuthType(req.AuthType).
			SetAPIBaseURL(req.APIBaseURL).
			SetModels(req.Models).
			SetPriority(req.Priority).
			SetWeight(req.Weight).
			SetConcurrency(req.Concurrency).
			SetEnabled(req.Enabled)
		if len(req.ModelAlias) > 0 {
			cr.SetModelAlias(req.ModelAlias)
		}
		if req.APIKey != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(req.APIKey))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			cr.SetAPIKey(enc)
		}
		if req.OauthToken != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(req.OauthToken))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			cr.SetOauthToken(enc)
		}
		if req.OauthRefresh != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(req.OauthRefresh))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			cr.SetOauthRefreshToken(enc)
		}
		if req.SetupToken != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(req.SetupToken))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			cr.SetSetupToken(enc)
		}
		ch, err := cr.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(d.Client, adm.ID, "channel.create", "channel", strconv.Itoa(ch.ID), map[string]any{"type": req.ChannelType}, c)
		httpx.OKMsg(c, "创建成功", gin.H{"id": ch.ID})
	}
}

func UpdateChannel(d ChannelDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			ChannelName string            `json:"channel_name"`
			APIKey      *string           `json:"api_key"`
			APIBaseURL  *string           `json:"api_base_url"`
			Models      []string          `json:"models"`
			ModelAlias  map[string]string `json:"model_alias"`
			Priority    *int              `json:"priority"`
			Weight      *int              `json:"weight"`
			Concurrency *int              `json:"concurrency"`
			Enabled     *bool             `json:"enabled"`
			Schedulable *bool             `json:"schedulable"`
			Health       *int              `json:"health_status"`
			ErrorCount   *int              `json:"error_count"`
			ResetErrors  *bool             `json:"reset_error_count"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		ch, err := d.Client.Channel.Get(ctx, id)
		if err != nil {
			httpx.BadRequest(c, "不存在")
			return
		}
		up := d.Client.Channel.UpdateOne(ch)
		if req.ChannelName != "" {
			up.SetChannelName(req.ChannelName)
		}
		if req.APIBaseURL != nil {
			up.SetAPIBaseURL(*req.APIBaseURL)
		}
		if len(req.Models) > 0 {
			up.SetModels(req.Models)
		}
		if req.ModelAlias != nil {
			up.SetModelAlias(req.ModelAlias)
		}
		if req.Priority != nil {
			up.SetPriority(*req.Priority)
		}
		if req.Weight != nil {
			up.SetWeight(*req.Weight)
		}
		if req.Concurrency != nil {
			up.SetConcurrency(*req.Concurrency)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		if req.Schedulable != nil {
			up.SetSchedulable(*req.Schedulable)
		}
		if req.Health != nil {
			up.SetHealthStatus(*req.Health)
		}
		if req.ResetErrors != nil && *req.ResetErrors {
			if err := channelhealth.ResetErrorCount(ctx, d.Client, id); err != nil {
				httpx.ServerError(c, "重置错误计数失败")
				return
			}
		} else if req.ErrorCount != nil {
			up.SetErrorCount(*req.ErrorCount)
		}
		if req.APIKey != nil && *req.APIKey != "" {
			enc, err := cryptokey.EncryptGCM(d.AESKey, []byte(*req.APIKey))
			if err != nil {
				httpx.ServerError(c, "加密失败")
				return
			}
			up.SetAPIKey(enc)
		}
		if err := up.Exec(ctx); err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(d.Client, adm.ID, "channel.update", "channel", strconv.Itoa(id), map[string]any{"body": req}, c)
		httpx.OKMsg(c, "已更新", nil)
	}
}

func DeleteChannel(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		ctx := c.Request.Context()
		if err := client.Channel.DeleteOneID(id).Exec(ctx); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "channel.delete", "channel", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

func ListGroups(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		gs, err := client.Group.Query().All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": gs, "total": len(gs)})
	}
}

func CreateGroup(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Name            string             `json:"name"`
			Description     string             `json:"description"`
			RateMultiplier  float64            `json:"rate_multiplier"`
			RPMLimit        int                `json:"rpm_limit"`
			ModelRouting    map[string][]int64 `json:"model_routing"`
			RoutingStrategy string             `json:"routing_strategy"`
			RoutingOptions  map[string]any     `json:"routing_options"`
			DefaultModel    string             `json:"default_model"`
			Enabled         bool               `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		cr := client.Group.Create().
			SetName(req.Name).
			SetDescription(req.Description).
			SetRateMultiplier(req.RateMultiplier).
			SetRpmLimit(req.RPMLimit).
			SetDefaultModel(req.DefaultModel).
			SetEnabled(req.Enabled)
		if len(req.ModelRouting) > 0 {
			cr.SetModelRouting(req.ModelRouting)
		}
		if req.RoutingStrategy != "" {
			if err := validateGroupRoutingStrategy(req.RoutingStrategy); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			cr.SetRoutingStrategy(req.RoutingStrategy)
		}
		if len(req.RoutingOptions) > 0 {
			if err := validateGroupRoutingOptions(req.RoutingOptions); err != nil {
				httpx.BadRequest(c, err.Error())
				return
			}
			cr.SetRoutingOptions(req.RoutingOptions)
		}
		g, err := cr.Save(ctx)
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(client, adm.ID, "group.create", "group", strconv.Itoa(g.ID), nil, c)
		httpx.OKMsg(c, "创建成功", gin.H{"id": g.ID})
	}
}

func PutGroupModelRates(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		gid, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			Rates []struct {
				ModelName  string  `json:"model_name"`
				InputRate  float64 `json:"input_rate"`
				OutputRate float64 `json:"output_rate"`
				Enabled    bool    `json:"enabled"`
				Billable   *bool   `json:"billable"`
			} `json:"rates"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		for _, r := range req.Rates {
			existing, err := client.GroupModelRate.Query().
				Where(groupmodelrate.GroupIDEQ(gid), groupmodelrate.ModelNameEQ(r.ModelName)).
				Only(ctx)
			billable := true
			if r.Billable != nil {
				billable = *r.Billable
			}
			if ent.IsNotFound(err) {
				_, err = client.GroupModelRate.Create().
					SetGroupID(gid).
					SetModelName(r.ModelName).
					SetInputRate(r.InputRate).
					SetOutputRate(r.OutputRate).
					SetEnabled(r.Enabled).
					SetBillable(billable).
					Save(ctx)
			} else if err == nil {
				up := client.GroupModelRate.UpdateOne(existing).
					SetInputRate(r.InputRate).
					SetOutputRate(r.OutputRate).
					SetEnabled(r.Enabled).
					SetBillable(billable)
				err = up.Exec(ctx)
			}
			if err != nil {
				httpx.ServerError(c, "保存倍率失败")
				return
			}
		}
		billing.InvalidateGroupModelRateCacheAdmin(gid)
		auditLog(client, adm.ID, "group.model_rates", "group", strconv.Itoa(gid), map[string]any{"count": len(req.Rates)}, c)
		httpx.OKMsg(c, "已保存", nil)
	}
}

func ListLogs(client *ent.Client) gin.HandlerFunc {
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
		q := client.UsageLog.Query()
		if uid := c.Query("user_id"); uid != "" {
			id, _ := strconv.Atoi(uid)
			q = q.Where(usagelog.UserIDEQ(id))
		}
		if em := strings.TrimSpace(c.Query("email")); em != "" {
			q = q.Where(usagelog.HasUserWith(user.EmailContainsFold(em)))
		}
		if model := c.Query("model"); model != "" {
			q = q.Where(usagelog.ModelEQ(model))
		}
		if ch := c.Query("channel_id"); ch != "" {
			id, _ := strconv.Atoi(ch)
			q = q.Where(usagelog.ChannelIDEQ(id))
		}
		if sc := c.Query("status_code"); sc != "" {
			id, _ := strconv.Atoi(sc)
			q = q.Where(usagelog.StatusCodeEQ(id))
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
		logs, err := q.WithUser().WithAPIKey().WithChannel().
			Order(ent.Desc(usagelog.FieldRequestTime)).
			Offset((page - 1) * ps).
			Limit(ps).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(logs))
		for _, lg := range logs {
			em := ""
			if lg.Edges.User != nil {
				em = lg.Edges.User.Email
			}
			keyName := ""
			keyPrefix := ""
			if lg.Edges.APIKey != nil {
				keyName = strings.TrimSpace(lg.Edges.APIKey.Name)
				keyPrefix = strings.TrimSpace(lg.Edges.APIKey.KeyPrefix)
			}
			chLabel := ""
			chType := ""
			if lg.Edges.Channel != nil {
				ch := lg.Edges.Channel
				chType = ch.ChannelType
				if strings.TrimSpace(ch.ChannelName) != "" {
					chLabel = strings.TrimSpace(ch.ChannelName)
				} else {
					chLabel = "#" + strconv.Itoa(ch.ID)
				}
			}
			items = append(items, gin.H{
				"id":                lg.ID,
				"user_id":           lg.UserID,
				"user_email":        em,
				"api_key_id":        lg.APIKeyID,
				"api_key_name":      keyName,
				"api_key_prefix":    keyPrefix,
				"channel_id":        lg.ChannelID,
				"channel_name":      chLabel,
				"channel_type":      chType,
				"request_id":        lg.RequestID,
				"model":             lg.Model,
				"model_upstream":    lg.ModelUpstream,
				"prompt_tokens":     lg.PromptTokens,
				"completion_tokens": lg.CompletionTokens,
				"total_tokens":      lg.TotalTokens,
				"total_cost":        lg.TotalCost,
				"status_code":       lg.StatusCode,
				"response_time_ms":  lg.ResponseTimeMs,
				"request_time":      lg.RequestTime,
				"ip_address":        lg.IPAddress,
				"error_message":     lg.ErrorMessage,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func BatchRedeemCodes(client *ent.Client) gin.HandlerFunc {
	const (
		maxRedeemCodeAmount int64 = 100_000_000_000_000 // 单码面额上限（内部额度单位）
		maxRedeemUses             = 10_000_000
		maxRedeemPrefixLen        = 64
	)
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Count     int    `json:"count"`
			CodeType  string `json:"code_type"`
			Amount    int64  `json:"amount"`
			MaxUses   int    `json:"max_uses"`
			ValidDays int    `json:"valid_days"`
			Prefix    string `json:"prefix"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Count < 1 || req.Count > 500 {
			httpx.BadRequest(c, "count 须在 1~500")
			return
		}
		if req.Amount <= 0 || req.Amount > maxRedeemCodeAmount {
			httpx.BadRequest(c, "amount 须为正数且不超过单码上限")
			return
		}
		if req.ValidDays < 0 {
			httpx.BadRequest(c, "valid_days 不能为负")
			return
		}
		if req.CodeType == "" {
			req.CodeType = "fixed"
		}
		if req.CodeType != "fixed" {
			httpx.BadRequest(c, "code_type 当前仅支持 fixed")
			return
		}
		if req.MaxUses < 1 {
			req.MaxUses = 1
		}
		if req.MaxUses > maxRedeemUses {
			httpx.BadRequest(c, "max_uses 超出上限")
			return
		}
		pfx := strings.TrimSpace(req.Prefix)
		if len(pfx) > maxRedeemPrefixLen {
			httpx.BadRequest(c, "prefix 过长")
			return
		}
		ctx := c.Request.Context()
		var codes []string
		until := time.Now().Add(time.Duration(req.ValidDays) * 24 * time.Hour)
		if pfx == "" {
			pfx = "TH-"
		}
		for i := 0; i < req.Count; i++ {
			buf := make([]byte, 16)
			_, _ = rand.Read(buf)
			code := pfx + strings.ToUpper(hex.EncodeToString(buf))
			cr := client.RedeemCode.Create().
				SetCode(code).
				SetCodeType(req.CodeType).
				SetAmount(req.Amount).
				SetMaxUses(req.MaxUses).
				SetStatus(1).
				SetCreatedBy(adm.ID)
			if req.ValidDays > 0 {
				cr.SetValidUntil(until)
			}
			if _, err := cr.Save(ctx); err != nil {
				httpx.ServerError(c, "生成失败")
				return
			}
			codes = append(codes, code)
		}
		auditLog(client, adm.ID, "redeem.batch", "redeem", "", map[string]any{
			"count":      len(codes),
			"amount":     req.Amount,
			"max_uses":   req.MaxUses,
			"valid_days": req.ValidDays,
		}, c)
		httpx.OK(c, gin.H{"codes": codes})
	}
}

// usageStatsRange 对 [start, end) 时间内的用量日志做 COUNT / SUM 聚合（避免全量拉取）。
func usageStatsRange(ctx context.Context, client *ent.Client, start, end time.Time) (requests int64, tokens int64, cost int64, err error) {
	var rows []struct {
		Cnt            int    `json:"count"`
		SumTotalCost   *int64 `json:"sum_total_cost"`
		SumTotalTokens *int64 `json:"sum_total_tokens"`
	}
	err = client.UsageLog.Query().
		Where(usagelog.RequestTimeGTE(start), usagelog.RequestTimeLT(end)).
		Aggregate(
			ent.As(ent.Count(), "count"),
			ent.As(ent.Sum(usagelog.FieldTotalCost), "sum_total_cost"),
			ent.As(ent.Sum(usagelog.FieldTotalTokens), "sum_total_tokens"),
		).
		Scan(ctx, &rows)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(rows) == 0 {
		return 0, 0, 0, nil
	}
	r := rows[0]
	requests = int64(r.Cnt)
	if r.SumTotalTokens != nil {
		tokens = *r.SumTotalTokens
	}
	if r.SumTotalCost != nil {
		cost = *r.SumTotalCost
	}
	return requests, tokens, cost, nil
}

func pctVsYesterday(curr, prev int64) *float64 {
	if prev == 0 {
		if curr == 0 {
			z := 0.0
			return &z
		}
		return nil
	}
	v := 100 * float64(curr-prev) / float64(prev)
	v = math.Round(v*10) / 10
	return &v
}

func DashboardSummary(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		users, err := client.User.Query().Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		channels, err := client.Channel.Query().Where(channel.Enabled(true)).Count(ctx)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		unit := int64(10000)
		if cfg != nil && cfg.Site.CurrencyQuotaPerYuan > 0 {
			unit = cfg.Site.CurrencyQuotaPerYuan
		}
		quotaToYuan := func(q int64) float64 {
			x := float64(q) / float64(unit)
			return math.Round(x*100) / 100
		}

		now := time.Now()
		todayStart := now.Truncate(24 * time.Hour)
		tomorrowStart := todayStart.Add(24 * time.Hour)
		yesterdayStart := todayStart.Add(-24 * time.Hour)
		// 自然日「近 7 日」含当日：从今日往前共 7 天
		sevenDayStart := todayStart.AddDate(0, 0, -6)

		todayReq, todayTok, todayCost, err := usageStatsRange(ctx, client, todayStart, tomorrowStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		yReq, yTok, yCost, err := usageStatsRange(ctx, client, yesterdayStart, todayStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}
		d7Req, d7Tok, d7Cost, err := usageStatsRange(ctx, client, sevenDayStart, tomorrowStart)
		if err != nil {
			httpx.ServerError(c, "统计失败")
			return
		}

		httpx.OK(c, gin.H{
			"total_users":             users,
			"enabled_channels":        channels,
			"today_requests":          todayReq,
			"today_tokens":            todayTok,
			"today_cost_quota":        todayCost,
			"today_cost_yuan":         quotaToYuan(todayCost),
			"currency_quota_per_yuan": unit,

			"yesterday_requests":   yReq,
			"yesterday_tokens":     yTok,
			"yesterday_cost_quota": yCost,
			"yesterday_cost_yuan":  quotaToYuan(yCost),

			"last_7d_requests":   d7Req,
			"last_7d_tokens":     d7Tok,
			"last_7d_cost_quota": d7Cost,
			"last_7d_cost_yuan":  quotaToYuan(d7Cost),

			"vs_yesterday_requests_pct": pctVsYesterday(todayReq, yReq),
			"vs_yesterday_tokens_pct":   pctVsYesterday(todayTok, yTok),
			"vs_yesterday_cost_pct":     pctVsYesterday(todayCost, yCost),
		})
	}
}
