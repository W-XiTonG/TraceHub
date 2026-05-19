package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/billing"
	"tracehub/internal/service/channelsecret"

	"github.com/gin-gonic/gin"
)

// 大文件上传：multipart 非文件部分内存上限；文件体可落盘。
const maxOpenAIFileMultipartMemory = 256 << 20

func upstreamQueryStripModel(c *gin.Context) string {
	q := c.Request.URL.Query()
	q.Del("model")
	return q.Encode()
}

func extractJSONModelField(body []byte) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return ""
	}
	raw, ok := m["model"]
	if !ok {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return strings.TrimSpace(s)
}

func enterpriseRoutingModel(c *gin.Context, body []byte, requireModelInJSON bool) (model string, ok bool) {
	if requireModelInJSON {
		if len(body) == 0 {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少请求体")
			return "", false
		}
		m := extractJSONModelField(body)
		if m == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 JSON 无效")
			return "", false
		}
		return m, true
	}
	m := routingModelForStatelessResource(c)
	if m == "" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请使用 Query 参数 model=…，或设置请求头 X-TraceHub-Model")
		return "", false
	}
	return m, true
}

func safeOpenAIResourceID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	return !strings.ContainsAny(id, "/\\")
}

func enterpriseResolveUpstream(c *gin.Context, d ChatDeps, routingModel string, prePrompt, preComp int) (
	u *ent.User, k *ent.APIKey, sel *router.Selected, secret, baseURL, ct string, ok bool, br billing.ModelBillRates,
) {
	uAny, _ := c.Get(middleware.CtxGateUserKey)
	kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
	u = uAny.(*ent.User)
	k = kAny.(*ent.APIKey)

	fresh, err := d.Client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(c.Request.Context())
	if err != nil {
		middleware.WriteOpenAIError(c, 500, "api_error", "用户状态异常")
		return nil, nil, nil, "", "", "", false, billing.ModelBillRates{}
	}
	u = fresh

	sticky := c.GetHeader("X-TraceHub-Session")
	if sticky == "" {
		sticky = c.GetHeader("X-Sticky-Session")
	}
	gid := u.GroupID
	if gid <= 0 {
		middleware.WriteOpenAIError(c, 403, "permission_error", "账户未分配分组，请联系管理员")
		return nil, nil, nil, "", "", "", false, billing.ModelBillRates{}
	}
	sel, err2 := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, routingModel, sticky)
	if err2 != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err2.Error())
		return nil, nil, nil, "", "", "", false, billing.ModelBillRates{}
	}
	ch := sel.Channel
	ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
	if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
		return nil, nil, nil, "", "", "", false, billing.ModelBillRates{}
	}

	br, billErr := d.Billing.ModelBillRates(c.Request.Context(), u, routingModel, ch.ID)
	if billErr != nil {
		middleware.WriteOpenAIError(c, 500, "api_error", "计费状态异常")
		return nil, nil, nil, "", "", "", false, billing.ModelBillRates{}
	}
	compAssumed := 0
	if br.NeedsTokenEstimate() {
		if prePrompt < 1 {
			prePrompt = 1
		}
		if preComp < 0 {
			preComp = 0
		}
		compAssumed = completionTokensFallback(preComp)
	}
	costPre, _ := br.PrecheckCost(prePrompt, compAssumed, 0)
	if u.Balance < costPre {
		pt, comp := prePrompt, compAssumed
		if br.IsPerRequest() {
			pt, comp = 0, 0
		}
		WriteInsufficientBalance402Precheck(c, u.Balance, costPre, pt, comp)
		return nil, nil, nil, "", "", "", false, br
	}

	baseURL = strings.TrimSpace(ch.APIBaseURL)
	if baseURL == "" {
		baseURL = router.DefaultBaseURL(ch.ChannelType)
	} else if nb, err := openaicompat.NormalizeOpenAIAPIRoot(baseURL); err == nil {
		baseURL = nb
	}
	switch ch.AuthType {
	case channel.AuthTypeAPIKey:
		secret = channelsecret.Plaintext(d.AESKey, ch.APIKey)
	case channel.AuthTypeOauth:
		secret = channelsecret.Plaintext(d.AESKey, ch.OauthToken)
	case channel.AuthTypeSetupToken:
		secret = channelsecret.Plaintext(d.AESKey, ch.SetupToken)
	}
	if strings.TrimSpace(secret) == "" {
		WriteGatewayChannelConfigError(c)
		return nil, nil, nil, "", "", "", false, br
	}
	return u, k, sel, secret, baseURL, ct, true, br
}

