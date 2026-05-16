package openaicompat

import "strings"

// IsArkAPIv3Root 火山方舟 OpenAI 兼容根路径为 …/api/v3（非 …/v1）。
func IsArkAPIv3Root(path string) bool {
	p := strings.TrimSuffix(strings.TrimSpace(path), "/")
	return strings.HasSuffix(p, "/api/v3")
}
