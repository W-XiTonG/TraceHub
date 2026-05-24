package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/pkg/klingvideo"
	"tracehub/internal/pkg/volcvideo"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/service/upstreamprobe"

	"github.com/gin-gonic/gin"
)

// VideosCreate POST /v1/videos
func VideosCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctHdr := strings.ToLower(c.GetHeader("Content-Type"))
		if strings.HasPrefix(ctHdr, "multipart/form-data") {
			videoCreateMultipart(c, d)
			return
		}
		videoCreateJSON(c, d)
	}
}

func videoCreateJSON(c *gin.Context, d ChatDeps) {
	uAny, _ := c.Get(middleware.CtxGateUserKey)
	kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
	u := uAny.(*ent.User)
	k := kAny.(*ent.APIKey)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
		return
	}
	parsed, err := parseVideoCreateBody(body)
	if err != nil || !videoCreateBodyValid(parsed, body) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model，或需提供 prompt 或 content 数组")
		return
	}
	runVideoCreate(c, d, u, k, parsed.Model, body, parsed, false)
}

func videoCreateMultipart(c *gin.Context, d ChatDeps) {
	uAny, _ := c.Get(middleware.CtxGateUserKey)
	kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
	u := uAny.(*ent.User)
	k := kAny.(*ent.APIKey)

	if err := c.Request.ParseMultipartForm(maxVideoMultipartMemory); err != nil {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
		return
	}
	form := c.Request.MultipartForm
	defer func() {
		if form != nil {
			_ = form.RemoveAll()
		}
	}()
	parsed := parseVideoCreateMultipart(form)
	if parsed.Model == "" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model")
		return
	}

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
	sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, parsed.Model, sticky)
	if err != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
		return
	}
	ch := sel.Channel
	ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
	if !supportsVideoChannel(ct, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
		return
	}

	vParams := videoJobParamsFromParsed(parsed)
	br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, parsed.Model, ch.ID)
	costPre := br.VideoJobCost(parsed.Model, vParams)
	if u.Balance < costPre {
		WriteInsufficientBalance402PrecheckVideo(c, u.Balance, costPre, vParams.Seconds)
		return
	}

	hasInputRef := false
	if form.File != nil && len(form.File["input_reference"]) > 0 {
		hasInputRef = true
	}
	secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, ch)
	if !ok {
		WriteGatewayChannelConfigError(c)
		return
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()

	var resp *http.Response
	var klingCreateKind klingvideo.CreateKind
	if isKlingVideoChannel(ct) {
		oaiBody := map[string]any{
			"model":   sel.UpstreamModel,
			"prompt":  parsed.PromptText,
			"seconds": parsed.Seconds,
			"size":    parsed.Size,
		}
		if hasInputRef {
			imgBytes, mime, got, rerr := readMultipartInputReference(form)
			if rerr != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取 input_reference 文件")
				return
			}
			if !got || len(imgBytes) == 0 {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "图生视频需要参考图（input_reference）")
				return
			}
			oaiBody["image"] = klingvideo.ImageDataFromBytes(mime, imgBytes)
		}
		oai, _ := json.Marshal(oaiBody)
		var kerr error
		resp, klingCreateKind, kerr = relay.ForwardKlingVideoCreate(c.Request.Context(), baseURL, secret, oai)
		err = kerr
	} else if isVolcengineVideoChannel(ct) {
		oaiBody := map[string]any{
			"model":   sel.UpstreamModel,
			"prompt":  parsed.PromptText,
			"seconds": parsed.Seconds,
			"size":    parsed.Size,
		}
		if hasInputRef {
			imgBytes, mime, got, rerr := readMultipartInputReference(form)
			if rerr != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取 input_reference 文件")
				return
			}
			if !got || len(imgBytes) == 0 {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "图生视频需要参考图（input_reference）")
				return
			}
			dataURL := volcvideo.ImageDataFromBytes(mime, imgBytes)
			if volcvideo.IsVolc3DGenModel(sel.UpstreamModel) {
				oaiBody["image_data"] = dataURL
			} else {
				oaiBody["image_url"] = dataURL
			}
		}
		oai, _ := json.Marshal(oaiBody)
		arkBody, aerr := volcvideo.PrepareVolcVideoCreateBody(oai)
		if aerr != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "视频请求参数无效，请检查 model 与 content")
			return
		}
		resp, err = relay.ForwardVolcVideoCreate(c.Request.Context(), baseURL, secret, arkBody)
	} else {
		mpBody, contentType, mperr := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
		if mperr != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
			return
		}
		resp, err = relay.ForwardOpenAIVideoMultipart(c.Request.Context(), baseURL, secret, contentType, mpBody, openAIPassthroughHeaders(c))
	}
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(parsed.Model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	defer relay.DrainBody(resp.Body)
	raw, _ := io.ReadAll(resp.Body)
	raw = normalizeVideoUpstreamBody(ct, raw)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if resp.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(parsed.Model, ct, http.StatusText(resp.StatusCode)).Inc()
		WriteClientProxy(c, resp.StatusCode, "application/json", raw)
		return
	}
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), costPre, false); err != nil {
		return
	}
	vid := parseVideoIDFromJSON(raw)
	if vid != "" {
		meta := videoJobMeta{
			Model: parsed.Model, Seconds: vParams.Seconds, Size: vParams.Size, Cost: costPre, ChannelID: ch.ID,
		}
		if isKlingVideoChannel(ct) && klingCreateKind != "" {
			meta.KlingKind = string(klingCreateKind)
		}
		storeVideoJobBillingMeta(c.Request.Context(), d.RDB, int64(u.ID), vid, meta)
	}
	metrics.GatewayRequests.WithLabelValues(parsed.Model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, parsed.Model, sel.UpstreamModel, vParams.Seconds, 0, 0, costPre, c.ClientIP(), resp.StatusCode, videoJobTraceRef(vid), time.Since(start))
	WriteClientProxy(c, resp.StatusCode, "application/json", raw)
}

