package relay

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// errAnthropicSSEHalt 表示已在 SSE 中写出错误帧并结束流，调用方勿再写响应体。
var errAnthropicSSEHalt = errors.New("anthropic stream halted with error frame")

// StreamClaudeMessagesAsOpenAIChatSSE 将 OpenAI Chat 请求走 Anthropic Messages 流式接口，并向下游写出 OpenAI chat.completion.chunk SSE。
func StreamClaudeMessagesAsOpenAIChatSSE(ctx context.Context, apiKey, baseURL string, openAIBody []byte, upstreamModel, clientModel string, w io.Writer, flusher http.Flusher) (promptTok, completionTok int, err error) {
	resp, err := ForwardClaudeMessages(ctx, apiKey, baseURL, openAIBody, upstreamModel, true)
	if err != nil {
		return 0, 0, err
	}
	defer DrainBody(resp.Body)
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return 0, 0, &UpstreamJSONError{StatusCode: resp.StatusCode, Body: raw}
	}
	return StreamClaudeAnthropicBodyAsOpenAI(resp.Body, clientModel, w, flusher)
}

// StreamClaudeAnthropicBodyAsOpenAI 将 Anthropic SSE 转为 OpenAI chat.completion.chunk SSE（供 failover 复用已建立的流式响应）。
func StreamClaudeAnthropicBodyAsOpenAI(body io.Reader, clientModel string, w io.Writer, flusher http.Flusher) (promptTok, completionTok int, err error) {
	created := time.Now().Unix()
	chatID := "chatcmpl-" + uuid.NewString()
	roleSent := false
	finish := "stop"

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

	dispatch := func(dataStr string) error {
		var payload map[string]any
		if json.Unmarshal([]byte(dataStr), &payload) != nil {
			return nil
		}
		typ, _ := payload["type"].(string)
		switch typ {
		case "error":
			if errObj, ok := payload["error"].(map[string]any); ok {
				_ = flushChunk(map[string]any{"error": errObj})
			}
			_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
			return errAnthropicSSEHalt
		case "message_start":
			if msg, ok := payload["message"].(map[string]any); ok {
				if id, _ := msg["id"].(string); id != "" {
					chatID = id
				}
				if usage, ok := msg["usage"].(map[string]any); ok {
					if v, ok := usage["input_tokens"].(float64); ok {
						promptTok = int(v)
					}
				}
			}
		case "content_block_delta":
			delta, _ := payload["delta"].(map[string]any)
			if delta == nil {
				return nil
			}
			dt, _ := delta["type"].(string)
			if dt != "text_delta" {
				return nil
			}
			text, _ := delta["text"].(string)
			if text == "" {
				return nil
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
					return err
				}
			}
			return flushChunk(map[string]any{
				"id":      chatID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   clientModel,
				"choices": []any{map[string]any{
					"index": 0,
					"delta": map[string]any{"content": text},
				}},
			})
		case "message_delta":
			if uu, ok := payload["usage"].(map[string]any); ok {
				if v, ok := uu["output_tokens"].(float64); ok {
					completionTok = int(v)
				}
			}
			if d, ok := payload["delta"].(map[string]any); ok {
				if sr, ok := d["stop_reason"].(string); ok && sr != "" {
					switch sr {
					case "end_turn":
						finish = "stop"
					case "max_tokens":
						finish = "length"
					default:
						finish = sr
					}
				}
			}
		}
		return nil
	}

	br := bufio.NewReader(body)
	var dataLines []string
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return promptTok, completionTok, err
		}
		line = strings.TrimRight(strings.TrimRight(line, "\n"), "\r")
		if line == "" {
			if len(dataLines) > 0 {
				dataStr := strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]
				if e := dispatch(dataStr); e != nil {
					if errors.Is(e, errAnthropicSSEHalt) {
						return promptTok, completionTok, nil
					}
					return promptTok, completionTok, e
				}
			}
			if err == io.EOF {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if err == io.EOF {
			if len(dataLines) > 0 {
				dataStr := strings.Join(dataLines, "\n")
				dataLines = nil
				if e := dispatch(dataStr); e != nil {
					if errors.Is(e, errAnthropicSSEHalt) {
						return promptTok, completionTok, nil
					}
					return promptTok, completionTok, e
				}
			}
			break
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
			"finish_reason": finish,
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
