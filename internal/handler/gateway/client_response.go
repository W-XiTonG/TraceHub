package gateway

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"tracehub/internal/middleware"
)

const (
	MsgServiceUnavailable    = "服务暂时不可用，请稍后重试"
	MsgRequestFailed         = "请求处理失败，请检查参数后重试"
	MsgChannelMisconfigured  = "服务配置异常，请联系管理员"
	MsgCapabilityUnsupported = "当前分组不支持该能力，请联系管理员配置渠道"
	// MsgUpstreamAuthFailed 上游渠道返回的认证类错误（经 WriteClientProxy 脱敏）；勿与 TraceHub API Key 校验失败混淆。
	MsgUpstreamAuthFailed = "上游模型服务认证失败（与您的 TraceHub API Key 无关），请联系管理员检查分组渠道的密钥与接口地址。"
)

var reUpstreamRequestID = regexp.MustCompile(`(?i)(request\s*id|r_id)\s*[:：]\s*[a-zA-Z0-9_-]+`)

// WriteClientProxy 写回代理响应；4xx/5xx 的 JSON 错误体会脱敏为 TraceHub 平台文案。
func WriteClientProxy(c *gin.Context, statusCode int, contentType string, body []byte) {
	body = SanitizeClientProxyBody(statusCode, contentType, body)
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/json"
	}
	c.Data(statusCode, contentType, body)
}

// SanitizeClientProxyBody 对错误 JSON 脱敏；成功响应原样返回。
func SanitizeClientProxyBody(statusCode int, contentType string, body []byte) []byte {
	if statusCode < 400 || len(body) == 0 {
		return body
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct != "" && !strings.Contains(ct, "json") {
		return mustBuildOpenAIErrorJSON(statusCode, 0)
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return mustBuildOpenAIErrorJSON(statusCode, 0)
	}
	return sanitizeErrorJSONBody(statusCode, body)
}

func sanitizeErrorJSONBody(statusCode int, body []byte) []byte {
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return mustBuildOpenAIErrorJSON(statusCode, 0)
	}
	legacyCode := 0
	switch v := raw["code"].(type) {
	case float64:
		legacyCode = int(v)
	case int:
		legacyCode = v
	}
	rawMsg := extractErrorMessage(raw)
	errType, msg := classifyClientError(rawMsg, statusCode, legacyCode)
	return mustMarshalOpenAIError(errType, msg)
}

func extractErrorMessage(m map[string]any) string {
	if errObj, ok := m["error"].(map[string]any); ok {
		if s, ok := errObj["message"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	if s, ok := m["message"].(string); ok {
		return s
	}
	if s, ok := m["error_message"].(string); ok {
		return s
	}
	return ""
}

func classifyClientError(raw string, statusCode int, legacyCode int) (errType, msg string) {
	lower := strings.ToLower(stripUpstreamLeakMarkers(raw))

	if legacyCode == 20012 && strings.Contains(lower, "model does not exist") {
		return "invalid_request_error", "该模型不支持对话补全接口，请改用图片生成接口或更换模型。"
	}

	switch {
	case strings.Contains(lower, "not activated") || strings.Contains(lower, "activate the model"):
		return "invalid_request_error", "该模型服务未在平台开通，请联系管理员配置渠道模型。"
	case strings.Contains(lower, "model does not exist"),
		strings.Contains(lower, "model_not_found"),
		strings.Contains(lower, "do not have access"),
		strings.Contains(lower, "no access to"),
		strings.Contains(lower, "invalid model"),
		strings.Contains(lower, "unknown model"):
		return "invalid_request_error", "模型不可用或当前分组未配置该模型，请检查模型 ID。"
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests"):
		return "rate_limit_error", "请求过于频繁，请稍后重试。"
	case strings.Contains(lower, "insufficient") && strings.Contains(lower, "quota"),
		strings.Contains(lower, "余额") || strings.Contains(lower, "balance"):
		return "insufficient_quota", "账户余额不足，请充值后重试。"
	case statusCode == http.StatusUnauthorized || strings.Contains(lower, "invalid api key") || strings.Contains(lower, "authentication"):
		return "authentication_error", MsgUpstreamAuthFailed
	case statusCode == http.StatusForbidden:
		return "permission_error", "无权访问此资源。"
	case statusCode == http.StatusNotFound:
		return "invalid_request_error", "请求的资源不存在。"
	case statusCode >= http.StatusInternalServerError:
		return "server_error", MsgServiceUnavailable
	default:
		return "api_error", MsgRequestFailed
	}
}

func stripUpstreamLeakMarkers(s string) string {
	return strings.TrimSpace(reUpstreamRequestID.ReplaceAllString(s, ""))
}

func mustBuildOpenAIErrorJSON(statusCode int, legacyCode int) []byte {
	_, msg := classifyClientError("", statusCode, legacyCode)
	errType := "api_error"
	if statusCode >= 500 {
		errType = "server_error"
		msg = MsgServiceUnavailable
	}
	return mustMarshalOpenAIError(errType, msg)
}

func mustMarshalOpenAIError(errType, msg string) []byte {
	out, err := json.Marshal(map[string]any{
		"error": map[string]any{
			"type":    errType,
			"message": msg,
		},
	})
	if err != nil {
		return []byte(`{"error":{"type":"api_error","message":"` + MsgRequestFailed + `"}}`)
	}
	return out
}

// WriteGatewayTransportError 传输失败时返回通用文案，不暴露 URL/dial 等细节。
func WriteGatewayTransportError(c *gin.Context, _ error) {
	noteUsageErrorFromContext(c, MsgServiceUnavailable)
	middleware.WriteOpenAIError(c, http.StatusServiceUnavailable, "api_error", MsgServiceUnavailable)
}

// WriteGatewayChannelConfigError 渠道凭证未配置等平台侧错误。
func WriteGatewayChannelConfigError(c *gin.Context) {
	noteUsageErrorFromContext(c, MsgChannelMisconfigured)
	middleware.WriteOpenAIError(c, http.StatusServiceUnavailable, "api_error", MsgChannelMisconfigured)
}
