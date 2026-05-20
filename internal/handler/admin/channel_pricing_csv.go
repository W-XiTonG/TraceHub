package adminhandler

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/pkg/config"
	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/httpx"
	"tracehub/internal/service/availmodels"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

var (
	channelPricingBatchDataExportHeader = []string{
		"channel_id", "channel_name", "model_name", "enabled", "quota_in_per_1k", "quota_out_per_1k",
		"input_rate", "output_rate", "markup_percent",
	}
	channelPricingBatchImportTemplateHeader = []string{
		"channel_id", "channel_name", "channel_type", "model_name", "enabled", "quota_in_per_1k", "quota_out_per_1k", "input_rate", "output_rate",
		"upstream_currency", "upstream_denominator_tokens", "upstream_price_in", "upstream_price_out", "upstream_margin_percent",
		"billing_mode", "quota_per_request", "capability_hint",
	}
)

const channelPricingCSVMaxRows = 5000

func parseChannelIDQuery(c *gin.Context) (int, string) {
	channelID, _ := strconv.Atoi(strings.TrimSpace(c.Query("channel_id")))
	if channelID <= 0 {
		return 0, "channel_id 必填且须为正整数"
	}
	return channelID, ""
}

func channelPricingMarkupCSVCell(row *ent.ChannelModelPricing) string {
	if row == nil || row.MarkupPercent == nil {
		return ""
	}
	return strconv.FormatFloat(*row.MarkupPercent, 'f', -1, 64)
}

type channelPricingCSVFields struct {
	enabled         bool
	qIn             string
	qOut            string
	qInPtr          *int64
	qOutPtr         *int64
	inR             float64
	outR            float64
	markup          string
	billingMode     string
	quotaPerRequest *int64
}

func channelPricingCSVFieldsFromRow(row *ent.ChannelModelPricing) channelPricingCSVFields {
	out := channelPricingCSVFields{
		enabled: true,
		inR:     1.0,
		outR:    1.0,
	}
	if row == nil {
		return out
	}
	out.enabled = row.Enabled
	out.inR, out.outR = row.InputRate, row.OutputRate
	out.qInPtr, out.qOutPtr = row.QuotaInPer1k, row.QuotaOutPer1k
	if row.QuotaInPer1k != nil {
		out.qIn = strconv.FormatInt(*row.QuotaInPer1k, 10)
	}
	if row.QuotaOutPer1k != nil {
		out.qOut = strconv.FormatInt(*row.QuotaOutPer1k, 10)
	}
	out.markup = channelPricingMarkupCSVCell(row)
	if row.BillingMode != nil {
		out.billingMode = strings.TrimSpace(*row.BillingMode)
	}
	out.quotaPerRequest = row.QuotaPerRequest
	return out
}

func globalItemsToChannelItems(global []globalPricingUpsertItem, channelID int) []channelPricingUpsertItem {
	out := make([]channelPricingUpsertItem, len(global))
	for i := range global {
		g := global[i]
		var bmPtr *string
		if strings.TrimSpace(g.BillingMode) != "" {
			b := strings.TrimSpace(g.BillingMode)
			bmPtr = &b
		}
		out[i] = channelPricingUpsertItem{
			ChannelID:       channelID,
			ModelName:       g.ModelName,
			Enabled:         g.Enabled,
			QuotaInPer1k:    g.QuotaInPer1k,
			QuotaOutPer1k:   g.QuotaOutPer1k,
			InputRate:       g.InputRate,
			OutputRate:      g.OutputRate,
			MarkupPercent:   g.MarkupPercent,
			BillingMode:     bmPtr,
			QuotaPerRequest: g.QuotaPerRequest,
		}
	}
	return out
}

func parseChannelPricingCSVBytes(ctx context.Context, client *ent.Client, body []byte, cfg *config.Config, fixedChannelID int) ([]channelPricingUpsertItem, string) {
	if fixedChannelID > 0 {
		global, errMsg := parseGlobalPricingCSVBytes(ctx, client, body, cfg)
		if errMsg != "" {
			return nil, errMsg
		}
		return globalItemsToChannelItems(global, fixedChannelID), ""
	}
	return parseChannelPricingBatchCSVBytes(ctx, client, body, cfg)
}

