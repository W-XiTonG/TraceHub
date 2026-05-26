package gateway

import (
	"context"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/channelsecret"
	"tracehub/internal/service/upstreamprobe"

	"github.com/gin-gonic/gin"
)

const maxAudioMultipartMemory = 64 << 20 // 64 MiB 内存缓冲，超出部分落盘

// AudioTranscriptions POST /v1/audio/transcriptions（multipart/form-data，与 OpenAI 一致）。
func AudioTranscriptions(d ChatDeps) gin.HandlerFunc {
	return audioMultipartHandler(d, "audio/transcriptions")
}

// AudioTranslations POST /v1/audio/translations。
func AudioTranslations(d ChatDeps) gin.HandlerFunc {
	return audioMultipartHandler(d, "audio/translations")
}

func audioMultipartHandler(d ChatDeps, upstreamRelPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := strings.ToLower(c.GetHeader("Content-Type"))
		if !strings.HasPrefix(ct, "multipart/form-data") {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "Content-Type must be multipart/form-data")
			return
		}
		if err := c.Request.ParseMultipartForm(maxAudioMultipartMemory); err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
			return
		}
		form := c.Request.MultipartForm
		defer func() {
			if form != nil {
				_ = form.RemoveAll()
			}
		}()

		modelVals := form.Value["model"]
		if len(modelVals) == 0 || strings.TrimSpace(modelVals[0]) == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing model")
			return
		}
		model := strings.TrimSpace(modelVals[0])

		hasFile := false
		var totalFile int64
		for _, fhs := range form.File {
			for _, fh := range fhs {
				hasFile = true
				totalFile += fh.Size
			}
		}
		if !hasFile {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing file")
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

		sticky := c.GetHeader("X-TraceHub-Session")
		if sticky == "" {
			sticky = c.GetHeader("X-Sticky-Session")
		}
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
		chType := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.UsesOpenAICompatRelay(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		audioTok := int(totalFile / 256)
		if audioTok < 80 {
			audioTok = 80
		}
		maxComp := 2048
		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
		var promptToks, promptToksPre, compAssumed int
		if br.NeedsTokenEstimate() {
			promptEst := multipartTextFieldsForBilling(form, "model")
			audioTokPre := int((totalFile*2 + 255) / 256)
			if audioTokPre < 80 {
				audioTokPre = 80
			}
			textTok, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptEst)
			promptToks = textTok + audioTok
			promptToksPre = textTok + audioTokPre
			if promptToks < 1 {
				promptToks = 1
			}
			if promptToksPre < 1 {
				promptToksPre = 1
			}
			compAssumed = completionTokensFallback(maxComp)
		}
		if _, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
			return BillTokenEst{Prompt: promptToksPre, Completion: compAssumed}, nil
		}); !ok {
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		if multipartFormWantsStream(form) {
			baseURL := strings.TrimSpace(ch.APIBaseURL)
			if baseURL == "" {
				baseURL = router.DefaultBaseURL(ch.ChannelType)
			} else if nb, err := openaicompat.NormalizeOpenAIAPIRoot(baseURL); err == nil {
				baseURL = nb
			}
			var secret string
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
				return
			}
			body, contentType, err := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
				return
			}
			resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, upstreamRelPath, contentType, body, openAIPassthroughHeaders(c), "text/event-stream")
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
				WriteGatewayTransportError(c, err)
				return
			}
			defer relay.DrainBody(resp.Body)
			defer func() {
				metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(start).Seconds())
			}()

			if resp.StatusCode >= 400 {
				raw, _ := io.ReadAll(resp.Body)
				metrics.GatewayRequests.WithLabelValues(model, chType, http.StatusText(resp.StatusCode)).Inc()
				logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
				outCT := resp.Header.Get("Content-Type")
				if outCT == "" {
					outCT = "application/json"
				}
				WriteClientProxy(c, resp.StatusCode, outCT, raw)
				return
			}
			for _, k := range []string{"Content-Type", "Cache-Control"} {
				if v := resp.Header.Get(k); v != "" {
					c.Writer.Header().Set(k, v)
				}
			}
			if c.Writer.Header().Get("X-Accel-Buffering") == "" {
				c.Writer.Header().Set("X-Accel-Buffering", "no")
			}
			c.Writer.WriteHeader(resp.StatusCode)
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
			capture := copyOpenAIChatSSEWithUsage(c.Writer, resp.Body)
			usage := ResolveOpenAIStreamUsage(capture, d.Billing, sel.UpstreamModel, promptToks, maxComp, false)
			pt := usage.PromptTokens
			comp := usage.CompletionTokens
			cached := usage.CachedPromptTokens
			if comp < 0 {
				comp = 0
			}
			cost := GatewayFinalCost(br, pt, comp, cached)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
				em := err.Error()
				metrics.GatewayRequests.WithLabelValues(model, chType, "deduct_err").Inc()
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), usageNoteForLog(usage.Note), time.Since(start))
			return
		}

		routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
		sel, res, err := routeDeps.executeWithFailoverRecorded(c.Request.Context(), int64(u.ID), gid, model, sticky, false, sel,
			func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
				secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
				if !ok {
					return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
				}
				mpBody, contentType, err := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
				if err != nil {
					return nil, err
				}
				resp, ferr := relay.ForwardOpenAIMultipart(ctx, baseURL, secret, upstreamRelPath, contentType, mpBody, openAIPassthroughHeaders(c), "")
				return HTTPResponseToUpstreamResult(resp, ferr)
			})
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
			WriteGatewayTransportError(c, err)
			return
		}
		ch = sel.Channel
		chType = strings.ToLower(strings.TrimSpace(ch.ChannelType))
		metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(start).Seconds())
		if res.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, chType, http.StatusText(res.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, res.Body)
			if rec := UsageRecorderFrom(c); rec != nil {
				rec.SetModel(model)
				rec.SetChannel(ch.ID, sel.UpstreamModel)
			}
			upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityAudio, res.StatusCode, res.Body, upstreamprobe.CapSourceRuntime)
			em := http.StatusText(res.StatusCode)
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, promptToks, maxComp, 0, 0, c.ClientIP(), res.StatusCode, &em, time.Since(start))
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
		applyPromptFallback(&usage, promptToks)
		applyCompletionFallback(&usage, maxComp, false)
		cost := GatewayFinalCost(br,usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityAudio, res.StatusCode, nil, upstreamprobe.CapSourceRuntime)
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
		outCT := "application/json"
		if res.Headers != nil {
			if h := res.Headers.Get("Content-Type"); h != "" {
				outCT = h
			}
		}
		WriteClientProxy(c, res.StatusCode, outCT, res.Body)
	}
}

