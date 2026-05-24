package sitecfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/alertconfig"
)

const (
	SystemConfigKeyGatewayAdminAlertEnabled = "gateway.admin_alert.enabled"
	SystemConfigKeyGatewayAdminAlertConfig  = "gateway.admin_alert.config"
	SystemConfigKeyGatewayAdminAlertSecrets = "gateway.admin_alert.secrets"
)

const gatewayAdminAlertCacheTTL = 5 * time.Second

const (
	defaultGatewayAdminAlertCooldownSec    = 300
	defaultGatewayAdminAlertMinIntervalSec = 30
	minGatewayAdminAlertCooldownSec        = 60
	maxGatewayAdminAlertCooldownSec        = 3600
	minGatewayAdminAlertMinIntervalSec     = 10
	maxGatewayAdminAlertMinIntervalSec     = 600
)

// GatewayAdminAlertTriggers 可独立开关的异常类型。
type GatewayAdminAlertTriggers struct {
	Notify5xx       bool `json:"notify_5xx"`
	NotifyTransport bool `json:"notify_transport"`
	NotifyFailover  bool `json:"notify_failover"`
	Notify4xx       bool `json:"notify_4xx"`
	Notify402       bool `json:"notify_402"`
}

// GatewayAdminAlertChannels 通知渠道开关。
type GatewayAdminAlertChannels struct {
	Email          bool `json:"email"`
	DingTalk       bool `json:"dingtalk"`
	Feishu         bool `json:"feishu"`
	WeCom          bool `json:"wecom"`
	GenericWebhook bool `json:"generic_webhook"`
	InApp          bool `json:"in_app"`
}

// GatewayAdminAlertWireConfig 持久化于 gateway.admin_alert.config（不含密钥）。
type GatewayAdminAlertWireConfig struct {
	Triggers           GatewayAdminAlertTriggers `json:"triggers"`
	Channels           GatewayAdminAlertChannels `json:"channels"`
	Emails             []string                  `json:"emails,omitempty"`
	DingTalkWebhook    string                    `json:"dingtalk_webhook,omitempty"`
	FeishuWebhook      string                    `json:"feishu_webhook,omitempty"`
	WeComWebhook       string                    `json:"wecom_webhook,omitempty"`
	GenericWebhookURL  string                    `json:"generic_webhook_url,omitempty"`
	CooldownSec        int                       `json:"cooldown_sec,omitempty"`
	MinIntervalSec     int                       `json:"min_interval_sec,omitempty"`
	IncludeUserEmail   bool                      `json:"include_user_email,omitempty"`
}

// GatewayAdminAlertEffective 运行时合并 enabled + config + secrets。
type GatewayAdminAlertEffective struct {
	Enabled bool
	GatewayAdminAlertWireConfig
	Secrets alertconfig.Stored
}

func DefaultGatewayAdminAlertWireConfig() GatewayAdminAlertWireConfig {
	return GatewayAdminAlertWireConfig{
		CooldownSec:    defaultGatewayAdminAlertCooldownSec,
		MinIntervalSec: defaultGatewayAdminAlertMinIntervalSec,
	}
}

func (c GatewayAdminAlertWireConfig) Normalized() GatewayAdminAlertWireConfig {
	out := c
	if out.CooldownSec <= 0 {
		out.CooldownSec = defaultGatewayAdminAlertCooldownSec
	}
	if out.CooldownSec < minGatewayAdminAlertCooldownSec {
		out.CooldownSec = minGatewayAdminAlertCooldownSec
	}
	if out.CooldownSec > maxGatewayAdminAlertCooldownSec {
		out.CooldownSec = maxGatewayAdminAlertCooldownSec
	}
	if out.MinIntervalSec <= 0 {
		out.MinIntervalSec = defaultGatewayAdminAlertMinIntervalSec
	}
	if out.MinIntervalSec < minGatewayAdminAlertMinIntervalSec {
		out.MinIntervalSec = minGatewayAdminAlertMinIntervalSec
	}
	if out.MinIntervalSec > maxGatewayAdminAlertMinIntervalSec {
		out.MinIntervalSec = maxGatewayAdminAlertMinIntervalSec
	}
	out.Emails = normalizeEmailList(out.Emails)
	return out
}

func (e GatewayAdminAlertEffective) AnyExternalChannel() bool {
	ch := e.Channels
	return ch.Email || ch.DingTalk || ch.Feishu || ch.WeCom || ch.GenericWebhook
}

