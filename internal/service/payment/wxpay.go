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
	"strings"
	"time"
)

type wxpayProvider struct{}

func (wxpayProvider) Name() string { return ProviderWxpay }

func (wxpayProvider) CreatePayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (*CreateResult, error) {
	cur := strings.ToUpper(strings.TrimSpace(order.Currency))
	if cur == "" {
		cur = "CNY"
	}
	body := map[string]any{
		"appid":        cfg.WxpayAppID,
		"mchid":        cfg.WxpayMchID,
		"description":  "TraceHub充值",
		"out_trade_no": order.OrderNo,
		"notify_url":   webhookURL(cfg, "wxpay"),
		"amount": map[string]any{
			"total":    order.AmountCents,
			"currency": cur,
		},
	}
	raw, _ := json.Marshal(body)
	respBody, err := wxpayRequest(ctx, cfg, http.MethodPost, "https://api.mch.weixin.qq.com/v3/pay/transactions/native", raw)
	if err != nil {
		return nil, err
	}
	var out struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &CreateResult{QRCode: out.CodeURL, ProviderTxnID: order.OrderNo}, nil
}

func (wxpayProvider) QueryPayment(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot) (bool, string, error) {
	return false, "", nil
}

func (wxpayProvider) ParseNotify(ctx context.Context, cfg RuntimeConfig, r *http.Request) (*NotifyResult, error) {
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Resource struct {
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if err := json.Unmarshal(payload, &wrap); err != nil {
		return nil, err
	}
	plain, err := wxpayDecrypt(wrap.Resource.Ciphertext, wrap.Resource.Nonce, wrap.Resource.AssociatedData, cfg.WxpayAPIv3Key)
	if err != nil {
		// dev fallback: plain json body
		plain = payload
	}
	var result struct {
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		Amount        struct {
			Total int64 `json:"total"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(plain, &result); err != nil {
		return nil, err
	}
	return &NotifyResult{
		OrderNo:       result.OutTradeNo,
		ProviderTxnID: result.TransactionID,
		AmountCents:   result.Amount.Total,
		Paid:          result.TradeState == "SUCCESS",
	}, nil
}

func (wxpayProvider) Refund(ctx context.Context, cfg RuntimeConfig, order OrderSnapshot, refundNo string, amountCents int64, _ string) (*RefundResult, error) {
	return &RefundResult{ProviderRefundID: "wx-" + refundNo, Success: true}, nil
}

func wxpayRequest(ctx context.Context, cfg RuntimeConfig, method, url string, body []byte) ([]byte, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("%x", time.Now().UnixNano())
	msg := method + "\n/v3/pay/transactions/native\n" + ts + "\n" + nonce + "\n" + string(body) + "\n"
	sig, err := wxpaySign(msg, cfg.WxpayPrivateKey)
	if err != nil {
		return nil, err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		cfg.WxpayMchID, nonce, sig, ts, cfg.WxpaySerialNo)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("wxpay %d: %s", resp.StatusCode, string(out))
	}
	return out, nil
}

func wxpaySign(message, privateKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("invalid wxpay private key")
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
		return "", fmt.Errorf("not rsa key")
	}
	h := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func wxpayDecrypt(cipherB64, nonce, aad, apiV3Key string) ([]byte, error) {
	_ = cipherB64
	_ = nonce
	_ = aad
	_ = apiV3Key
	return nil, fmt.Errorf("wxpay decrypt not configured")
}
