package billing

import (
	"math"

	"tracehub/internal/ent"
)

// ResolveMarkupPercent 返回扣费有效成本加成 %：渠道行 > 全站行 > 全站默认。
func ResolveMarkupPercent(siteMarkupPercent float64, glo *ent.GlobalModelPricing, ch *ent.ChannelModelPricing) float64 {
	if ch != nil && ch.Enabled && ch.MarkupPercent != nil {
		return *ch.MarkupPercent
	}
	if glo != nil && glo.Enabled && glo.MarkupPercent != nil {
		return *glo.MarkupPercent
	}
	return siteMarkupPercent
}

// ValidMarkupPercent 校验加成百分比（须 > -100）。
func ValidMarkupPercent(m float64) bool {
	return !math.IsNaN(m) && !math.IsInf(m, 0) && m > -100
}
