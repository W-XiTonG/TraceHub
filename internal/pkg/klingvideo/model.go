package klingvideo

import (
	"encoding/json"
	"strconv"
	"strings"
)

// ExtractModelField 从 JSON 提取 model_name 或 model（选路用）。
func ExtractModelField(body []byte) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	for _, key := range []string{"model_name", "model"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// PatchModelInBody 将 model_name 与 model 同步为上游别名（双写，兼容官方字段）。
func PatchModelInBody(body []byte, upstream string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	b, err := json.Marshal(upstream)
	if err != nil {
		return nil, err
	}
	m["model_name"] = json.RawMessage(b)
	m["model"] = json.RawMessage(b)
	return json.Marshal(m)
}

// JobParamsFromBody 从创建请求解析视频计费参数。
func JobParamsFromBody(body []byte) (seconds int, size string) {
	var m map[string]any
	if json.Unmarshal(body, &m) != nil {
		return 5, ""
	}
	tmp := map[string]any{}
	for k, v := range m {
		tmp[k] = v
	}
	if _, ok := m["seconds"]; !ok {
		if d, ok := m["duration"]; ok {
			tmp["seconds"] = d
		}
	}
	if s, ok := m["size"].(string); ok {
		size = s
	}
	if ar, ok := m["aspect_ratio"].(string); ok && size == "" {
		size = ar
	}
	return clampDuration(parseDurationMap(tmp)), size
}

func parseDurationMap(m map[string]any) int {
	for _, k := range []string{"seconds", "duration"} {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case string:
				if n, err := parseIntString(t); err == nil {
					return n
				}
			case float64:
				return int(t)
			case int:
				return t
			}
		}
	}
	return 5
}

func parseIntString(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}
