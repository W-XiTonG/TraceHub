package userhandler

import (
	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
)

// PubSiteMeta GET /api/pub/site-meta — 无需登录；品牌名与图标供前端壳层使用。
func PubSiteMeta(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 避免浏览器/代理缓存导致恢复默认后仍读到旧的 brand_icon_url
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		ctx := c.Request.Context()
		out := gin.H{
			"brand_name":     sitecfg.EffectiveBrandName(ctx, client),
			"brand_icon_url": sitecfg.EffectiveBrandIconURL(ctx, client),
		}
		if promo := sitecfg.EffectiveRegisterPromoText(ctx, client, cfg); promo != "" {
			out["register_promo_text"] = promo
		}
		if api := sitecfg.EffectiveAPIPublicBaseURL(ctx, client); api != "" {
			out["api_public_base_url"] = api
		}
		httpx.OK(c, out)
	}
}
