package gateway

import (
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
)

// logUpstreamQuotaDeclineIfLikely 在疑似上游额度/账务类拒绝时写一条服务端日志（英文、低开信息量），不向客户端改写响应体。
func logUpstreamQuotaDeclineIfLikely(c *gin.Context, httpStatus int, raw []byte) {
	var p struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return
	}
	msg := strings.ToLower(strings.TrimSpace(p.Message))
	if p.Code != 30001 &&
		!strings.Contains(msg, "balance is insufficient") &&
		!strings.Contains(msg, "insufficient balance") &&
		!strings.Contains(msg, "account balance") {
		return
	}
	slog.Info("upstream request not served",
		"http_status", httpStatus,
		"path", c.Request.URL.Path,
		"detail", "provider-side limit or availability constraint",
	)
}
