package gateway

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"tracehub/internal/service/billing"
)

const usageNotePrefix = "usage_note:"

// StreamUsageCapture 流式 SSE 透传过程中采集的用量信息。
type StreamUsageCapture struct {
	PromptTokens       int
	CompletionTokens   int
	CachedPromptTokens int
	DeltaContent       string
	ClientDisconnected bool
	HasUpstreamUsage   bool
}

// ResolvedStreamUsage 流式结束后用于计费的 token 与说明。
type ResolvedStreamUsage struct {
	PromptTokens       int
	CompletionTokens   int
	CachedPromptTokens int
	Note               string
}

// copyOpenAIChatSSEWithUsage 透传 OpenAI 兼容 SSE，解析 usage 并累积 delta 文本。
func copyOpenAIChatSSEWithUsage(w http.ResponseWriter, r io.Reader) StreamUsageCapture {
	var cap StreamUsageCapture
	var contentBuf strings.Builder
	out := WriterFlushWrap(w)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if _, werr := io.WriteString(out, line); werr != nil {
			cap.ClientDisconnected = true
			cap.DeltaContent = contentBuf.String()
			return cap
		}
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(s, "data:"))
			if payload != "" && payload != "[DONE]" {
				appendOpenAIStreamDelta(&contentBuf, []byte(payload))
				u := ParseOpenAIUsageJSON([]byte(payload))
				if u.PromptTokens > 0 || u.CompletionTokens > 0 || u.CachedPromptTokens > 0 {
					cap.HasUpstreamUsage = true
					cap.PromptTokens = u.PromptTokens
					cap.CompletionTokens = u.CompletionTokens
					cap.CachedPromptTokens = u.CachedPromptTokens
				} else {
					flex := parseUsageFlexible([]byte(payload))
					if flex.PromptTokens > 0 || flex.CompletionTokens > 0 || flex.CachedPromptTokens > 0 {
						cap.HasUpstreamUsage = true
						cap.PromptTokens = flex.PromptTokens
						cap.CompletionTokens = flex.CompletionTokens
						cap.CachedPromptTokens = flex.CachedPromptTokens
					}
				}
			}
		}
		if err != nil {
			cap.DeltaContent = contentBuf.String()
			if err == io.EOF {
				return cap
			}
			return cap
		}
	}
}

func appendOpenAIStreamDelta(b *strings.Builder, payload []byte) {
	var chunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
			Text string `json:"text"`
		} `json:"choices"`
	}
	if json.Unmarshal(payload, &chunk) != nil {
		return
	}
	for _, ch := range chunk.Choices {
		if s := ch.Delta.Content; s != "" {
			b.WriteString(s)
		}
		if s := ch.Delta.ReasoningContent; s != "" {
			b.WriteString(s)
		}
		if s := ch.Text; s != "" {
			b.WriteString(s)
		}
	}
}

// ResolveOpenAIStreamUsage 按上游 usage → 流式内容 tiktoken → 显式 max_tokens 顺序解析用量。
func ResolveOpenAIStreamUsage(capture StreamUsageCapture, eng *billing.Engine, upstreamModel string, promptEst, maxOut int, maxOutExplicit bool) ResolvedStreamUsage {
	out := ResolvedStreamUsage{
		PromptTokens:       capture.PromptTokens,
		CachedPromptTokens: capture.CachedPromptTokens,
	}
	if out.PromptTokens < 1 {
		out.PromptTokens = promptEst
	}
	if capture.CompletionTokens > 0 {
		out.CompletionTokens = capture.CompletionTokens
		if capture.HasUpstreamUsage {
			out.Note = usageNotePrefix + "upstream"
		}
		return out
	}
	content := capture.DeltaContent
	if content != "" && eng != nil {
		if est, err := eng.EstimateTokens(upstreamModel, content); err == nil && est >= 0 {
			out.CompletionTokens = est
			out.Note = usageNotePrefix + "stream_content"
			return out
		}
	}
	if capture.ClientDisconnected && content == "" {
		out.CompletionTokens = 0
		out.Note = usageNotePrefix + "client_disconnect"
		return out
	}
	if maxOutExplicit && maxOut > 0 {
		out.CompletionTokens = completionTokensFallback(maxOut)
		out.Note = usageNotePrefix + "max_tokens"
		return out
	}
	out.CompletionTokens = 0
	if content == "" {
		out.Note = usageNotePrefix + "no_output"
	} else {
		out.Note = usageNotePrefix + "stream_content"
	}
	return out
}

func precheckCompletionAssumed(maxOut int, explicit bool, defaultWhenUnset int) int {
	if explicit && maxOut > 0 {
		return completionTokensFallback(maxOut)
	}
	if defaultWhenUnset > 0 {
		return completionTokensFallback(defaultWhenUnset)
	}
	return completionTokensFallback(512)
}

func usageNoteForLog(note string) *string {
	if note == "" || note == usageNotePrefix+"upstream" {
		return nil
	}
	n := note
	return &n
}

func isUsageNoteMessage(msg string) bool {
	return strings.HasPrefix(strings.TrimSpace(msg), usageNotePrefix)
}
