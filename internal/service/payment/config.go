package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/cryptokey"
)

// ErrInvalidPaymentConfig 支付配置校验失败。
var ErrInvalidPaymentConfig = errors.New("invalid payment config")

// RuntimeConfig 支付运行时配置（从 system_configs 加载）。
type RuntimeConfig struct {
	// OnlineEnabled 在线支付总开关；关闭时仅保留兑换码/人工调账，所有下单与回调不可用。
	OnlineEnabled bool `json:"online_enabled"`

	EnabledProviders []string `json:"enabled_providers"`

	MockEnabled bool `json:"mock_enabled"`

	AlipayEnabled    bool   `json:"alipay_enabled"`
	AlipayAppID      string `json:"alipay_app_id"`
	AlipayPrivateKey string `json:"alipay_private_key"`
	AlipayPublicKey  string `json:"alipay_public_key"`
	AlipaySandbox    bool   `json:"alipay_sandbox"`

	WxpayEnabled     bool   `json:"wxpay_enabled"`
	WxpayMchID       string `json:"wxpay_mch_id"`
	WxpayAppID       string `json:"wxpay_app_id"`
	WxpayAPIv3Key    string `json:"wxpay_api_v3_key"`
	WxpaySerialNo    string `json:"wxpay_serial_no"`
	WxpayPrivateKey  string `json:"wxpay_private_key"`

	StripeEnabled           bool   `json:"stripe_enabled"`
	StripePublishableKey    string `json:"stripe_publishable_key"`
	StripeSecretKey         string `json:"stripe_secret_key"`
	StripeWebhookSecret     string `json:"stripe_webhook_secret"`

	EasyPayEnabled  bool   `json:"easypay_enabled"`
	EasyPayPID      string `json:"easypay_pid"`
	EasyPayKey      string `json:"easypay_key"`
	EasyPayAPIURL   string `json:"easypay_api_url"`

	MinAmountCents int64 `json:"min_amount_cents"`
	MaxAmountCents int64 `json:"max_amount_cents"`

	SiteBaseURL string `json:"site_base_url"`
}

const configKeyPayment = "payment.runtime"

// ConfigStore 加载并缓存支付配置。
type ConfigStore struct {
	client *ent.Client
	aesKey []byte
	mu     sync.RWMutex
	cfg    RuntimeConfig
}

func NewConfigStore(client *ent.Client, aesKey []byte) *ConfigStore {
	return &ConfigStore{client: client, aesKey: aesKey}
}

func (s *ConfigStore) Load(ctx context.Context) (RuntimeConfig, error) {
	row, err := s.client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(configKeyPayment)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return s.defaults(), nil
		}
		return RuntimeConfig{}, err
	}
	var cfg RuntimeConfig
	if strings.TrimSpace(row.ConfigValue) != "" {
		if err := json.Unmarshal([]byte(row.ConfigValue), &cfg); err != nil {
			return RuntimeConfig{}, err
		}
	}
	s.decryptSecrets(&cfg)
	if cfg.MinAmountCents <= 0 {
		cfg.MinAmountCents = 100
	}
	if cfg.MaxAmountCents <= 0 {
		cfg.MaxAmountCents = 1_000_000_00
	}
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	return cfg, nil
}

func (s *ConfigStore) Cached() RuntimeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *ConfigStore) Save(ctx context.Context, cfg RuntimeConfig, encrypt bool) error {
	toStore := cfg
	if encrypt {
		s.encryptSecrets(&toStore)
	}
	b, err := json.Marshal(toStore)
	if err != nil {
		return err
	}
	val := string(b)
	existing, err := s.client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(configKeyPayment)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = s.client.SystemConfig.Create().
			SetConfigKey(configKeyPayment).
			SetConfigValue(val).
			SetDescription("支付渠道运行时配置").
			Save(ctx)
	} else if err == nil {
		_, err = existing.Update().SetConfigValue(val).Save(ctx)
	}
	if err == nil {
		s.decryptSecrets(&cfg)
		s.mu.Lock()
		s.cfg = cfg
		s.mu.Unlock()
	}
	return err
}

