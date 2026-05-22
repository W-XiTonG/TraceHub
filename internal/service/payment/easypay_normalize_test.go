package payment

import "testing"

func TestNormalizeEasyPayCreateResult(t *testing.T) {
	r := normalizeEasyPayCreateResult(&CreateResult{
		PayURL: "",
		QRCode: "https://pay.example.com/cashier/abc",
	})
	if r.PayURL != "https://pay.example.com/cashier/abc" || r.QRCode != "" {
		t.Fatalf("got pay=%q qr=%q", r.PayURL, r.QRCode)
	}

	r2 := normalizeEasyPayCreateResult(&CreateResult{
		PayURL: "https://pay.example.com/pay",
		QRCode: "https://pay.example.com/pay",
	})
	if r2.QRCode != "" {
		t.Fatalf("duplicate qr should clear: %q", r2.QRCode)
	}

	r3 := normalizeEasyPayCreateResult(&CreateResult{
		PayURL: "https://pay.example.com/pay",
		QRCode: "https://cdn.example.com/qr.png",
	})
	if r3.QRCode == "" {
		t.Fatal("image qrcode should remain")
	}
}