// AudioSpeech POST /v1/audio/speech（application/json）。
func AudioSpeech(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, inputText, err := parseSpeechBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 input")
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

		sticky := c.GetHeader("X-TraceHub-Session")
		if sticky == "" {
			sticky = c.GetHeader("X-Sticky-Session")
		}
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
		chType := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.UsesOpenAICompatRelay(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
		var promptToks, compAssumed int
		if br.NeedsTokenEstimate() {
			promptToks, _ = d.Billing.EstimateTokens(sel.UpstreamModel, inputText)
			if promptToks < 1 {
				promptToks = 1
			}
			inRunes := utf8.RuneCountInString(inputText)
			compEst := inRunes * 4
			if compEst < 512 {
				compEst = 512
			}
			compAssumed = completionTokensFallback(compEst)
		}
		if _, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
			return BillTokenEst{Prompt: promptToks, Completion: compAssumed}, nil
		}); !ok {
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()
		accept := c.GetHeader("Accept")
		routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

		sel, res, err := ExecuteModelForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), sel, body, nil, nil,
			func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
				secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
				if !ok {
					return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
				}
				resp, ferr := relay.ForwardOpenAISpeech(ctx, baseURL, secret, patched, accept, openAIPassthroughHeaders(c))
				return HTTPResponseToUpstreamResult(resp, ferr)
			})
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
			WriteGatewayTransportError(c, err)
			return
		}
		ch = sel.Channel
		chType = strings.ToLower(strings.TrimSpace(ch.ChannelType))
		metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(start).Seconds())

		if res.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, chType, http.StatusText(res.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, res.Body)
			if rec := UsageRecorderFrom(c); rec != nil {
				rec.SetModel(model)
				rec.SetChannel(ch.ID, sel.UpstreamModel)
			}
			upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityAudio, res.StatusCode, res.Body, upstreamprobe.CapSourceRuntime)
			em := http.StatusText(res.StatusCode)
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, promptToks, compAssumed, 0, 0, c.ClientIP(), res.StatusCode, &em, time.Since(start))
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
		pt2, ct2 := usage.PromptTokens, usage.CompletionTokens
		if pt2 == 0 {
			pt2 = promptToks
		}
		if ct2 == 0 {
			ct2 = len(res.Body) / 400
			if ct2 < 256 {
				ct2 = 256
			}
			if compAssumed > 0 && ct2 < compAssumed {
				ct2 = compAssumed
			}
			ct2 = completionTokensFallback(ct2)
		}
		cost := GatewayFinalCost(br,pt2, ct2, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		upstreamprobe.NotifyCapabilityOutcome(ch.ID, model, upstreamprobe.CapabilityAudio, res.StatusCode, nil, upstreamprobe.CapSourceRuntime)
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))

		outCT := "application/octet-stream"
		if res.Headers != nil {
			if h := res.Headers.Get("Content-Type"); h != "" {
				outCT = h
			}
		}
		WriteClientProxy(c, res.StatusCode, outCT, res.Body)
	}
}

