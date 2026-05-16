package billing

import (
	"fmt"
	"math"
	"strings"
)

// SitePricingFactors 将站点 YAML 中与「标价 → 额度」相关的字段打平，便于单测注入。
type SitePricingFactors struct {
	QuotaPerYuan int64   // site.currency_quota_per_yuan，须 >0（CNY 路径或 USD 经汇率路径）
	QuotaPerUsd  int64   // site.currency_quota_per_usd，>0 时美元路径优先
	CnyPerUsd    float64 // site.pricing_cny_per_usd，表示 1 USD = R CNY
}

// QuotaPer1KFromUpstreamPrice 将「每 denomTokens 个 token 的上游标价」换算为每 1000 token 的额度（向上取整）。
// price 为上游货币计价（与 currency 一致）；marginPercent 如 5 表示在基准额度上再乘 1.05。
func QuotaPer1KFromUpstreamPrice(factors SitePricingFactors, currency string, denomTokens int64, price float64, marginPercent float64) (quota int64, factorDesc string, err error) {
	cur := strings.ToLower(strings.TrimSpace(currency))
	if denomTokens != 1 && denomTokens != 1000 && denomTokens != 1_000_000 {
		return 0, "", fmt.Errorf("denom_tokens must be 1, 1000, or 1000000")
	}
	if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, "", fmt.Errorf("price must be a non-negative finite number")
	}
	if math.IsNaN(marginPercent) || math.IsInf(marginPercent, 0) {
		return 0, "", fmt.Errorf("margin_percent must be finite")
	}
	if marginPercent <= -100 {
		return 0, "", fmt.Errorf("margin_percent must be greater than -100")
	}

	costPer1k := price * (1000.0 / float64(denomTokens))
	if costPer1k < 0 {
		return 0, "", fmt.Errorf("internal: negative costPer1k")
	}

	mul := 1.0 + marginPercent/100.0
	var raw float64
	switch cur {
	case "cny", "rmb":
		if factors.QuotaPerYuan <= 0 {
			return 0, "", fmt.Errorf("currency_quota_per_yuan must be > 0 for CNY upstream prices")
		}
		raw = costPer1k * float64(factors.QuotaPerYuan) * mul
		factorDesc = fmt.Sprintf("cny: cost_per_1k_tokens=%g CNY × quota_per_yuan=%d × (1+margin%%)", costPer1k, factors.QuotaPerYuan)
	case "usd":
		switch {
		case factors.QuotaPerUsd > 0:
			raw = costPer1k * float64(factors.QuotaPerUsd) * mul
			factorDesc = fmt.Sprintf("usd: cost_per_1k_tokens=$%g × quota_per_usd=%d × (1+margin%%)", costPer1k, factors.QuotaPerUsd)
		case factors.CnyPerUsd > 0 && factors.QuotaPerYuan > 0:
			raw = costPer1k * factors.CnyPerUsd * float64(factors.QuotaPerYuan) * mul
			factorDesc = fmt.Sprintf("usd: cost_per_1k_tokens=$%g × cny_per_usd=%g × quota_per_yuan=%d × (1+margin%%)", costPer1k, factors.CnyPerUsd, factors.QuotaPerYuan)
		default:
			return 0, "", fmt.Errorf("for USD upstream prices set currency_quota_per_usd > 0, or set pricing_cny_per_usd > 0 with currency_quota_per_yuan > 0")
		}
	default:
		return 0, "", fmt.Errorf("currency must be cny or usd")
	}

	q := int64(math.Ceil(raw))
	if q < 0 {
		q = 0
	}
	return q, factorDesc + "; round=ceil_to_int64", nil
}

// DeriveGlobalQuotaPer1KPair 计算输入/输出两侧每千 token 额度；priceOut<=0 或 NaN 时复用 priceIn。
func DeriveGlobalQuotaPer1KPair(factors SitePricingFactors, currency string, denomTokens int64, priceIn, priceOut float64, marginPercent float64) (qIn, qOut int64, resolvedIn, resolvedOut string, err error) {
	pOut := priceOut
	if pOut <= 0 || math.IsNaN(pOut) {
		pOut = priceIn
	}
	qIn, resolvedIn, err = QuotaPer1KFromUpstreamPrice(factors, currency, denomTokens, priceIn, marginPercent)
	if err != nil {
		return 0, 0, "", "", err
	}
	qOut, resolvedOut, err = QuotaPer1KFromUpstreamPrice(factors, currency, denomTokens, pOut, marginPercent)
	if err != nil {
		return 0, 0, "", "", err
	}
	return qIn, qOut, resolvedIn, resolvedOut, nil
}

// UpstreamPriceFromQuotaPer1K 为 QuotaPer1KFromUpstreamPrice 的近似反算（用于展示代码基准等价上游价；因正向取整，反算价略偏低）。
func UpstreamPriceFromQuotaPer1K(factors SitePricingFactors, currency string, denomTokens int64, quotaPer1k int64, marginPercent float64) (price float64, err error) {
	if quotaPer1k < 0 {
		return 0, fmt.Errorf("quota must be non-negative")
	}
	if quotaPer1k == 0 {
		return 0, nil
	}
	cur := strings.ToLower(strings.TrimSpace(currency))
	if denomTokens != 1 && denomTokens != 1000 && denomTokens != 1_000_000 {
		return 0, fmt.Errorf("denom_tokens must be 1, 1000, or 1000000")
	}
	if math.IsNaN(marginPercent) || math.IsInf(marginPercent, 0) {
		return 0, fmt.Errorf("margin_percent must be finite")
	}
	if marginPercent <= -100 {
		return 0, fmt.Errorf("margin_percent must be greater than -100")
	}
	mul := 1.0 + marginPercent/100.0
	costPer1k := float64(quotaPer1k) / mul
	switch cur {
	case "cny", "rmb":
		if factors.QuotaPerYuan <= 0 {
			return 0, fmt.Errorf("currency_quota_per_yuan must be > 0")
		}
		price = costPer1k / float64(factors.QuotaPerYuan) * (float64(denomTokens) / 1000.0)
	case "usd":
		switch {
		case factors.QuotaPerUsd > 0:
			price = costPer1k / float64(factors.QuotaPerUsd) * (float64(denomTokens) / 1000.0)
		case factors.CnyPerUsd > 0 && factors.QuotaPerYuan > 0:
			price = costPer1k / (factors.CnyPerUsd * float64(factors.QuotaPerYuan)) * (float64(denomTokens) / 1000.0)
		default:
			return 0, fmt.Errorf("for USD set currency_quota_per_usd > 0, or pricing_cny_per_usd with currency_quota_per_yuan")
		}
	default:
		return 0, fmt.Errorf("currency must be cny or usd")
	}
	if price < 0 || math.IsNaN(price) || math.IsInf(price, 0) {
		return 0, fmt.Errorf("internal: invalid upstream price")
	}
	return price, nil
}
