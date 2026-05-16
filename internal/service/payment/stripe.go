package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

type stripeProvider struct{}

func (stripeProvider) Name() string { return ProviderStripe }

func (stripeProvider) CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	cur := strings.ToLower(strings.TrimSpace(order.Currency))
	if cur == "" {
		cur = "cny"
	}
	form := url.Values{}
	form.Set("mode", "payment")
	form.Set("client_reference_id", order.OrderNo)
	form.Set("success_url", successURL(cfg, order.OrderNo))
	form.Set("cancel_url", cancelURL(cfg, order.OrderNo))
	form.Set("line_items[0][price_data][currency]", cur)
	form.Set("line_items[0][price_data][unit_amount]", fmt.Sprintf("%d", order.AmountCents))
	form.Set("line_items[0][price_data][product_data][name]", "TraceHub Balance")
	form.Set("line_items[0][quantity]", "1")
	form.Set("metadata[order_no]", order.OrderNo)
	body, err := stripeAPI(ctx, cfg, http.MethodPost, "/v1/checkout/sessions", form)
	if err != nil {
		return nil, err
	}
	var sess struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &sess); err != nil {
		return nil, err
	}
	return &CreateResult{PayURL: sess.URL, ProviderTxnID: sess.ID}, nil
}

func (stripeProvider) QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	return false, "", nil
}

func (stripeProvider) ParseNotify(ctx context.Context, cfg RuntimeConfig, r *http.Request) (*NotifyResult, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if sec := strings.TrimSpace(cfg.StripeWebhookSecret); sec != "" {
		if !verifyStripeSig(body, r.Header.Get("Stripe-Signature"), sec) {
			return nil, fmt.Errorf("stripe signature invalid")
		}
	}
	var evt struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ClientReferenceID string `json:"client_reference_id"`
				PaymentStatus     string `json:"payment_status"`
				AmountTotal       int64  `json:"amount_total"`
				PaymentIntent     struct {
					ID string `json:"id"`
				} `json:"payment_intent"`
				Metadata map[string]string `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, err
	}
	if evt.Type != "checkout.session.completed" {
		return &NotifyResult{Paid: false}, nil
	}
	obj := evt.Data.Object
	orderNo := obj.ClientReferenceID
	if orderNo == "" && obj.Metadata != nil {
		orderNo = obj.Metadata["order_no"]
	}
	return &NotifyResult{
		OrderNo:       orderNo,
		ProviderTxnID: obj.PaymentIntent.ID,
		AmountCents:   obj.AmountTotal,
		Paid:          obj.PaymentStatus == "paid",
	}, nil
}

func (stripeProvider) Refund(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, _ string) (*RefundResult, error) {
	form := url.Values{}
	form.Set("amount", fmt.Sprintf("%d", amountCents))
	if order.ProviderTxnID != "" {
		form.Set("payment_intent", order.ProviderTxnID)
	}
	body, err := stripeAPI(ctx, cfg, http.MethodPost, "/v1/refunds", form)
	if err != nil {
		return nil, err
	}
	var rf struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &rf); err != nil {
		return nil, err
	}
	ok := rf.Status == "succeeded" || rf.Status == "pending"
	return &RefundResult{ProviderRefundID: rf.ID, Success: ok}, nil
}

func stripeAPI(ctx context.Context, cfg RuntimeConfig, method, path string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, "https://api.stripe.com"+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.StripeSecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("stripe api %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func verifyStripeSig(payload []byte, header, secret string) bool {
	// simplified: whsec_ verification for test environments
	if secret == "" || header == "" {
		return true
	}
	parts := strings.Split(header, ",")
	var ts, sig string
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			ts = kv[1]
		case "v1":
			sig = kv[1]
		}
	}
	if ts == "" || sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "."))
	_, _ = mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
