package sitecfg

import (
	"context"
	"strings"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

// SystemConfigKeyRegisterPromoText 注册推广文案：写入 system_configs 后覆盖 YAML site.register_promo_text。
const SystemConfigKeyRegisterPromoText = "site.register_promo_text"

// EffectiveRegisterPromoText 官网注册页等展示的推广说明：库中非空则优先，否则配置文件。
func EffectiveRegisterPromoText(ctx context.Context, client *ent.Client, cfg *config.Config) string {
	fallback := strings.TrimSpace(cfg.Site.RegisterPromoText)
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyRegisterPromoText)).Only(ctx)
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

// RegisterPromoTextDBMeta 是否已在库中写入该键，以及原始值（供管理端展示）。
func RegisterPromoTextDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyRegisterPromoText)).
		Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}

// ValidateRegisterPromoText 管理端写入前校验；允许空串表示跟回 YAML；支持多行。
func ValidateRegisterPromoText(s string) error {
	if utf8.RuneCountInString(strings.TrimSpace(s)) > 1000 {
		return errRegisterPromoTooLong
	}
	return nil
}
