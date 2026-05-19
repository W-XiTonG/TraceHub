package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestShouldSkipUsageLog_modelsList(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	if !shouldSkipUsageLog(c) {
		t.Fatal("expected skip for GET /v1/models")
	}
}

func TestFallbackModel_path(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	got := fallbackModel(c)
	if got != "route:/v1/chat/completions" {
		t.Fatalf("fallback=%q", got)
	}
}

func TestUsageRecorder_MarkLogged_skipsFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &UsageRecorder{logged: false}
	rec.MarkLogged()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/x", nil)
	c.Writer.WriteHeader(500)
	rec.maybeFlushFailure(c)
	if !rec.IsLogged() {
		t.Fatal("expected still logged, no flush")
	}
}

func TestUsageCostForStatus_4xxZero(t *testing.T) {
	if got := usageCostForStatus(429, 1000); got != 0 {
		t.Fatalf("429: got %d want 0", got)
	}
	if got := usageCostForStatus(500, 100); got != 0 {
		t.Fatalf("500: got %d want 0", got)
	}
	if got := usageCostForStatus(200, 1000); got != 1000 {
		t.Fatalf("200: got %d want 1000", got)
	}
	if got := usageCostForStatus(399, 50); got != 50 {
		t.Fatalf("399: got %d want 50", got)
	}
}

func TestUsageRecorder_NoteError(t *testing.T) {
	rec := &UsageRecorder{}
	rec.NoteError("余额不足")
	rec.mu.Lock()
	msg := rec.errMsg
	rec.mu.Unlock()
	if msg == "" {
		t.Fatal("expected error message")
	}
}
