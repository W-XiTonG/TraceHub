package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

// SystemConfigKeyDefaultNewUserBalance 与站点 YAML 中 site.default_new_user_balance 语义一致；
// 写入 system_configs 后由运行时读取，覆盖配置文件（便于管理后台调整新用户初始额度）。
const SystemConfigKeyDefaultNewUserBalance = "site.default_new_user_balance"

// EffectiveDefaultNewUserBalance 新用户注册时的初始余额（额度单位）：若库中存在该键且值为合法非负整数，则以库为准；否则使用配置文件。
func EffectiveDefaultNewUserBalance(ctx context.Context, client *ent.Client, cfg *config.Config) int64 {
	fallback := cfg.Site.DefaultNewUserBalance
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyDefaultNewUserBalance)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return fallback
		}
		return fallback
	}
	s := strings.TrimSpace(row.ConfigValue)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

// DefaultNewUserBalanceDBMeta 是否已在库中写入该键，以及原始值（供管理端展示）。
func DefaultNewUserBalanceDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyDefaultNewUserBalance)).
		Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}

// ValidateDefaultNewUserBalance 管理端写入前校验；须为十进制非负整数，且不超过 int64 上限。
func ValidateDefaultNewUserBalance(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errDefaultNewUserBalanceEmpty
	}
	if len(s) > 20 {
		return errDefaultNewUserBalanceRange
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return errDefaultNewUserBalanceInvalid
	}
	if v < 0 {
		return errDefaultNewUserBalanceRange
	}
	return nil
}
