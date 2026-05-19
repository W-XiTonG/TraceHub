package gateway

import (
	"tracehub/internal/ent"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

// BillTokenEst 预检用 token 估算（仅 token / hybrid_floor 模式会执行 tokenEstFn）。
type BillTokenEst struct {
	Prompt, Completion, Cached int
}

// TokenEstFn 惰性估算；按次模型不会调用。
type TokenEstFn func() (BillTokenEst, error)

// GatewayBillingPrecheck 余额预检。tokenEstFn 在 NeedsTokenEstimate 为 true 时调用。
// 返回 false 表示已写 402 或 tokenEstFn 出错（调用方须处理 err）。
func GatewayBillingPrecheck(c *gin.Context, u *ent.User, br billing.ModelBillRates, tokenEstFn TokenEstFn) (est BillTokenEst, ok bool) {
	if br.NeedsTokenEstimate() && tokenEstFn != nil {
		var err error
		est, err = tokenEstFn()
		if err != nil {
			return est, false
		}
	}
	costPre, _ := br.PrecheckCost(est.Prompt, est.Completion, est.Cached)
	if u.Balance < costPre {
		pt, comp := est.Prompt, est.Completion
		if br.IsPerRequest() {
			pt, comp = 0, 0
		}
		WriteInsufficientBalance402Precheck(c, u.Balance, costPre, pt, comp)
		return est, false
	}
	return est, true
}

// GatewayFinalCost 成功响应实扣额度。
func GatewayFinalCost(br billing.ModelBillRates, prompt, completion, cached int) int64 {
	return br.FinalCost(prompt, completion, cached)
}

// GatewayChargeAndDeduct 计算实扣并扣余额。
func GatewayChargeAndDeduct(c *gin.Context, eng *billing.Engine, userID int64, br billing.ModelBillRates, prompt, completion, cached int, afterResponse bool) error {
	cost := GatewayFinalCost(br, prompt, completion, cached)
	return gatewayDeduct(c, eng, userID, cost, afterResponse)
}
