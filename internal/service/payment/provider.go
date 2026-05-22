package payment

import (
	"context"
	"net/http"
)

// Provider 支付渠道适配器。
type Provider interface {
	Name() string
	CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error)
	QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (paid bool, providerTxnID string, err error)
	ParseNotify(ctx context.Context, cfg RuntimeConfig, r *http.Request) (*NotifyResult, error)
	Refund(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, reason string) (*RefundResult, error)
}

// OrderSnapshot 下单/回调所需订单字段。
type OrderSnapshot struct {
	OrderNo         string
	AmountCents     int64
	Currency        string
	QuotaAmount     int64
	UserID          int
	ProviderTxnID   string
	EasyPayType     string // alipay | wxpay，仅 easypay 渠道
}
