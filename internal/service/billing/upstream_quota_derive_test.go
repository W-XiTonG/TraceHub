package billing

import "testing"

func TestQuotaPer1KFromUpstreamPrice_CNY_1M(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000}
	// 4 CNY per 1M input -> 0.004 CNY per 1k -> 40 quota
	q, desc, err := QuotaPer1KFromUpstreamPrice(f, "cny", 1_000_000, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	if q != 40 {
		t.Fatalf("got %d want 40 (%s)", q, desc)
	}
}

func TestQuotaPer1KFromUpstreamPrice_CNY_Ceil(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 3}
	q, _, err := QuotaPer1KFromUpstreamPrice(f, "cny", 1000, 1.01, 0)
	if err != nil {
		t.Fatal(err)
	}
	// cost per 1k = 1.01 CNY, *3 = 3.03 -> ceil 4
	if q != 4 {
		t.Fatalf("got %d want 4", q)
	}
}

func TestQuotaPer1KFromUpstreamPrice_USD_QuotaPerUsd(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000, QuotaPerUsd: 50000, CnyPerUsd: 0}
	// $5 per 1M -> $0.005 per 1k -> *50000 = 250
	q, _, err := QuotaPer1KFromUpstreamPrice(f, "usd", 1_000_000, 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if q != 250 {
		t.Fatalf("got %d want 250", q)
	}
}

func TestQuotaPer1KFromUpstreamPrice_USD_ViaCnyRate(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000, QuotaPerUsd: 0, CnyPerUsd: 7.2}
	// $1 per 1M -> $0.001 per 1k -> *7.2 CNY *10000 = 72
	q, _, err := QuotaPer1KFromUpstreamPrice(f, "usd", 1_000_000, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if q != 72 {
		t.Fatalf("got %d want 72", q)
	}
}

func TestQuotaPer1KFromUpstreamPrice_USD_NoFactors(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000, QuotaPerUsd: 0, CnyPerUsd: 0}
	_, _, err := QuotaPer1KFromUpstreamPrice(f, "usd", 1_000_000, 1, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQuotaPer1KFromUpstreamPrice_Margin(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 100}
	// 1 * 100 * 1.5 = 150（1.5 为二进制可精确表示的浮点）
	q, _, err := QuotaPer1KFromUpstreamPrice(f, "cny", 1000, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	if q != 150 {
		t.Fatalf("got %d want 150", q)
	}
}

func TestUpstreamPriceFromQuotaPer1K_RoundTripApprox(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000}
	q, _, err := QuotaPer1KFromUpstreamPrice(f, "cny", 1_000_000, 4, 0)
	if err != nil {
		t.Fatal(err)
	}
	p, err := UpstreamPriceFromQuotaPer1K(f, "cny", 1_000_000, q, 0)
	if err != nil {
		t.Fatal(err)
	}
	if p < 3.99 || p > 4.01 {
		t.Fatalf("round-trip price %g want ~4", p)
	}
}

func TestDeriveGlobalQuotaPer1KPair_ReuseOut(t *testing.T) {
	f := SitePricingFactors{QuotaPerYuan: 10000}
	a, b, _, _, err := DeriveGlobalQuotaPer1KPair(f, "cny", 1_000_000, 4, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a != 40 || b != 40 {
		t.Fatalf("in=%d out=%d want 40,40", a, b)
	}
}