func enterpriseForwardJSON(
	c *gin.Context, d ChatDeps,
	routingModel string,
	prePrompt, preComp int,
	method, relPath, rawQuery string,
	body []byte,
	patchModel bool,
	useLongClient bool,
) {
	u, k, sel, _, _, ct, ok, br := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
	if !ok {
		return
	}
	model := routingModel

	sticky := c.GetHeader("X-TraceHub-Session")
	if sticky == "" {
		sticky = c.GetHeader("X-Sticky-Session")
	}
	gid := u.GroupID

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	h := openAIPassthroughHeaders(c)
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

	forwardHTTP := func(ctx context.Context, sel *router.Selected, upBody []byte) (*router.UpstreamResult, error) {
		secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
		if !ok {
			return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
		}
		var resp *http.Response
		var ferr error
		if useLongClient {
			resp, ferr = relay.ForwardOpenAIHTTPLong(ctx, baseURL, secret, method, relPath, upBody, rawQuery, h)
		} else {
			resp, ferr = relay.ForwardOpenAIHTTP(ctx, baseURL, secret, method, relPath, upBody, rawQuery, h)
		}
		return HTTPResponseToUpstreamResult(resp, ferr)
	}

	var res *router.UpstreamResult
	var err error
	if len(body) == 0 {
		sel, res, err = ExecuteModelHTTPForward(c.Request.Context(), routeDeps, int64(u.ID), gid, routingModel, sticky, sel,
			func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
				return forwardHTTP(ctx, sel, nil)
			})
	} else {
		var patchFn func([]byte, string) ([]byte, error)
		if patchModel && extractJSONModelField(body) != "" {
			patchFn = router.PatchModelInBody
		} else {
			patchFn = func(b []byte, _ string) ([]byte, error) { return b, nil }
		}
		sel, res, err = ExecuteModelForward(c.Request.Context(), routeDeps, int64(u.ID), gid, routingModel, sticky, false, sel, body, patchFn, nil,
			func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
				return forwardHTTP(ctx, sel, patched)
			})
	}
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	ch := sel.Channel
	ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if res.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, res.Body)
		outCT := "application/json"
		if res.Headers != nil {
			if h := res.Headers.Get("Content-Type"); h != "" {
				outCT = h
			}
		}
		WriteClientProxy(c, res.StatusCode, outCT, res.Body)
		return
	}
	usage := parseUsageFlexible(res.Body)
	applyPromptFallback(&usage, prePrompt)
	applyCompletionFallback(&usage, preComp)
	cost := GatewayFinalCost(br,usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
	outCT := "application/json"
	if res.Headers != nil {
		if h := res.Headers.Get("Content-Type"); h != "" {
			outCT = h
		}
	}
	WriteClientProxy(c, res.StatusCode, outCT, res.Body)
}

func enterpriseForwardStream(
	c *gin.Context, d ChatDeps,
	routingModel string,
	prePrompt, preComp int,
	method, relPath, rawQuery string,
) {
	u, k, sel, secret, baseURL, ct, ok, br := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
	if !ok {
		return
	}
	ch := sel.Channel
	model := routingModel

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	h := openAIPassthroughHeaders(c)
	if h == nil {
		h = http.Header{}
	}
	if h.Get("Accept") == "" {
		h.Set("Accept", "text/event-stream")
	}

	resp, err := relay.ForwardOpenAIHTTPLong(c.Request.Context(), baseURL, secret, method, relPath, nil, rawQuery, h)
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	defer relay.DrainBody(resp.Body)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		WriteClientProxy(c, resp.StatusCode, outCT, raw)
		return
	}

	for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(key); v != "" {
			c.Writer.Header().Set(key, v)
		}
	}
	if c.Writer.Header().Get("X-Accel-Buffering") == "" {
		c.Writer.Header().Set("X-Accel-Buffering", "no")
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	streamPT, streamCT, streamCached := copyOpenAIChatSSEWithUsage(c.Writer, resp.Body)
	pt := streamPT
	comp := streamCT
	cached := streamCached
	if pt < 1 {
		pt = prePrompt
	}
	if comp < 0 {
		comp = 0
	}
	if streamPT == 0 && streamCT == 0 {
		comp = completionTokensFallback(preComp)
	}
	cost := GatewayFinalCost(br,pt, comp, cached)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
		em := err.Error()
		metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
}

