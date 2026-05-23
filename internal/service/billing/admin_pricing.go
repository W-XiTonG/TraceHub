package billing

import (
	"context"
	"math"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channelmodelpricing"
	"tracehub/internal/ent/globalmodelpricing"
	"tracehub/internal/ent/groupmodelrate"
)

// GlobalPricingUpsertItem 全站模型计价 upsert 行。
type GlobalPricingUpsertItem struct {
	ModelName       string
	Enabled         bool
	QuotaInPer1k    *int64
	QuotaOutPer1k   *int64
	InputRate       float64
	OutputRate      float64
	MarkupPercent   *float64
	BillingMode     string
	QuotaPerRequest *int64
}

// ChannelPricingUpsertItem 渠道模型计价 upsert 行。
type ChannelPricingUpsertItem struct {
	ChannelID       int
	ModelName       string
	Enabled         bool
	QuotaInPer1k    *int64
	QuotaOutPer1k   *int64
	InputRate       float64
	OutputRate      float64
	MarkupPercent   *float64
	BillingMode     *string
	QuotaPerRequest *int64
}

// GroupModelRateUpsertItem 分组模型倍率 upsert 行。
type GroupModelRateUpsertItem struct {
	ModelName  string
	InputRate  float64
	OutputRate float64
	Enabled    bool
	Billable   bool
}

// UpsertGlobalModelPricingBatch 批量 upsert 全站模型计价；badRequest 非空为校验错误。
func UpsertGlobalModelPricingBatch(ctx context.Context, client *ent.Client, items []GlobalPricingUpsertItem) (badRequest string, err error) {
	for _, it := range items {
		name := strings.TrimSpace(it.ModelName)
		if name == "" {
			return "model_name 不能为空", nil
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
		if it.MarkupPercent != nil && !ValidMarkupPercent(*it.MarkupPercent) {
			return "markup_percent 须为大于 -100 的有效数字", nil
		}
		bm := strings.TrimSpace(it.BillingMode)
		if bm == "" {
			bm = BillingModeToken
		}
		switch bm {
		case BillingModeToken, BillingModePerRequest, BillingModeHybridFloor, BillingModeHybridSettle:
		default:
			return "billing_mode 无效（token|per_request|hybrid_floor|hybrid_settle）", nil
		}
		if bm != BillingModeToken {
			qpr := int64(0)
			if it.QuotaPerRequest != nil {
				qpr = *it.QuotaPerRequest
			}
			if qpr <= 0 {
				return "按次/混合模式须设置 quota_per_request > 0", nil
			}
		}
		if it.QuotaPerRequest != nil && *it.QuotaPerRequest < 0 {
			return "quota_per_request 不能为负", nil
		}
		existing, qerr := client.GlobalModelPricing.Query().
			Where(globalmodelpricing.ModelNameEQ(name)).
			Only(ctx)
		if ent.IsNotFound(qerr) {
			cr := client.GlobalModelPricing.Create().
				SetModelName(name).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR).
				SetBillingMode(bm)
			if it.QuotaInPer1k != nil {
				cr.SetQuotaInPer1k(*it.QuotaInPer1k)
			}
			if it.QuotaOutPer1k != nil {
				cr.SetQuotaOutPer1k(*it.QuotaOutPer1k)
			}
			if it.MarkupPercent != nil {
				cr.SetMarkupPercent(*it.MarkupPercent)
			}
			if it.QuotaPerRequest != nil {
				cr.SetQuotaPerRequest(*it.QuotaPerRequest)
			}
			if _, err := cr.Save(ctx); err != nil {
				return "", err
			}
		} else if qerr == nil {
			up := client.GlobalModelPricing.UpdateOne(existing).
				SetEnabled(it.Enabled).
				SetInputRate(inR).
				SetOutputRate(outR).
				SetBillingMode(bm)
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
			if it.QuotaPerRequest != nil {
				up.SetQuotaPerRequest(*it.QuotaPerRequest)
			} else {
				up.ClearQuotaPerRequest()
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

// UpsertChannelModelPricingBatch 批量 upsert 渠道模型计价。
func UpsertChannelModelPricingBatch(ctx context.Context, client *ent.Client, items []ChannelPricingUpsertItem) (badRequest string, err error) {
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
		if it.MarkupPercent != nil && !ValidMarkupPercent(*it.MarkupPercent) {
			return "markup_percent 须为大于 -100 的有效数字", nil
		}
		if it.BillingMode != nil {
			bm := strings.TrimSpace(*it.BillingMode)
			if bm != "" {
				switch bm {
				case BillingModeToken, BillingModePerRequest, BillingModeHybridFloor, BillingModeHybridSettle:
				default:
					return "billing_mode 无效（token|per_request|hybrid_floor|hybrid_settle）", nil
				}
			}
		}
		if it.QuotaPerRequest != nil && *it.QuotaPerRequest < 0 {
			return "quota_per_request 不能为负", nil
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
			if it.BillingMode != nil {
				bm := strings.TrimSpace(*it.BillingMode)
				if bm != "" {
					cr.SetBillingMode(bm)
				}
			}
			if it.QuotaPerRequest != nil {
				cr.SetQuotaPerRequest(*it.QuotaPerRequest)
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
			if it.BillingMode != nil {
				bm := strings.TrimSpace(*it.BillingMode)
				if bm != "" {
					up.SetBillingMode(bm)
				} else {
					up.ClearBillingMode()
				}
			}
			if it.QuotaPerRequest != nil {
				up.SetQuotaPerRequest(*it.QuotaPerRequest)
			} else {
				up.ClearQuotaPerRequest()
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

// UpsertGroupModelRatesBatch 批量 upsert 指定分组的模型倍率。
func UpsertGroupModelRatesBatch(ctx context.Context, client *ent.Client, groupID int, items []GroupModelRateUpsertItem) error {
	for _, r := range items {
		existing, err := client.GroupModelRate.Query().
			Where(groupmodelrate.GroupIDEQ(groupID), groupmodelrate.ModelNameEQ(r.ModelName)).
			Only(ctx)
		if ent.IsNotFound(err) {
			_, err = client.GroupModelRate.Create().
				SetGroupID(groupID).
				SetModelName(r.ModelName).
				SetInputRate(r.InputRate).
				SetOutputRate(r.OutputRate).
				SetEnabled(r.Enabled).
				SetBillable(r.Billable).
				Save(ctx)
		} else if err == nil {
			err = client.GroupModelRate.UpdateOne(existing).
				SetInputRate(r.InputRate).
				SetOutputRate(r.OutputRate).
				SetEnabled(r.Enabled).
				SetBillable(r.Billable).
				Exec(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
