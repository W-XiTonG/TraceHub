package modelkind

import "strings"

const (
	UsageBillingToken         = "token"
	UsageBillingVideoDuration = "video_duration"
)

// UsageBillingMode 调用日志/流水展示用：视频任务将秒数写入 prompt_tokens、completion 恒为 0。
func UsageBillingMode(model string, promptTokens, completionTokens int) string {
	if completionTokens != 0 || promptTokens <= 0 || promptTokens > 3600 {
		return UsageBillingToken
	}
	id := strings.ToLower(strings.TrimSpace(model))
	if id == "" {
		return UsageBillingToken
	}
	for _, lbl := range inferFromID(id) {
		if lbl == "video" {
			return UsageBillingVideoDuration
		}
	}
	return UsageBillingToken
}
