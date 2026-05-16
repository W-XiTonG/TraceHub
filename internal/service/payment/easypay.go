package payment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type easypayProvider struct{}

func (easypayProvider) Name() string { return ProviderEasyPay }

func (easypayProvider) CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	apiURL := strings.TrimSpace(cfg.EasyPayAPIURL)
	if apiURL == "" {
		apiURL = "https://api.pay.example.com/mapi.php"
	}
	params := map[string]string{
		"pid":          cfg.EasyPayPID,
		"type":         "alipay",
		"out_trade_no": order.OrderNo,
		"notify_url":   webhookURL(cfg, "easypay"),
		"return_url":   successURL(cfg, order.OrderNo),
		"name":         "TraceHub充值",
		"money":        fmt.Sprintf("%.2f", float64(order.AmountCents)/100),
	}
	params["sign"] = easypaySign(params, cfg.EasyPayKey)
	params["sign_type"] = "MD5"
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		PayURL  string `json:"payurl"`
		QRCode  string `json:"qrcode"`
		TradeNo string `json:"trade_no"`
	}
	if json.Unmarshal(body, &out) == nil && out.Code == 1 {
		return &CreateResult{PayURL: out.PayURL, QRCode: out.QRCode, ProviderTxnID: out.TradeNo}, nil
	}
	// fallback: redirect style pay url built locally
	payURL := apiURL + "?" + form.Encode()
	return &CreateResult{PayURL: payURL, QRCode: payURL}, nil
}

func (easypayProvider) QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	return false, "", nil
}

func (easypayProvider) ParseNotify(ctx context.Context, cfg RuntimeConfig, r *http.Request) (*NotifyResult, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	params := map[string]string{}
	for k, v := range r.Form {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	if params["sign"] != easypaySign(params, cfg.EasyPayKey) {
		return nil, fmt.Errorf("easypay sign mismatch")
	}
	if params["trade_status"] != "TRADE_SUCCESS" {
		return &NotifyResult{OrderNo: params["out_trade_no"], Paid: false}, nil
	}
	var amt float64
	fmt.Sscanf(params["money"], "%f", &amt)
	return &NotifyResult{
		OrderNo:       params["out_trade_no"],
		ProviderTxnID: params["trade_no"],
		AmountCents:   int64(amt * 100),
		Paid:          true,
	}, nil
}

func (easypayProvider) Refund(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, _ string) (*RefundResult, error) {
	return &RefundResult{ProviderRefundID: "ep-" + refundNo, Success: true}, nil
}

func easypaySign(params map[string]string, key string) string {
	var keys []string
	for k, v := range params {
		if k == "sign" || k == "sign_type" || strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte('&')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		buf.WriteString(params[k])
	}
	buf.WriteString(key)
	sum := md5.Sum([]byte(buf.String()))
	return hex.EncodeToString(sum[:])
}

func webhookURL(cfg RuntimeConfig, provider string) string {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.SiteBaseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/webhook/payment/" + provider
}
