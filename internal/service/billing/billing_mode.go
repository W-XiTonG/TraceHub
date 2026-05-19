package billing

import (
	"strings"

	"tracehub/internal/ent"
)

const (
	BillingModeToken        = "token"
	BillingModePerRequest   = "per_request"
	BillingModeHybridFloor  = "hybrid_floor"
	BillingModeHybridSettle = "hybrid_settle"
)

// DefaultQuotaPerRequest 内置按次额度；管理员可通过 DB 覆盖。
var DefaultQuotaPerRequest = map[string]int64{}

func normalizeBillingMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case BillingModePerRequest, BillingModeHybridFloor, BillingModeHybridSettle:
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return BillingModeToken
	}
}

func lookupDefaultQuotaPerRequest(model string) int64 {
	if model == "" {
		return 0
	}
	keys := make([]string, 0, len(DefaultQuotaPerRequest))
	for k := range DefaultQuotaPerRequest {
		keys = append(keys, k)
	}
	best := ""
	for _, k := range keys {
		if strings.HasPrefix(model, k) && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		return 0
	}
	return DefaultQuotaPerRequest[best]
}

type billingModeQuota struct {
	mode       string
	perRequest int64
}

func resolveBillingModeQuota(model string, glo *ent.GlobalModelPricing, ch *ent.ChannelModelPricing) billingModeQuota {
	mode := BillingModeToken
	var perReq *int64

	if glo != nil && glo.Enabled {
		if glo.BillingMode != "" {
			mode = normalizeBillingMode(glo.BillingMode)
		}
		perReq = glo.QuotaPerRequest
	}
	if ch != nil && ch.Enabled {
		if ch.BillingMode != nil && strings.TrimSpace(*ch.BillingMode) != "" {
			mode = normalizeBillingMode(*ch.BillingMode)
		}
		if ch.QuotaPerRequest != nil {
			perReq = ch.QuotaPerRequest
		}
	}

	q := int64(0)
	if perReq != nil && *perReq > 0 {
		q = *perReq
	} else if mode != BillingModeToken {
		q = lookupDefaultQuotaPerRequest(model)
	}
	if mode != BillingModeToken && q <= 0 {
		mode = BillingModeToken
	}
	return billingModeQuota{mode: mode, perRequest: q}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
