package billing

import "testing"

func TestModelBillRates_RequestCost(t *testing.T) {
	br := ModelBillRates{
		Billable:      true,
		perRequest:    1000,
		inRate:        1.5,
		markupPercent: 10,
		groupMul:      2,
		userMul:       1,
		billingMode:   BillingModePerRequest,
	}
	got := br.RequestCost()
	// 1000 * 1.5 * 1.1 * 2 = 3300
	if got != 3300 {
		t.Fatalf("RequestCost() = %d, want 3300", got)
	}
}

func TestModelBillRates_PrecheckCost_perRequest(t *testing.T) {
	br := ModelBillRates{
		Billable: true, perRequest: 500, billingMode: BillingModePerRequest,
		inRate: 1, groupMul: 1, userMul: 1,
	}
	cost, need := br.PrecheckCost(9999, 9999, 0)
	if need {
		t.Fatal("per_request should not need token estimate")
	}
	if cost != 500 {
		t.Fatalf("PrecheckCost = %d, want 500", cost)
	}
}

func TestModelBillRates_FinalCost_hybridFloor(t *testing.T) {
	br := ModelBillRates{
		Billable: true, perRequest: 200, billingMode: BillingModeHybridFloor,
		in1k: 100, out1k: 100, inRate: 1, outRate: 1, groupMul: 1, userMul: 1,
	}
	low := br.FinalCost(10, 10, 0)
	if low != 200 {
		t.Fatalf("hybrid floor low usage = %d, want 200", low)
	}
	high := br.FinalCost(5000, 5000, 0)
	tokenCost := br.Cost(5000, 5000, 0)
	if high != tokenCost || high <= 200 {
		t.Fatalf("hybrid floor high usage = %d, token=%d", high, tokenCost)
	}
}

func TestNormalizeBillingMode(t *testing.T) {
	if normalizeBillingMode("PER_REQUEST") != BillingModePerRequest {
		t.Fatal("normalize failed")
	}
	if normalizeBillingMode("unknown") != BillingModeToken {
		t.Fatal("unknown should default to token")
	}
}