func runVideoCreate(c *gin.Context, d ChatDeps, u *ent.User, k *ent.APIKey, model string, body []byte, parsed videoCreateParsed, _ bool) {
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
	sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
	if err != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
		return
	}
	ch := sel.Channel
	ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
	if !supportsVideoChannel(ct, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
		return
	}

	vParams := videoJobParamsFromParsed(parsed)
	br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
	costPre := br.VideoJobCost(model, vParams)
	if u.Balance < costPre {
		WriteInsufficientBalance402PrecheckVideo(c, u.Balance, costPre, vParams.Seconds)
		return
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
	volc := isVolcengineVideoChannel(ct)
	kling := isKlingVideoChannel(ct)
	var klingCreateKind klingvideo.CreateKind

	sel, res, err := ExecuteModelForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), sel, body,
		func(b []byte, upstream string) ([]byte, error) {
			return patchVideoCreateBody(b, upstream, volc)
		}, nil,
		func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
			if !ok {
				return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
			}
			ctSel := strings.ToLower(strings.TrimSpace(sel.Channel.ChannelType))
			var resp *http.Response
			var ferr error
			if isKlingVideoChannel(ctSel) {
				resp, klingCreateKind, ferr = relay.ForwardKlingVideoCreate(ctx, baseURL, secret, patched)
			} else if isVolcengineVideoChannel(ctSel) {
				resp, ferr = relay.ForwardVolcVideoCreate(ctx, baseURL, secret, patched)
			} else {
				resp, ferr = relay.ForwardOpenAIVideoCreate(ctx, baseURL, secret, patched)
			}
			return HTTPResponseToUpstreamResult(resp, ferr)
		})
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	ch = sel.Channel
	ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	outBody := normalizeVideoUpstreamBody(ct, res.Body)
	if res.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetModel(model)
			rec.SetChannel(ch.ID, sel.UpstreamModel)
		}
		upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityVideo, res.StatusCode, res.Body, upstreamprobe.CapSourceRuntime)
		em := http.StatusText(res.StatusCode)
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, vParams.Seconds, 0, 0, 0, c.ClientIP(), res.StatusCode, &em, time.Since(start))
		WriteClientProxy(c, res.StatusCode, "application/json", outBody)
		return
	}
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), costPre, false); err != nil {
		em := err.Error()
		metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, vParams.Seconds, 0, 0, costPre, c.ClientIP(), res.StatusCode, &em, time.Since(start))
		return
	}
	vid := parseVideoIDFromJSON(outBody)
	if vid != "" {
		meta := videoJobMeta{
			Model: model, Seconds: vParams.Seconds, Size: vParams.Size, Cost: costPre, ChannelID: ch.ID,
		}
		if kling && klingCreateKind != "" {
			meta.KlingKind = string(klingCreateKind)
		}
		storeVideoJobBillingMeta(c.Request.Context(), d.RDB, int64(u.ID), vid, meta)
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityVideo, res.StatusCode, nil, upstreamprobe.CapSourceRuntime)
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, vParams.Seconds, 0, 0, costPre, c.ClientIP(), res.StatusCode, videoJobTraceRef(vid), time.Since(start))
	WriteClientProxy(c, res.StatusCode, "application/json", outBody)
}

