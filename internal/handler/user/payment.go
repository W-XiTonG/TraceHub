package userhandler

import (
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/invoiceprofile"
	"tracehub/internal/ent/paymentorder"
	"tracehub/internal/ent/rechargepackage"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/payment"

	"github.com/gin-gonic/gin"
)

type PaymentDeps struct {
	Svc *payment.Service
}

func ListRechargePackages(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		list, err := client.RechargePackage.Query().
			Where(rechargepackage.EnabledEQ(true)).
			Order(ent.Asc(rechargepackage.FieldSort), ent.Desc(rechargepackage.FieldID)).
			All(ctx)
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		items := make([]gin.H, 0, len(list))
		for _, p := range list {
			items = append(items, gin.H{
				"id":            p.ID,
				"name":          p.Name,
				"amount_cents":  p.AmountCents,
				"quota_amount":  p.QuotaAmount,
				"bonus_quota":   p.BonusQuota,
				"currency":      p.Currency,
			})
		}
		httpx.OK(c, gin.H{"items": items})
	}
}

func CreatePaymentOrder(d PaymentDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Provider          string `json:"provider"`
			PackageID         *int   `json:"package_id"`
			CustomAmountCents *int64 `json:"custom_amount_cents"`
			Currency          string `json:"currency"`
			InvoiceRequested  bool   `json:"invoice_requested"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		out, err := d.Svc.CreateOrder(c.Request.Context(), payment.CreateOrderInput{
			UserID:            u.ID,
			Provider:          req.Provider,
			PackageID:         req.PackageID,
			CustomAmountCents: req.CustomAmountCents,
			Currency:          req.Currency,
			InvoiceRequested:  req.InvoiceRequested,
		})
		if err != nil {
			switch err {
			case payment.ErrOnlineDisabled:
				httpx.Forbidden(c, "在线支付功能未开通")
			case payment.ErrProviderDisabled:
				httpx.BadRequest(c, "该支付渠道未启用")
			case payment.ErrInvalidAmount:
				httpx.BadRequest(c, "金额无效")
			default:
				httpx.ServerError(c, err.Error())
			}
			return
		}
		httpx.OK(c, out)
	}
}

func ListPaymentOrders(client *ent.Client) gin.HandlerFunc {
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
		q := client.PaymentOrder.Query().Where(paymentorder.UserIDEQ(u.ID))
		if st := strings.TrimSpace(c.Query("status")); st != "" {
			q = q.Where(paymentorder.StatusEQ(st))
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
			items = append(items, paymentOrderJSON(o))
		}
		httpx.OK(c, gin.H{"items": items, "total": total, "page": page, "page_size": ps})
	}
}

func GetPaymentOrder(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		orderNo := strings.TrimSpace(c.Param("order_no"))
		o, err := client.PaymentOrder.Query().
			Where(paymentorder.OrderNoEQ(orderNo), paymentorder.UserIDEQ(u.ID)).
			Only(c.Request.Context())
		if err != nil {
			httpx.BadRequest(c, "订单不存在")
			return
		}
		httpx.OK(c, paymentOrderJSON(o))
	}
}

func RefundRequestPaymentOrder(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		orderNo := strings.TrimSpace(c.Param("order_no"))
		o, err := client.PaymentOrder.Query().
			Where(paymentorder.OrderNoEQ(orderNo), paymentorder.UserIDEQ(u.ID)).
			Only(c.Request.Context())
		if err != nil || o.Status != "paid" {
			httpx.BadRequest(c, "订单不可申请退款")
			return
		}
		httpx.OKMsg(c, "退款申请已提交，请等待管理员处理", nil)
	}
}

func CancelPaymentOrder(d PaymentDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		orderNo := strings.TrimSpace(c.Param("order_no"))
		if err := d.Svc.CancelOrder(c.Request.Context(), u.ID, orderNo); err != nil {
			httpx.BadRequest(c, "无法取消")
			return
		}
		httpx.OKMsg(c, "已取消", nil)
	}
}

func GetInvoiceProfile(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		p, err := client.InvoiceProfile.Query().Where(invoiceprofile.UserIDEQ(u.ID)).Only(c.Request.Context())
		if err != nil {
			httpx.OK(c, gin.H{})
			return
		}
		httpx.OK(c, invoiceProfileJSON(p))
	}
}

func PutInvoiceProfile(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		u := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			TitleType   string `json:"title_type"`
			CompanyName string `json:"company_name"`
			TaxID       string `json:"tax_id"`
			Address     string `json:"address"`
			Email       string `json:"email"`
			Phone       string `json:"phone"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		existing, err := client.InvoiceProfile.Query().Where(invoiceprofile.UserIDEQ(u.ID)).Only(ctx)
		if ent.IsNotFound(err) {
			p, err := client.InvoiceProfile.Create().
				SetUserID(u.ID).
				SetTitleType(strings.TrimSpace(req.TitleType)).
				SetCompanyName(req.CompanyName).
				SetTaxID(req.TaxID).
				SetAddress(req.Address).
				SetEmail(req.Email).
				SetPhone(req.Phone).
				Save(ctx)
			if err != nil {
				httpx.ServerError(c, "保存失败")
				return
			}
			httpx.OK(c, invoiceProfileJSON(p))
			return
		}
		if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		}
		p, err := existing.Update().
			SetTitleType(strings.TrimSpace(req.TitleType)).
			SetCompanyName(req.CompanyName).
			SetTaxID(req.TaxID).
			SetAddress(req.Address).
			SetEmail(req.Email).
			SetPhone(req.Phone).
			Save(ctx)
		if err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		httpx.OK(c, invoiceProfileJSON(p))
	}
}

func paymentOrderJSON(o *ent.PaymentOrder) gin.H {
	h := gin.H{
		"id":                o.ID,
		"order_no":          o.OrderNo,
		"provider":          o.Provider,
		"amount_cents":      o.AmountCents,
		"currency":          o.Currency,
		"quota_amount":      o.QuotaAmount,
		"status":            o.Status,
		"pay_url":           o.PayURL,
		"qr_code":           o.QrCode,
		"package_id":        o.PackageID,
		"invoice_requested": o.InvoiceRequested,
		"expire_at":         o.ExpireAt,
		"paid_at":           o.PaidAt,
		"created_at":        o.CreatedAt,
	}
	if o.ProviderTxnID != nil {
		h["provider_txn_id"] = *o.ProviderTxnID
	}
	return h
}

func invoiceProfileJSON(p *ent.InvoiceProfile) gin.H {
	return gin.H{
		"title_type":   p.TitleType,
		"company_name": p.CompanyName,
		"tax_id":       p.TaxID,
		"address":      p.Address,
		"email":        p.Email,
		"phone":        p.Phone,
		"updated_at":   p.UpdatedAt,
	}
}
