package billing

import (
	"math"

	"tracehub/internal/ent"
)

// ResolveMarkupPercent 返回扣费有效成本加成 %：单模型行内非 nil 时优先，否则全站默认（由调用方传入已缓存的全站值）。
func ResolveMarkupPercent(siteMarkupPercent float64, glo *ent.GlobalModelPricing) float64 {
	if glo != nil && glo.MarkupPercent != nil {
		return *glo.MarkupPercent
	}
	return siteMarkupPercent
}

// ValidMarkupPercent 校验加成百分比（须 > -100）。
func ValidMarkupPercent(m float64) bool {
	return !math.IsNaN(m) && !math.IsInf(m, 0) && m > -100
}
