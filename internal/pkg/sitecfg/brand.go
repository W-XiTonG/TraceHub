package sitecfg

import (
	"context"
	"strings"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	// SystemConfigKeyBrandName 站点对外展示名称（控制台侧栏、登录页等）。
	SystemConfigKeyBrandName = "site.brand.name"
	// SystemConfigKeyBrandIconURL 站点图标 URL：须为同源相对路径（以 / 开头）或 http(s) 绝对地址。
	SystemConfigKeyBrandIconURL = "site.brand.icon_url"
)

const (
	DefaultBrandName    = "TraceHub"
	DefaultBrandIconURL = "/tracehub-icon.svg"
)

func stringFromConfigRow(ctx context.Context, client *ent.Client, key string) (string, bool, error) {
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return row.ConfigValue, true, nil
}

// EffectiveBrandName 对外展示名：库中有非空值则用库，否则 DefaultBrandName。
func EffectiveBrandName(ctx context.Context, client *ent.Client) string {
	defaultName := DefaultBrandName
	v, _, err := stringFromConfigRow(ctx, client, SystemConfigKeyBrandName)
	if err != nil {
		return defaultName
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return defaultName
	}
	return s
}

// StoredBrandIconURL 库中保存的站点图标 URL（trim 后为空表示未配置），不含默认回退；供管理端表单编辑。
func StoredBrandIconURL(ctx context.Context, client *ent.Client) string {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyBrandIconURL)
	if err != nil || !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// EffectiveBrandIconURL 图标地址：库中有非空值则用库，否则 DefaultBrandIconURL。
func EffectiveBrandIconURL(ctx context.Context, client *ent.Client) string {
	defaultURL := DefaultBrandIconURL
	v, _, err := stringFromConfigRow(ctx, client, SystemConfigKeyBrandIconURL)
	if err != nil {
		return defaultURL
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return defaultURL
	}
	return s
}

// ValidateBrandName 管理端写入前校验。
func ValidateBrandName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.ContainsAny(s, "\r\n\x00") {
		return errBrandNameInvalid
	}
	if utf8.RuneCountInString(s) > 64 {
		return errBrandNameTooLong
	}
	return nil
}

// ValidateBrandIconURL 管理端写入前校验；空串表示恢复默认图标。
func ValidateBrandIconURL(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if len(s) > 512 {
		return errBrandIconTooLong
	}
	if strings.HasPrefix(s, "/") {
		if strings.Contains(s, "..") {
			return errBrandIconPath
		}
		return nil
	}
	if err := validateHTTPURL(s); err != nil {
		return err
	}
	return nil
}
