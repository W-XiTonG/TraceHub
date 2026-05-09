package middleware

import (
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/jwtx"

	"github.com/gin-gonic/gin"
)

const (
	CtxClaimsKey = "tracehub_jwt_claims"
	CtxUserKey   = "tracehub_user_ent"
)

func JWTAuth(secret []byte, client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			httpx.Unauthorized(c, "缺少 Authorization")
			c.Abort()
			return
		}
		raw := strings.TrimSpace(h[7:])
		cl, err := jwtx.ParseAccess(secret, raw)
		if err != nil {
			httpx.Unauthorized(c, "令牌无效或已过期")
			c.Abort()
			return
		}
		u, err := client.User.Get(c.Request.Context(), int(cl.UserID))
		if err != nil || u.Status != 1 {
			httpx.Unauthorized(c, "用户不可用")
			c.Abort()
			return
		}
		c.Set(CtxClaimsKey, cl)
		c.Set(CtxUserKey, u)
		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	set := map[string]struct{}{}
	for _, r := range roles {
		set[r] = struct{}{}
	}
	return func(c *gin.Context) {
		v, ok := c.Get(CtxClaimsKey)
		if !ok {
			httpx.Unauthorized(c, "未认证")
			c.Abort()
			return
		}
		cl := v.(*jwtx.Claims)
		if _, ok := set[cl.Role]; !ok {
			httpx.Forbidden(c, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}
