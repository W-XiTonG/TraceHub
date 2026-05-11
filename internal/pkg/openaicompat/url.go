package openaicompat

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeOpenAIAPIRoot 将渠道配置的 api_base_url 规范为「OpenAI 兼容根路径」…/v1，
// 去掉误粘贴的 /chat/completions、/images/generations 等后缀，避免拼出 …/chat/completions/images/generations 之类 404。
func NormalizeOpenAIAPIRoot(raw string) (string, error) {
	raw = strings.TrimSpace(strings.TrimRight(raw, "/"))
	if raw == "" {
		return "", fmt.Errorf("empty api_base_url")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid api_base_url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("api_base_url 仅支持 http/https")
	}
	p := strings.TrimSuffix(u.Path, "/")
	for {
		changed := false
		for _, suf := range []string{
			"/v1/chat/completions",
			"/v1/images/generations",
			"/v1/images/edits",
			"/v1/images/variations",
			"/v1/completions",
			"/v1/embeddings",
			"/v1/moderations",
			"/v1/audio/transcriptions",
			"/v1/audio/translations",
			"/v1/audio/speech",
			"/v1/realtime/transcription_sessions",
			"/v1/realtime/client_secrets",
			"/v1/realtime/calls",
			"/v1/realtime/sessions",
			"/v1/realtime/translations",
			"/v1/realtime",
			"/v1/responses",
			"/v1/files",
			"/v1/batches",
			"/v1/fine_tuning/jobs",
			"/chat/completions",
			"/images/generations",
			"/images/edits",
			"/images/variations",
			"/completions",
			"/embeddings",
			"/moderations",
			"/audio/transcriptions",
			"/audio/translations",
			"/audio/speech",
			"/realtime/transcription_sessions",
			"/realtime/client_secrets",
			"/realtime/calls",
			"/realtime/sessions",
			"/realtime/translations",
			"/realtime",
			"/responses",
			"/files",
			"/batches",
			"/fine_tuning/jobs",
			"/models",
		} {
			if strings.HasSuffix(p, suf) {
				p = strings.TrimSuffix(p, suf)
				p = strings.TrimSuffix(p, "/")
				changed = true
				break
			}
		}
		if !changed {
			break
		}
	}
	if p == "" || p == "/" {
		p = "/v1"
	} else if !strings.HasSuffix(p, "/v1") {
		p = p + "/v1"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}
