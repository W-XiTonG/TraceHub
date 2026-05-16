package userhandler

import (
	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/availmodels"
	"tracehub/internal/service/billing"
	"tracehub/internal/service/payment"

	"github.com/gin-gonic/gin"
)

// PubSiteMeta GET /api/pub/site-meta — 无需登录；品牌名与图标供前端壳层使用。
func PubSiteMeta(client *ent.Client, cfg *config.Config, payStore *payment.ConfigStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 避免浏览器/代理缓存导致恢复默认后仍读到旧的 brand_icon_url
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		ctx := c.Request.Context()
		out := gin.H{
			"brand_name":     sitecfg.EffectiveBrandName(ctx, client),
			"brand_icon_url": sitecfg.EffectiveBrandIconURL(ctx, client),
			"app_version":    cfg.AppVersion(),
		}
		if promo := sitecfg.EffectiveRegisterPromoText(ctx, client, cfg); promo != "" {
			out["register_promo_text"] = promo
		}
		if api := sitecfg.EffectiveAPIPublicBaseURL(ctx, client); api != "" {
			out["api_public_base_url"] = api
		}
		if pc := sitecfg.EffectivePublicContactData(ctx, client); sitecfg.PublicContactHasVisibleContent(pc) {
			qr := make([]gin.H, 0, len(pc.QRItems))
			for _, it := range pc.QRItems {
				qr = append(qr, gin.H{"label": it.Label, "image_url": it.ImageURL})
			}
			out["public_contact"] = gin.H{
				"title":    pc.Title,
				"qr_items": qr,
				"emails":   pc.Emails,
			}
		}
		// 全站已启用渠道声明的模型键去重数（models + model_alias 键）；失败时省略该字段。
		if n, err := availmodels.DistinctDeclaredModelCountSiteWide(ctx, client); err == nil {
			if badge := billing.ModelCountHundredPlusBadge(n); badge != "" {
				out["declared_model_count_badge"] = badge
			}
		}
		if payStore != nil {
			if cfgPay, err := payStore.Load(ctx); err == nil {
				out["online_payment_enabled"] = cfgPay.OnlineEnabled
			} else {
				out["online_payment_enabled"] = false
			}
		} else {
			out["online_payment_enabled"] = false
		}
		httpx.OK(c, out)
	}
}
