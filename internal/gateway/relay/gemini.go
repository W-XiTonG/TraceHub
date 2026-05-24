package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BuildGeminiGenerateContentBody OpenAI Chat JSON -> Gemini generateContent 请求体。
func BuildGeminiGenerateContentBody(openAIBody []byte) ([]byte, error) {
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

	var sysText []string
	var contents []map[string]any
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		text, _ := stringifyContent(msg["content"])
		switch role {
		case "system":
			sysText = append(sysText, text)
		case "assistant":
			contents = append(contents, map[string]any{
				"role": "model",
				"parts": []map[string]any{
					{"text": text},
				},
			})
		default:
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"text": text},
				},
			})
		}
	}

	payload := map[string]any{
		"contents": contents,
	}
	if len(sysText) > 0 {
		payload["systemInstruction"] = map[string]any{
			"parts": []map[string]any{{"text": strings.Join(sysText, "\n")}},
		}
	}

	return json.Marshal(payload)
}

// ForwardGeminiGenerateContent 转发至 Gemini generateContent，返回原始 HTTP 响应。
func ForwardGeminiGenerateContent(ctx context.Context, apiKey, baseURL string, openAIBody []byte, upstreamModel string, stream bool) (*http.Response, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	m := strings.TrimPrefix(upstreamModel, "models/")
	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:%s?key=%s", base, url.PathEscape(m), action, url.QueryEscape(apiKey))
	if stream {
		apiURL += "&alt=sse"
	}

	body, err := BuildGeminiGenerateContentBody(openAIBody)
	if err != nil {
		return nil, err
	}
	reqCtx := ctx
	if stream {
		reqCtx = StreamUpstreamContext(ctx)
	}
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
		httpReq.Header.Set("Accept-Encoding", "identity")
		return httpStreamUpstream.Do(httpReq)
	}
	return httpUpstream.Do(httpReq)
}

// CallGeminiGenerateContent OpenAI Chat -> Gemini generateContent（非流式）。
func CallGeminiGenerateContent(ctx context.Context, apiKey string, baseURL string, openAIBody []byte, upstreamModel string, clientModel string) ([]byte, int, error) {
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com"
	}
	m := strings.TrimPrefix(upstreamModel, "models/")
	apiURL := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", base, url.PathEscape(m), url.QueryEscape(apiKey))

	body, err := BuildGeminiGenerateContentBody(openAIBody)
	if err != nil {
		return nil, 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
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

	var gem map[string]any
	if err := json.Unmarshal(raw, &gem); err != nil {
		return nil, resp.StatusCode, err
	}
	openAI, err := geminiToOpenAI(gem, clientModel)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	out, err := json.Marshal(openAI)
	return out, resp.StatusCode, err
}

func geminiToOpenAI(gem map[string]any, model string) (map[string]any, error) {
	text := ""
	if cands, ok := gem["candidates"].([]any); ok && len(cands) > 0 {
		if cd, ok := cands[0].(map[string]any); ok {
			if content, ok := cd["content"].(map[string]any); ok {
				if parts, ok := content["parts"].([]any); ok {
					for _, p := range parts {
						pm, ok := p.(map[string]any)
						if !ok {
							continue
						}
						if t, ok := pm["text"].(string); ok {
							text += t
						}
					}
				}
			}
		}
	}
	pt, ct := 0, 0
	if um, ok := gem["usageMetadata"].(map[string]any); ok {
		if v, ok := um["promptTokenCount"].(float64); ok {
			pt = int(v)
		}
		if v, ok := um["candidatesTokenCount"].(float64); ok {
			ct = int(v)
		}
	}
	return map[string]any{
		"id":      "chatcmpl-" + uuid.NewString(),
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
				"finish_reason": "stop",
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     pt,
			"completion_tokens": ct,
			"total_tokens":      pt + ct,
		},
	}, nil
}
