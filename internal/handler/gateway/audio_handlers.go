package gateway

import (
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
		if !router.IsOpenAICompatChannelType(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持音频接口，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
			return
		}

		promptEst := multipartTextFieldsForBilling(form, "model")
		audioTok := int(totalFile / 256)
		if audioTok < 80 {
			audioTok = 80
		}
		textTok, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptEst)
		promptToks := textTok + audioTok
		if promptToks < 1 {
			promptToks = 1
		}
		maxComp := 2048
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, maxComp)
		if u.Balance < costPre {
			middleware.WriteOpenAIError(c, 402, "insufficient_balance_error", "余额不足")
			return
		}

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
			middleware.WriteOpenAIError(c, 503, "upstream_error", "渠道未配置有效凭证")
			return
		}

		body, contentType, err := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		acceptHdr := ""
		if multipartFormWantsStream(form) {
			acceptHdr = "text/event-stream"
		}
		resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, upstreamRelPath, contentType, body, openAIPassthroughHeaders(c), acceptHdr)
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
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
			c.Data(resp.StatusCode, outCT, raw)
			return
		}

		// OpenAI：multipart 中 stream=true 时返回 SSE，需边读边写客户端，不能整包缓冲。
		if multipartFormWantsStream(form) {
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
			streamPT, streamCT := copyOpenAIChatSSEWithUsage(c.Writer, resp.Body)
			pt := streamPT
			comp := streamCT
			if pt < 1 {
				pt = promptToks
			}
			if comp < 0 {
				comp = 0
			}
			if streamPT == 0 && streamCT == 0 {
				comp = maxComp
			}
			cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt, comp)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
				em := err.Error()
				metrics.GatewayRequests.WithLabelValues(model, chType, "deduct_err").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
			return
		}

		raw, _ := io.ReadAll(resp.Body)
		pt2, ct2 := parseUsageFlexible(raw)
		if pt2 == 0 {
			pt2 = promptToks
		}
		if ct2 == 0 {
			ct2 = maxComp
		}
		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))

		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		c.Data(resp.StatusCode, outCT, raw)
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
		if !router.IsOpenAICompatChannelType(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持音频接口，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
			return
		}

		inRunes := utf8.RuneCountInString(inputText)
		promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, inputText)
		if promptToks < 1 {
			promptToks = 1
		}
		compEst := inRunes * 4
		if compEst < 512 {
			compEst = 512
		}
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, compEst)
		if u.Balance < costPre {
			middleware.WriteOpenAIError(c, 402, "insufficient_balance_error", "余额不足")
			return
		}

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
			middleware.WriteOpenAIError(c, 503, "upstream_error", "渠道未配置有效凭证")
			return
		}

		patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()
		accept := c.GetHeader("Accept")

		resp, err := relay.ForwardOpenAISpeech(c.Request.Context(), baseURL, secret, patched, accept, openAIPassthroughHeaders(c))
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(start).Seconds())

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, chType, http.StatusText(resp.StatusCode)).Inc()
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
			pt2 = promptToks
		}
		if ct2 == 0 {
			ct2 = len(raw) / 400
			if ct2 < 256 {
				ct2 = 256
			}
			if ct2 < compEst {
				ct2 = compEst
			}
		}
		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))

		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/octet-stream"
		}
		c.Data(resp.StatusCode, outCT, raw)
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

// parseUsageFlexible 解析 OpenAI 及兼容上游在音频等接口中可能返回的 usage（含 duration / input_tokens 等）。
func parseUsageFlexible(raw []byte) (promptTokens, completionTokens int) {
	pt, ct := parseUsageFromOpenAIResponse(raw)
	if pt > 0 || ct > 0 {
		return pt, ct
	}
	var wrap struct {
		Usage struct {
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
			InputTokens      int     `json:"input_tokens"`
			OutputTokens     int     `json:"output_tokens"`
			TotalTokens      int     `json:"total_tokens"`
			Seconds          float64 `json:"seconds"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return 0, 0
	}
	if wrap.Usage.TotalTokens > 0 {
		return wrap.Usage.TotalTokens, 0
	}
	if wrap.Usage.InputTokens > 0 || wrap.Usage.OutputTokens > 0 {
		return wrap.Usage.InputTokens, wrap.Usage.OutputTokens
	}
	if wrap.Usage.Seconds > 0 {
		return int(wrap.Usage.Seconds * 100), 0
	}
	return 0, 0
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
