package gateway

import (
	"encoding/json"
	"strings"
)

// 硅基等聚合商常见：生图模型在对话补全上会返回 code=20012；提示用户改试玩模式即可，避免罗列接口路径。
const upstreamHintModelNotExistChat = " [TraceHub：该模型不支持当前对话试玩，请在「API 试玩」侧栏把「试玩类型」切换为「文生图」后重试。]"

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
