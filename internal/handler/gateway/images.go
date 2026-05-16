package gateway

import (
	"context"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
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

// 上游若未返回 usage，按 prompt 估算 + 固定「输出侧」占位计费，避免 0 成本。
const imageGenDefaultCompletionTokens = 2048

const maxImageMultipartMemory = 64 << 20 // 与音频 multipart 一致，大图编辑/变体可能较大

// streamUpstreamImageSuccessBody 将上游 2xx 响应体边写入客户端（按块 Flush）边写入 buf，供解析 usage 与扣费。
// 首字节写出后须使用 gatewayDeduct(..., true)。若 io.Copy 失败，客户端可能已收到截断体。
func streamUpstreamImageSuccessBody(w http.ResponseWriter, resp *http.Response, buf *bytes.Buffer) error {
	if v := resp.Header.Get("Content-Type"); v != "" {
		w.Header().Set("Content-Type", v)
	}
	w.WriteHeader(resp.StatusCode)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	mw := io.MultiWriter(WriterFlushWrap(w), buf)
	_, err := io.Copy(mw, resp.Body)
	return err
}

// ImageEdits OpenAI 兼容 POST /v1/images/edits（multipart/form-data）。
func ImageEdits(d ChatDeps) gin.HandlerFunc {
	return imageMultipartRelay(d, "images/edits", true)
}

// ImageVariations OpenAI 兼容 POST /v1/images/variations（multipart/form-data）。
func ImageVariations(d ChatDeps) gin.HandlerFunc {
	return imageMultipartRelay(d, "images/variations", false)
}

func imageMultipartRelay(d ChatDeps, upstreamRelPath string, requirePrompt bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctHdr := strings.ToLower(c.GetHeader("Content-Type"))
		if !strings.HasPrefix(ctHdr, "multipart/form-data") {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "Content-Type must be multipart/form-data")
			return
		}
		if err := c.Request.ParseMultipartForm(maxImageMultipartMemory); err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
			return
		}
		form := c.Request.MultipartForm
		defer func() {
			if form != nil {
				_ = form.RemoveAll()
			}
		}()

		if len(form.File["image"]) == 0 {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing image file")
			return
		}
		if requirePrompt {
			prompts := form.Value["prompt"]
			if len(prompts) == 0 || strings.TrimSpace(prompts[0]) == "" {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing prompt")
				return
			}
		}

		model := "dall-e-2"
		if vals := form.Value["model"]; len(vals) > 0 && strings.TrimSpace(vals[0]) != "" {
			model = strings.TrimSpace(vals[0])
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
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		var totalFile int64
		for _, fhs := range form.File {
			for _, fh := range fhs {
				totalFile += fh.Size
			}
		}
		textEst := multipartTextFieldsForBilling(form, "model")
		imgTok := int(totalFile / 256)
		if imgTok < 80 {
			imgTok = 80
		}
		textTok, _ := d.Billing.EstimateTokens(sel.UpstreamModel, textEst)
		promptToks := textTok + imgTok
		if promptToks < 1 {
			promptToks = 1
		}
		maxOut := imageGenDefaultCompletionTokens
		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model)
		compAssumed := completionTokensFallback(maxOut)
		costPre := br.Cost(promptToks, compAssumed, 0)
		if u.Balance < costPre {
			WriteInsufficientBalance402Precheck(c, u.Balance, costPre, promptToks, compAssumed)
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
			WriteGatewayChannelConfigError(c)
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

		resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, upstreamRelPath, contentType, body, openAIPassthroughHeaders(c), "")
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			WriteGatewayTransportError(c, err)
			return
		}
		defer relay.DrainBody(resp.Body)

		if resp.StatusCode >= 400 {
			raw, _ := io.ReadAll(resp.Body)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			WriteClientProxy(c, resp.StatusCode, "application/json", raw)
			return
		}
		var buf bytes.Buffer
		if err := streamUpstreamImageSuccessBody(c.Writer, resp, &buf); err != nil {
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			slog.Warn("image multipart relay body copy", "err", err)
			return
		}
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
		raw := buf.Bytes()
		usage := ParseOpenAIUsageJSON(raw)
		applyPromptFallback(&usage, promptToks)
		applyCompletionFallback(&usage, maxOut)
		cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
			em := err.Error()
			metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
	}
}

func ImageGenerations(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		uAny, _ := c.Get(middleware.CtxGateUserKey)
		kAny, _ := c.Get(middleware.CtxGateAPIKeyKey)
		u := uAny.(*ent.User)
		k := kAny.(*ent.APIKey)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}

		model, promptText, err := parseImageGenBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或请求 JSON 无效")
			return
		}

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
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptText)
		if promptToks < 1 {
			promptToks = 1
		}
		maxOut := imageGenDefaultCompletionTokens
		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model)
		compAssumed := completionTokensFallback(maxOut)
		costPre := br.Cost(promptToks, compAssumed, 0)
		if u.Balance < costPre {
			WriteInsufficientBalance402Precheck(c, u.Balance, costPre, promptToks, compAssumed)
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)

		start := time.Now()
		routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
		patchImageGen := func(body []byte, upstream string) ([]byte, error) {
			patched, err := router.PatchModelInBody(body, upstream)
			if err != nil {
				return nil, err
			}
			return patchImageGenUpstreamJSON(patched)
		}
		sel, res, err := ExecuteModelForward(c.Request.Context(), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), sel, body, patchImageGen, nil,
			func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
				secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
				if !ok {
					return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
				}
				resp, ferr := relay.ForwardOpenAIImageGenerations(ctx, baseURL, secret, patched)
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
		if res.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, res.Body)
			WriteClientProxy(c, res.StatusCode, "application/json", res.Body)
			return
		}
		usage := ParseOpenAIUsageJSON(res.Body)
		applyPromptFallback(&usage, promptToks)
		applyCompletionFallback(&usage, maxOut)
		cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			em := err.Error()
			metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, &em, time.Since(start))
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
		WriteClientProxy(c, res.StatusCode, "application/json", res.Body)
	}
}

func parseImageGenBody(body []byte) (model string, promptText string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	var b strings.Builder
	if raw, ok := m["prompt"]; ok {
		var p string
		if json.Unmarshal(raw, &p) == nil {
			b.WriteString(p)
		}
	}
	if raw, ok := m["image"]; ok {
		var img string
		if json.Unmarshal(raw, &img) == nil && img != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			// 参考图常为超长 base64，计费估算时抽样长度即可
			if len(img) > 32000 {
				b.WriteString(img[:32000])
				b.WriteString("\n<image_truncated>")
			} else {
				b.WriteString(img)
			}
		}
	}
	return strings.TrimSpace(model), b.String(), nil
}

// patchImageGenUpstreamJSON 兼容硅基等上游：常见客户端用 cfg 表示 guidance_scale。
func patchImageGenUpstreamJSON(body []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body, err
	}
	if _, has := m["guidance_scale"]; !has {
		if raw, ok := m["cfg"]; ok && len(raw) > 0 {
			m["guidance_scale"] = raw
		}
	}
	return json.Marshal(m)
}
