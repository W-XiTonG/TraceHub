package middleware

import (
	"log/slog"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/apikey"
	"tracehub/internal/pkg/apikeyhash"
	"tracehub/internal/pkg/ipcheck"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/pkg/usagelogpublic"

	"github.com/gin-gonic/gin"
)

const (
	CtxGateUserKey      = "tracehub_gate_user"
	CtxGateAPIKeyKey    = "tracehub_gate_apikey"
	CtxGatewayErrMsgKey = "tracehub_gateway_err_msg" // 供 UsageRecorder 写入 usage_logs.error_message
)

func GatewayAPIKeyAuth(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			WriteOpenAIAuthErr(c)
			c.Abort()
			return
		}
		raw := strings.TrimSpace(h[7:])
		clientIP := c.ClientIP()
		if bl := sitecfg.CachedGatewayIPBlacklist(c.Request.Context(), client); len(bl) > 0 {
			if ipcheck.Blocked(clientIP, bl) {
				WriteOpenAIError(c, 403, "permission_error", "IP 已被全局网关黑名单拒绝")
				c.Abort()
				return
			}
		}
		hash := apikeyhash.Hash(raw)
		k, err := client.APIKey.Query().
			Where(apikey.KeyHashEQ(hash), apikey.Enabled(true)).
			WithUser(func(q *ent.UserQuery) { q.WithGroup() }).
			Only(c.Request.Context())
		if err != nil {
			WriteOpenAIAuthErr(c)
			c.Abort()
			return
		}
		u := k.Edges.User
		if u == nil || u.Status != 1 {
			WriteOpenAIAuthErr(c)
			c.Abort()
			return
		}
		if !ipcheck.Allowed(clientIP, k.IPWhitelist) {
			WriteOpenAIError(c, 403, "permission_error", "IP 不在 API Key 白名单内")
			c.Abort()
			return
		}
		if !ipcheck.Allowed(clientIP, u.IPWhitelist) {
			WriteOpenAIError(c, 403, "permission_error", "IP 不在账户白名单内")
			c.Abort()
			return
		}
		if !k.ExpiresAt.IsZero() && time.Now().After(k.ExpiresAt) {
			WriteOpenAIError(c, 401, "authentication_error", "API Key 已过期")
			c.Abort()
			return
		}
		c.Set(CtxGateUserKey, u)
		c.Set(CtxGateAPIKeyKey, k)
		c.Next()
	}
}

func WriteOpenAIAuthErr(c *gin.Context) {
	WriteOpenAIError(c, 401, "authentication_error", "无效的 API Key")
}

// WriteOpenAIError OpenAI 风格的错误响应。
func WriteOpenAIError(c *gin.Context, httpStatus int, code, msg string) {
	WriteOpenAIErrorWithHeaders(c, httpStatus, code, msg, nil)
}

// WriteOpenAIErrorWithHeaders 与 WriteOpenAIError 相同，并在写 JSON 前设置可选响应头（如网关计费诊断）。
func WriteOpenAIErrorWithHeaders(c *gin.Context, httpStatus int, code, msg string, headers map[string]string) {
	clientMsg, internalDetail := usagelogpublic.SanitizeGatewayErrorMessage(httpStatus, msg)
	if internalDetail != "" {
		c.Set(CtxGatewayErrMsgKey, internalDetail)
		slog.Info("gateway openai error response",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", httpStatus,
			"error_type", code,
			"detail", internalDetail,
		)
	} else if m := strings.TrimSpace(msg); m != "" {
		c.Set(CtxGatewayErrMsgKey, m)
	}
	msg = clientMsg
	for k, v := range headers {
		if k != "" && v != "" {
			c.Header(k, v)
		}
	}
	c.JSON(httpStatus, gin.H{
		"error": gin.H{
			"type":    code,
			"message": msg,
		},
	})
}
