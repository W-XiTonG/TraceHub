package modelkind

import "strings"

// IsVideoBillingModel 是否应按视频（每秒）计费展示与计价；与 inferFromID 中 video 规则一致。
func IsVideoBillingModel(modelID string) bool {
	id := strings.ToLower(strings.TrimSpace(modelID))
	if id == "" {
		return false
	}
	if strings.HasPrefix(id, "route:") {
		path := strings.TrimSpace(strings.TrimPrefix(id, "route:"))
		return strings.Contains(path, "/videos") || strings.Contains(path, "/video/")
	}
	for _, lbl := range inferFromID(id) {
		if lbl == "video" {
			return true
		}
	}
	return false
}