func parseChannelPricingBatchCSVBytes(ctx context.Context, client *ent.Client, body []byte, cfg *config.Config) ([]channelPricingUpsertItem, string) {
	if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
		body = body[3:]
	}
	r := csv.NewReader(bytes.NewReader(body))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	header, err := r.Read()
	if err != nil {
		return nil, "CSV 表头无效"
	}
	col := parseGlobalPricingCSVHeaderIndex(header)
	if _, ok := col["channel_id"]; !ok {
		return nil, "缺少列 channel_id（多渠道导入须含 channel_id）"
	}
	reqCols := []string{"model_name", "enabled", "input_rate", "output_rate", "quota_in_per_1k", "quota_out_per_1k"}
	for _, k := range reqCols {
		if _, ok := col[k]; !ok {
			return nil, "缺少列 " + k + "（请使用「导出」生成的表头；额度列可留空表示清空覆盖）"
		}
	}
	var items []channelPricingUpsertItem
	line := 1
	for {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行解析失败", line+1)
		}
		line++
		if line > channelPricingCSVMaxRows+1 {
			return nil, fmt.Sprintf("超过 %d 行数据上限", channelPricingCSVMaxRows)
		}
		get := func(key string) string {
			i, ok := col[key]
			if !ok || i >= len(rec) {
				return ""
			}
			return rec[i]
		}
		cidStr := strings.TrimSpace(get("channel_id"))
		if cidStr == "" {
			continue
		}
		cid, err := strconv.Atoi(cidStr)
		if err != nil || cid <= 0 {
			return nil, fmt.Sprintf("第 %d 行: channel_id 须为正整数", line)
		}
		name := strings.TrimSpace(get("model_name"))
		if name == "" {
			continue
		}
		en, err := parseGlobalPricingEnabledCell(get("enabled"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行: %v", line, err)
		}
		inS := strings.TrimSpace(get("input_rate"))
		outS := strings.TrimSpace(get("output_rate"))
		inR, err := strconv.ParseFloat(inS, 64)
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 input_rate 无效", line)
		}
		outR, err := strconv.ParseFloat(outS, 64)
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 output_rate 无效", line)
		}
		vIn, err := parseGlobalPricingOptionalInt64Cell(get("quota_in_per_1k"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 quota_in_per_1k: %v", line, err)
		}
		vOut, err := parseGlobalPricingOptionalInt64Cell(get("quota_out_per_1k"))
		if err != nil {
			return nil, fmt.Sprintf("第 %d 行 quota_out_per_1k: %v", line, err)
		}
		qIn, qOut, rowMarkup, upErr := applyUpstreamQuotaDerivationCSV(ctx, client, line, get, cfg, vIn, vOut)
		if upErr != "" {
			return nil, upErr
		}
		var markup *float64
		if _, ok := col["markup_percent"]; ok {
			markup, err = parseGlobalPricingOptionalFloat64Cell(get("markup_percent"))
			if err != nil {
				return nil, fmt.Sprintf("第 %d 行 markup_percent: %v", line, err)
			}
			if markup != nil && !billing.ValidMarkupPercent(*markup) {
				return nil, fmt.Sprintf("第 %d 行: markup_percent 须大于 -100", line)
			}
		}
		if rowMarkup != nil {
			markup = rowMarkup
		}
		bm, qpr, billErr := parseGlobalPricingOptionalBillingCSV(line, col, get)
		if billErr != "" {
			return nil, billErr
		}
		bmPtr := &bm
		items = append(items, channelPricingUpsertItem{
			ChannelID:       cid,
			ModelName:       name,
			Enabled:         en,
			QuotaInPer1k:    qIn,
			QuotaOutPer1k:   qOut,
			InputRate:       inR,
			OutputRate:      outR,
			MarkupPercent:   markup,
			BillingMode:     bmPtr,
			QuotaPerRequest: qpr,
		})
	}
	if len(items) == 0 {
		return nil, "没有有效数据行"
	}
	return items, ""
}

type channelPricingPreviewOut struct {
	ChannelID     int      `json:"channel_id,omitempty"`
	ModelName     string   `json:"model_name"`
	Enabled       bool     `json:"enabled"`
	InputRate     float64  `json:"input_rate"`
	OutputRate    float64  `json:"output_rate"`
	QuotaInPer1k  *int64   `json:"quota_in_per_1k,omitempty"`
	QuotaOutPer1k *int64   `json:"quota_out_per_1k,omitempty"`
	MarkupPercent *float64 `json:"markup_percent,omitempty"`
}

