package health

import (
	"context"
	"net/http"
	"time"

	"tracehub/internal/ent"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

// Version 与产品文档运维端点示例一致，发布时可由 ldflags 覆盖。
const Version = "1.0.0"

var serverStarted = time.Now()

func Liveness() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"version": Version,
			"uptime":  int64(time.Since(serverStarted).Seconds()),
		})
	}
}

func Readiness(client *ent.Client, rdb *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{}
		if _, err := client.User.Query().Limit(1).Count(ctx); err != nil {
			checks["database"] = "error: " + err.Error()
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
			return
		}
		checks["database"] = "ok"
		if rdb != nil {
			if err := rdb.Ping(ctx).Err(); err != nil {
				checks["redis"] = "error: " + err.Error()
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "checks": checks})
				return
			}
			checks["redis"] = "ok"
		} else {
			checks["redis"] = "skipped"
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
	}
}
