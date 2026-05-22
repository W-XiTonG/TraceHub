package modelkind

import "strings"

const (
	UsageBillingToken         = "token"
	UsageBillingVideoDuration = "video_duration"
)

// UsageBillingMode 调用日志/流水展示用：视频任务将秒数写入 prompt_tokens、completion 恒为 0。
// 失败轮询、路由占位等未写入秒数时，仍按模型/路径识别为 video_duration 以便展示「视频·按秒」。
func UsageBillingMode(model string, promptTokens, completionTokens int) string {
	if completionTokens != 0 {
		return UsageBillingToken
	}
	if IsVideoBillingModel(model) {
		return UsageBillingVideoDuration
	}
	if promptTokens > 0 && promptTokens <= 3600 {
		id := strings.ToLower(strings.TrimSpace(model))
		for _, lbl := range inferFromID(id) {
			if lbl == "video" {
				return UsageBillingVideoDuration
			}
		}
	}
	return UsageBillingToken
}
