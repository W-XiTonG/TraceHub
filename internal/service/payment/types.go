package payment

const (
	ProviderMock    = "mock"
	ProviderAlipay  = "alipay"
	ProviderWxpay   = "wxpay"
	ProviderStripe  = "stripe"
	ProviderEasyPay = "easypay"

	EasyPayTypeAlipay = "alipay"
	EasyPayTypeWxpay  = "wxpay"

	StatusPending        = "pending"
	StatusPaid           = "paid"
	StatusExpired        = "expired"
	StatusCancelled      = "cancelled"
	StatusRefunding      = "refunding"
	StatusRefunded       = "refunded"
	StatusPartialRefund  = "partial_refund"

	LedgerRecharge = "recharge"
	LedgerRefund   = "refund"

	RefundPending = "pending"
	RefundSuccess = "success"
	RefundFailed  = "failed"

	DefaultOrderTTL = 30 * 60 // seconds
)

// CreateResult 渠道下单结果。
type CreateResult struct {
	PayURL        string
	QRCode        string
	ProviderTxnID string
}

// NotifyResult 支付回调解析结果。
type NotifyResult struct {
	OrderNo       string
	ProviderTxnID string
	AmountCents   int64
	Paid          bool
}

// RefundResult 退款结果。
type RefundResult struct {
	ProviderRefundID string
	Success          bool
}
