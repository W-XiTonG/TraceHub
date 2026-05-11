package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BuildAnthropicMessagesBody 将 OpenAI Chat Completions JSON 转为 Anthropic Messages 请求体。
func BuildAnthropicMessagesBody(openAIBody []byte, upstreamModel string, stream bool) ([]byte, error) {
	var req map[string]json.RawMessage
	if err := json.Unmarshal(openAIBody, &req); err != nil {
		return nil, err
	}

	msgsRaw, ok := req["messages"]
	if !ok {
		return nil, fmt.Errorf("missing messages")
	}
	var messages []map[string]any
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return nil, err
	}

	var sysParts []string
	var anthMsgs []map[string]any
	for _, m := range messages {
		role, _ := m["role"].(string)
		switch role {
		case "system":
			if s, ok := stringifyContent(m["content"]); ok {
				sysParts = append(sysParts, s)
			}
		case "user", "assistant":
			anthMsgs = append(anthMsgs, map[string]any{
				"role":    role,
				"content": m["content"],
			})
		default:
			if s, ok := stringifyContent(m["content"]); ok {
				anthMsgs = append(anthMsgs, map[string]any{"role": "user", "content": s})
			}
		}
	}

	maxTok := 1024
	if v, ok := req["max_tokens"]; ok {
		var mt int
		_ = json.Unmarshal(v, &mt)
		if mt > 0 {
			maxTok = mt
		}
	}
	if v, ok := req["max_completion_tokens"]; ok {
		var mt int
		_ = json.Unmarshal(v, &mt)
		if mt > 0 {
			maxTok = mt
		}
	}

	payload := map[string]any{
		"model":       upstreamModel,
		"max_tokens":  maxTok,
		"messages":    anthMsgs,
		"temperature": unmarshalFloat(req, "temperature", 1.0),
		"stream":      stream,
	}
	if len(sysParts) > 0 {
		payload["system"] = strings.Join(sysParts, "\n")
	}

	return json.Marshal(payload)
}

// CallClaudeMessages 将 OpenAI Chat Completions 请求转换为 Anthropic Messages API（非流式）。
// clientModel 为返回给下游的 model 字段（通常与用户请求的 model 一致）。
func CallClaudeMessages(ctx context.Context, apiKey string, baseURL string, openAIBody []byte, upstreamModel string, clientModel string) ([]byte, int, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	url := base + "/messages"

	body, err := BuildAnthropicMessagesBody(openAIBody, upstreamModel, false)
	if err != nil {
		return nil, 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpUpstream.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer DrainBody(resp.Body)
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return raw, resp.StatusCode, fmt.Errorf("upstream error: %s", string(raw))
	}

	var anth map[string]any
	if err := json.Unmarshal(raw, &anth); err != nil {
		return nil, resp.StatusCode, err
	}
	openAI, err := anthropicToOpenAI(anth, clientModel)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	out, err := json.Marshal(openAI)
	return out, resp.StatusCode, err
}

func unmarshalFloat(req map[string]json.RawMessage, key string, def float64) float64 {
	v, ok := req[key]
	if !ok {
		return def
	}
	var f float64
	if json.Unmarshal(v, &f) == nil {
		return f
	}
	return def
}

func stringifyContent(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	default:
		b, err := json.Marshal(v)
		return string(b), err == nil
	}
}

func anthropicToOpenAI(anth map[string]any, model string) (map[string]any, error) {
	id, _ := anth["id"].(string)
	if id == "" {
		id = "chatcmpl-" + uuid.NewString()
	}
	text := ""
	if contents, ok := anth["content"].([]any); ok {
		for _, c := range contents {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if cm["type"] == "text" {
				if s, ok := cm["text"].(string); ok {
					text += s
				}
			}
		}
	}
	finish := "stop"
	if sr, ok := anth["stop_reason"].(string); ok && sr != "" {
		switch sr {
		case "end_turn":
			finish = "stop"
		case "max_tokens":
			finish = "length"
		default:
			finish = sr
		}
	}
	pt, ct := 0, 0
	if usage, ok := anth["usage"].(map[string]any); ok {
		if v, ok := usage["input_tokens"].(float64); ok {
			pt = int(v)
		}
		if v, ok := usage["output_tokens"].(float64); ok {
			ct = int(v)
		}
	}
	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": text,
				},
				"finish_reason": finish,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     pt,
			"completion_tokens": ct,
			"total_tokens":      pt + ct,
		},
	}, nil
}
