package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StreamGeminiGenerateContentAsOpenAIChatSSE OpenAI Chat -> Gemini streamGenerateContent（alt=sse），并写出 OpenAI chat.completion.chunk SSE。
func StreamGeminiGenerateContentAsOpenAIChatSSE(ctx context.Context, apiKey, baseURL string, openAIBody []byte, upstreamModel, clientModel string, w io.Writer, flusher http.Flusher) (promptTok, completionTok int, err error) {
	resp, err := ForwardGeminiGenerateContent(ctx, apiKey, baseURL, openAIBody, upstreamModel, true)
	if err != nil {
		return 0, 0, err
	}
	defer DrainBody(resp.Body)
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return 0, 0, &UpstreamJSONError{StatusCode: resp.StatusCode, Body: raw}
	}
	return StreamGeminiSSEBodyAsOpenAI(resp.Body, clientModel, w, flusher)
}

// StreamGeminiSSEBodyAsOpenAI 将 Gemini alt=sse 响应转为 OpenAI chat.completion.chunk SSE。
func StreamGeminiSSEBodyAsOpenAI(body io.Reader, clientModel string, w io.Writer, flusher http.Flusher) (promptTok, completionTok int, err error) {
	created := time.Now().Unix()
	chatID := "chatcmpl-" + uuid.NewString()
	roleSent := false
	lastAssistant := ""

	flushChunk := func(obj map[string]any) error {
		b, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	br := bufio.NewReader(body)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return promptTok, completionTok, err
		}
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "" || raw == "[DONE]" {
			continue
		}

		var payload map[string]any
		if json.Unmarshal([]byte(raw), &payload) != nil {
			continue
		}

		if um, ok := payload["usageMetadata"].(map[string]any); ok {
			if v, ok := um["promptTokenCount"].(float64); ok {
				promptTok = int(v)
			}
			if v, ok := um["candidatesTokenCount"].(float64); ok {
				completionTok = int(v)
			}
		}

		full := geminiCandidateFullText(payload)
		var delta string
		if full != "" {
			if lastAssistant != "" && strings.HasPrefix(full, lastAssistant) {
				delta = full[len(lastAssistant):]
			} else {
				delta = full
			}
			lastAssistant = full
		}
		if delta == "" {
			continue
		}

		if !roleSent {
			roleSent = true
			if err := flushChunk(map[string]any{
				"id":      chatID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   clientModel,
				"choices": []any{map[string]any{
					"index": 0,
					"delta": map[string]any{"role": "assistant"},
				}},
			}); err != nil {
				return promptTok, completionTok, err
			}
		}
		if err := flushChunk(map[string]any{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   clientModel,
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"content": delta},
			}},
		}); err != nil {
			return promptTok, completionTok, err
		}
	}

	if !roleSent {
		_ = flushChunk(map[string]any{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   clientModel,
			"choices": []any{map[string]any{
				"index": 0,
				"delta": map[string]any{"role": "assistant"},
			}},
		})
	}

	usageObj := map[string]any{}
	if promptTok > 0 || completionTok > 0 {
		usageObj["prompt_tokens"] = promptTok
		usageObj["completion_tokens"] = completionTok
		usageObj["total_tokens"] = promptTok + completionTok
	}
	last := map[string]any{
		"id":      chatID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   clientModel,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
	}
	if len(usageObj) > 0 {
		last["usage"] = usageObj
	}
	if err := flushChunk(last); err != nil {
		return promptTok, completionTok, err
	}
	if _, err := fmt.Fprintf(w, "data: [DONE]\n\n"); err != nil {
		return promptTok, completionTok, err
	}
	if flusher != nil {
		flusher.Flush()
	}
	return promptTok, completionTok, nil
}

func geminiCandidateFullText(resp map[string]any) string {
	cands, ok := resp["candidates"].([]any)
	if !ok || len(cands) == 0 {
		return ""
	}
	cd, ok := cands[0].(map[string]any)
	if !ok {
		return ""
	}
	content, ok := cd["content"].(map[string]any)
	if !ok {
		return ""
	}
	parts, ok := content["parts"].([]any)
	if !ok {
		return ""
	}
	var b strings.Builder
	for _, p := range parts {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		if t, ok := pm["text"].(string); ok {
			b.WriteString(t)
		}
	}
	return b.String()
}
