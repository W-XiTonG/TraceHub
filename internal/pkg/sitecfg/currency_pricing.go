package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

const (
	SystemConfigKeyCurrencyQuotaPerYuan = "site.currency_quota_per_yuan"
	SystemConfigKeyCurrencyQuotaPerUsd  = "site.currency_quota_per_usd"
	SystemConfigKeyPricingCnyPerUsd       = "site.pricing_cny_per_usd"
)

// CurrencyPricing 全站「标价 → 额度」换算参数（YAML 可被 system_configs 覆盖）。
type CurrencyPricing struct {
	QuotaPerYuan int64
	QuotaPerUsd  int64
	CnyPerUsd    float64
}

func EffectiveCurrencyQuotaPerYuan(ctx context.Context, client *ent.Client, cfg *config.Config) int64 {
	fallback := int64(0)
	if cfg != nil {
		fallback = cfg.Site.CurrencyQuotaPerYuan
	}
	return effectiveInt64Config(ctx, client, SystemConfigKeyCurrencyQuotaPerYuan, fallback, true)
}

func EffectiveCurrencyQuotaPerUsd(ctx context.Context, client *ent.Client, cfg *config.Config) int64 {
	fallback := int64(0)
	if cfg != nil {
		fallback = cfg.Site.CurrencyQuotaPerUsd
	}
	return effectiveInt64Config(ctx, client, SystemConfigKeyCurrencyQuotaPerUsd, fallback, false)
}

func EffectivePricingCnyPerUsd(ctx context.Context, client *ent.Client, cfg *config.Config) float64 {
	fallback := 0.0
	if cfg != nil {
		fallback = cfg.Site.PricingCnyPerUsd
	}
	return effectiveFloat64Config(ctx, client, SystemConfigKeyPricingCnyPerUsd, fallback)
}

func EffectiveCurrencyPricing(ctx context.Context, client *ent.Client, cfg *config.Config) CurrencyPricing {
	return CurrencyPricing{
		QuotaPerYuan: EffectiveCurrencyQuotaPerYuan(ctx, client, cfg),
		QuotaPerUsd:  EffectiveCurrencyQuotaPerUsd(ctx, client, cfg),
		CnyPerUsd:    EffectivePricingCnyPerUsd(ctx, client, cfg),
	}
}

func CurrencyQuotaPerYuanDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configDBMeta(ctx, client, SystemConfigKeyCurrencyQuotaPerYuan)
}

func CurrencyQuotaPerUsdDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configDBMeta(ctx, client, SystemConfigKeyCurrencyQuotaPerUsd)
}

func PricingCnyPerUsdDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configDBMeta(ctx, client, SystemConfigKeyPricingCnyPerUsd)
}

func ValidateCurrencyQuotaPerYuan(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errCurrencyQuotaPerYuanEmpty
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return errCurrencyQuotaPerYuanInvalid
	}
	return nil
}

func ValidateCurrencyQuotaPerUsd(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errCurrencyQuotaPerUsdEmpty
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return errCurrencyQuotaPerUsdInvalid
	}
	return nil
}

func ValidatePricingCnyPerUsd(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errPricingCnyPerUsdEmpty
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return errPricingCnyPerUsdInvalid
	}
	return nil
}

func effectiveInt64Config(ctx context.Context, client *ent.Client, key string, fallback int64, requirePositive bool) int64 {
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
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
	if err != nil {
		return fallback
	}
	if requirePositive && v <= 0 {
		return fallback
	}
	if !requirePositive && v < 0 {
		return fallback
	}
	return v
}

func effectiveFloat64Config(ctx context.Context, client *ent.Client, key string, fallback float64) float64 {
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
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
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func configDBMeta(ctx context.Context, client *ent.Client, key string) (hasRow bool, raw string) {
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}
