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
