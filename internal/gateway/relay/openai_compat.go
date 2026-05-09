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

func DrainBody(rc io.ReadCloser) {
	if rc != nil {
		_, _ = io.Copy(io.Discard, rc)
		_ = rc.Close()
	}
}
