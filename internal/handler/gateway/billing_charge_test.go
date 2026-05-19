package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

func TestGatewayBillingPrecheck_skipsTokenEstForPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	br := billing.NewModelBillRatesForTest(billing.ModelBillRatesTestOpts{
		Billable:   true,
		Mode:       billing.BillingModePerRequest,
		PerRequest: 100,
		InRate:     1,
		GroupMul:   1,
		UserMul:    1,
	})
	u := &ent.User{Balance: 50}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	called := false
	_, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
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
	br := billing.NewModelBillRatesForTest(billing.ModelBillRatesTestOpts{
		Billable:   true,
		Mode:       billing.BillingModeHybridFloor,
		PerRequest: 200,
		InPer1k:    100,
		OutPer1k:   100,
		InRate:     1,
		OutRate:    1,
		GroupMul:   1,
		UserMul:    1,
	})
	if got := GatewayFinalCost(br, 1, 1, 0); got != 200 {
		t.Fatalf("low usage = %d, want floor 200", got)
	}
}
