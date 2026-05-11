package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strings"

	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"

	"github.com/gin-gonic/gin"
)

// RealtimeSessions OpenAI 推荐：POST /v1/realtime/sessions，创建含 client_secret 的临时会话（浏览器/WebRTC 用）。
// 路由模型：JSON 顶层 model，或 Query ?model=（与官方兼容且便于网关选渠道）。
func RealtimeSessions(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, est, err := extractRealtimeSessionRoutingModel(body, c.Request.URL.Query())
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model：请在 JSON 中设置 model，或在 Query 增加 model=（与 OpenAI 兼容并用于选渠道）")
			return
		}
		openAIForwardJSONCustomPatch(c, d, body, model, est, 2048, "realtime/sessions", router.PatchModelInBody)
	}
}

// RealtimeTranscriptionSessions POST /v1/realtime/transcription_sessions（实时转写专用会话 + client_secret）。
// 路由模型：优先 input_audio_transcription.model，其次顶层 model，再次 Query ?model=。
func RealtimeTranscriptionSessions(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, est, err := extractTranscriptionSessionRoutingModel(body, c.Request.URL.Query())
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model：请在 input_audio_transcription.model、顶层 model 或 Query model= 中指定（网关选渠道需要）")
			return
		}
		openAIForwardJSONCustomPatch(c, d, body, model, est, 2048, "realtime/transcription_sessions", patchTranscriptionSessionBody)
	}
}

func normalizeRealtimeJSONBody(body []byte) []byte {
	b := bytes.TrimSpace(body)
	if len(b) == 0 {
		return []byte("{}")
	}
	return b
}

func extractRealtimeSessionRoutingModel(body []byte, q url.Values) (model string, promptEst string, err error) {
	b := normalizeRealtimeJSONBody(body)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["model"]; ok {
		var s string
		_ = json.Unmarshal(raw, &s)
		model = strings.TrimSpace(s)
	}
	if model == "" {
		model = strings.TrimSpace(q.Get("model"))
	}
	return model, string(b), nil
}

func extractTranscriptionSessionRoutingModel(body []byte, q url.Values) (model string, promptEst string, err error) {
	b := normalizeRealtimeJSONBody(body)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["input_audio_transcription"]; ok {
		var iat map[string]json.RawMessage
		if json.Unmarshal(raw, &iat) == nil {
			if raw2, ok2 := iat["model"]; ok2 {
				var s string
				_ = json.Unmarshal(raw2, &s)
				model = strings.TrimSpace(s)
			}
		}
	}
	if model == "" {
		if raw, ok := m["model"]; ok {
			var s string
			_ = json.Unmarshal(raw, &s)
			model = strings.TrimSpace(s)
		}
	}
	if model == "" {
		model = strings.TrimSpace(q.Get("model"))
	}
	return model, string(b), nil
}

// patchTranscriptionSessionBody 将 input_audio_transcription.model 映射为上游名；保留其余字段。
func patchTranscriptionSessionBody(body []byte, upstream string) ([]byte, error) {
	b := normalizeRealtimeJSONBody(body)
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	iat, ok := m["input_audio_transcription"].(map[string]interface{})
	if !ok || iat == nil {
		m["input_audio_transcription"] = map[string]interface{}{"model": upstream}
	} else {
		iat["model"] = upstream
		m["input_audio_transcription"] = iat
	}
	if _, has := m["model"]; has {
		m["model"] = upstream
	}
	return json.Marshal(m)
}
