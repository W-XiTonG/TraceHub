package gateway

import (
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

func TestGatewayBillingPrecheck_skipsTokenEstForPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	br := billing.ModelBillRates{
		Billable:      true,
		billingMode:   billing.BillingModePerRequest,
		perRequest:    100,
		inRate:        1,
		groupMul:      1,
		userMul:       1,
		markupPercent: 0,
	}
	u := &ent.User{Balance: 50}
	w := gin.CreateTestContextOnly(nil, gin.New())
	called := false
	_, ok := GatewayBillingPrecheck(w, u, br, func() (BillTokenEst, error) {
		called = true
		return BillTokenEst{Prompt: 9999, Completion: 9999}, nil
	})
	if called {
		t.Fatal("per_request must not invoke tokenEstFn")
	}
	if ok {
		t.Fatal("expected precheck failure with balance 50 < 100")
	}
}

func TestGatewayFinalCost_delegatesHybridFloor(t *testing.T) {
	br := billing.ModelBillRates{
		Billable: true, billingMode: billing.BillingModeHybridFloor, perRequest: 200,
		in1k: 100, out1k: 100, inRate: 1, outRate: 1, groupMul: 1, userMul: 1,
	}
	if got := GatewayFinalCost(br, 1, 1, 0); got != 200 {
		t.Fatalf("low usage = %d, want floor 200", got)
	}
}
