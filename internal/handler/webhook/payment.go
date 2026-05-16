package webhookhandler

import (
	"net/http"
	"strings"

	"tracehub/internal/service/payment"

	"github.com/gin-gonic/gin"
)

type PaymentWebhookDeps struct {
	Svc *payment.Service
}

func PaymentNotify(d PaymentWebhookDeps, provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !d.Svc.OnlineEnabled(c.Request.Context()) {
			c.String(http.StatusNotFound, "fail")
			return
		}
		cfg, err := d.Svc.RuntimeConfig(c.Request.Context())
		if err != nil {
			c.String(http.StatusInternalServerError, "fail")
			return
		}
		p, ok := d.Svc.Provider(provider)
		if !ok {
			c.String(http.StatusNotFound, "fail")
			return
		}
		notify, err := p.ParseNotify(c.Request.Context(), cfg, c.Request)
		if err != nil || notify == nil {
			c.String(http.StatusBadRequest, "fail")
			return
		}
		if !notify.Paid {
			c.String(http.StatusOK, "success")
			return
		}
		if err := d.Svc.SettleOrder(c.Request.Context(), notify.OrderNo, notify.ProviderTxnID, notify.AmountCents); err != nil {
			if strings.Contains(err.Error(), "pending") {
				c.String(http.StatusOK, "success")
				return
			}
			c.String(http.StatusInternalServerError, "fail")
			return
		}
		c.String(http.StatusOK, "success")
	}
}

// MockPay 开发环境模拟支付成功（GET/POST）。
func MockPay(d PaymentWebhookDeps) gin.HandlerFunc {
	return PaymentNotify(d, payment.ProviderMock)
}
