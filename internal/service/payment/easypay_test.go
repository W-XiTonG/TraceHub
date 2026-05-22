package payment

import (
	"testing"
)

func TestEasypaySign(t *testing.T) {
	params := map[string]string{
		"money":        "1.00",
		"name":         "TraceHub充值",
		"notify_url":   "https://example.com/webhook/payment/easypay",
		"out_trade_no": "TH20250101000001",
		"pid":          "1001",
		"return_url":   "https://example.com/billing",
		"type":         "alipay",
	}
	key := "testkey"
	got := easypaySign(params, key)
	if len(got) != 32 {
		t.Fatalf("sign length want 32 got %d: %q", len(got), got)
	}
	// 确定性：同输入同输出
	if easypaySign(params, key) != got {
		t.Fatal("sign not deterministic")
	}
	params2 := map[string]string{
		"sign":      "ignored",
		"sign_type": "MD5",
		"money":     "1.00",
		"pid":       "1001",
	}
	if easypaySign(params2, key) == got {
		t.Fatal("sign should differ when params differ")
	}
}

func TestEasypayAPIURLFromMapi(t *testing.T) {
	cfg := RuntimeConfig{EasyPayAPIURL: "https://pay.example.com/mapi.php"}
	u, err := easypayAPIURL(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if u != "https://pay.example.com/api.php" {
		t.Fatalf("got %q", u)
	}
}

func TestEasypayAPIURLMissing(t *testing.T) {
	_, err := easypayAPIURL(RuntimeConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEasypayTypeValidation(t *testing.T) {
	if EasyPayTypeAlipay != "alipay" || EasyPayTypeWxpay != "wxpay" {
		t.Fatal("easypay type constants")
	}
}
