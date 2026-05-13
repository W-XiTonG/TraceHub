package gateway

import (
	"encoding/json"
	"strings"

	"tracehub/internal/service/billing"
)

// balancePrecheck402Message 预检阶段 402 文案，便于与 403 权限/选路及成功后的扣费失败区分。
func balancePrecheck402Message() string {
	return "余额不足（预检）：当前余额小于本次请求预估消耗。若为模型或渠道不可用，请查看 HTTP 403 与 error.type=permission_error。"
}

const (
	visionTokensLow      = 170  // 文档 low ≈85，预检取 2×
	visionTokensHighAuto = 2000 // high/auto 上限量级保守值，远低于误计 base64
)

func visionTokensForImageDetail(detail string) int {
	switch strings.ToLower(strings.TrimSpace(detail)) {
	case "low":
		return visionTokensLow
	case "high", "auto", "":
		return visionTokensHighAuto
	default:
		return visionTokensHighAuto
	}
}

// estimateTokensFromChatMessagesRaw 从 chat.completions 的 messages JSON 估算 prompt token：
// 文本走 estimateFn（通常为 tiktoken），图片 URL 不进入文本；每条 image 按 detail 累加视觉 token。
func estimateTokensFromChatMessagesRaw(msgsRaw json.RawMessage, upstreamModel string, estimateFn func(model, text string) (int, error)) (int, error) {
	var messages []map[string]any
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return 0, err
	}
	return estimateTokensFromChatMessages(messages, upstreamModel, estimateFn)
}

func estimateTokensFromChatMessages(messages []map[string]any, upstreamModel string, estimateFn func(model, text string) (int, error)) (int, error) {
	var sb strings.Builder
	imgExtra := 0
	for _, msg := range messages {
		content, ok := msg["content"]
		if !ok {
			continue
		}
		imgExtra += appendMessageContentForEstimate(&sb, content)
	}
	text := sb.String()
	textTok, err := estimateFn(upstreamModel, text)
	if err != nil {
		return 0, err
	}
	n := textTok + imgExtra
	if n < 1 {
		n = 1
	}
	return n, nil
}

func appendMessageContentForEstimate(sb *strings.Builder, content any) (imageExtra int) {
	switch v := content.(type) {
	case string:
		sb.WriteString(v)
		sb.WriteByte('\n')
		return 0
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			imageExtra += appendOneContentPart(sb, m)
		}
		return imageExtra
	default:
		// 少见结构：不参与 tiktoken，避免整段 JSON/base64
		return 0
	}
}

func appendOneContentPart(sb *strings.Builder, part map[string]any) (imageExtra int) {
	typ, _ := part["type"].(string)
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "text":
		if t, ok := part["text"].(string); ok {
			sb.WriteString(t)
			sb.WriteByte('\n')
		}
		return 0
	case "image_url":
		if ustr, ok := part["image_url"].(string); ok {
			return imageURLForBillingEstimate(sb, ustr, "")
		}
		iu, _ := part["image_url"].(map[string]any)
		if iu == nil {
			sb.WriteString("\n<image>\n")
			return visionTokensHighAuto
		}
		detail, _ := iu["detail"].(string)
		urlStr, _ := iu["url"].(string)
		return imageURLForBillingEstimate(sb, urlStr, detail)
	case "input_image":
		// Responses 风格多模态块
		detail, _ := part["detail"].(string)
		urlStr, _ := part["image_url"].(string)
		if urlStr == "" {
			if m, ok := part["image_url"].(map[string]any); ok {
				urlStr, _ = m["url"].(string)
				if d, ok := m["detail"].(string); ok && detail == "" {
					detail = d
				}
			}
		}
		return imageURLForBillingEstimate(sb, urlStr, detail)
	default:
		return 0
	}
}

func imageURLForBillingEstimate(sb *strings.Builder, urlStr, detail string) int {
	urlStr = strings.TrimSpace(urlStr)
	if urlStr == "" {
		sb.WriteString("\n<image>\n")
		return visionTokensForImageDetail(detail)
	}
	lu := strings.ToLower(urlStr)
	if strings.HasPrefix(lu, "data:image/") {
		sb.WriteString("\n<image_data_url>\n")
		return visionTokensForImageDetail(detail)
	}
	// 疑似内联超长（无标准前缀的 base64），避免整串进 tiktoken
	if len(urlStr) > 2048 {
		sb.WriteString("\n<image_inline_long>\n")
		return visionTokensForImageDetail(detail)
	}
	sb.WriteString(urlStr)
	sb.WriteByte('\n')
	return visionTokensForImageDetail(detail)
}