func channelPricingItemsToPreview(items []channelPricingUpsertItem, includeChannelID bool) []channelPricingPreviewOut {
	out := make([]channelPricingPreviewOut, len(items))
	for i := range items {
		it := items[i]
		o := channelPricingPreviewOut{
			ModelName:     it.ModelName,
			Enabled:       it.Enabled,
			InputRate:     it.InputRate,
			OutputRate:    it.OutputRate,
			QuotaInPer1k:  it.QuotaInPer1k,
			QuotaOutPer1k: it.QuotaOutPer1k,
			MarkupPercent: it.MarkupPercent,
		}
		if includeChannelID {
			o.ChannelID = it.ChannelID
		}
		out[i] = o
	}
	return out
}

func importChannelPricingCSVCommon(c *gin.Context, client *ent.Client, cfg *config.Config, fixedChannelID int, auditAction string) {
	adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
	body, errMsg := readGlobalPricingUpload(c)
	if errMsg != "" {
		httpx.BadRequest(c, errMsg)
		return
	}
	ctx := c.Request.Context()
	items, perr := parseChannelPricingCSVBytes(ctx, client, body, cfg, fixedChannelID)
	if perr != "" {
		httpx.BadRequest(c, perr)
		return
	}
	if br, err := upsertChannelModelPricingBatch(ctx, client, items); br != "" {
		httpx.BadRequest(c, br)
		return
	} else if err != nil {
		httpx.ServerError(c, "导入保存失败")
		return
	}
	channelIDs := make(map[int]struct{})
	for _, it := range items {
		channelIDs[it.ChannelID] = struct{}{}
	}
	for cid := range channelIDs {
		billing.InvalidateChannelModelPricingCacheAdmin(cid)
	}
	audit := map[string]any{"count": len(items)}
	if fixedChannelID > 0 {
		audit["channel_id"] = fixedChannelID
	}
	auditLog(client, adm.ID, auditAction, "channel_model_pricing", "", audit, c)
	httpx.OK(c, gin.H{"count": len(items)})
}

func writeChannelWorkspaceDataRow(w *csv.Writer, it availmodels.ChannelPricingWorkspaceItem, withUpstream bool, factors billing.SitePricingFactors, currency string, denom int64) {
	ch := it.Channel
	f := channelPricingCSVFieldsFromRow(ch)
	if withUpstream {
		denomStr := strconv.FormatInt(denom, 10)
		pIn := upstreamPriceCellFromQuota(factors, currency, denom, f.qInPtr)
		pOut := upstreamPriceCellFromQuota(factors, currency, denom, f.qOutPtr)
		if pOut == "" && pIn != "" {
			pOut = pIn
		}
		row := appendGlobalPricingImportTemplateExtras([]string{
			it.ModelName,
			strconv.FormatBool(f.enabled),
			f.qIn,
			f.qOut,
			strconv.FormatFloat(f.inR, 'f', -1, 64),
			strconv.FormatFloat(f.outR, 'f', -1, 64),
			currency,
			denomStr,
			pIn,
			pOut,
			f.markup,
		}, it.ModelName, f.billingMode, f.quotaPerRequest)
		_ = w.Write(row)
	} else {
		_ = w.Write([]string{
			it.ModelName,
			strconv.FormatBool(f.enabled),
			f.qIn,
			f.qOut,
			strconv.FormatFloat(f.inR, 'f', -1, 64),
			strconv.FormatFloat(f.outR, 'f', -1, 64),
			f.markup,
		})
	}
}