func (s *ConfigStore) defaults() RuntimeConfig {
	return RuntimeConfig{
		OnlineEnabled:    false,
		EnabledProviders: nil,
		MockEnabled:      false,
		MinAmountCents:   100,
		MaxAmountCents:   1_000_000_00,
	}
}

func (s *ConfigStore) encryptSecrets(cfg *RuntimeConfig) {
	if s.aesKey == nil {
		return
	}
	cfg.AlipayPrivateKey = encryptIfPlain(s.aesKey, cfg.AlipayPrivateKey)
	cfg.WxpayPrivateKey = encryptIfPlain(s.aesKey, cfg.WxpayPrivateKey)
	cfg.StripeSecretKey = encryptIfPlain(s.aesKey, cfg.StripeSecretKey)
	cfg.StripeWebhookSecret = encryptIfPlain(s.aesKey, cfg.StripeWebhookSecret)
	cfg.EasyPayKey = encryptIfPlain(s.aesKey, cfg.EasyPayKey)
}

func (s *ConfigStore) decryptSecrets(cfg *RuntimeConfig) {
	if s.aesKey == nil {
		return
	}
	cfg.AlipayPrivateKey = decryptIfEnc(s.aesKey, cfg.AlipayPrivateKey)
	cfg.WxpayPrivateKey = decryptIfEnc(s.aesKey, cfg.WxpayPrivateKey)
	cfg.StripeSecretKey = decryptIfEnc(s.aesKey, cfg.StripeSecretKey)
	cfg.StripeWebhookSecret = decryptIfEnc(s.aesKey, cfg.StripeWebhookSecret)
	cfg.EasyPayKey = decryptIfEnc(s.aesKey, cfg.EasyPayKey)
}

func encryptIfPlain(aesKey []byte, v string) string {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "enc:") {
		return v
	}
	enc, err := cryptokey.EncryptGCM(aesKey, []byte(v))
	if err != nil {
		return v
	}
	return "enc:" + enc
}

func decryptIfEnc(aesKey []byte, v string) string {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "enc:") {
		return v
	}
	plain, err := cryptokey.DecryptGCM(aesKey, strings.TrimPrefix(v, "enc:"))
	if err != nil {
		return ""
	}
	return string(plain)
}

// PublicConfig 管理端展示（敏感字段掩码）。
func PublicConfig(cfg RuntimeConfig) map[string]any {
	return map[string]any{
		"online_enabled":       cfg.OnlineEnabled,
		"enabled_providers":    cfg.EnabledProviders,
		"mock_enabled":         cfg.MockEnabled,
		"alipay_enabled":       cfg.AlipayEnabled,
		"alipay_app_id":        cfg.AlipayAppID,
		"alipay_public_key":    cfg.AlipayPublicKey,
		"alipay_sandbox":       cfg.AlipaySandbox,
		"alipay_private_key_set": strings.TrimSpace(cfg.AlipayPrivateKey) != "",
		"wxpay_enabled":        cfg.WxpayEnabled,
		"wxpay_mch_id":         cfg.WxpayMchID,
		"wxpay_app_id":         cfg.WxpayAppID,
		"wxpay_serial_no":      cfg.WxpaySerialNo,
		"wxpay_private_key_set": strings.TrimSpace(cfg.WxpayPrivateKey) != "",
		"stripe_enabled":       cfg.StripeEnabled,
		"stripe_publishable_key": cfg.StripePublishableKey,
		"stripe_secret_key_set": strings.TrimSpace(cfg.StripeSecretKey) != "",
		"stripe_webhook_secret_set": strings.TrimSpace(cfg.StripeWebhookSecret) != "",
		"easypay_enabled":      cfg.EasyPayEnabled,
		"easypay_pid":          cfg.EasyPayPID,
		"easypay_api_url":      cfg.EasyPayAPIURL,
		"easypay_key_set":      strings.TrimSpace(cfg.EasyPayKey) != "",
		"min_amount_cents":     cfg.MinAmountCents,
		"max_amount_cents":     cfg.MaxAmountCents,
		"site_base_url":        cfg.SiteBaseURL,
	}
}

