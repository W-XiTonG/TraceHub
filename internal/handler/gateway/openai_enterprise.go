package gateway

import (
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
	u *ent.User, k *ent.APIKey, sel *router.Selected, secret, baseURL, ct string, ok bool,
) {
	uAny, _ := c.Get(middleware.CtxGateUserKey)
	kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
	u = uAny.(*ent.User)
	k = kAny.(*ent.APIKey)

	fresh, err := d.Client.User.Query().Where(user.IDEQ(u.ID)).WithGroup().Only(c.Request.Context())
	if err != nil {
		middleware.WriteOpenAIError(c, 500, "api_error", "用户状态异常")
		return nil, nil, nil, "", "", "", false
	}
	u = fresh

	sticky := c.GetHeader("X-TraceHub-Session")
	if sticky == "" {
		sticky = c.GetHeader("X-Sticky-Session")
	}
	gid := u.GroupID
	if gid <= 0 {
		middleware.WriteOpenAIError(c, 403, "permission_error", "账户未分配分组，请联系管理员")
		return nil, nil, nil, "", "", "", false
	}
	sel, err2 := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, routingModel, sticky)
	if err2 != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err2.Error())
		return nil, nil, nil, "", "", "", false
	}
	ch := sel.Channel
	ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
	if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持该 OpenAI 企业管理接口，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
		return nil, nil, nil, "", "", "", false
	}

	if prePrompt < 1 {
		prePrompt = 1
	}
	if preComp < 0 {
		preComp = 0
	}
	costPre, _ := d.Billing.Charge(c.Request.Context(), u, routingModel, prePrompt, preComp)
	if u.Balance < costPre {
		middleware.WriteOpenAIError(c, 402, "insufficient_balance_error", "余额不足")
		return nil, nil, nil, "", "", "", false
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
		middleware.WriteOpenAIError(c, 503, "upstream_error", "渠道未配置有效凭证")
		return nil, nil, nil, "", "", "", false
	}
	return u, k, sel, secret, baseURL, ct, true
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
	u, k, sel, secret, baseURL, ct, ok := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
	if !ok {
		return
	}
	ch := sel.Channel
	model := routingModel
	if patchModel && len(body) > 0 && extractJSONModelField(body) != "" {
		patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}
		body = patched
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	h := openAIPassthroughHeaders(c)

	var resp *http.Response
	var err error
	if useLongClient {
		resp, err = relay.ForwardOpenAIHTTPLong(c.Request.Context(), baseURL, secret, method, relPath, body, rawQuery, h)
	} else {
		resp, err = relay.ForwardOpenAIHTTP(c.Request.Context(), baseURL, secret, method, relPath, body, rawQuery, h)
	}
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
		return
	}
	defer relay.DrainBody(resp.Body)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		c.Data(resp.StatusCode, outCT, raw)
		return
	}
	pt2, ct2 := parseUsageFlexible(raw)
	if pt2 == 0 {
		pt2 = prePrompt
	}
	if ct2 == 0 {
		ct2 = preComp
	}
	cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
	_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
	outCT := resp.Header.Get("Content-Type")
	if outCT == "" {
		outCT = "application/json"
	}
	c.Data(resp.StatusCode, outCT, raw)
}

func enterpriseForwardStream(
	c *gin.Context, d ChatDeps,
	routingModel string,
	prePrompt, preComp int,
	method, relPath, rawQuery string,
) {
	u, k, sel, secret, baseURL, ct, ok := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
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
		middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
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
		c.Data(resp.StatusCode, outCT, raw)
		return
	}

	for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(key); v != "" {
			c.Writer.Header().Set(key, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	_, _ = io.Copy(c.Writer, resp.Body)

	pt := prePrompt
	comp := preComp
	cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt, comp)
	_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
}

// enterpriseForwardPostSSE POST JSON 且上游返回 text/event-stream（如 Thread Run 流式）。
func enterpriseForwardPostSSE(c *gin.Context, d ChatDeps, routingModel string, prePrompt, preComp int, relPath, rawQuery string, body []byte) {
	u, k, sel, secret, baseURL, ct, ok := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
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
		middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
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
		c.Data(resp.StatusCode, outCT, raw)
		return
	}

	for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id"} {
		if v := resp.Header.Get(key); v != "" {
			c.Writer.Header().Set(key, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)
	if f, ok := c.Writer.(http.Flusher); ok {
		f.Flush()
	}
	_, _ = io.Copy(c.Writer, resp.Body)

	pt := prePrompt
	comp := preComp
	cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt, comp)
	_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
}

func enterpriseForwardBinaryDownload(
	c *gin.Context, d ChatDeps,
	routingModel string,
	prePrompt, preComp int,
	relPath, rawQuery string,
) {
	u, k, sel, secret, baseURL, ct, ok := enterpriseResolveUpstream(c, d, routingModel, prePrompt, preComp)
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
		middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
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
		c.Data(resp.StatusCode, outCT, raw)
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
	n, _ := io.Copy(c.Writer, resp.Body)

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
	cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt, comp)
	_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
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
		u, k, sel, secret, baseURL, ct, ok2 := enterpriseResolveUpstream(c, d, model, imgTok, 2048)
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
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			c.Data(resp.StatusCode, "application/json", raw)
			return
		}
		pt2, ct2 := parseUsageFlexible(raw)
		if pt2 == 0 {
			pt2 = imgTok
		}
		if ct2 == 0 {
			ct2 = 2048
		}
		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
		_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
		c.Data(resp.StatusCode, "application/json", raw)
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