// enterpriseForwardPostSSE POST JSON 且上游返回 text/event-stream（如 Thread Run 流式）。
func enterpriseForwardPostSSE(c *gin.Context, d ChatDeps, routingModel string, prePrompt, preComp int, relPath, rawQuery string, body []byte) {
	u, k, sel, secret, baseURL, ct, ok, br := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
	if !ok {
		return
	}
	ch := sel.Channel
	model := routingModel
	outBody := body
	if len(body) > 0 && extractJSONModelField(body) != "" {
		patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}
		outBody = patched
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	h := openAIPassthroughHeaders(c)
	if h == nil {
		h = http.Header{}
	}
	if h.Get("Accept") == "" {
		h.Set("Accept", "text/event-stream")
	}

	resp, err := relay.ForwardOpenAIHTTPLong(c.Request.Context(), baseURL, secret, http.MethodPost, relPath, outBody, rawQuery, h)
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	defer relay.DrainBody(resp.Body)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		WriteClientProxy(c, resp.StatusCode, outCT, raw)
		return
	}

	for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(key); v != "" {
			c.Writer.Header().Set(key, v)
		}
	}
	if c.Writer.Header().Get("X-Accel-Buffering") == "" {
		c.Writer.Header().Set("X-Accel-Buffering", "no")
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	streamPT, streamCT, streamCached := copyOpenAIChatSSEWithUsage(c.Writer, resp.Body)
	pt := streamPT
	comp := streamCT
	cached := streamCached
	if pt < 1 {
		pt = prePrompt
	}
	if comp < 0 {
		comp = 0
	}
	if streamPT == 0 && streamCT == 0 {
		comp = completionTokensFallback(preComp)
	}
	cost := GatewayFinalCost(br,pt, comp, cached)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
		em := err.Error()
		metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
}

func enterpriseForwardBinaryDownload(
	c *gin.Context, d ChatDeps,
	routingModel string,
	prePrompt, preComp int,
	relPath, rawQuery string,
) {
	u, k, sel, secret, baseURL, ct, ok, br := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
	if !ok {
		return
	}
	ch := sel.Channel
	model := routingModel

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	resp, err := relay.ForwardOpenAIHTTPLong(c.Request.Context(), baseURL, secret, http.MethodGet, relPath, nil, rawQuery, openAIPassthroughHeaders(c))
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	defer relay.DrainBody(resp.Body)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		WriteClientProxy(c, resp.StatusCode, outCT, raw)
		return
	}

	for _, key := range []string{"Content-Type", "Content-Disposition", "Content-Length", "Cache-Control"} {
		if v := resp.Header.Get(key); v != "" {
			c.Writer.Header().Set(key, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	n, _ := copyStreamFlushing(c.Writer, resp.Body)

	pt := prePrompt
	if cl := resp.ContentLength; cl > 0 {
		pt = int(cl / 512)
		if pt < prePrompt {
			pt = prePrompt
		}
	} else if n > 0 {
		pt = int(n / 512)
		if pt < prePrompt {
			pt = prePrompt
		}
	}
	comp := preComp
	cost := GatewayFinalCost(br,pt, comp, 0)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
		em := err.Error()
		metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, 0, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, 0, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
}

// --- Files ---

func OpenAIFilesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := strings.ToLower(c.GetHeader("Content-Type"))
		if !strings.HasPrefix(ct, "multipart/form-data") {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "Content-Type must be multipart/form-data")
			return
		}
		if err := c.Request.ParseMultipartForm(maxOpenAIFileMultipartMemory); err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
			return
		}
		form := c.Request.MultipartForm
		defer func() {
			if form != nil {
				_ = form.RemoveAll()
			}
		}()
		if len(form.File["file"]) == 0 {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing file")
			return
		}
		if vals := form.Value["purpose"]; len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing purpose")
			return
		}

		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}

		var totalFile int64
		for _, fhs := range form.File {
			for _, fh := range fhs {
				totalFile += fh.Size
			}
		}
		imgTok := int(totalFile / 256)
		if imgTok < 128 {
			imgTok = 128
		}
		u, k, sel, secret, baseURL, ct, ok2, br := enterpriseResolveUpstream(c, d, model, imgTok, 2048)
		if !ok2 {
			return
		}
		ch := sel.Channel

		body, contentType, err := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()
		resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, "files", contentType, body, openAIPassthroughHeaders(c), "")
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			WriteGatewayTransportError(c, err)
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			WriteClientProxy(c, resp.StatusCode, "application/json", raw)
			return
		}
		usage := parseUsageFlexible(raw)
		if usage.PromptTokens == 0 {
			usage.PromptTokens = imgTok
		}
		applyCompletionFallback(&usage, 2048)
		cost := GatewayFinalCost(br,usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
		WriteClientProxy(c, resp.StatusCode, "application/json", raw)
	}
}

