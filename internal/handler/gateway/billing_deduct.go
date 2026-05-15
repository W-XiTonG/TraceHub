package gateway

import (
	"errors"
	"log/slog"

	"tracehub/internal/middleware"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

// gatewayDeduct runs atomic balance deduction after upstream work.
// If afterResponse is true (e.g. streaming body already flushed), errors are only logged.
func gatewayDeduct(c *gin.Context, eng *billing.Engine, userID int64, cost int64, afterResponse bool) error {
	if eng == nil || cost <= 0 {
		return nil
	}
	err := eng.DeductBalance(c.Request.Context(), userID, cost)
	if err == nil {
		return nil
	}
	if errors.Is(err, billing.ErrInsufficientBalance) {
		if afterResponse {
			slog.Warn("gateway deduct: insufficient balance after response started",
				"user_id", userID, "cost", cost)
			return err
		}
		bal := int64(0)
		if eng != nil && eng.Client != nil {
			if u2, err2 := eng.Client.User.Get(c.Request.Context(), int(userID)); err2 == nil {
				bal = u2.Balance
			}
		}
		WriteInsufficientBalance402PostDebit(c, bal, cost)
		return err
	}
	if afterResponse {
		slog.Error("gateway deduct failed after response started",
			"user_id", userID, "cost", cost, "err", err)
		return err
	}
	middleware.WriteOpenAIError(c, 500, "api_error", "扣费失败，请稍后重试或联系管理员")
	return err
}
