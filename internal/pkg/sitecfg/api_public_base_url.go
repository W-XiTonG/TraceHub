package sitecfg

import (
	"context"
	"net/url"
	"strings"

	"tracehub/internal/ent"
)

const (
	// SystemConfigKeyAPIPublicBaseURL 对外 OpenAI 兼容 API 的 Base URL（须含 https?:// 与路径 /v1，或仅根由规范化补齐 /v1）。
	SystemConfigKeyAPIPublicBaseURL = "site.api.public_base_url"
	maxAPIPublicBaseURLLen          = 512
)

// NormalizeAPIPublicBaseURL 规范化管理员输入：空串表示清除；否则须为 http(s) URL，路径仅允许空、/、/v1（大小写敏感）。
func NormalizeAPIPublicBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if len(s) > maxAPIPublicBaseURLLen {
		return "", errAPIPublicBaseURLTooLong
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errAPIPublicBaseURLInvalid
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errAPIPublicBaseURLInvalid
	}
	if strings.Contains(u.Path, "..") || strings.Contains(u.RawPath, "..") {
		return "", errAPIPublicBaseURLInvalid
	}
	p := strings.Trim(strings.TrimSpace(u.Path), "/")
	switch p {
	case "":
		u.Path = "/v1"
	case "v1":
		u.Path = "/v1"
	default:
		return "", errAPIPublicBaseURLPath
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.User = nil
	out := strings.TrimSuffix(u.String(), "/")
	if !strings.HasSuffix(out, "/v1") {
		return "", errAPIPublicBaseURLInvalid
	}
	return out, nil
}

// ValidateAPIPublicBaseURL 管理端写入前校验（空表示恢复「跟随当前站点」）。
func ValidateAPIPublicBaseURL(s string) error {
	_, err := NormalizeAPIPublicBaseURL(s)
	return err
}

// StoredAPIPublicBaseURL 库中原始值（trim），供管理端表单展示。
func StoredAPIPublicBaseURL(ctx context.Context, client *ent.Client) string {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyAPIPublicBaseURL)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// EffectiveAPIPublicBaseURL 对外展示用的 OpenAI base（含 /v1、无末尾斜杠）；未配置时返回空串，由前端回退到 window.location.origin。
func EffectiveAPIPublicBaseURL(ctx context.Context, client *ent.Client) string {
	s := StoredAPIPublicBaseURL(ctx, client)
	if s == "" {
		return ""
	}
	n, err := NormalizeAPIPublicBaseURL(s)
	if err != nil || n == "" {
		return ""
	}
	return n
}