// estimateTokensFromResponsesInputRaw 解析 Responses API 的 input 字段（string | 数组），用于预检。
func estimateTokensFromResponsesInputRaw(inputRaw json.RawMessage, upstreamModel string, estimateFn func(model, text string) (int, error)) (int, error) {
	var s string
	if json.Unmarshal(inputRaw, &s) == nil {
		n, err := estimateFn(upstreamModel, s)
		if err != nil {
			return 0, err
		}
		if n < 1 {
			return 1, nil
		}
		return n, nil
	}
	var arr []any
	if json.Unmarshal(inputRaw, &arr) != nil {
		// 无法解析则退回极短占位，避免整包 JSON+base64
		n, err := estimateFn(upstreamModel, "<unparsed_input>\n")
		if err != nil {
			return 0, err
		}
		if n < 1 {
			return 1, nil
		}
		return n, nil
	}
	var sb strings.Builder
	imgExtra := 0
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := m["type"].(string)
		switch strings.ToLower(strings.TrimSpace(typ)) {
		case "message":
			if content, ok := m["content"]; ok {
				imgExtra += appendMessageContentForEstimate(&sb, content)
			}
		case "input_text":
			if t, ok := m["text"].(string); ok {
				sb.WriteString(t)
				sb.WriteByte('\n')
			}
		default:
			imgExtra += appendOneContentPart(&sb, m)
		}
	}
	textTok, err := estimateFn(upstreamModel, sb.String())
	if err != nil {
		return 0, err
	}
	n := textTok + imgExtra
	if n < 1 {
		n = 1
	}
	return n, nil
}

// EstimateChatPromptTokensFromRequestBody 从 chat.completions JSON 估算 prompt token（多模态安全）。
func EstimateChatPromptTokensFromRequestBody(body []byte, upstreamModel string, eng *billing.Engine) (int, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return 0, err
	}
	msgsRaw, ok := m["messages"]
	if !ok {
		tok, err := eng.EstimateTokens(upstreamModel, "")
		if err != nil {
			return 0, err
		}
		if tok < 1 {
			return 1, nil
		}
		return tok, nil
	}
	return estimateTokensFromChatMessagesRaw(msgsRaw, upstreamModel, eng.EstimateTokens)
}

// EstimateResponsesInputPromptTokens 从 Responses API 的 input 字段估算 prompt token。
func EstimateResponsesInputPromptTokens(inputRaw json.RawMessage, upstreamModel string, eng *billing.Engine) (int, error) {
	return estimateTokensFromResponsesInputRaw(inputRaw, upstreamModel, eng.EstimateTokens)
}

// EstimatePromptTokensFromOpenAIBody 供透传预检：优先 messages，其次 input，再次 prompt 字符串。
// routingModel 在无正文字段时作为 tiktoken 估算回退（与旧逻辑 promptEst=model 一致）。
func EstimatePromptTokensFromOpenAIBody(body []byte, upstreamModel, routingModel string, eng *billing.Engine) (int, error) {
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return cappedStringEstimate(upstreamModel, strings.TrimSpace(routingModel), eng)
	}
	if raw, ok := m["messages"]; ok {
		return estimateTokensFromChatMessagesRaw(raw, upstreamModel, eng.EstimateTokens)
	}
	if raw, ok := m["input"]; ok {
		return estimateTokensFromResponsesInputRaw(raw, upstreamModel, eng.EstimateTokens)
	}
	if raw, ok := m["prompt"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return cappedStringEstimate(upstreamModel, s, eng)
		}
		var ss []string
		if json.Unmarshal(raw, &ss) == nil {
			return cappedStringEstimate(upstreamModel, strings.Join(ss, "\n"), eng)
		}
		return cappedStringEstimate(upstreamModel, string(raw), eng)
	}
	tok, err := eng.EstimateTokens(upstreamModel, strings.TrimSpace(routingModel))
	if err != nil {
		return 0, err
	}
	if tok < 1 {
		return 1, nil
	}
	return tok, nil
}

// cappedStringEstimate 避免超长单字段进入 tiktoken（与图片生成 parseImageGenBody 截断思路一致）。
func cappedStringEstimate(upstreamModel, s string, eng *billing.Engine) (int, error) {
	const maxRunes = 32000
	r := []rune(s)
	if len(r) > maxRunes {
		s = string(r[:maxRunes]) + "\n<truncated>\n"
	}
	tok, err := eng.EstimateTokens(upstreamModel, s)
	if err != nil {
		return 0, err
	}
	if tok < 1 {
		return 1, nil
	}
	return tok, nil
}
