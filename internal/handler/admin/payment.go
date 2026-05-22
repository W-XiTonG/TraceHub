package adminhandler

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/paymentorder"
	"tracehub/internal/ent/rechargepackage"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/payment"

	"github.com/gin-gonic/gin"
)

type PaymentAdminDeps struct {
	Svc      *payment.Service
	AESKey   []byte
}

func GetPaymentConfig(d PaymentAdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := d.Svc.RuntimeConfig(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "加载失败")
			return
		}
		httpx.OK(c, payment.PublicConfig(cfg))
	}
}

func PutPaymentConfig(d PaymentAdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var patch map[string]any
		if err := c.ShouldBindJSON(&patch); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		cur, err := d.Svc.RuntimeConfig(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "加载失败")
			return
		}
		merged := payment.MergeUpdate(cur, patch)
		if err := payment.ValidateRuntimeConfig(merged); err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		if err := d.Svc.ConfigStore.Save(c.Request.Context(), merged, true); err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		if merged.OnlineEnabled {
			SeedDefaultPackages(d.Svc.Client)
		}
		auditLog(d.Svc.Client, adm.ID, "payment.config", "payment", "runtime", patch, c)
		httpx.OKMsg(c, "已保存", payment.PublicConfig(merged))
	}
}

func ListPaymentOrders(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
		if page < 1 {
			page = 1
		}
		if ps < 1 || ps > 100 {
			ps = 20
		}
		ctx := c.Request.Context()
		q := client.PaymentOrder.Query()
		if uid := strings.TrimSpace(c.Query("user_id")); uid != "" {
			if id, err := strconv.Atoi(uid); err == nil {
				q = q.Where(paymentorder.UserIDEQ(id))
			}
		}
		if st := strings.TrimSpace(c.Query("status")); st != "" {
			q = q.Where(paymentorder.StatusEQ(st))
		}
		if pr := strings.TrimSpace(c.Query("provider")); pr != "" {
			q = q.Where(paymentorder.ProviderEQ(pr))
		}
		total, _ := q.Clone().Count(ctx)
		list, err := q.Order(ent.Desc(paymentorder.FieldCreatedAt)).
			Offset((page - 1) * ps).Limit(ps).All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, o := range list {
			items = append(items, gin.H{
				"id":           o.ID,
				"order_no":     o.OrderNo,
				"user_id":      o.UserID,
				"provider":     o.Provider,
				"amount_cents": o.AmountCents,
				"quota_amount": o.QuotaAmount,
				"status":       o.Status,
				"created_at":   o.CreatedAt,
				"paid_at":      o.PaidAt,
			})
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func SyncPaymentOrder(d PaymentAdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		o, err := d.Svc.SyncOrder(c.Request.Context(), id)
		if err != nil {
			switch err {
			case payment.ErrOrderNotFound:
				httpx.BadRequest(c, "订单不存在")
			case payment.ErrSyncNotSupported:
				httpx.BadRequest(c, "仅易支付待支付订单可同步")
			case payment.ErrOrderNotPending:
				httpx.BadRequest(c, "订单状态不可同步")
			default:
				httpx.ServerError(c, err.Error())
			}
			return
		}
		auditLog(d.Svc.Client, adm.ID, "payment.order.sync", "payment_order", strconv.Itoa(id), gin.H{"status": o.Status}, c)
		httpx.OK(c, gin.H{
			"id":       o.ID,
			"order_no": o.OrderNo,
			"status":   o.Status,
			"paid_at":  o.PaidAt,
		})
	}
}

func RefundPaymentOrder(d PaymentAdminDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			httpx.BadRequest(c, "无效 id")
			return
		}
		var req struct {
			AmountCents  int64  `json:"amount_cents"`
			QuotaRevoke  int64  `json:"quota_revoke"`
			Reason       string `json:"reason"`
		}
		_ = c.ShouldBindJSON(&req)
		rf, err := d.Svc.CreateRefund(c.Request.Context(), id, req.AmountCents, req.QuotaRevoke, req.Reason, adm.ID)
		if err != nil {
			if err == payment.ErrInsufficientBal {
				httpx.BadRequest(c, "用户余额不足，无法扣回额度")
				return
			}
			httpx.ServerError(c, err.Error())
			return
		}
		auditLog(d.Svc.Client, adm.ID, "payment.refund", "payment_order", strconv.Itoa(id), gin.H{"refund_no": rf.RefundNo}, c)
		httpx.OK(c, gin.H{"refund_no": rf.RefundNo, "status": rf.Status})
	}
}