// VideosRetrieve GET /v1/videos/:video_id
func VideosRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		videosResourceHTTP(c, d, http.MethodGet, "")
	}
}

// VideosDelete DELETE /v1/videos/:video_id
func VideosDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		videosResourceHTTP(c, d, http.MethodDelete, "")
	}
}

// VideosList GET /v1/videos
func VideosList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		videosResourceHTTP(c, d, http.MethodGet, "list")
	}
}

// VideosContent GET /v1/videos/:video_id/content
func VideosContent(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := normalizeVideoIDParam(c)
		if !ok {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid video id")
			return
		}
		model := routingModelForStatelessResource(c)
		if model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请使用 Query 参数 model=…，或设置请求头 X-TraceHub-Model")
			return
		}
		u, _, gid, sticky, previewSel, err := loadVideoGatewayUser(c, d, model)
		if err != nil {
			return
		}
		ctPreview := strings.ToLower(strings.TrimSpace(previewSel.Channel.ChannelType))
		if !supportsVideoChannel(ctPreview, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持视频接口")
			return
		}

		start := time.Now()
		routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

		sel, res, err := ExecuteModelHTTPForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, previewSel,
			func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
				secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
				if !ok {
					return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
				}
				var resp *http.Response
				var ferr error
				ctSel := strings.ToLower(strings.TrimSpace(sel.Channel.ChannelType))
				if isKlingVideoChannel(ctSel) {
					resp, ferr = relay.ForwardKlingVideoContent(ctx, baseURL, secret, id, klingKindForVideo(c.Request.Context(), d.RDB, int64(u.ID), id), klingPathForVideo(c.Request.Context(), d.RDB, int64(u.ID), id), openAIPassthroughHeaders(c))
				} else if isVolcengineVideoChannel(ctSel) {
					resp, ferr = relay.ForwardVolcVideoContent(ctx, baseURL, secret, id, openAIPassthroughHeaders(c))
				} else {
					resp, ferr = relay.ForwardOpenAIVideoContent(ctx, baseURL, secret, id, openAIPassthroughHeaders(c))
				}
				return HTTPResponseToUpstreamResult(resp, ferr)
			})
		if err != nil {
			WriteGatewayTransportError(c, err)
			return
		}
		ct := strings.ToLower(strings.TrimSpace(sel.Channel.ChannelType))
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
		if res.StatusCode >= 400 {
			outCT := "application/json"
			if res.Headers != nil {
				if h := res.Headers.Get("Content-Type"); h != "" {
					outCT = h
				}
			}
			WriteClientProxy(c, res.StatusCode, outCT, res.Body)
			return
		}
		outCT := "video/mp4"
		if res.Headers != nil {
			if h := res.Headers.Get("Content-Type"); h != "" {
				outCT = h
			}
			if cd := res.Headers.Get("Content-Disposition"); cd != "" {
				c.Header("Content-Disposition", cd)
			}
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		WriteClientProxy(c, res.StatusCode, outCT, res.Body)
	}
}