func (e GatewayAdminAlertEffective) IMStored() alertconfig.Stored {
	return alertconfig.Stored{
		Emails:                   e.Emails,
		DingTalkWebhook:          e.DingTalkWebhook,
		FeishuWebhook:            e.FeishuWebhook,
		WeComWebhook:             e.WeComWebhook,
		DingTalkSecret:           e.Secrets.DingTalkSecret,
		FeishuSecret:             e.Secrets.FeishuSecret,
		WeComSecret:              e.Secrets.WeComSecret,
		GenericWebhookSignSecret: e.Secrets.GenericWebhookSignSecret,
	}
}

var (
	gwAdminAlertCacheMu sync.Mutex
	gwAdminAlertCache   GatewayAdminAlertEffective
	gwAdminAlertUntil   time.Time
)

// InvalidateGatewayAdminAlertCache 管理端更新 gateway.admin_alert.* 后调用。
func InvalidateGatewayAdminAlertCache() {
	gwAdminAlertCacheMu.Lock()
	gwAdminAlertCache = GatewayAdminAlertEffective{}
	gwAdminAlertUntil = time.Time{}
	gwAdminAlertCacheMu.Unlock()
}

// CachedGatewayAdminAlert 短 TTL 缓存，供热路径 O(1) 读取。
func CachedGatewayAdminAlert(ctx context.Context, client *ent.Client) GatewayAdminAlertEffective {
	now := time.Now()
	gwAdminAlertCacheMu.Lock()
	if !gwAdminAlertUntil.IsZero() && now.Before(gwAdminAlertUntil) {
		out := gwAdminAlertCache
		gwAdminAlertCacheMu.Unlock()
		return out
	}
	gwAdminAlertCacheMu.Unlock()

	eff, err := LoadGatewayAdminAlert(ctx, client)
	if err != nil {
		gwAdminAlertCacheMu.Lock()
		gwAdminAlertCache = GatewayAdminAlertEffective{}
		gwAdminAlertUntil = now.Add(2 * time.Second)
		gwAdminAlertCacheMu.Unlock()
		return GatewayAdminAlertEffective{}
	}
	gwAdminAlertCacheMu.Lock()
	gwAdminAlertCache = eff
	gwAdminAlertUntil = now.Add(gatewayAdminAlertCacheTTL)
	gwAdminAlertCacheMu.Unlock()
	return eff
}

// LoadGatewayAdminAlert 从 system_configs 读取并合并 config + secrets。
func LoadGatewayAdminAlert(ctx context.Context, client *ent.Client) (GatewayAdminAlertEffective, error) {
	enabled, err := loadGatewayAdminAlertEnabled(ctx, client)
	if err != nil {
		return GatewayAdminAlertEffective{}, err
	}
	cfg, err := loadGatewayAdminAlertConfigJSON(ctx, client)
	if err != nil {
		return GatewayAdminAlertEffective{}, err
	}
	sec, err := loadGatewayAdminAlertSecretsJSON(ctx, client)
	if err != nil {
		return GatewayAdminAlertEffective{}, err
	}
	cfg = cfg.Normalized()
	return GatewayAdminAlertEffective{
		Enabled:                       enabled,
		GatewayAdminAlertWireConfig:   cfg,
		Secrets:                       sec,
	}, nil
}

// GatewayAdminAlertDBMeta 管理端 GET site-settings 用。
func GatewayAdminAlertDBMeta(ctx context.Context, client *ent.Client) (enabled bool, enabledSet bool, cfg GatewayAdminAlertWireConfig, cfgSet bool, cfgRaw string, secretsConfigured map[string]bool) {
	v, ok, _ := stringFromConfigRow(ctx, client, SystemConfigKeyGatewayAdminAlertEnabled)
	enabledSet = ok
	if ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			enabled = b
		}
	}
	cfgRow, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyGatewayAdminAlertConfig)).
		Only(ctx)
	if err == nil {
		cfgSet = true
		cfgRaw = cfgRow.ConfigValue
		if parsed, perr := parseGatewayAdminAlertConfigJSON(cfgRow.ConfigValue); perr == nil {
			cfg = parsed.Normalized()
		}
	} else if !ent.IsNotFound(err) {
		return false, false, DefaultGatewayAdminAlertWireConfig(), false, "", nil
	} else {
		cfg = DefaultGatewayAdminAlertWireConfig()
	}
	sec, _ := loadGatewayAdminAlertSecretsJSON(ctx, client)
	secretsConfigured = map[string]bool{
		"dingtalk_sign_configured":        strings.TrimSpace(sec.DingTalkSecret) != "",
		"feishu_sign_configured":          strings.TrimSpace(sec.FeishuSecret) != "",
		"wecom_sign_configured":           strings.TrimSpace(sec.WeComSecret) != "",
		"generic_webhook_sign_configured": strings.TrimSpace(sec.GenericWebhookSignSecret) != "",
	}
	if !enabledSet {
		enabled = false
	}
	return enabled, enabledSet, cfg, cfgSet, cfgRaw, secretsConfigured
}