func ListRechargePackagesAdmin(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := client.RechargePackage.Query().
			Order(ent.Asc(rechargepackage.FieldSort), ent.Desc(rechargepackage.FieldID)).
			All(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		httpx.OK(c, gin.H{"items": list, "total": len(list)})
	}
}

func CreateRechargePackage(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Name        string `json:"name"`
			AmountCents int64  `json:"amount_cents"`
			QuotaAmount int64  `json:"quota_amount"`
			BonusQuota  int64  `json:"bonus_quota"`
			Currency    string `json:"currency"`
			Sort        int    `json:"sort"`
			Enabled     bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.QuotaAmount <= 0 {
			httpx.BadRequest(c, "参数无效")
			return
		}
		cur := strings.TrimSpace(req.Currency)
		if cur == "" {
			cur = "CNY"
		}
		p, err := client.RechargePackage.Create().
			SetName(req.Name).
			SetAmountCents(req.AmountCents).
			SetQuotaAmount(req.QuotaAmount).
			SetBonusQuota(req.BonusQuota).
			SetCurrency(cur).
			SetSort(req.Sort).
			SetEnabled(req.Enabled).
			Save(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "创建失败")
			return
		}
		auditLog(client, adm.ID, "payment.package.create", "recharge_package", strconv.Itoa(p.ID), gin.H{"name": req.Name, "amount_cents": req.AmountCents}, c)
		httpx.OK(c, p)
	}
}

func UpdateRechargePackage(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, _ := strconv.Atoi(c.Param("id"))
		var req struct {
			Name        *string `json:"name"`
			AmountCents *int64  `json:"amount_cents"`
			QuotaAmount *int64  `json:"quota_amount"`
			BonusQuota  *int64  `json:"bonus_quota"`
			Currency    *string `json:"currency"`
			Sort        *int    `json:"sort"`
			Enabled     *bool   `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		up := client.RechargePackage.UpdateOneID(id)
		if req.Name != nil {
			up.SetName(*req.Name)
		}
		if req.AmountCents != nil {
			up.SetAmountCents(*req.AmountCents)
		}
		if req.QuotaAmount != nil {
			up.SetQuotaAmount(*req.QuotaAmount)
		}
		if req.BonusQuota != nil {
			up.SetBonusQuota(*req.BonusQuota)
		}
		if req.Currency != nil {
			up.SetCurrency(*req.Currency)
		}
		if req.Sort != nil {
			up.SetSort(*req.Sort)
		}
		if req.Enabled != nil {
			up.SetEnabled(*req.Enabled)
		}
		p, err := up.Save(c.Request.Context())
		if err != nil {
			httpx.ServerError(c, "更新失败")
			return
		}
		auditLog(client, adm.ID, "payment.package.update", "recharge_package", strconv.Itoa(id), gin.H{"id": id}, c)
		httpx.OK(c, p)
	}
}

func DeleteRechargePackage(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		id, _ := strconv.Atoi(c.Param("id"))
		if err := client.RechargePackage.DeleteOneID(id).Exec(c.Request.Context()); err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		auditLog(client, adm.ID, "payment.package.delete", "recharge_package", strconv.Itoa(id), nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}

// SeedDefaultPackages 若无套餐则写入默认套餐。
func SeedDefaultPackages(client *ent.Client) {
	ctx := context.Background()
	n, _ := client.RechargePackage.Query().Count(ctx)
	if n > 0 {
		return
	}
	defaults := []struct {
		name string
		cent int64
		quota int64
	}{
		{"体验包 ¥10", 1000, 100000},
		{"标准包 ¥50", 5000, 500000},
		{"专业包 ¥200", 20000, 2000000},
	}
	for i, d := range defaults {
		_, _ = client.RechargePackage.Create().
			SetName(d.name).
			SetAmountCents(d.cent).
			SetQuotaAmount(d.quota).
			SetCurrency("CNY").
			SetSort(i).
			SetEnabled(true).
			Save(ctx)
	}
}