func videosResourceHTTP(c *gin.Context, d ChatDeps, method, mode string) {
	var id string
	var rel string
	if mode == "list" {
		rel = "videos"
	} else {
		var ok bool
		id, ok = normalizeVideoIDParam(c)
		if !ok {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid video id")
			return
		}
		rel = "videos/" + id
	}
	model := routingModelForStatelessResource(c)
	if model == "" && mode != "list" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请使用 Query 参数 model=…，或设置请求头 X-TraceHub-Model")
		return
	}
	if mode == "list" {
		model = strings.TrimSpace(c.Query("model"))
		if model == "" {
			model = strings.TrimSpace(c.GetHeader("X-TraceHub-Model"))
		}
		if model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "列表接口需要 model 选路参数")
			return
		}
	}

	u, k, gid, sticky, previewSel, err := loadVideoGatewayUser(c, d, model)
	if err != nil {
		return
	}
	ctPreview := strings.ToLower(strings.TrimSpace(previewSel.Channel.ChannelType))
	if !supportsVideoChannel(ctPreview, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持视频接口")
		return
	}
	if isKlingVideoChannel(ctPreview) && mode == "list" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "可灵渠道不支持视频列表接口")
		return
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
	rawQuery := c.Request.URL.RawQuery

	sel, res, err := ExecuteModelHTTPForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, previewSel,
		func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
			if !ok {
				return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
			}
			var resp *http.Response
			var ferr error
			ctSel := strings.ToLower(strings.TrimSpace(sel.Channel.ChannelType))
			if isKlingVideoChannel(ctSel) {
				resp, ferr = relay.ForwardKlingVideoTaskHTTP(ctx, baseURL, secret, method, id, klingKindForVideo(ctx, d.RDB, int64(u.ID), id), klingPathForVideo(ctx, d.RDB, int64(u.ID), id), rawQuery, openAIPassthroughHeaders(c))
			} else if isVolcengineVideoChannel(ctSel) {
				taskID := id
				if mode == "list" {
					taskID = ""
				}
				resp, ferr = relay.ForwardVolcVideoTaskHTTP(ctx, baseURL, secret, method, taskID, rawQuery, openAIPassthroughHeaders(c))
			} else {
				resp, ferr = relay.ForwardOpenAIVideoHTTP(ctx, baseURL, secret, method, rel, nil, rawQuery, openAIPassthroughHeaders(c))
			}
			return HTTPResponseToUpstreamResult(resp, ferr)
		})
	if err != nil {
		WriteGatewayTransportError(c, err)
		return
	}
	ct := strings.ToLower(strings.TrimSpace(sel.Channel.ChannelType))
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	outBody := normalizeVideoUpstreamBody(ct, res.Body)
	if res.StatusCode >= 400 {
		if id != "" {
			if rec := UsageRecorderFrom(c); rec != nil {
				if ref := videoJobTraceRef(id); ref != nil {
					rec.SetTraceRef(*ref)
				}
			}
		}
		WriteClientProxy(c, res.StatusCode, "application/json", outBody)
		return
	}
	var settle videoSettleOutcome
	if mode != "list" && id != "" && method == http.MethodGet {
		settle = trySettleVideoOnComplete(c.Request.Context(), d.Billing, d.RDB, u, model, id, outBody)
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	if settle.Settled && settle.Adjust != 0 {
		adjCost := settle.Adjust
		if adjCost < 0 {
			adjCost = -adjCost
		}
		tag := "video_settle_extra"
		if settle.Adjust < 0 {
			tag = "video_settle_refund"
		}
		em := tag + ":" + id
		RecordUsageOK(c, d.Client, u.ID, k.ID, sel.Channel.ID, rid, model, sel.UpstreamModel, settle.Seconds, 0, 0, adjCost, c.ClientIP(), res.StatusCode, &em, time.Since(start))
	}
	WriteClientProxy(c, res.StatusCode, "application/json", outBody)
}

func normalizeVideoIDParam(c *gin.Context) (id string, ok bool) {
	id = strings.TrimSpace(c.Param("video_id"))
	if id == "" || strings.ContainsAny(id, "/\\") {
		return "", false
	}
	return id, true
}

func stickySessionHeader(c *gin.Context) string {
	sticky := c.GetHeader("X-TraceHub-Session")
	if sticky == "" {
		sticky = c.GetHeader("X-Sticky-Session")
	}
	return sticky
}

func noteGatewayModelForUsage(c *gin.Context, model string) {
	if rec := UsageRecorderFrom(c); rec != nil {
		if m := strings.TrimSpace(model); m != "" {
			rec.SetModel(m)
		}
	}
}

// videoJobTraceRef 写入 usage_logs.error_message，供控制台按 video_id 聚合（非用户可见错误）。
func videoJobTraceRef(videoID string) *string {
	id := strings.TrimSpace(videoID)
	if id == "" {
		return nil
	}
	s := "video_job:" + id
	return &s
}

func loadVideoGatewayUser(c *gin.Context, d ChatDeps, model string) (*ent.User, *ent.APIKey, int, string, *router.Selected, error) {
	uAny, _ := c.Get(middleware.CtxGateUserKey)
	kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
	u := uAny.(*ent.User)
	k := kAny.(*ent.APIKey)
	model = strings.TrimSpace(model)
	noteGatewayModelForUsage(c, model)

	fresh, err := d.Client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(c.Request.Context())
	if err != nil {
		middleware.WriteOpenAIError(c, 500, "api_error", "用户状态异常")
		return nil, nil, 0, "", nil, err
	}
	u = fresh
	sticky := stickySessionHeader(c)
	gid := u.GroupID
	if gid <= 0 {
		middleware.WriteOpenAIError(c, 403, "permission_error", "账户未分配分组，请联系管理员")
		return nil, nil, 0, "", nil, err
	}
	if model == "" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model")
		return nil, nil, 0, "", nil, errors.New("missing model")
	}
	sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
	if err != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
		return nil, nil, 0, "", nil, err
	}
	return u, k, gid, sticky, sel, nil
}

// volcvideoPatchMultipartJSONFields 对 multipart 做火山适配（首期透传，图生视频建议 JSON 创建）。
func volcvideoPatchMultipartJSONFields(body []byte, _ string) ([]byte, error) {
	return body, nil
}
