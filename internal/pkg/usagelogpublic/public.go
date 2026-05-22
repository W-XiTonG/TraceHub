// Package usagelogpublic 控制 usage_logs 错误文案对用户/管理端的可见范围。
package usagelogpublic

import (
	"strings"
)

// InternalPrefix 网关内部错误写入 error_message 时的前缀；用户 API 不得原样返回。
const InternalPrefix = "internal:"

// WrapInternal 将网关内部说明写入库（仅管理端应解析展示）。
func WrapInternal(detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return InternalPrefix + "gateway_denied"
	}
	return InternalPrefix + detail
}

// UserFacing 返回用户控制台/API 可见的错误摘要（隐藏渠道、选路、分组等内部信息）。
func UserFacing(stored *string, statusCode *int) *string {
	if stored == nil {
		return nil
	}
	return userFacingMsg(strings.TrimSpace(*stored), statusCode)
}

// UserFacingFields 便于 ent 非指针字段调用。
func UserFacingFields(stored string, statusCode int) *string {
	msg := strings.TrimSpace(stored)
	if msg == "" {
		return nil
	}
	var sc *int
	if statusCode != 0 {
		sc = &statusCode
	}
	return userFacingMsg(msg, sc)
}

func userFacingMsg(msg string, statusCode *int) *string {
	if msg == "" || isTraceOnly(msg) {
		return nil
	}
	if shouldRedactForUser(msg) {
		g := genericForStatus(statusCode)
		if g == "" {
			return nil
		}
		return &g
	}
	out := msg
	return &out
}

// AdminFacing 管理端展示：解开 internal 前缀，保留运维细节。
func AdminFacing(stored *string) *string {
	if stored == nil {
		return nil
	}
	return adminFacingMsg(strings.TrimSpace(*stored))
}

// AdminFacingField 便于 ent 非指针字段调用。
func AdminFacingField(stored string) *string {
	return adminFacingMsg(strings.TrimSpace(stored))
}

func adminFacingMsg(msg string) *string {
	if msg == "" || isTraceOnly(msg) {
		return nil
	}
	if strings.HasPrefix(msg, InternalPrefix) {
		msg = strings.TrimSpace(strings.TrimPrefix(msg, InternalPrefix))
		if msg == "" {
			return nil
		}
	}
	return &msg
}

func isTraceOnly(msg string) bool {
	if strings.HasPrefix(msg, "video_job:") {
		return true
	}
	for _, p := range []string{
		"video_settle_extra:", "video_settle_refund:",
		"kling_settle_extra:", "kling_settle_refund:",
	} {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}

func shouldRedactForUser(msg string) bool {
	if strings.HasPrefix(msg, InternalPrefix) {
		return true
	}
	lower := strings.ToLower(msg)
	internal := []string{
		"渠道", "选路", "分组", "upstream", "路由配置", "没有可用渠道",
		"未分配分组", "capability", "不支持", "whitelist", "blacklist",
		"白名单", "黑名单", "tracehub", "gateway:",
	}
	for _, k := range internal {
		if strings.Contains(lower, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func genericForStatus(statusCode *int) string {
	if statusCode == nil {
		return "请求未成功"
	}
	return GenericMessageForHTTPStatus(*statusCode)
}

// GenericMessageForHTTPStatus 网关 OpenAI 兼容错误体对用户可见的通用说明。
func GenericMessageForHTTPStatus(httpStatus int) string {
	switch {
	case httpStatus == 401:
		return "认证失败"
	case httpStatus == 403:
		return "请求被拒绝"
	case httpStatus == 404:
		return "资源不存在"
	case httpStatus >= 400 && httpStatus < 500:
		return "请求未成功"
	case httpStatus >= 500:
		return "服务暂时不可用"
	default:
		return "请求未成功"
	}
}

// SanitizeGatewayErrorMessage 若含渠道/选路等内部信息，返回对用户安全的 message；internalDetail 为原始说明（仅写日志/库）。
func SanitizeGatewayErrorMessage(httpStatus int, msg string) (clientMsg string, internalDetail string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return GenericMessageForHTTPStatus(httpStatus), ""
	}
	if shouldRedactForUser(msg) {
		return GenericMessageForHTTPStatus(httpStatus), msg
	}
	return msg, ""
}
