package klingvideo

import (
	"encoding/json"
	"strconv"
	"strings"
)

// NormalizeResponseBody 将可灵任务响应规范为 OpenAI Video 对象字段。
func NormalizeResponseBody(upstreamBody []byte) []byte {
	var root map[string]any
	if err := json.Unmarshal(upstreamBody, &root); err != nil {
		return upstreamBody
	}
	m := unwrapData(root)
	if m == nil {
		m = root
	}
	out := make(map[string]any, len(m)+4)
	for k, v := range m {
		out[k] = v
	}
	if id, ok := stringFieldAny(m, "task_id", "id", "video_id"); ok && out["id"] == nil {
		out["id"] = id
	}
	if st, ok := stringFieldAny(m, "task_status", "status", "state"); ok {
		out["status"] = mapKlingStatus(st)
	}
	if url, ok := extractVideoURL(m); ok && out["video_url"] == nil {
		out["video_url"] = url
	}
	if out["seconds"] == nil {
		if sec := extractDurationSeconds(m); sec > 0 {
			out["seconds"] = strconv.Itoa(sec)
		}
	}
	normalized, err := json.Marshal(out)
	if err != nil {
		return upstreamBody
	}
	return normalized
}

// ExtractVideoURLFromTaskBody 从任务查询响应提取可下载视频 URL。
func ExtractVideoURLFromTaskBody(body []byte) (string, bool) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return "", false
	}
	m := unwrapData(root)
	if m == nil {
		m = root
	}
	return extractVideoURL(m)
}

func unwrapData(root map[string]any) map[string]any {
	if d, ok := root["data"].(map[string]any); ok {
		return d
	}
	return nil
}

func stringFieldAny(m map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

func mapKlingStatus(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "succeed", "succeeded", "success", "done", "completed":
		return "completed"
	case "processing", "running", "in_progress":
		return "in_progress"
	case "submit", "submitted", "pending", "queued":
		return "queued"
	case "failed", "error":
		return "failed"
	default:
		return s
	}
}

func extractVideoURL(m map[string]any) (string, bool) {
	if tr, ok := m["task_result"].(map[string]any); ok {
		if u, ok := videoURLFromResult(tr); ok {
			return u, true
		}
	}
	if u, ok := m["video_url"].(string); ok && strings.TrimSpace(u) != "" {
		return strings.TrimSpace(u), true
	}
	return "", false
}

func videoURLFromResult(tr map[string]any) (string, bool) {
	videos, ok := tr["videos"].([]any)
	if !ok || len(videos) == 0 {
		return "", false
	}
	first, ok := videos[0].(map[string]any)
	if !ok {
		return "", false
	}
	for _, k := range []string{"url", "video_url", "video_url_download"} {
		if u, ok := first[k].(string); ok && strings.TrimSpace(u) != "" {
			return strings.TrimSpace(u), true
		}
	}
	return "", false
}

func extractDurationSeconds(m map[string]any) int {
	if tr, ok := m["task_result"].(map[string]any); ok {
		if videos, ok := tr["videos"].([]any); ok && len(videos) > 0 {
			if first, ok := videos[0].(map[string]any); ok {
				if d, ok := first["duration"].(string); ok {
					if n, err := strconv.Atoi(strings.TrimSpace(d)); err == nil && n > 0 {
						return n
					}
				}
			}
		}
	}
	if d, ok := m["duration"]; ok {
		switch t := d.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
				return n
			}
		}
	}
	return 0
}
