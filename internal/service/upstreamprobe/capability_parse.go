package upstreamprobe

import (
	"encoding/json"
	"strconv"
	"strings"
)

const maxErrorSummaryLen = 200

// ParseUpstreamErrorBody 从上游 4xx/5xx JSON 提取 code 与摘要。
func ParseUpstreamErrorBody(body []byte) (code, summary string) {
	body = trimBodySnippet(body)
	if len(body) == 0 {
		return "", ""
	}
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return "", truncate(string(body), maxErrorSummaryLen)
	}
	if c, ok := raw["code"].(string); ok && strings.TrimSpace(c) != "" {
		code = strings.TrimSpace(c)
	} else if n, ok := raw["code"].(float64); ok {
		code = strconv.Itoa(int(n))
	}
	if msg, ok := raw["message"].(string); ok {
		summary = strings.TrimSpace(msg)
	}
	if errObj, ok := raw["error"].(map[string]any); ok {
		if summary == "" {
			if em, ok := errObj["message"].(string); ok {
				summary = strings.TrimSpace(em)
			}
		}
		if ec, ok := errObj["code"].(string); ok && code == "" {
			code = strings.TrimSpace(ec)
		}
	}
	if summary == "" {
		if em, ok := raw["error_message"].(string); ok {
			summary = strings.TrimSpace(em)
		}
	}
	return code, truncate(summary, maxErrorSummaryLen)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}

func trimBodySnippet(body []byte) []byte {
	const max = 2048
	if len(body) > max {
		return body[:max]
	}
	return body
}