func writeChannelBatchDataRow(w *csv.Writer, channelID int, channelName, channelType string, it availmodels.ChannelPricingWorkspaceItem, withUpstream bool, factors billing.SitePricingFactors, currency string, denom int64) {
	ch := it.Channel
	f := channelPricingCSVFieldsFromRow(ch)
	chName := strings.TrimSpace(channelName)
	ct := strings.TrimSpace(channelType)
	if withUpstream {
		denomStr := strconv.FormatInt(denom, 10)
		pIn := upstreamPriceCellFromQuota(factors, currency, denom, f.qInPtr)
		pOut := upstreamPriceCellFromQuota(factors, currency, denom, f.qOutPtr)
		if pOut == "" && pIn != "" {
			pOut = pIn
		}
		row := append([]string{
			strconv.Itoa(channelID),
			chName,
			ct,
		}, appendGlobalPricingImportTemplateExtras([]string{
			it.ModelName,
			strconv.FormatBool(f.enabled),
			f.qIn,
			f.qOut,
			strconv.FormatFloat(f.inR, 'f', -1, 64),
			strconv.FormatFloat(f.outR, 'f', -1, 64),
			currency,
			denomStr,
			pIn,
			pOut,
			f.markup,
		}, it.ModelName, f.billingMode, f.quotaPerRequest)...)
		_ = w.Write(row)
	} else {
		_ = w.Write([]string{
			strconv.Itoa(channelID),
			chName,
			it.ModelName,
			strconv.FormatBool(f.enabled),
			f.qIn,
			f.qOut,
			strconv.FormatFloat(f.inR, 'f', -1, 64),
			strconv.FormatFloat(f.outR, 'f', -1, 64),
			f.markup,
		})
	}
}

func loadAllChannelsForBatchExport(ctx context.Context, client *ent.Client) ([]*ent.Channel, string) {
	list, err := client.Channel.Query().Order(ent.Asc(channel.FieldID)).All(ctx)
	if err != nil {
		return nil, "导出失败"
	}
	return list, ""
}

func flattenChannelBatchWorkspace(ctx context.Context, client *ent.Client, channels []*ent.Channel) ([]struct {
	ch   *ent.Channel
	item availmodels.ChannelPricingWorkspaceItem
}, int, string) {
	var rows []struct {
		ch   *ent.Channel
		item availmodels.ChannelPricingWorkspaceItem
	}
	total := 0
	for _, ch := range channels {
		items, err := availmodels.AllChannelPricingWorkspaceItems(ctx, client, ch.ID)
		if err != nil {
			if ent.IsNotFound(err) {
				continue
			}
			return nil, 0, "导出失败"
		}
		for _, it := range items {
			total++
			if total > channelPricingCSVMaxRows {
				return nil, 0, fmt.Sprintf("导出行数超过 %d 上限，请分渠道导出或缩小范围", channelPricingCSVMaxRows)
			}
			rows = append(rows, struct {
				ch   *ent.Channel
				item availmodels.ChannelPricingWorkspaceItem
			}{ch: ch, item: it})
		}
	}
	return rows, total, ""
}

// ExportChannelModelPricingCSV GET /api/admin/channel-model-pricing/export?channel_id=
func ExportChannelModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID, errMsg := parseChannelIDQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		currency, denom, qerr := parseGlobalPricingExportQuery(c)
		if qerr != "" {
			httpx.BadRequest(c, qerr)
			return
		}
		withUpstream := currency != ""
		ctx := c.Request.Context()
		if _, err := client.Channel.Get(ctx, channelID); err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "渠道不存在")
				return
			}
			httpx.ServerError(c, "导出失败")
			return
		}
		list, err := availmodels.AllChannelPricingWorkspaceItems(ctx, client, channelID)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		ts := time.Now().Format("20060102-150405")
		var fn string
		if withUpstream {
			fn = fmt.Sprintf("channel-%d-model-pricing-data-%s-%s-%s.csv",
				channelID, currency, globalPricingDenomFilenameSuffix(denom), ts)
		} else {
			fn = fmt.Sprintf("channel-%d-model-pricing-data-%s.csv", channelID, ts)
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		var factors billing.SitePricingFactors
		if withUpstream && cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		if withUpstream {
			_ = w.Write(globalPricingImportTemplateHeader)
		} else {
			_ = w.Write(globalPricingDataExportHeader)
		}
		for _, it := range list {
			writeChannelWorkspaceDataRow(w, it, withUpstream, factors, currency, denom)
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		audit := map[string]any{"rows": len(list), "channel_id": channelID}
		if withUpstream {
			audit["currency"] = currency
			audit["denom"] = denom
		}
		auditLog(client, adm.ID, "channel_model_pricing.export_csv", "channel_model_pricing", strconv.Itoa(channelID), audit, c)
	}
}

