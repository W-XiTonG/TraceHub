package router

import (
	"net/http"
	"strings"

	"tracehub/internal/pkg/sitecfg"
)

// FailoverExecMeta 供 handler 设置响应头（如 X-TraceHub-Attempts）。
type FailoverExecMeta struct {
	Attempts int
}

// IsRetryableUpstream 判断上游错误是否可 failover 重试（5xx/429/传输错误）。
func IsRetryableUpstream(statusCode int, transportErr error) bool {
	if transportErr != nil {
		return true
	}
	if statusCode >= 500 {
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	return false
}

// IsRetryableUpstreamForFailover 结合 4xx 模式与剩余候选渠道判断是否重试。
func IsRetryableUpstreamForFailover(cfg RoutingConfig, statusCode int, transportErr error, body []byte, remainingCandidates int) bool {
	if remainingCandidates < 1 {
		return false
	}
	if IsRetryableUpstream(statusCode, transportErr) {
		return true
	}
	if statusCode < 400 || statusCode >= 500 {
		return false
	}
	if isFailoverBlacklistBody(body) {
		return false
	}
	mode := strings.TrimSpace(cfg.Failover4xxMode)
	if mode == "" {
		mode = sitecfg.Failover4xxModeOff
	}
	switch mode {
	case sitecfg.Failover4xxModeAggressive:
		return true
	case sitecfg.Failover4xxModeSelective:
		if !isFailover4xxCodeEnabled(cfg, statusCode) {
			return false
		}
		return isSelective4xxRetryable(statusCode, body)
	case sitecfg.Failover4xxModeCustom:
		return isFailover4xxCodeEnabled(cfg, statusCode)
	default:
		return false
	}
}

func isFailover4xxCodeEnabled(cfg RoutingConfig, statusCode int) bool {
	effective := sitecfg.EffectiveFailover4xxCodes(cfg.Failover4xxMode, cfg.Failover4xxCodes)
	set := sitecfg.Failover4xxCodeSet(effective)
	_, ok := set[statusCode]
	return ok
}

func isFailoverBlacklistBody(body []byte) bool {
	lower := strings.ToLower(string(body))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "insufficient") && strings.Contains(lower, "quota") {
		return true
	}
	if strings.Contains(lower, "余额") {
		return true
	}
	if strings.Contains(lower, "balance") && (strings.Contains(lower, "insufficient") || strings.Contains(lower, "不足")) {
		return true
	}
	return false
}

func isSelective4xxRetryable(statusCode int, body []byte) bool {
	lower := strings.ToLower(string(body))
	switch statusCode {
	case http.StatusUnauthorized:
		return strings.Contains(lower, "invalid api key") ||
			strings.Contains(lower, "authentication") ||
			strings.Contains(lower, "unauthorized") ||
			strings.Contains(lower, "api key")
	case http.StatusForbidden:
		return strings.Contains(lower, "not activated") ||
			strings.Contains(lower, "activate the model") ||
			strings.Contains(lower, "do not have access") ||
			strings.Contains(lower, "no access to") ||
			strings.Contains(lower, "permission")
	case http.StatusBadRequest:
		return strings.Contains(lower, "model does not exist") ||
			strings.Contains(lower, "model_not_found") ||
			strings.Contains(lower, "invalid model") ||
			strings.Contains(lower, "unknown model") ||
			strings.Contains(lower, "do not have access") ||
			strings.Contains(lower, "no access to")
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}
