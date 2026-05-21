package gateway

import (
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/klingvideo"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

const maxKlingMultipartMemory = 64 << 20

// KlingNativeSettings 可灵原生 API 透传（/v1/kling/*）。
type KlingNativeSettings struct {
	Enabled          bool
	Allowlist        klingvideo.AllowlistConfig
	PatchModelInJSON bool
}

// KlingV1Passthrough 将 /v1/kling/{path} 转发至可灵官方 API（仅 kling 渠道）。
func KlingV1Passthrough(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := d.KlingNative
		if !cfg.Enabled {
			middleware.WriteOpenAIError(c, http.StatusNotFound, "invalid_request_error", "not_found")
			return
		}

		var model string
		klingPath := strings.Trim(strings.TrimSpace(c.Param("klingPath")), "/")
		if !klingvideo.PassthroughAllowed(klingPath, cfg.Allowlist) {
			middleware.WriteOpenAIError(c, http.StatusNotFound, "invalid_request_error", "not_found")
			return
		}

		method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
		var body []byte
		contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
		isMultipart := strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data")

		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			if isMultipart {
				if err := c.Request.ParseMultipartForm(maxKlingMultipartMemory); err != nil {
					middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
					return
				}
				model = extractKlingModelFromMultipart(c.Request.MultipartForm)
			} else {
				var err error
				body, err = io.ReadAll(c.Request.Body)
				if err != nil {
					middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
					return
				}
			}
		}

		if model == "" {
			model = klingvideo.ExtractModelField(body)
		}
		if model == "" {
			model = routingModelForStatelessResource(c)
		}
		if model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请在 JSON 中提供 model_name/model，或使用 Query model=… / 请求头 X-TraceHub-Model")
			return
		}

		uAny, _ := c.Get(middleware.CtxGateUserKey)
		kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
		u := uAny.(*ent.User)
		k := kAny.(*ent.APIKey)

		fresh, err := d.Client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(c.Request.Context())
		if err != nil {
			middleware.WriteOpenAIError(c, 500, "api_error", "用户状态异常")
			return
		}
		u = fresh

		sticky := stickySessionHeader(c)
		gid := u.GroupID
		if gid <= 0 {
			middleware.WriteOpenAIError(c, 403, "permission_error", "账户未分配分组，请联系管理员")
			return
		}

		sel, err := router.SelectKlingChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
		if err != nil {
			middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
			return
		}
		ch := sel.Channel
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))

		var costPre int64
		var vParams billing.VideoJobParams
		if klingvideo.IsBillableCreate(method, klingPath) {
			sec, size := klingvideo.JobParamsFromBody(body)
			vParams = billing.VideoJobParams{Seconds: sec, Size: size}
			br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
			costPre = br.VideoJobCost(model, vParams)
			if u.Balance < costPre {
				WriteInsufficientBalance402Precheck(c, u.Balance, costPre, vParams.Seconds, 0)
				return
			}
		}

		secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, ch)
		if !ok {
			WriteGatewayChannelConfigError(c)
			return
		}
		if strings.TrimSpace(baseURL) == "" {
			baseURL = router.DefaultBaseURL(ch.ChannelType)
		}

		upBody := body
		if cfg.PatchModelInJSON && len(body) > 0 && json.Valid(body) {
			patched, perr := router.PatchKlingModelInBody(body, sel.UpstreamModel)
			if perr != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
				return
			}
			upBody = patched
		}

		upstreamRel := klingvideo.UpstreamRelativePath(klingPath)
		rawQuery := c.Request.URL.RawQuery
		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		var resp *http.Response
		var ferr error
		if isMultipart && c.Request.MultipartForm != nil {
			mpBody, mpCT, mperr := rebuildMultipartWithUpstreamModel(c.Request.MultipartForm, sel.UpstreamModel)
			if mperr != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
				return
			}
			resp, ferr = relay.ForwardKlingNativeMultipart(c.Request.Context(), baseURL, secret, method, upstreamRel, mpCT, mpBody, openAIPassthroughHeaders(c))
		} else {
			resp, ferr = relay.ForwardKlingNativeHTTP(c.Request.Context(), baseURL, secret, method, upstreamRel, upBody, rawQuery, openAIPassthroughHeaders(c))
		}
		if ferr != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			WriteGatewayTransportError(c, ferr)
			return
		}
		defer relay.DrainBody(resp.Body)
		raw, _ := io.ReadAll(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		outBody := raw
		if klingvideo.IsTaskQuery(method, klingPath) || (resp.StatusCode < 400 && json.Valid(raw)) {
			outBody = klingvideo.NormalizeResponseBody(raw)
		}

		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			oct := resp.Header.Get("Content-Type")
			if oct == "" {
				oct = "application/json"
			}
			WriteClientProxy(c, resp.StatusCode, oct, outBody)
			return
		}

		if costPre > 0 {
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), costPre, false); err != nil {
				return
			}
		}

		vid := parseVideoIDFromJSON(outBody)
		if vid != "" && costPre > 0 {
			storeVideoJobBillingMeta(c.Request.Context(), d.RDB, int64(u.ID), vid, videoJobMeta{
				Model:     model,
				Seconds:   vParams.Seconds,
				Size:      vParams.Size,
				Cost:      costPre,
				ChannelID: ch.ID,
				KlingKind: string(klingvideo.CreateKindFromRest(klingPath)),
				KlingPath: klingPath,
			})
		}

		var settle videoSettleOutcome
		if klingvideo.IsTaskQuery(method, klingPath) && vid != "" {
			settle = trySettleVideoOnComplete(c.Request.Context(), d.Billing, d.RDB, u, model, vid, outBody)
		}

		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		if settle.Settled && settle.Adjust != 0 {
			adjCost := settle.Adjust
			if adjCost < 0 {
				adjCost = -adjCost
			}
			tag := "kling_settle_extra"
			if settle.Adjust < 0 {
				tag = "kling_settle_refund"
			}
			em := tag + ":" + vid
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, settle.Seconds, 0, 0, adjCost, c.ClientIP(), resp.StatusCode, &em, time.Since(start))
		} else {
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, vParams.Seconds, 0, 0, costPre, c.ClientIP(), resp.StatusCode, nil, time.Since(start))
		}

		oct := resp.Header.Get("Content-Type")
		if oct == "" {
			oct = "application/json"
		}
		WriteClientProxy(c, resp.StatusCode, oct, outBody)
	}
}

func extractKlingModelFromMultipart(form *multipart.Form) string {
	if form == nil || form.Value == nil {
		return ""
	}
	for _, key := range []string{"model_name", "model"} {
		vals := form.Value[key]
		if len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			return strings.TrimSpace(vals[0])
		}
	}
	return ""
}