// ExportChannelModelPricingImportTemplate GET /api/admin/channel-model-pricing/export/template?channel_id=
func ExportChannelModelPricingImportTemplate(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID, errMsg := parseChannelIDQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		currency, denom, qerr := parseGlobalPricingImportTemplateQuery(c)
		if qerr != "" {
			httpx.BadRequest(c, qerr)
			return
		}
		ctx := c.Request.Context()
		if _, err := client.Channel.Get(ctx, channelID); err != nil {
			if ent.IsNotFound(err) {
				httpx.BadRequest(c, "渠道不存在")
				return
			}
			httpx.ServerError(c, "导出失败")
			return
		}
		list, err := availmodels.AllChannelPricingWorkspaceItems(ctx, client, channelID)
		if err != nil {
			httpx.ServerError(c, "导出失败")
			return
		}
		var factors billing.SitePricingFactors
		if cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		denomStr := strconv.FormatInt(denom, 10)
		fn := fmt.Sprintf("channel-%d-model-pricing-import-%s-%s-%s.csv",
			channelID, currency, globalPricingDenomFilenameSuffix(denom), time.Now().Format("20060102-150405"))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		_ = w.Write(globalPricingImportTemplateHeader)
		for _, it := range list {
			ch := it.Channel
			f := channelPricingCSVFieldsFromRow(ch)
			pIn := upstreamPriceCellFromQuota(factors, currency, denom, f.qInPtr)
			pOut := upstreamPriceCellFromQuota(factors, currency, denom, f.qOutPtr)
			if pOut == "" && pIn != "" {
				pOut = pIn
			}
			row := appendGlobalPricingImportTemplateExtras([]string{
				it.ModelName,
				strconv.FormatBool(f.enabled),
				"", "",
				strconv.FormatFloat(f.inR, 'f', -1, 64),
				strconv.FormatFloat(f.outR, 'f', -1, 64),
				currency,
				denomStr,
				pIn,
				pOut,
				f.markup,
			}, it.ModelName, f.billingMode, f.quotaPerRequest)
			_ = w.Write(row)
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		auditLog(client, adm.ID, "channel_model_pricing.export_import_template", "channel_model_pricing", strconv.Itoa(channelID), map[string]any{
			"rows": len(list), "currency": currency, "denom": denom, "channel_id": channelID,
		}, c)
	}
}

// PreviewChannelModelPricingCSV POST /api/admin/channel-model-pricing/import/preview?channel_id=
func PreviewChannelModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID, errMsg := parseChannelIDQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		body, uploadErr := readGlobalPricingUpload(c)
		if uploadErr != "" {
			httpx.BadRequest(c, uploadErr)
			return
		}
		items, perr := parseChannelPricingCSVBytes(c.Request.Context(), client, body, cfg, channelID)
		if perr != "" {
			httpx.BadRequest(c, perr)
			return
		}
		httpx.OK(c, gin.H{"count": len(items), "items": channelPricingItemsToPreview(items, false)})
	}
}

// ImportChannelModelPricingCSV POST /api/admin/channel-model-pricing/import?channel_id=
func ImportChannelModelPricingCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelID, errMsg := parseChannelIDQuery(c)
		if errMsg != "" {
			httpx.BadRequest(c, errMsg)
			return
		}
		importChannelPricingCSVCommon(c, client, cfg, channelID, "channel_model_pricing.import_csv")
	}
}

// ExportChannelModelPricingBatchCSV GET /api/admin/channel-model-pricing/export/batch
func ExportChannelModelPricingBatchCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currency, denom, qerr := parseGlobalPricingExportQuery(c)
		if qerr != "" {
			httpx.BadRequest(c, qerr)
			return
		}
		withUpstream := currency != ""
		ctx := c.Request.Context()
		channels, errMsg := loadAllChannelsForBatchExport(ctx, client)
		if errMsg != "" {
			httpx.ServerError(c, errMsg)
			return
		}
		flat, _, flatErr := flattenChannelBatchWorkspace(ctx, client, channels)
		if flatErr != "" {
			httpx.BadRequest(c, flatErr)
			return
		}
		ts := time.Now().Format("20060102-150405")
		var fn string
		if withUpstream {
			fn = fmt.Sprintf("channel-model-pricing-all-%s-%s-%s.csv",
				currency, globalPricingDenomFilenameSuffix(denom), ts)
		} else {
			fn = fmt.Sprintf("channel-model-pricing-all-%s.csv", ts)
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		var factors billing.SitePricingFactors
		if withUpstream && cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		if withUpstream {
			_ = w.Write(append([]string{"channel_id", "channel_name", "channel_type"}, globalPricingImportTemplateHeader...))
		} else {
			_ = w.Write(channelPricingBatchDataExportHeader)
		}
		for _, row := range flat {
			writeChannelBatchDataRow(w, row.ch.ID, row.ch.ChannelName, row.ch.ChannelType, row.item, withUpstream, factors, currency, denom)
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		audit := map[string]any{"rows": len(flat)}
		if withUpstream {
			audit["currency"] = currency
			audit["denom"] = denom
		}
		auditLog(client, adm.ID, "channel_model_pricing.export_csv_batch", "channel_model_pricing", "", audit, c)
	}
}

