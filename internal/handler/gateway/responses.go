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

// ResponsesCreate POST /v1/responses（OpenAI Responses API，推荐用于新集成；支持 stream SSE）。
func ResponsesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, stream, maxOut, inputRaw, err := parseResponsesBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或请求 JSON 无效")
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
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 Responses API，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
			return
		}

		var promptToks int
		if len(inputRaw) == 0 {
			promptToks, err = d.Billing.EstimateTokens(sel.UpstreamModel, "")
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法估算 input token")
				return
			}
		} else {
			promptToks, err = EstimateResponsesInputPromptTokens(inputRaw, sel.UpstreamModel, d.Billing)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "input 无效或无法估算 token")
				return
			}
		}
		if promptToks < 1 {
			promptToks = 1
		}
		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model)
		costPre := br.Cost(promptToks, completionTokensFallback(maxOut), 0)
		if u.Balance < costPre {
			middleware.WriteOpenAIError(c, 402, "insufficient_balance_error", balancePrecheck402Message())
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
		if stream {
			patched, _ = injectStreamUsageJSON(patched)
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		accept := ""
		if stream {
			accept = "text/event-stream"
		}
		resp, err := relay.ForwardOpenAIPostWithHeaders(c.Request.Context(), baseURL, secret, "responses", patched, openAIPassthroughHeaders(c), accept)
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		if stream {
			h := resp.Header.Get("Content-Type")
			if h != "" {
				c.Writer.Header().Set("Content-Type", h)
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
				pt = promptToks
			}
			if comp < 0 {
				comp = 0
			}
			if streamPT == 0 && streamCT == 0 {
				comp = completionTokensFallback(maxOut)
			}
			cost := br.Cost(pt, comp, cached)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
				em := err.Error()
				metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
			return
		}

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			c.Data(resp.StatusCode, "application/json", raw)
			return
		}
		usage := parseUsageFlexible(raw)
		applyPromptFallback(&usage, promptToks)
		applyCompletionFallback(&usage, maxOut)
		cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
		c.Data(resp.StatusCode, "application/json", raw)
	}
}

func parseResponsesBody(body []byte) (model string, stream bool, maxOut int, inputRaw json.RawMessage, err error) {
	maxOut = 2048
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", false, 0, nil, err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	if raw, ok := m["stream"]; ok {
		_ = json.Unmarshal(raw, &stream)
	}
	if raw, ok := m["max_output_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
		}
	}
	if raw, ok := m["input"]; ok {
		inputRaw = raw
	}
	return strings.TrimSpace(model), stream, maxOut, inputRaw, nil
}
