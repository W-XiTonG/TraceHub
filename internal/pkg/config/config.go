package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Security  SecurityConfig  `mapstructure:"security"`
	Site      SiteConfig      `mapstructure:"site"`
	Email     EmailConfig     `mapstructure:"email"`
	Bootstrap BootstrapConfig `mapstructure:"bootstrap"`
}

// EmailConfig 用于发送登录验证码等系统邮件；未启用时仍可密码登录。
type EmailConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	SMTPHost            string `mapstructure:"smtp_host"`
	SMTPPort            int    `mapstructure:"smtp_port"`
	SMTPUser            string `mapstructure:"smtp_user"`
	SMTPPassword        string `mapstructure:"smtp_password"`
	From                string `mapstructure:"from"`
	StartTLS            bool   `mapstructure:"starttls"`
	InsecureSkipVerify  bool   `mapstructure:"insecure_skip_verify"`
	LoginCodeSubject    string `mapstructure:"login_code_subject"`
	RegisterCodeSubject string `mapstructure:"register_code_subject"`
	LoginCodeTTLMin     int    `mapstructure:"login_code_ttl_minutes"`
	LoginCodeResendSec  int    `mapstructure:"login_code_resend_seconds"`
	LoginCodeMaxHour    int    `mapstructure:"login_code_max_per_hour"`
	// CodeEmailTrilingual 为 true 时，验证码邮件正文固定包含简中、繁中、英文三段（忽略请求 locale）。
	CodeEmailTrilingual bool `mapstructure:"code_email_trilingual"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
	Mode string `mapstructure:"mode"`
}

type DatabaseConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type SecurityConfig struct {
	JWTSecret          string `mapstructure:"jwt_secret"`
	AESKeyBase64       string `mapstructure:"aes_key_base64"`
	MetricsBearerToken string `mapstructure:"metrics_bearer_token"`
}

type SiteConfig struct {
	// AppVersion 对外展示的应用版本号（首页页脚、/health、管理后台运维等）；空则默认 1.0.0。
	AppVersion           string `mapstructure:"app_version"`
	APIKeyPrefix         string `mapstructure:"api_key_prefix"`
	CurrencyQuotaPerYuan int64  `mapstructure:"currency_quota_per_yuan"`
	// CurrencyQuotaPerUsd 每 1 美元上游成本对应额度；>0 时「美元标价→额度」优先用本字段（否则用 pricing_cny_per_usd × currency_quota_per_yuan）。
	CurrencyQuotaPerUsd int64 `mapstructure:"currency_quota_per_usd"`
	// PricingCnyPerUsd 参考汇率：1 USD = R CNY；仅在未配置 currency_quota_per_usd 时用于美元标价换算。
	PricingCnyPerUsd      float64  `mapstructure:"pricing_cny_per_usd"`
	DefaultNewUserBalance int64    `mapstructure:"default_new_user_balance"`
	CORSOrigins           []string `mapstructure:"cors_origins"`
	// OperatorDisplayName 非空时写入 /health、/ready 的 JSON 字段 operator，便于运维与对外展示运营方。
	OperatorDisplayName string `mapstructure:"operator_display_name"`
	// Realtime 通话子资源：call_id → 渠道映射（需 Redis）；未命中时要求 Query / X-TraceHub-Model / JSON model。
	RealtimeCallMappingEnabled bool `mapstructure:"realtime_call_mapping_enabled"`
	RealtimeCallMappingTTLSec  int  `mapstructure:"realtime_call_mapping_ttl_sec"`
	// RealtimeCallControlBillRejectHangup 为 true 时对 reject / hangup 按次预估扣费；默认 false。
	RealtimeCallControlBillRejectHangup bool `mapstructure:"realtime_call_control_bill_reject_hangup"`
	// OpenAICompatChannelTypes：除 openai/deepseek 外，仍按 OpenAI 兼容协议转发的渠道类型名（如 siliconflow、custom_openai）。
	OpenAICompatChannelTypes []string `mapstructure:"openai_compat_channel_types"`
	// BlockOpenAIAccountAdminPaths：拦截 OpenAI 控制台/账号类路径（组织、账单等），避免用户 Key 误代转发；默认 true。
	BlockOpenAIAccountAdminPaths bool `mapstructure:"block_openai_account_admin_paths"`
	// OpenAIAccountAdminDenyPrefixes：在 block 开启时，相对 /v1/ 的额外拒绝前缀（不含前导 /）。
	OpenAIAccountAdminDenyPrefixes []string                `mapstructure:"openai_account_admin_deny_prefixes"`
	OpenAIPassthrough              OpenAIPassthroughConfig `mapstructure:"openai_passthrough"`
	// LoginCaptcha：登录页图形验证码；开启后密码登录与「发送邮箱登录验证码」需携带 captcha_id / captcha_answer，依赖 Redis。
	LoginCaptcha SiteLoginCaptchaConfig `mapstructure:"login_captcha"`
	// BrandUploadDir 管理后台本地上传品牌图标的磁盘目录（可写）；空则默认 data/site_brand。对外固定通过 /uploads/site-brand/ 访问。
	BrandUploadDir string `mapstructure:"brand_upload_dir"`
	// RegisterPromoText 注册页与官网展示的推广说明（纯文本，可含换行）；空则不展示。
	RegisterPromoText string `mapstructure:"register_promo_text"`
	// NoBillModels 全局不计费模型 ID 列表（逻辑模型名）；分组 model_rates.billable 可覆盖。
	NoBillModels []string `mapstructure:"no_bill_models"`
}

// SiteLoginCaptchaConfig 控制台登录人机校验（图形验证码）。
type SiteLoginCaptchaConfig struct {
	Enabled bool `mapstructure:"enabled"`
	// TTLSeconds 验证码在 Redis 中的存活时间（秒）；≤0 时默认 300。
	TTLSeconds int `mapstructure:"ttl_seconds"`
}

// OpenAIPassthroughConfig 可选：将未显式注册的路径按白名单透传到 OpenAI 兼容上游（需配置 allow 前缀）。
type OpenAIPassthroughConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowPrefixes    []string `mapstructure:"allow_prefixes"`
	DenyPrefixes     []string `mapstructure:"deny_prefixes"`
	PatchModelInJSON bool     `mapstructure:"patch_model_in_json"`
}

type BootstrapConfig struct {
	AdminEmail       string `mapstructure:"admin_email"`
	AdminPassword    string `mapstructure:"admin_password"`
	DefaultGroupName string `mapstructure:"default_group_name"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("TRACEHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}

	v.SetDefault("server.addr", ":8080")
	v.SetDefault("server.mode", "release")
	v.SetDefault("site.app_version", "1.0.0")
	v.SetDefault("site.api_key_prefix", "sk-th-")
	v.SetDefault("site.currency_quota_per_yuan", 10000)
	v.SetDefault("site.currency_quota_per_usd", 0)
	v.SetDefault("site.pricing_cny_per_usd", 0.0)
	v.SetDefault("site.default_new_user_balance", 0)
	v.SetDefault("site.cors_origins", []string{"*"})
	v.SetDefault("site.realtime_call_mapping_enabled", false)
	v.SetDefault("site.realtime_call_mapping_ttl_sec", 86400)
	v.SetDefault("site.realtime_call_control_bill_reject_hangup", false)
	v.SetDefault("site.block_openai_account_admin_paths", true)
	v.SetDefault("site.openai_passthrough.enabled", false)
	v.SetDefault("site.openai_passthrough.patch_model_in_json", true)
	v.SetDefault("site.login_captcha.enabled", true)
	v.SetDefault("site.login_captcha.ttl_seconds", 300)
	v.SetDefault("site.register_promo_text", "新用户注册即送 5000 额度（约合 ¥0.5），注册成功后自动到账。")
	v.SetDefault("site.no_bill_models", []string{})
	v.SetDefault("bootstrap.default_group_name", "基础分组")
	v.SetDefault("email.smtp_port", 587)
	v.SetDefault("email.starttls", true)
	v.SetDefault("email.login_code_ttl_minutes", 10)
	v.SetDefault("email.login_code_resend_seconds", 60)
	v.SetDefault("email.login_code_max_per_hour", 8)
	v.SetDefault("email.code_email_trilingual", false)
	v.SetDefault("email.login_code_subject", "TraceHub 登录验证码")
	v.SetDefault("email.register_code_subject", "TraceHub 注册验证码")

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	if c.Database.DSN == "" {
		c.Database.DSN = v.GetString("database.dsn")
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = v.GetString("redis.addr")
	}
	if c.Security.JWTSecret == "" {
		c.Security.JWTSecret = v.GetString("security.jwt_secret")
	}
	// 嵌套 login_captcha 未在配置文件中写出时，Unmarshal 会得到零值；此处用 Get* 合并 SetDefault。
	c.Site.LoginCaptcha.Enabled = v.GetBool("site.login_captcha.enabled")
	if ts := v.GetInt("site.login_captcha.ttl_seconds"); ts > 0 {
		c.Site.LoginCaptcha.TTLSeconds = ts
	}
	return &c, nil
}

// AppVersion 返回 trim 后的站点应用版本；未配置时为 1.0.0。
func (c *Config) AppVersion() string {
	if c == nil {
		return "1.0.0"
	}
	v := strings.TrimSpace(c.Site.AppVersion)
	if v == "" {
		return "1.0.0"
	}
	return v
}