// ExportChannelModelPricingBatchImportTemplate GET /api/admin/channel-model-pricing/export/template/batch
func ExportChannelModelPricingBatchImportTemplate(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		currency, denom, qerr := parseGlobalPricingImportTemplateQuery(c)
		if qerr != "" {
			httpx.BadRequest(c, qerr)
			return
		}
		ctx := c.Request.Context()
		channels, errMsg := loadAllChannelsForBatchExport(ctx, client)
		if errMsg != "" {
			httpx.ServerError(c, errMsg)
			return
		}
		flat, _, flatErr := flattenChannelBatchWorkspace(ctx, client, channels)
		if flatErr != "" {
			httpx.BadRequest(c, flatErr)
			return
		}
		var factors billing.SitePricingFactors
		if cfg != nil {
			factors = adminSitePricingFactors(ctx, client, cfg)
		}
		denomStr := strconv.FormatInt(denom, 10)
		fn := fmt.Sprintf("channel-model-pricing-import-all-%s-%s-%s.csv",
			currency, globalPricingDenomFilenameSuffix(denom), time.Now().Format("20060102-150405"))
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+fn)
		writeGlobalPricingCSVBOM(c.Writer)
		w := csv.NewWriter(c.Writer)
		_ = w.Write(channelPricingBatchImportTemplateHeader)
		for _, row := range flat {
			ch := row.item.Channel
			f := channelPricingCSVFieldsFromRow(ch)
			pIn := upstreamPriceCellFromQuota(factors, currency, denom, f.qInPtr)
			pOut := upstreamPriceCellFromQuota(factors, currency, denom, f.qOutPtr)
			if pOut == "" && pIn != "" {
				pOut = pIn
			}
			chName := strings.TrimSpace(row.ch.ChannelName)
			cells := appendGlobalPricingImportTemplateExtras([]string{
				row.item.ModelName,
				strconv.FormatBool(f.enabled),
				"", "",
				strconv.FormatFloat(f.inR, 'f', -1, 64),
				strconv.FormatFloat(f.outR, 'f', -1, 64),
				currency,
				denomStr,
				pIn,
				pOut,
				f.markup,
			}, row.item.ModelName, f.billingMode, f.quotaPerRequest)
			_ = w.Write(append([]string{
				strconv.Itoa(row.ch.ID),
				chName,
				strings.TrimSpace(row.ch.ChannelType),
			}, cells...))
		}
		w.Flush()
		adm := c.MustGet(middleware.CtxUserKey).(*ent.User)
		auditLog(client, adm.ID, "channel_model_pricing.export_import_template_batch", "channel_model_pricing", "", map[string]any{
			"rows": len(flat), "currency": currency, "denom": denom,
		}, c)
	}
}

// PreviewChannelModelPricingBatchCSV POST /api/admin/channel-model-pricing/import/preview/batch
func PreviewChannelModelPricingBatchCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, uploadErr := readGlobalPricingUpload(c)
		if uploadErr != "" {
			httpx.BadRequest(c, uploadErr)
			return
		}
		items, perr := parseChannelPricingCSVBytes(c.Request.Context(), client, body, cfg, 0)
		if perr != "" {
			httpx.BadRequest(c, perr)
			return
		}
		httpx.OK(c, gin.H{"count": len(items), "items": channelPricingItemsToPreview(items, true)})
	}
}

// ImportChannelModelPricingBatchCSV POST /api/admin/channel-model-pricing/import/batch
func ImportChannelModelPricingBatchCSV(client *ent.Client, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		importChannelPricingCSVCommon(c, client, cfg, 0, "channel_model_pricing.import_csv_batch")
	}
}