func OpenAIFilesList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 64, 512, http.MethodGet, "files", q, nil, false, false)
	}
}

func OpenAIFilesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid file id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 64, 512, http.MethodGet, "files/"+id, q, nil, false, false)
	}
}

func OpenAIFilesContent(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid file id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardBinaryDownload(c, d, model, 256, 512, "files/"+id+"/content", q)
	}
}

func OpenAIFilesDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid file id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 32, 256, http.MethodDelete, "files/"+id, q, nil, false, false)
	}
}

// --- Batches ---

func OpenAIBatchesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		enterpriseForwardJSON(c, d, model, 256, 2048, http.MethodPost, "batches", "", body, false, false)
	}
}

func OpenAIBatchesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("batch_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid batch id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 128, 1024, http.MethodGet, "batches/"+id, q, nil, false, true)
	}
}

func OpenAIBatchesList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 128, 1024, http.MethodGet, "batches", q, nil, false, true)
	}
}

func OpenAIBatchesCancel(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("batch_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid batch id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 64, 512, http.MethodPost, "batches/"+id+"/cancel", q, nil, false, true)
	}
}

// --- Fine-tuning ---

func OpenAIFineTuningJobsCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, ok := enterpriseRoutingModel(c, body, true)
		if !ok {
			return
		}
		enterpriseForwardJSON(c, d, model, 512, 4096, http.MethodPost, "fine_tuning/jobs", "", body, true, false)
	}
}

func OpenAIFineTuningJobRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("job_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid job id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 128, 1024, http.MethodGet, "fine_tuning/jobs/"+id, q, nil, false, true)
	}
}

func OpenAIFineTuningJobsList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 128, 1024, http.MethodGet, "fine_tuning/jobs", q, nil, false, true)
	}
}

func OpenAIFineTuningJobCancel(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("job_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid job id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 64, 512, http.MethodPost, "fine_tuning/jobs/"+id+"/cancel", q, nil, false, true)
	}
}

func OpenAIFineTuningJobEvents(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("job_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid job id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		if strings.EqualFold(c.Query("stream"), "true") {
			enterpriseForwardStream(c, d, model, 256, 2048, http.MethodGet, "fine_tuning/jobs/"+id+"/events", q)
			return
		}
		enterpriseForwardJSON(c, d, model, 256, 2048, http.MethodGet, "fine_tuning/jobs/"+id+"/events", q, nil, false, true)
	}
}

func OpenAIFineTuningJobCheckpoints(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("job_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid job id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		enterpriseForwardJSON(c, d, model, 128, 1024, http.MethodGet, "fine_tuning/jobs/"+id+"/checkpoints", q, nil, false, true)
	}
}

func OpenAIFineTuningJobCheckpointRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		jobID := strings.TrimSpace(c.Param("job_id"))
		ck := strings.TrimSpace(c.Param("checkpoint_id"))
		if !safeOpenAIResourceID(jobID) || !safeOpenAIResourceID(ck) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid job or checkpoint id")
			return
		}
		model, ok := enterpriseRoutingModel(c, nil, false)
		if !ok {
			return
		}
		q := upstreamQueryStripModel(c)
		rel := "fine_tuning/jobs/" + jobID + "/checkpoints/" + ck
		enterpriseForwardJSON(c, d, model, 64, 512, http.MethodGet, rel, q, nil, false, true)
	}
}
