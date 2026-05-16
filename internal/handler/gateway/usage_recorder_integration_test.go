//go:build cgo

package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/enttest"
	"tracehub/internal/ent/usagelog"
	"tracehub/internal/middleware"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

func TestGatewayUsageRecorder_logs402OnPrecheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := enttest.Open(t, "sqlite3", "file:usagelog402?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	u, err := client.User.Create().SetEmail("u402@test.local").SetUsername("u402").SetPasswordHash("x").SetBalance(0).Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	k, err := client.APIKey.Create().SetUserID(u.ID).SetKeyHash("kh").SetKeyPrefix("sk-t").SetName("k").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.CtxGateUserKey, u)
		c.Set(middleware.CtxGateAPIKeyKey, k)
		c.Set("request_id", "req-402")
		c.Next()
	})
	r.Use(GatewayUsageRecorder(client))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetModel("gpt-test")
		}
		WriteInsufficientBalance402Precheck(c, 0, 100, 10, 0)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.ServeHTTP(w, req)
	if w.Code != 402 {
		t.Fatalf("status=%d", w.Code)
	}

	waitUsageLogs(t, client, 1)
	logs, err := client.UsageLog.Query().Where(usagelog.UserIDEQ(u.ID)).All(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].StatusCode != 402 {
		t.Fatalf("status_code=%d", logs[0].StatusCode)
	}
	if logs[0].ErrorMessage == "" || logs[0].Model != "gpt-test" {
		t.Fatalf("model=%q err=%q", logs[0].Model, logs[0].ErrorMessage)
	}
}

func TestGatewayUsageRecorder_skipsWhenSaveOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client := enttest.Open(t, "sqlite3", "file:usagelogok?mode=memory&cache=shared&_fk=1")
	defer client.Close()

	u, err := client.User.Create().SetEmail("uok@test.local").SetUsername("uok").SetPasswordHash("x").SetBalance(0).Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	k, err := client.APIKey.Create().SetUserID(u.ID).SetKeyHash("kh2").SetKeyPrefix("sk-u").SetName("k").Save(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.CtxGateUserKey, u)
		c.Set(middleware.CtxGateAPIKeyKey, k)
		c.Set("request_id", "req-ok")
		c.Next()
	})
	r.Use(GatewayUsageRecorder(client))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetModel("gpt-ok")
			rec.SaveOK(1, 2, 0, 3, 200, nil)
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.ServeHTTP(w, req)

	waitUsageLogs(t, client, 1)
	count, err := client.UsageLog.Query().Where(usagelog.UserIDEQ(u.ID)).Count(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 log (no duplicate), got %d", count)
	}
}

func waitUsageLogs(t *testing.T, client *ent.Client, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, err := client.UsageLog.Query().Count(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if n >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %d usage logs", want)
}
