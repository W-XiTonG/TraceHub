package userhandler

import (
	"tracehub/internal/ent"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
)

// PubSiteMeta GET /api/pub/site-meta — 无需登录；品牌名与图标供前端壳层使用。
func PubSiteMeta(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		httpx.OK(c, gin.H{
			"brand_name":     sitecfg.EffectiveBrandName(ctx, client),
			"brand_icon_url": sitecfg.EffectiveBrandIconURL(ctx, client),
		})
	}
}