func loadGatewayAdminAlertEnabled(ctx context.Context, client *ent.Client) (bool, error) {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyGatewayAdminAlertEnabled)
	if err != nil || !ok {
		return false, err
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, nil
	}
	return b, nil
}

func loadGatewayAdminAlertConfigJSON(ctx context.Context, client *ent.Client) (GatewayAdminAlertWireConfig, error) {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyGatewayAdminAlertConfig)
	if err != nil {
		return GatewayAdminAlertWireConfig{}, err
	}
	if !ok || strings.TrimSpace(v) == "" {
		return DefaultGatewayAdminAlertWireConfig(), nil
	}
	return parseGatewayAdminAlertConfigJSON(v)
}

func loadGatewayAdminAlertSecretsJSON(ctx context.Context, client *ent.Client) (alertconfig.Stored, error) {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyGatewayAdminAlertSecrets)
	if err != nil || !ok || strings.TrimSpace(v) == "" {
		return alertconfig.Stored{}, err
	}
	var sec alertconfig.Stored
	if err := json.Unmarshal([]byte(v), &sec); err != nil {
		return alertconfig.Stored{}, err
	}
	return sec, nil
}

// LoadGatewayAdminAlertSecretsForMerge 供管理端 PUT secrets patch 时读取现有密钥。
func LoadGatewayAdminAlertSecretsForMerge(ctx context.Context, client *ent.Client) (alertconfig.Stored, error) {
	return loadGatewayAdminAlertSecretsJSON(ctx, client)
}

func parseGatewayAdminAlertConfigJSON(raw string) (GatewayAdminAlertWireConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultGatewayAdminAlertWireConfig(), nil
	}
	var cfg GatewayAdminAlertWireConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return GatewayAdminAlertWireConfig{}, err
	}
	return cfg, nil
}

// ValidateGatewayAdminAlertEnabled 校验总开关。
func ValidateGatewayAdminAlertEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errGatewayAdminAlertEnabledBool
	}
	return nil
}

// ValidateGatewayAdminAlertConfigJSON 校验主配置 JSON。
func ValidateGatewayAdminAlertConfigJSON(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	cfg, err := parseGatewayAdminAlertConfigJSON(v)
	if err != nil {
		return fmt.Errorf("%w: %v", errGatewayAdminAlertConfigJSON, err)
	}
	cfg = cfg.Normalized()
	if cfg.CooldownSec < minGatewayAdminAlertCooldownSec || cfg.CooldownSec > maxGatewayAdminAlertCooldownSec {
		return errGatewayAdminAlertCooldownRange
	}
	if cfg.MinIntervalSec < minGatewayAdminAlertMinIntervalSec || cfg.MinIntervalSec > maxGatewayAdminAlertMinIntervalSec {
		return errGatewayAdminAlertMinIntervalRange
	}
	for _, em := range cfg.Emails {
		if strings.TrimSpace(em) == "" {
			return errGatewayAdminAlertEmptyEmail
		}
	}
	return nil
}

// ValidateGatewayAdminAlertSecretsJSON 校验密钥 JSON（alertconfig.Stored 子集）。
func ValidateGatewayAdminAlertSecretsJSON(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	var sec alertconfig.Stored
	if err := json.Unmarshal([]byte(v), &sec); err != nil {
		return errGatewayAdminAlertSecretsJSON
	}
	return nil
}

// MergeGatewayAdminAlertSecrets 合并管理端提交的 secrets patch。
func MergeGatewayAdminAlertSecrets(existing alertconfig.Stored, patchJSON []byte) (alertconfig.Stored, error) {
	return alertconfig.ApplySecretPatch(existing, patchJSON)
}

func normalizeEmailList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var (
	errGatewayAdminAlertEnabledBool     = errors.New("gateway.admin_alert.enabled 须为 true/false")
	errGatewayAdminAlertConfigJSON      = errors.New("gateway.admin_alert.config 不是合法 JSON")
	errGatewayAdminAlertSecretsJSON     = errors.New("gateway.admin_alert.secrets 不是合法 JSON")
	errGatewayAdminAlertCooldownRange     = errors.New("cooldown_sec 须在 60–3600 之间")
	errGatewayAdminAlertMinIntervalRange  = errors.New("min_interval_sec 须在 10–600 之间")
	errGatewayAdminAlertEmptyEmail        = errors.New("emails 数组元素不能为空")
)