// multipartFormWantsStream 与 OpenAI 文档一致：form 字段 stream=true 时返回 transcription 的 SSE。
func multipartFormWantsStream(form *multipart.Form) bool {
	for _, v := range form.Value["stream"] {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "on":
			return true
		}
	}
	return false
}

func sortedFormValueKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedFormFileKeys(m map[string][]*multipart.FileHeader) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func rebuildMultipartWithUpstreamModel(form *multipart.Form, upstreamModel string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, name := range sortedFormValueKeys(form.Value) {
		for _, v := range form.Value[name] {
			val := v
			if name == "model" {
				val = upstreamModel
			}
			if err := w.WriteField(name, val); err != nil {
				return nil, "", err
			}
		}
	}
	for _, name := range sortedFormFileKeys(form.File) {
		for _, fh := range form.File[name] {
			fn := fh.Filename
			if fn == "" {
				fn = "audio"
			}
			part, err := w.CreateFormFile(name, fn)
			if err != nil {
				return nil, "", err
			}
			f, err := fh.Open()
			if err != nil {
				return nil, "", err
			}
			_, err = io.Copy(part, f)
			_ = f.Close()
			if err != nil {
				return nil, "", err
			}
		}
	}
	ct := w.FormDataContentType()
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), ct, nil
}

func multipartTextFieldsForBilling(form *multipart.Form, skipField string) string {
	var b strings.Builder
	for _, name := range sortedFormValueKeys(form.Value) {
		if name == skipField {
			continue
		}
		for _, v := range form.Value[name] {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(v)
		}
	}
	return b.String()
}

// openAIPassthroughHeaders 透传 OpenAI 官方可选头（组织 / 项目 / beta 功能等）。
func openAIPassthroughHeaders(c *gin.Context) http.Header {
	h := make(http.Header)
	for _, k := range []string{"OpenAI-Organization", "OpenAI-Project", "OpenAI-Beta"} {
		if v := strings.TrimSpace(c.GetHeader(k)); v != "" {
			h.Set(k, v)
		}
	}
	if len(h) == 0 {
		return nil
	}
	return h
}

func parseSpeechBody(body []byte) (model string, input string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	if raw, ok := m["input"]; ok {
		_ = json.Unmarshal(raw, &input)
	}
	return strings.TrimSpace(model), input, nil
}
