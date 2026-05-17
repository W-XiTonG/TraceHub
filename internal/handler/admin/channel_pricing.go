package adminhandler

import (
	"context"
	"math"
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

type channelPricingUpsertItem struct {
	ChannelID     int
	ModelName     string
	Enabled       bool
	QuotaInPer1k  *int64
	QuotaOutPer1k *int64
	InputRate     float64
	OutputRate    float64
	MarkupPercent *float64
}

func upsertChannelModelPricingBatch(ctx context.Context, client *ent.Client, items []channelPricingUpsertItem) (badRequest string, err error) {
	for _, it := range items {
		if it.ChannelID <= 0 {
			return "channel_id 无效", nil
		}
		name := strings.TrimSpace(it.ModelName)
		if name == "" {
			return "model_name 不能为空", nil
		}
		if _, err := client.Channel.Get(ctx, it.ChannelID); err != nil {
			if ent.IsNotFound(err) {
				return "渠道不存在", nil
			}
			return "", err
		}
		if it.QuotaInPer1k != nil && *it.QuotaInPer1k < 0 {
			return "quota_in_per_1k 不能为负", nil
		}
		if it.QuotaOutPer1k != nil && *it.QuotaOutPer1k < 0 {
			return "quota_out_per_1k 不能为负", nil
		}
		inR, outR := it.InputRate, it.OutputRate
		if inR <= 0 || math.IsNaN(inR) || math.IsInf(inR, 0) {
			return "input_rate 须大于 0", nil
		}
		if outR <= 0 || math.IsNaN(outR) || math.IsInf(outR, 0) {
			return "output_rate 须大于 0", nil
		}
		if it.MarkupPercent != nil && !billing.ValidMarkupPercent(*it.MarkupPercent) {
			return "markup_percent 须为大于 -100 的有效数字", nil
		}
		existing, qerr := client.ChannelModelPricing.Query().
			Where(
				channelmodelpricing.ChannelIDEQ(it.ChannelID),
				channelmodelpricing.ModelNameEQ(name),
			).
			Only(ctx)
		if ent.IsNotFound(qerr) {
			cr := client.ChannelModelPricing.Create().
				SetChannelID(it.ChannelID).
				SetModelName(name).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR)
			if it.QuotaInPer1k != nil {
				cr.SetQuotaInPer1k(*it.QuotaInPer1k)
			}
			if it.QuotaOutPer1k != nil {
				cr.SetQuotaOutPer1k(*it.QuotaOutPer1k)
			}
			if it.MarkupPercent != nil {
				cr.SetMarkupPercent(*it.MarkupPercent)
			}
			if _, err := cr.Save(ctx); err != nil {
				return "", err
			}
		} else if qerr == nil {
			up := client.ChannelModelPricing.UpdateOne(existing).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR)
			if it.QuotaInPer1k != nil {
				up.SetQuotaInPer1k(*it.QuotaInPer1k)
			} else {
				up.ClearQuotaInPer1k()
			}
			if it.QuotaOutPer1k != nil {
				up.SetQuotaOutPer1k(*it.QuotaOutPer1k)
			} else {
				up.ClearQuotaOutPer1k()
			}
			if it.MarkupPercent != nil {
				up.SetMarkupPercent(*it.MarkupPercent)
			} else {
				up.ClearMarkupPercent()
			}
			if err := up.Exec(ctx); err != nil {
				return "", err
			}
		} else {
			return "", qerr
		}
	}
	return "", nil
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
				ChannelID     int      `json:"channel_id"`
				ModelName     string   `json:"model_name"`
				Enabled       bool     `json:"enabled"`
				QuotaInPer1k  *int64   `json:"quota_in_per_1k"`
				QuotaOutPer1k *int64   `json:"quota_out_per_1k"`
				InputRate     float64  `json:"input_rate"`
				OutputRate    float64  `json:"output_rate"`
				MarkupPercent *float64 `json:"markup_percent"`
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
				ChannelID:     it.ChannelID,
				ModelName:     it.ModelName,
				Enabled:       it.Enabled,
				QuotaInPer1k:  it.QuotaInPer1k,
				QuotaOutPer1k: it.QuotaOutPer1k,
				InputRate:     it.InputRate,
				OutputRate:    it.OutputRate,
				MarkupPercent: it.MarkupPercent,
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