// MergeUpdate 合并管理端 PUT（仅非空敏感字段覆盖）。
func MergeUpdate(cur RuntimeConfig, patch map[string]any) RuntimeConfig {
	b, _ := json.Marshal(patch)
	var p RuntimeConfig
	_ = json.Unmarshal(b, &p)
	if v, ok := patch["enabled_providers"]; ok {
		if arr, ok := v.([]any); ok {
			var ps []string
			for _, x := range arr {
				if s, ok := x.(string); ok && s != "" {
					ps = append(ps, s)
				}
			}
			cur.EnabledProviders = ps
		}
	}
	setBool := func(key string, dst *bool, src bool) {
		if _, ok := patch[key]; ok {
			*dst = src
		}
	}
	setBool("online_enabled", &cur.OnlineEnabled, p.OnlineEnabled)
	setBool("mock_enabled", &cur.MockEnabled, p.MockEnabled)
	setBool("alipay_enabled", &cur.AlipayEnabled, p.AlipayEnabled)
	setBool("wxpay_enabled", &cur.WxpayEnabled, p.WxpayEnabled)
	setBool("stripe_enabled", &cur.StripeEnabled, p.StripeEnabled)
	setBool("easypay_enabled", &cur.EasyPayEnabled, p.EasyPayEnabled)
	setBool("alipay_sandbox", &cur.AlipaySandbox, p.AlipaySandbox)
	if s := strings.TrimSpace(p.AlipayAppID); s != "" {
		cur.AlipayAppID = s
	}
	if s := strings.TrimSpace(p.AlipayPublicKey); s != "" {
		cur.AlipayPublicKey = s
	}
	if s := strings.TrimSpace(p.AlipayPrivateKey); s != "" {
		cur.AlipayPrivateKey = s
	}
	if s := strings.TrimSpace(p.WxpayMchID); s != "" {
		cur.WxpayMchID = s
	}
	if s := strings.TrimSpace(p.WxpayAppID); s != "" {
		cur.WxpayAppID = s
	}
	if s := strings.TrimSpace(p.WxpayAPIv3Key); s != "" {
		cur.WxpayAPIv3Key = s
	}
	if s := strings.TrimSpace(p.WxpaySerialNo); s != "" {
		cur.WxpaySerialNo = s
	}
	if s := strings.TrimSpace(p.WxpayPrivateKey); s != "" {
		cur.WxpayPrivateKey = s
	}
	if s := strings.TrimSpace(p.StripePublishableKey); s != "" {
		cur.StripePublishableKey = s
	}
	if s := strings.TrimSpace(p.StripeSecretKey); s != "" {
		cur.StripeSecretKey = s
	}
	if s := strings.TrimSpace(p.StripeWebhookSecret); s != "" {
		cur.StripeWebhookSecret = s
	}
	if s := strings.TrimSpace(p.EasyPayPID); s != "" {
		cur.EasyPayPID = s
	}
	if s := strings.TrimSpace(p.EasyPayKey); s != "" {
		cur.EasyPayKey = s
	}
	if s := strings.TrimSpace(p.EasyPayAPIURL); s != "" {
		cur.EasyPayAPIURL = s
	}
	if s := strings.TrimSpace(p.SiteBaseURL); s != "" {
		cur.SiteBaseURL = s
	}
	if v, ok := patch["min_amount_cents"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			cur.MinAmountCents = int64(n)
		}
	}
	if v, ok := patch["max_amount_cents"]; ok {
		if n, ok := v.(float64); ok && n > 0 {
			cur.MaxAmountCents = int64(n)
		}
	}
	return cur
}

// ValidateRuntimeConfig 保存前校验运行时配置。
func ValidateRuntimeConfig(cfg RuntimeConfig) error {
	if cfg.EasyPayEnabled {
		if strings.TrimSpace(cfg.EasyPayPID) == "" ||
			strings.TrimSpace(cfg.EasyPayKey) == "" ||
			strings.TrimSpace(cfg.EasyPayAPIURL) == "" {
			return fmt.Errorf("%w: 易支付已开启，请填写商户 ID、密钥与 API 地址", ErrInvalidPaymentConfig)
		}
	}
	return nil
}
