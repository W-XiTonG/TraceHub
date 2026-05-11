package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpUpstream = &http.Client{
	Timeout: 120 * time.Second,
}

// 文生图 / 图生图耗时更长，单独放宽超时。
var httpImageUpstream = &http.Client{
	Timeout: 600 * time.Second,
}

// ForwardOpenAIChat 将 OpenAI 兼容请求转发至上游（OpenAI / DeepSeek 等）。
func ForwardOpenAIChat(ctx context.Context, baseURL, apiKey string, body []byte, stream bool) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	url := base + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return httpUpstream.Do(req)
}

// ForwardOpenAIImageGenerations 转发 OpenAI 兼容的图片生成（如硅基流动 /v1/images/generations）。
func ForwardOpenAIImageGenerations(ctx context.Context, baseURL, apiKey string, body []byte) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	url := base + "/images/generations"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	return httpImageUpstream.Do(req)
}

// ForwardOpenAIPost 向 OpenAI 兼容根路径（…/v1）下子路径发起 POST，例如 embeddings、completions、moderations。
func ForwardOpenAIPost(ctx context.Context, baseURL, apiKey, relativePath string, body []byte) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	rel := strings.Trim(strings.TrimSpace(relativePath), "/")
	url := base + "/" + rel
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	return httpUpstream.Do(req)
}

func DrainBody(rc io.ReadCloser) {
	if rc != nil {
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
}
