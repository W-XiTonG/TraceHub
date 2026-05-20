package router

import "strings"

// 默认可走 OpenAI 兼容 JSON/multipart/HTTP relay 的渠道类型。
var builtinOpenAICompatChannelTypes = map[string]struct{}{
	"openai":   {},
	"deepseek": {},
}

// IsOpenAICompatChannelType 判断渠道是否应按 OpenAI 兼容协议转发（含站点配置的额外类型，如硅基等自建 openai 类型名）。
func IsOpenAICompatChannelType(channelType string, extra []string) bool {
	ct := strings.ToLower(strings.TrimSpace(channelType))
	if _, ok := builtinOpenAICompatChannelTypes[ct]; ok {
		return true
	}
	for _, e := range extra {
		if strings.ToLower(strings.TrimSpace(e)) == ct {
			return true
		}
	}
	return false
}

// UsesOpenAICompatRelay 是否走 OpenAI 兼容 JSON/SSE/multipart relay（含火山方舟 Ark /api/v3）。
func UsesOpenAICompatRelay(channelType string, extra []string) bool {
	if IsOpenAICompatChannelType(channelType, extra) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(channelType), "volcengine")
}
