package sitecfg

import (
	"context"
	"strings"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

// SystemConfigKeyOperatorDisplayName 写入后覆盖 YAML site.operator_display_name（如 /health 的 operator 字段）。
const SystemConfigKeyOperatorDisplayName = "site.operator_display_name"

// EffectiveOperatorDisplayName 运营方展示名：库中非空则优先，否则配置文件。
func EffectiveOperatorDisplayName(ctx context.Context, client *ent.Client, cfg *config.Config) string {
	fallback := strings.TrimSpace(cfg.Site.OperatorDisplayName)
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyOperatorDisplayName)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fallback
		}
		return fallback
	}
	v := strings.TrimSpace(row.ConfigValue)
	if v == "" {
		return fallback
	}
	return v
}

// ValidateOperatorDisplayName 管理端写入前校验；允许空串表示跟回 YAML。
func ValidateOperatorDisplayName(s string) error {
	if utf8.RuneCountInString(strings.TrimSpace(s)) > 200 {
		return errOperatorTooLong
	}
	return nil
}
