package openaicompat

import (
	"fmt"
	"net/url"
	"strings"
)

// WebSocketURLFromOpenAIRoot 将 OpenAI 兼容 HTTPS 根（…/v1）转为 WebSocket URL，并拼接 path（不含前导 /v1/，例如 realtime 或 realtime/translations）。
func WebSocketURLFromOpenAIRoot(httpsRoot, pathUnderV1 string, query url.Values) (string, error) {
	root, err := NormalizeOpenAIAPIRoot(httpsRoot)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(root)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("unsupported scheme for websocket: %s", u.Scheme)
	}
	p := strings.Trim(strings.TrimSpace(pathUnderV1), "/")
	if p == "" {
		p = "realtime"
	}
	basePath := strings.TrimSuffix(u.Path, "/")
	u.Path = basePath + "/" + p
	if query != nil {
		u.RawQuery = query.Encode()
	} else {
		u.RawQuery = ""
	}
	u.Fragment = ""
	return u.String(), nil
}
