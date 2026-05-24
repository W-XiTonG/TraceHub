package relay

import (
	"bytes"
	"context"
	"net/http"
	"strings"
)

// ForwardOpenAIVideoCreate 转发 POST /videos（JSON）。
func ForwardOpenAIVideoCreate(ctx context.Context, baseURL, apiKey string, body []byte) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	url := base + "/videos"
	req, err := http.NewRequestWithContext(LongRunningUpstreamContext(ctx), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	return httpVideoUpstream().Do(req)
}

// ForwardOpenAIVideoMultipart 转发 POST /videos（multipart，含 input_reference 等）。
func ForwardOpenAIVideoMultipart(ctx context.Context, baseURL, apiKey, contentType string, body []byte, passthrough http.Header) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	url := base + "/videos"
	req, err := http.NewRequestWithContext(LongRunningUpstreamContext(ctx), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.ContentLength = int64(len(body))
	applyPassthroughHeaders(req, passthrough)
	return httpVideoUpstream().Do(req)
}

// ForwardOpenAIVideoHTTP 视频资源 GET/DELETE（状态查询等），使用长超时。
func ForwardOpenAIVideoHTTP(ctx context.Context, baseURL, apiKey, method, relativePath string, body []byte, rawQuery string, passthrough http.Header) (*http.Response, error) {
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream(), baseURL, apiKey, method, relativePath, body, rawQuery, passthrough, true)
}

// ForwardOpenAIVideoContent 下载视频内容，使用长超时客户端。
func ForwardOpenAIVideoContent(ctx context.Context, baseURL, apiKey, videoID string, passthrough http.Header) (*http.Response, error) {
	rel := "videos/" + strings.Trim(strings.TrimSpace(videoID), "/")
	return forwardOpenAIHTTPClient(ctx, httpVideoUpstream(), baseURL, apiKey, http.MethodGet, rel+"/content", nil, "", passthrough, true)
}
