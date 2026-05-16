package payment

import (
	"strings"

	"tracehub/internal/pkg/config"
)

// QuotaFromCNYCents 根据人民币分计算额度。
func QuotaFromCNYCents(cents int64, cfg *config.Config) int64 {
	unit := cfg.Site.CurrencyQuotaPerYuan
	if unit <= 0 {
		unit = 10000
	}
	return cents * unit / 100
}

// QuotaFromUSDCents 根据美元分计算额度。
func QuotaFromUSDCents(cents int64, cfg *config.Config) int64 {
	if cfg.Site.CurrencyQuotaPerUsd > 0 {
		return cents * cfg.Site.CurrencyQuotaPerUsd / 100
	}
	cnyPerUSD := cfg.Site.PricingCnyPerUsd
	unit := cfg.Site.CurrencyQuotaPerYuan
	if unit <= 0 {
		unit = 10000
	}
	if cnyPerUSD <= 0 {
		return 0
	}
	cnyCents := int64(float64(cents) * cnyPerUSD)
	return cnyCents * unit / 100
}

// QuotaForAmount 按币种计算额度。
func QuotaForAmount(cents int64, currency string, cfg *config.Config) int64 {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "CNY"
	}
	if cur == "USD" {
		return QuotaFromUSDCents(cents, cfg)
	}
	return QuotaFromCNYCents(cents, cfg)
}
