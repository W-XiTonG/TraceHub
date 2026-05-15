package gateway

import (
	"encoding/json"
	"strings"
)

// 硅基等聚合商常见：生图模型在对话补全上会返回 code=20012；补充说明引导改用生图接口或核对模型 ID（不限于控制台试玩）。
const upstreamHintModelNotExistChat = " [TraceHub：当前请求为对话补全（/v1/chat/completions），该模型在此路径不可用（常见于仅支持文生图的模型）。请改用 /v1/images/generations，或更换为支持 chat completions 的模型 ID。]"

func augmentUpstreamJSONModelNotExistChatHint(raw []byte) []byte {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return raw
	}
	code := 0
	switch v := m["code"].(type) {
	case float64:
		code = int(v)
	}
	if code != 20012 {
		return raw
	}
	msg, ok := m["message"].(string)
	if !ok {
		return raw
	}
	lm := strings.ToLower(strings.TrimSpace(msg))
	if !strings.Contains(lm, "model does not exist") {
		return raw
	}
	if strings.Contains(msg, "[TraceHub：") {
		return raw
	}
	m["message"] = strings.TrimSpace(msg) + upstreamHintModelNotExistChat
	out, err := json.Marshal(m)
	if err != nil {
		return raw
	}
	return out
}
