package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

// SystemConfigKeyLoginCaptchaEnabled 与站点 YAML 中 site.login_captcha.enabled 语义一致；
// 写入 system_configs 后由运行时读取，覆盖配置文件（便于管理后台开关）。
const SystemConfigKeyLoginCaptchaEnabled = "site.login_captcha.enabled"

// EffectiveLoginCaptchaEnabled 返回是否启用登录图形验证码：若库中存在该键且值为合法布尔，则以库为准；否则使用配置文件。
func EffectiveLoginCaptchaEnabled(ctx context.Context, client *ent.Client, cfg *config.Config) bool {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyLoginCaptchaEnabled)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return cfg.Site.LoginCaptcha.Enabled
		}
		return cfg.Site.LoginCaptcha.Enabled
	}
	s := strings.TrimSpace(row.ConfigValue)
	if s == "" {
		return cfg.Site.LoginCaptcha.Enabled
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return cfg.Site.LoginCaptcha.Enabled
	}
	return b
}

// LoginCaptchaDBMeta 是否已在库中写入该键（含空串），以及原始值（用于运营看板展示）。
func LoginCaptchaDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyLoginCaptchaEnabled)).
		Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}
