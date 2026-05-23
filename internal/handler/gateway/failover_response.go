package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"

	"github.com/gin-gonic/gin"
)

// ApplyFailoverResponseHeaders 在多次上游尝试后设置可选诊断头与 Retry-After。
func ApplyFailoverResponseHeaders(c *gin.Context, client *ent.Client, groupID int, res *router.UpstreamResult, attempts int) {
	if attempts < 2 {
		return
	}
	c.Header("X-TraceHub-Attempts", strconv.Itoa(attempts))
	if res == nil {
		return
	}
	cfg, err := router.ResolveRoutingConfig(c.Request.Context(), client, groupID)
	if err != nil {
		return
	}
	retryAfter := cfg.FailoverRetryAfterSec
	if retryAfter < 1 {
		retryAfter = 15
	}
	status := res.StatusCode
	if status >= 500 || res.TransportErr != nil {
		c.Header("Retry-After", strconv.Itoa(retryAfter))
	}
	if status == http.StatusTooManyRequests {
		c.Header("Retry-After", strconv.Itoa(max(retryAfter, 30)))
	}
}

// WriteClientProxyWithFailover 写回代理响应并附加 failover 相关头。
func WriteClientProxyWithFailover(c *gin.Context, client *ent.Client, groupID int, statusCode int, contentType string, body []byte, res *router.UpstreamResult) {
	attempts := 1
	if res != nil && res.FailoverAttempts > 0 {
		attempts = res.FailoverAttempts
	}
	ApplyFailoverResponseHeaders(c, client, groupID, res, attempts)
	WriteClientProxy(c, statusCode, contentType, body)
}

// WriteGatewayTransportErrorWithFailover 传输失败响应，附带 failover 尝试信息。
func WriteGatewayTransportErrorWithFailover(c *gin.Context, client *ent.Client, groupID int, attempts int, err error) {
	if attempts >= 2 {
		cfg, _ := router.ResolveRoutingConfig(context.Background(), client, groupID)
		ra := 15
		if cfg.FailoverRetryAfterSec > 0 {
			ra = cfg.FailoverRetryAfterSec
		}
		headers := map[string]string{
			"Retry-After":           strconv.Itoa(ra),
			"X-TraceHub-Attempts":   strconv.Itoa(attempts),
		}
		noteUsageErrorFromContext(c, MsgServiceUnavailable)
		middleware.WriteOpenAIErrorWithHeaders(c, http.StatusServiceUnavailable, "api_error", MsgServiceUnavailable, headers)
		return
	}
	WriteGatewayTransportError(c, err)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FailoverAttemptsFromResult 从 UpstreamResult 读取尝试次数。
func FailoverAttemptsFromResult(res *router.UpstreamResult) int {
	if res == nil || res.FailoverAttempts < 1 {
		return 1
	}
	return res.FailoverAttempts
}

// FormatFailoverAttemptsHeader 供测试或其它 handler 使用。
func FormatFailoverAttemptsHeader(attempts int) string {
	return fmt.Sprintf("%d", attempts)
}
