package middleware

import (
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/apikey"
	"tracehub/internal/pkg/apikeyhash"
	"tracehub/internal/pkg/ipcheck"

	"github.com/gin-gonic/gin"
)

const (
	CtxGateUserKey   = "tracehub_gate_user"
	CtxGateAPIKeyKey = "tracehub_gate_apikey"
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
		clientIP := c.ClientIP()
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
	c.JSON(httpStatus, gin.H{
		"error": gin.H{
			"type":    code,
			"message": msg,
		},
	})
}
