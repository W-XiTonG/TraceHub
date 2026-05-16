package payment

import (
	"context"
	"errors"

	"tracehub/internal/pkg/httpx"

	"github.com/gin-gonic/gin"
)

// ErrOnlineDisabled 在线支付总开关关闭。
var ErrOnlineDisabled = errors.New("online payment disabled")

// OnlineEnabled 是否已开通在线支付（总开关）。
func (s *Service) OnlineEnabled(ctx context.Context) bool {
	cfg, err := s.RuntimeConfig(ctx)
	if err != nil {
		return false
	}
	return cfg.OnlineEnabled
}

// OnlineOnly 拒绝未开通时的用户/管理业务接口（配置页除外，由路由单独挂载）。
func OnlineOnly(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !svc.OnlineEnabled(c.Request.Context()) {
			httpx.Forbidden(c, "在线支付功能未开通")
			c.Abort()
			return
		}
		c.Next()
	}
}
