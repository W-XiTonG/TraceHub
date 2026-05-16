package payment

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type mockProvider struct{}

func (mockProvider) Name() string { return ProviderMock }

func (mockProvider) CreatePayment(_ context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.SiteBaseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	payURL := fmt.Sprintf("%s/webhook/payment/mock/pay?order_no=%s", base, order.OrderNo)
	return &CreateResult{
		PayURL:        payURL,
		QRCode:        payURL,
		ProviderTxnID: "mock-" + order.OrderNo,
	}, nil
}

func (mockProvider) QueryPayment(_ context.Context, _ RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	return false, "mock-" + order.OrderNo, nil
}

func (mockProvider) ParseNotify(_ context.Context, _ RuntimeConfig, r *http.Request) (*NotifyResult, error) {
	orderNo := strings.TrimSpace(r.URL.Query().Get("order_no"))
	if orderNo == "" {
		orderNo = strings.TrimSpace(r.FormValue("order_no"))
	}
	if orderNo == "" {
		return nil, fmt.Errorf("missing order_no")
	}
	amountStr := strings.TrimSpace(r.URL.Query().Get("amount_cents"))
	if amountStr == "" {
		amountStr = strings.TrimSpace(r.FormValue("amount_cents"))
	}
	var amount int64
	if amountStr != "" {
		fmt.Sscanf(amountStr, "%d", &amount)
	}
	return &NotifyResult{
		OrderNo:       orderNo,
		ProviderTxnID: "mock-" + orderNo,
		AmountCents:   amount,
		Paid:          true,
	}, nil
}

func (mockProvider) Refund(_ context.Context, _ RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, _ string) (*RefundResult, error) {
	return &RefundResult{
		ProviderRefundID: "mock-refund-" + refundNo,
		Success:          true,
	}, nil
}
