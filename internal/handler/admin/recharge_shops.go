package adminhandler

import (
	"encoding/json"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/pkg/sitecfg"

	"github.com/gin-gonic/gin"
)

func rechargeShopsResponse(d sitecfg.RechargeShopsData) gin.H {
	shops := make([]gin.H, 0, len(d.Shops))
	for _, it := range d.Shops {
		item := gin.H{
			"label":       it.Label,
			"url":         it.URL,
			"description": it.Description,
			"enabled":     it.Enabled,
			"sort":        it.Sort,
			"show_qr":     it.ShowQR,
		}
		if it.QRImageURL != "" {
			item["qr_image_url"] = it.QRImageURL
		}
		shops = append(shops, item)
	}
	return gin.H{
		"title": d.Title,
		"hint":  d.Hint,
		"shops": shops,
	}
}

// GetRechargeShops GET /api/admin/payment/recharge-shops
func GetRechargeShops(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		d := sitecfg.AdminRechargeShopsData(c.Request.Context(), client)
		httpx.OK(c, rechargeShopsResponse(d))
	}
}

// PutRechargeShops PUT /api/admin/payment/recharge-shops
func PutRechargeShops(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var body sitecfg.RechargeShopsData
		if err := c.ShouldBindJSON(&body); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		raw, err := json.Marshal(body)
		if err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		canon, err := sitecfg.CanonicalRechargeShopsJSON(string(raw))
		if err != nil {
			httpx.BadRequest(c, err.Error())
			return
		}
		ctx := c.Request.Context()
		ex, err := client.SystemConfig.Query().
			Where(systemconfig.ConfigKeyEQ(sitecfg.SystemConfigKeyRechargeShopsJSON)).
			Only(ctx)
		if ent.IsNotFound(err) {
			_, err = client.SystemConfig.Create().
				SetConfigKey(sitecfg.SystemConfigKeyRechargeShopsJSON).
				SetConfigValue(canon).
				SetDescription("充值外部商铺链接配置").
				Save(ctx)
		} else if err != nil {
			httpx.ServerError(c, "查询失败")
			return
		} else {
			err = client.SystemConfig.UpdateOne(ex).SetConfigValue(canon).Exec(ctx)
		}
		if err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		auditLog(client, adm.ID, "payment.recharge_shops", "payment", "recharge_shops", gin.H{"shops_count": len(body.Shops)}, c)
		d := sitecfg.AdminRechargeShopsData(ctx, client)
		httpx.OKMsg(c, "已保存", rechargeShopsResponse(d))
	}
}
