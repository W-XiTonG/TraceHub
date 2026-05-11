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

// 上游若未返回 usage，按 prompt 估算 + 固定「输出侧」占位计费，避免 0 成本。
const imageGenDefaultCompletionTokens = 2048

const maxImageMultipartMemory = 64 << 20 // 与音频 multipart 一致，大图编辑/变体可能较大

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
		if ct != "openai" && ct != "deepseek" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型暂不支持图片编辑/变体，请使用 openai / deepseek 渠道")
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
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, maxOut)
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

		resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, upstreamRelPath, contentType, body, openAIPassthroughHeaders(c), "")
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
		pt2, ct2 := parseUsageFromOpenAIResponse(raw)
		if pt2 == 0 {
			pt2 = promptToks
		}
		if ct2 == 0 {
			ct2 = maxOut
		}
		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
		_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
		c.Data(resp.StatusCode, "application/json", raw)
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

		promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptText)
		if promptToks < 1 {
			promptToks = 1
		}
		maxOut := imageGenDefaultCompletionTokens
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, maxOut)
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
		patched, err = patchImageGenUpstreamJSON(patched)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)

		start := time.Now()
		switch ct {
		case "openai", "deepseek":
			resp, err := relay.ForwardOpenAIImageGenerations(c.Request.Context(), baseURL, secret, patched)
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
			pt2, ct2 := parseUsageFromOpenAIResponse(raw)
			if pt2 == 0 {
				pt2 = promptToks
			}
			if ct2 == 0 {
				ct2 = maxOut
			}
			cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
			_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
			c.Data(resp.StatusCode, "application/json", raw)

		default:
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型暂不支持图片生成，请使用 OpenAI 兼容渠道（如硅基流动配置为 openai 类型）")
		}
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
