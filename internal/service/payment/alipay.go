package payment

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type alipayProvider struct{}

func (alipayProvider) Name() string { return ProviderAlipay }

func (alipayProvider) gateway(cfg RuntimeConfig) string {
	if cfg.AlipaySandbox {
		return "https://openapi.alipaydev.com/gateway.do"
	}
	return "https://openapi.alipay.com/gateway.do"
}

func (alipayProvider) CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	biz := map[string]any{
		"out_trade_no": order.OrderNo,
		"total_amount": fmt.Sprintf("%.2f", float64(order.AmountCents)/100),
		"subject":      "TraceHub充值",
	}
	body, err := alipayRequest(ctx, cfg, "alipay.trade.precreate", biz, alipayProvider{}.notifyURL(cfg))
	if err != nil {
		return nil, err
	}
	var rsp struct {
		Response struct {
			Code       string `json:"code"`
			Msg        string `json:"msg"`
			SubCode    string `json:"sub_code"`
			SubMsg     string `json:"sub_msg"`
			QRCode     string `json:"qr_code"`
			OutTradeNo string `json:"out_trade_no"`
		} `json:"alipay_trade_precreate_response"`
	}
	if err := json.Unmarshal(body, &rsp); err != nil {
		return nil, err
	}
	if rsp.Response.Code != "10000" {
		return nil, fmt.Errorf("alipay: %s %s", rsp.Response.SubCode, rsp.Response.SubMsg)
	}
	return &CreateResult{QRCode: rsp.Response.QRCode, ProviderTxnID: rsp.Response.OutTradeNo}, nil
}

func (alipayProvider) QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	biz := map[string]any{"out_trade_no": order.OrderNo}
	body, err := alipayRequest(ctx, cfg, "alipay.trade.query", biz, "")
	if err != nil {
		return false, "", err
	}
	var rsp struct {
		Response struct {
			Code        string `json:"code"`
			TradeStatus string `json:"trade_status"`
			TradeNo     string `json:"trade_no"`
		} `json:"alipay_trade_query_response"`
	}
	if err := json.Unmarshal(body, &rsp); err != nil {
		return false, "", err
	}
	paid := rsp.Response.TradeStatus == "TRADE_SUCCESS" || rsp.Response.TradeStatus == "TRADE_FINISHED"
	return paid && rsp.Response.Code == "10000", rsp.Response.TradeNo, nil
}

func (alipayProvider) ParseNotify(ctx context.Context, cfg RuntimeConfig, r *http.Request) (*NotifyResult, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	orderNo := r.FormValue("out_trade_no")
	status := r.FormValue("trade_status")
	var amt float64
	fmt.Sscanf(r.FormValue("total_amount"), "%f", &amt)
	paid := status == "TRADE_SUCCESS" || status == "TRADE_FINISHED"
	return &NotifyResult{
		OrderNo:       orderNo,
		ProviderTxnID: r.FormValue("trade_no"),
		AmountCents:   int64(amt * 100),
		Paid:          paid,
	}, nil
}

func (alipayProvider) Refund(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, reason string) (*RefundResult, error) {
	biz := map[string]any{
		"out_trade_no":   order.OrderNo,
		"out_request_no": refundNo,
		"refund_amount":  fmt.Sprintf("%.2f", float64(amountCents)/100),
		"refund_reason":  reason,
	}
	body, err := alipayRequest(ctx, cfg, "alipay.trade.refund", biz, "")
	if err != nil {
		return nil, err
	}
	var rsp struct {
		Response struct {
			Code    string `json:"code"`
			TradeNo string `json:"trade_no"`
		} `json:"alipay_trade_refund_response"`
	}
	if err := json.Unmarshal(body, &rsp); err != nil {
		return nil, err
	}
	if rsp.Response.Code != "10000" {
		return nil, fmt.Errorf("alipay refund failed")
	}
	return &RefundResult{ProviderRefundID: rsp.Response.TradeNo, Success: true}, nil
}

func (alipayProvider) notifyURL(cfg RuntimeConfig) string {
	return webhookURL(cfg, "alipay")
}

func alipayRequest(ctx context.Context, cfg RuntimeConfig, method string, biz map[string]any, notifyURL string) ([]byte, error) {
	bizJSON, _ := json.Marshal(biz)
	params := map[string]string{
		"app_id":    cfg.AlipayAppID,
		"method":    method,
		"format":    "JSON",
		"charset":   "utf-8",
		"sign_type": "RSA2",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
		"version":   "1.0",
		"biz_content": string(bizJSON),
	}
	if notifyURL != "" {
		params["notify_url"] = notifyURL
	}
	sign, err := alipaySign(params, cfg.AlipayPrivateKey)
	if err != nil {
		return nil, err
	}
	params["sign"] = sign
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, alipayProvider{}.gateway(cfg), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func alipaySign(params map[string]string, privateKeyPEM string) (string, error) {
	var keys []string
	for k, v := range params {
		if k == "sign" || strings.TrimSpace(v) == "" {
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
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("invalid alipay private key")
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		keyAny, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
	}
	rsaKey, ok := keyAny.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("not rsa private key")
	}
	h := sha256.Sum256([]byte(buf.String()))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}
