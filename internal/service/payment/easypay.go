package payment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

var (
	errEasyPayNotConfigured = errors.New("easypay api url not configured")
	easypayHTTPClient       = &http.Client{Timeout: 10 * time.Second}
)

type easypayProvider struct{}

func (easypayProvider) Name() string { return ProviderEasyPay }

func (easypayProvider) CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	apiURL, err := easypayMapiURL(cfg)
	if err != nil {
		return nil, err
	}
	payType := strings.TrimSpace(order.EasyPayType)
	if payType == "" {
		payType = EasyPayTypeAlipay
	}
	if payType != EasyPayTypeAlipay && payType != EasyPayTypeWxpay {
		return nil, ErrInvalidPayType
	}
	params := map[string]string{
		"pid":          cfg.EasyPayPID,
		"type":         payType,
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
	resp, err := easypayHTTPClient.Do(req)
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
		return normalizeEasyPayCreateResult(&CreateResult{
			PayURL:          sanitizePostgresText(out.PayURL, 2000),
			QRCode:          sanitizePostgresText(out.QRCode, 4000),
			ProviderTxnID:   sanitizePostgresText(out.TradeNo, 128),
		}), nil
	}
	if out.Msg != "" {
		return nil, fmt.Errorf("easypay create: %s", out.Msg)
	}
	return nil, fmt.Errorf("easypay create failed: %s", strings.TrimSpace(string(body)))
}

func (easypayProvider) QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	apiURL, err := easypayAPIURL(cfg)
	if err != nil {
		return false, "", err
	}
	q := url.Values{}
	q.Set("act", "order")
	q.Set("pid", cfg.EasyPayPID)
	q.Set("key", cfg.EasyPayKey)
	q.Set("out_trade_no", order.OrderNo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"?"+q.Encode(), nil)
	if err != nil {
		return false, "", err
	}
	resp, err := easypayHTTPClient.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Code    int    `json:"code"`
		Msg     string `json:"msg"`
		Status  int    `json:"status"`
		TradeNo string `json:"trade_no"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, "", fmt.Errorf("easypay query parse: %w", err)
	}
	if out.Code != 1 {
		if out.Msg != "" {
			return false, "", fmt.Errorf("easypay query: %s", out.Msg)
		}
		return false, "", nil
	}
	paid := out.Status == 1
	return paid, out.TradeNo, nil
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
	apiURL, err := easypayAPIURL(cfg)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("act", "refund")
	q.Set("pid", cfg.EasyPayPID)
	q.Set("key", cfg.EasyPayKey)
	q.Set("out_trade_no", order.OrderNo)
	q.Set("money", fmt.Sprintf("%.2f", float64(amountCents)/100))
	q.Set("out_refund_no", refundNo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(q.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := easypayHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("easypay refund parse: %w", err)
	}
	if out.Code != 1 {
		msg := strings.TrimSpace(out.Msg)
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		return nil, fmt.Errorf("easypay refund failed: %s", msg)
	}
	return &RefundResult{ProviderRefundID: refundNo, Success: true}, nil
}

// normalizeEasyPayCreateResult：易支付 qrcode 常为收银台链接而非图片，统一落到 PayURL。
func normalizeEasyPayCreateResult(res *CreateResult) *CreateResult {
	if res == nil {
		return res
	}
	pay := strings.TrimSpace(res.PayURL)
	qr := strings.TrimSpace(res.QRCode)
	switch {
	case pay == "" && qr != "":
		res.PayURL = qr
		res.QRCode = ""
	case qr != "" && (qr == pay || !easypayIsImageURL(qr)):
		res.QRCode = ""
	case qr != "" && easypayIsImageURL(qr):
		// 保留图片型 qrcode 供前端展示
	}
	return res
}

func easypayIsImageURL(u string) bool {
	u = strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(u, "data:image/") {
		return true
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp"} {
		if strings.Contains(u, ext) {
			return true
		}
	}
	return strings.Contains(u, "format=png") || strings.Contains(u, "format=jpg") ||
		strings.Contains(u, "/qr/") || strings.Contains(u, "qrcode=")
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

func easypayMapiURL(cfg RuntimeConfig) (string, error) {
	u := strings.TrimSpace(cfg.EasyPayAPIURL)
	if u == "" {
		return "", errEasyPayNotConfigured
	}
	return u, nil
}

func easypayAPIURL(cfg RuntimeConfig) (string, error) {
	mapi, err := easypayMapiURL(cfg)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(mapi)
	if err != nil {
		return "", fmt.Errorf("invalid easypay api url: %w", err)
	}
	path := parsed.Path
	if strings.HasSuffix(strings.ToLower(path), "mapi.php") {
		path = path[:len(path)-len("mapi.php")] + "api.php"
	} else if !strings.HasSuffix(strings.ToLower(path), "api.php") {
		if strings.HasSuffix(path, "/") {
			path += "api.php"
		} else {
			path = strings.TrimSuffix(path, "/") + "/api.php"
		}
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func webhookURL(cfg RuntimeConfig, provider string) string {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.SiteBaseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return base + "/webhook/payment/" + provider
}
