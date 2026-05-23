package adminhandler

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channelmodelpricing"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/availmodels"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

type channelPricingUpsertItem = billing.ChannelPricingUpsertItem

func upsertChannelModelPricingBatch(ctx context.Context, client *ent.Client, items []channelPricingUpsertItem) (badRequest string, err error) {
	return billing.UpsertChannelModelPricingBatch(ctx, client, items)
}

// GetChannelModelPricingWorkspace GET /api/admin/channel-model-pricing/workspace
func GetChannelModelPricingWorkspace(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID, _ := strconv.Atoi(strings.TrimSpace(c.Query("channel_id")))
		if channelID <= 0 {
			httpx.BadRequest(c, "channel_id 必填且须为正整数")
			return
		}
		ctx := c.Request.Context()
		q := strings.TrimSpace(c.Query("q"))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		ps, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
		items, total, pageOut, err := availmodels.ChannelModelPricingWorkspacePage(ctx, client, channelID, q, page, ps)
		if err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "渠道不存在")
				return
			}
			httpx.ServerError(c, "查询失败")
			return
		}
		outPS := ps
		if outPS <= 0 {
			outPS = 50
		}
		if outPS > 200 {
			outPS = 200
		}
		httpx.OK(c, gin.H{
			"channel_id": channelID,
			"items":      items,
			"total":      total,
			"page":       pageOut,
			"page_size":  outPS,
			"q":          q,
		})
	}
}

// PutChannelModelPricing PUT /api/admin/channel-model-pricing
func PutChannelModelPricing(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		var req struct {
			Items []struct {
				ChannelID       int      `json:"channel_id"`
				ModelName       string   `json:"model_name"`
				Enabled         bool     `json:"enabled"`
				QuotaInPer1k    *int64   `json:"quota_in_per_1k"`
				QuotaOutPer1k   *int64   `json:"quota_out_per_1k"`
				InputRate       float64  `json:"input_rate"`
				OutputRate      float64  `json:"output_rate"`
				MarkupPercent   *float64 `json:"markup_percent"`
				BillingMode     *string  `json:"billing_mode"`
				QuotaPerRequest *int64   `json:"quota_per_request"`
			} `json:"items"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.BadRequest(c, "参数无效")
			return
		}
		ctx := c.Request.Context()
		items := make([]channelPricingUpsertItem, 0, len(req.Items))
		channelIDs := make(map[int]struct{})
		for _, it := range req.Items {
			items = append(items, channelPricingUpsertItem{
				ChannelID:       it.ChannelID,
				ModelName:       it.ModelName,
				Enabled:         it.Enabled,
				QuotaInPer1k:    it.QuotaInPer1k,
				QuotaOutPer1k:   it.QuotaOutPer1k,
				InputRate:       it.InputRate,
				OutputRate:      it.OutputRate,
				MarkupPercent:   it.MarkupPercent,
				BillingMode:     it.BillingMode,
				QuotaPerRequest: it.QuotaPerRequest,
			})
			if it.ChannelID > 0 {
				channelIDs[it.ChannelID] = struct{}{}
			}
		}
		if br, err := upsertChannelModelPricingBatch(ctx, client, items); br != "" {
			httpx.BadRequest(c, br)
			return
		} else if err != nil {
			httpx.ServerError(c, "保存失败")
			return
		}
		for cid := range channelIDs {
			billing.InvalidateChannelModelPricingCacheAdmin(cid)
		}
		auditLog(client, adm.ID, "channel_model_pricing.put", "channel_model_pricing", "", map[string]any{"count": len(req.Items)}, c)
		httpx.OKMsg(c, "已保存", nil)
	}
}

// DeleteChannelModelPricing DELETE /api/admin/channel-model-pricing?channel_id=&model_name=
func DeleteChannelModelPricing(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		channelID, _ := strconv.Atoi(strings.TrimSpace(c.Query("channel_id")))
		name := strings.TrimSpace(c.Query("model_name"))
		if channelID <= 0 {
			httpx.BadRequest(c, "channel_id 必填且须为正整数")
			return
		}
		if name == "" {
			httpx.BadRequest(c, "model_name 不能为空")
			return
		}
		ctx := c.Request.Context()
		n, err := client.ChannelModelPricing.Delete().
			Where(
				channelmodelpricing.ChannelIDEQ(channelID),
				channelmodelpricing.ModelNameEQ(name),
			).
			Exec(ctx)
		if err != nil {
			httpx.ServerError(c, "删除失败")
			return
		}
		if n == 0 {
			httpx.BadRequest(c, "记录不存在")
			return
		}
		billing.InvalidateChannelModelPricingCacheAdmin(channelID)
		auditLog(client, adm.ID, "channel_model_pricing.delete", "channel_model_pricing", strconv.Itoa(channelID)+":"+name, nil, c)
		httpx.OKMsg(c, "已删除", nil)
	}
}
