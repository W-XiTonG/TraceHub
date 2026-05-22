package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
)

// HTTPSPolicy 按缓存的站点策略强制 HTTPS 跳转并可选下发 HSTS。
func HTTPSPolicy(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := sitecfg.CachedHTTPSPolicy(c.Request.Context(), client, cfg)
		if policy.ForceRedirect {
			if !requestIsHTTPS(c) {
				target := "https://" + c.Request.Host + c.Request.RequestURI
				c.Redirect(http.StatusMovedPermanently, target)
				c.Abort()
				return
			}
		}
		if policy.HSTSEnabled && requestIsHTTPS(c) {
			maxAge := policy.HSTSMaxAgeSeconds
			if maxAge <= 0 {
				maxAge = 31536000
			}
			c.Writer.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d", maxAge))
		}
		c.Next()
	}
}

func requestIsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		proto = strings.TrimSpace(c.GetHeader("X-Forwarded-Protocol"))
	}
	return strings.EqualFold(proto, "https")
}
