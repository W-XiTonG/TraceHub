package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

var defaultOpenAIAccountAdminPrefixes = []string{
	"organizations",
	"organization",
	"dashboard",
	"billing",
}

// pathMatchesDeniedPrefix 判断相对 /v1/ 的路径是否命中拒绝前缀（如 organizations/foo）。
func pathMatchesDeniedPrefix(rest string, prefix string) bool {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return false
	}
	if rest == p {
		return true
	}
	return strings.HasPrefix(rest, p+"/")
}

// DenyOpenAIAccountAdminUnderV1 拦截 OpenAI 平台账号/控制台类路径，避免网关用户 Key 误转发（对应产品文档「四、官方存在但通常不在代转发范围内」）。
func DenyOpenAIAccountAdminUnderV1(enabled bool, extraPrefixes []string) gin.HandlerFunc {
	deny := append([]string{}, defaultOpenAIAccountAdminPrefixes...)
	for _, e := range extraPrefixes {
		e = strings.TrimSpace(e)
		if e != "" {
			deny = append(deny, e)
		}
	}
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/") && path != "/v1" {
			c.Next()
			return
		}
		rest := strings.TrimPrefix(path, "/v1/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			c.Next()
			return
		}
		for _, p := range deny {
			if pathMatchesDeniedPrefix(rest, p) {
				WriteOpenAIError(c, http.StatusForbidden, "permission_error", "该路径属于平台账号/管理类接口，网关不对用户 API Key 开放；如需使用请在管理端或上游控制台操作")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
