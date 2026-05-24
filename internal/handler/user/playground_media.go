package userhandler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/safehttp"

	"github.com/gin-gonic/gin"
)

const playgroundMediaMaxBytes = 100 << 20 // 100 MiB

var playgroundMediaHTTP = &http.Client{
	Timeout: 90 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return safehttp.EgressURLHostSafe(req.URL.String())
	},
}

// PlaygroundMediaProxy 为控制台试玩区代理下载外链图片/视频（绕过浏览器 CORS）。
func PlaygroundMediaProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimSpace(c.Query("url"))
		if raw == "" {
			httpx.BadRequest(c, "缺少 url")
			return
		}
		if err := safehttp.EgressURLHostSafe(raw); err != nil {
			httpx.BadRequest(c, "URL 不可访问")
			return
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, raw, nil)
		if err != nil {
			httpx.BadRequest(c, "无效 URL")
			return
		}
		req.Header.Set("User-Agent", "TraceHub-Playground/1.0")
		req.Header.Set("Accept", "*/*")

		resp, err := playgroundMediaHTTP.Do(req)
		if err != nil {
			httpx.BadRequest(c, "拉取资源失败")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			httpx.BadRequest(c, fmt.Sprintf("上游返回 %d", resp.StatusCode))
			return
		}

		limited := io.LimitReader(resp.Body, playgroundMediaMaxBytes+1)
		body, err := io.ReadAll(limited)
		if err != nil {
			httpx.ServerError(c, "读取失败")
			return
		}
		if len(body) > playgroundMediaMaxBytes {
			httpx.BadRequest(c, "文件过大")
			return
		}

		ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if ct == "" {
			ct = "application/octet-stream"
		}
		if i := strings.Index(ct, ";"); i >= 0 {
			ct = strings.TrimSpace(ct[:i])
		}
		c.Data(http.StatusOK, ct, body)
	}
}
