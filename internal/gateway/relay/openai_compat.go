package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

func applyPassthroughHeaders(req *http.Request, passthrough http.Header) {
	if passthrough == nil {
		return
	}
	for k, vals := range passthrough {
		k = http.CanonicalHeaderKey(k)
		if k == "Authorization" || k == "Content-Type" || k == "Content-Length" {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
}

var httpUpstream = &http.Client{
	Timeout: 120 * time.Second,
}

// 文生图 / 图生图耗时更长，单独放宽超时。
var httpImageUpstream = &http.Client{
	Timeout: 600 * time.Second,
}

// 音频：大文件转写、长 TTS。
var httpAudioUpstream = &http.Client{
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
	return ForwardOpenAIPostWithHeaders(ctx, baseURL, apiKey, relativePath, body, nil, "")
}

// ForwardOpenAIPostWithHeaders 同 ForwardOpenAIPost，可透传头；accept 非空时设置 Accept（如 Responses 流式 text/event-stream）。
func ForwardOpenAIPostWithHeaders(ctx context.Context, baseURL, apiKey, relativePath string, body []byte, passthrough http.Header, accept string) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	rel := strings.Trim(strings.TrimSpace(relativePath), "/")
	url := base + "/" + rel
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	req.ContentLength = int64(len(body))
	applyPassthroughHeaders(req, passthrough)
	return httpUpstream.Do(req)
}

// ForwardOpenAIMultipart 转发 OpenAI 兼容的 multipart 音频接口（transcriptions / translations）。
// accept 非空时设置 Accept（流式转写建议 text/event-stream）。
func ForwardOpenAIMultipart(ctx context.Context, baseURL, apiKey, relativePath, contentType string, body []byte, passthrough http.Header, accept string) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	rel := strings.Trim(strings.TrimSpace(relativePath), "/")
	url := base + "/" + rel
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	req.ContentLength = int64(len(body))
	applyPassthroughHeaders(req, passthrough)
	return httpAudioUpstream.Do(req)
}

// ForwardOpenAISpeech 转发 POST /v1/audio/speech（JSON 请求体，响应多为音频二进制）。
func ForwardOpenAISpeech(ctx context.Context, baseURL, apiKey string, body []byte, accept string, passthrough http.Header) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	url := base + "/audio/speech"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(accept) != "" {
		req.Header.Set("Accept", accept)
	}
	req.ContentLength = int64(len(body))
	applyPassthroughHeaders(req, passthrough)
	return httpAudioUpstream.Do(req)
}

func DrainBody(rc io.ReadCloser) {
	if rc != nil {
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
}
