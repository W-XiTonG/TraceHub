package health

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"tracehub/internal/ent"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

var serverStarted = time.Now()

func Liveness(operator, appVersion string) gin.HandlerFunc {
	if strings.TrimSpace(appVersion) == "" {
		appVersion = "1.0.0"
	}
	return func(c *gin.Context) {
		body := gin.H{
			"status":     "ok",
			"version":    appVersion,
			"go_version": runtime.Version(),
			"uptime":     UptimeSeconds(),
		}
		if operator != "" {
			body["operator"] = operator
		}
		c.JSON(http.StatusOK, body)
	}
}

// UptimeSeconds 自进程启动以来的秒数（与 /health 的 uptime 一致）。
func UptimeSeconds() int64 {
	return int64(time.Since(serverStarted).Seconds())
}

func Readiness(client *ent.Client, rdb *redis.Client, operator string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{}
		fail := func() {
			body := gin.H{"status": "not_ready", "checks": checks}
			if operator != "" {
				body["operator"] = operator
			}
			c.JSON(http.StatusServiceUnavailable, body)
		}
		if _, err := client.User.Query().Limit(1).Count(ctx); err != nil {
			checks["database"] = "error: " + err.Error()
			fail()
			return
		}
		checks["database"] = "ok"
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				checks["redis"] = "error: " + err.Error()
				fail()
				return
			}
			checks["redis"] = "ok"
		} else {
			checks["redis"] = "skipped"
		}
		body := gin.H{"status": "ready", "checks": checks}
		if operator != "" {
			body["operator"] = operator
		}
		c.JSON(http.StatusOK, body)
	}
}
