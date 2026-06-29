package sitecfg

import (
	"testing"
)

func TestParseAndSanitizeRechargeShops(t *testing.T) {
	raw := `{
		"title": "在线充值",
		"hint": "购买后兑换",
		"shops": [
			{"label": "官方商城", "url": "https://pay.ldxp.cn/shop/MVCBXF75", "enabled": true, "sort": 1},
			{"label": "备用", "url": "https://example.com/shop", "enabled": false, "sort": 0}
		]
	}`
	d, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if d.Title != "在线充值" {
		t.Fatalf("title=%q", d.Title)
	}
	if len(d.Shops) != 2 {
		t.Fatalf("shops len=%d", len(d.Shops))
	}
	vis := VisibleRechargeShops(d)
	if len(vis.Shops) != 1 {
		t.Fatalf("visible shops=%d", len(vis.Shops))
	}
	if vis.Shops[0].URL != "https://pay.ldxp.cn/shop/MVCBXF75" {
		t.Fatalf("url=%q", vis.Shops[0].URL)
	}
}

func TestParseAndSanitizeRechargeShopsWithQR(t *testing.T) {
	raw := `{
		"shops": [
			{
				"label": "扫码商城",
				"url": "https://pay.ldxp.cn/shop/MVCBXF75",
				"enabled": true,
				"show_qr": true,
				"qr_image_url": "/uploads/site-brand/qr.png"
			},
			{
				"label": "链接商城",
				"url": "https://example.com/shop",
				"enabled": true,
				"show_qr": true
			}
		]
	}`
	d, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Shops) != 2 {
		t.Fatalf("shops len=%d", len(d.Shops))
	}
	if !d.Shops[0].ShowQR || d.Shops[0].QRImageURL != "/uploads/site-brand/qr.png" {
		t.Fatalf("shop0=%+v", d.Shops[0])
	}
	if !d.Shops[1].ShowQR || d.Shops[1].QRImageURL != "" {
		t.Fatalf("shop1=%+v", d.Shops[1])
	}
}

func TestParseAndSanitizeRechargeShopsInvalidURL(t *testing.T) {
	raw := `{"shops":[{"label":"x","url":"ftp://bad"}]}`
	_, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAndSanitizeRechargeShopsInvalidQRURL(t *testing.T) {
	raw := `{"shops":[{"label":"x","url":"https://example.com","qr_image_url":"ftp://bad"}]}`
	_, err := ParseAndSanitizeRechargeShops([]byte(raw))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRechargeShopsJSONEmpty(t *testing.T) {
	if err := ValidateRechargeShopsJSON(""); err != nil {
		t.Fatal(err)
	}
}
