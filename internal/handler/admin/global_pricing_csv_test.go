package adminhandler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseGlobalPricingImportTemplateQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		query   string
		wantCur string
		wantDen int64
		wantErr bool
	}{
		{"cny 1m", "currency=cny&denom=1000000", "cny", 1_000_000, false},
		{"usd 1k alt param", "currency=usd&denominator_tokens=1000", "usd", 1000, false},
		{"rmb", "currency=rmb&denom=1", "cny", 1, false},
		{"bad currency", "currency=eur&denom=1000", "", 0, true},
		{"bad denom", "currency=usd&denom=500", "", 0, true},
		{"missing denom", "currency=cny", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
			cur, den, errMsg := parseGlobalPricingImportTemplateQuery(c)
			if tc.wantErr {
				if errMsg == "" {
					t.Fatal("expected error")
				}
				return
			}
			if errMsg != "" {
				t.Fatalf("unexpected err: %s", errMsg)
			}
			if cur != tc.wantCur || den != tc.wantDen {
				t.Fatalf("got %s,%d want %s,%d", cur, den, tc.wantCur, tc.wantDen)
			}
		})
	}
}
