package feedbackpreset

// AllowedPresetKeys 非空 preset_key 必须在此集合中；空字符串表示用户自定义分类。
var AllowedPresetKeys = map[string]struct{}{
	"billing":         {},
	"api_error":       {},
	"model_routing":   {},
	"latency_timeout": {},
	"playground_ui":   {},
	"docs_unclear":    {},
	"feature_request": {},
	"security":        {},
	"other":           {},
}

// ValidPresetKey 返回 true 当 k 表示自定义（空）或已知预设。
func ValidPresetKey(k string) bool {
	if k == "" {
		return true
	}
	_, ok := AllowedPresetKeys[k]
	return ok
}
