package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const ReqIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(ReqIDHeader)
		if rid == "" {
			rid = uuid.NewString()
		}
		c.Writer.Header().Set(ReqIDHeader, rid)
		c.Set("request_id", rid)
		c.Next()
	}
}

func ZapLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		log.Info("http",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqOrigin := strings.TrimSpace(c.GetHeader("Origin"))
		allowOrigin, allowCreds := corsAllowOrigin(origins, reqOrigin)
		if allowOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			if allowCreds {
				c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-TraceHub-Session, X-Sticky-Session")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		// 浏览器 fetch 默认可读响应体；自定义头需显式暴露（网关排错：选路 model vs 实际上游 model、计费预检等）
		c.Writer.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, X-TraceHub-Billing-Phase, X-TraceHub-Balance, X-TraceHub-Estimated-Cost, X-TraceHub-Estimated-Prompt-Tokens, X-TraceHub-Estimated-Completion-Tokens, X-TraceHub-Debit-Cost")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// corsAllowOrigin 按白名单回显 Origin；未配置或显式 * 时返回通配且不携带 Credentials（避免浏览器拒绝无效组合）。
func corsAllowOrigin(configured []string, requestOrigin string) (origin string, credentials bool) {
	hasWildcard := false
	var list []string
	for _, o := range configured {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			hasWildcard = true
			continue
		}
		list = append(list, o)
	}
	if len(list) == 0 {
		if hasWildcard || len(configured) == 0 {
			return "*", false
		}
	}
	if requestOrigin != "" {
		for _, o := range list {
			if o == requestOrigin {
				return requestOrigin, true
			}
		}
	}
	if hasWildcard {
		return "*", false
	}
	if len(list) == 1 && requestOrigin == "" {
		return list[0], true
	}
	return "", false
}

// MetricsBearerAuth 非空 token 时要求 GET/HEAD /metrics 携带 Authorization: Bearer <token>。
func MetricsBearerAuth(token string) gin.HandlerFunc {
	want := strings.TrimSpace(token)
	return func(c *gin.Context) {
		if want == "" {
			c.Next()
			return
		}
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			c.AbortWithStatus(401)
			return
		}
		got := strings.TrimSpace(h[7:])
		if got != want {
			c.AbortWithStatus(401)
			return
		}
		c.Next()
	}
}
