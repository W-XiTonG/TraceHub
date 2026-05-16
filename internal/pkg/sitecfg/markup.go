package sitecfg

import (
	"context"
	"math"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
)

const SystemConfigKeyDefaultPricingMarkupPercent = "site.default_pricing_markup_percent"

func EffectiveDefaultPricingMarkupPercent(ctx context.Context, client *ent.Client, cfg *config.Config) float64 {
	fallback := 0.0
	if cfg != nil {
		fallback = cfg.Site.DefaultPricingMarkupPercent
	}
	return effectiveFloat64MarkupConfig(ctx, client, SystemConfigKeyDefaultPricingMarkupPercent, fallback)
}

func DefaultPricingMarkupPercentDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configDBMeta(ctx, client, SystemConfigKeyDefaultPricingMarkupPercent)
}

func ValidateDefaultPricingMarkupPercent(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errDefaultPricingMarkupEmpty
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return errDefaultPricingMarkupInvalid
	}
	if v <= -100 {
		return errDefaultPricingMarkupTooLow
	}
	return nil
}

func effectiveFloat64MarkupConfig(ctx context.Context, client *ent.Client, key string, fallback float64) float64 {
	v := effectiveFloat64Config(ctx, client, key, fallback)
	if v <= -100 {
		return fallback
	}
	return v
}
