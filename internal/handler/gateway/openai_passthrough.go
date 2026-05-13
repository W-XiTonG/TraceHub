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

// OpenAIPassthroughSettings 来自站点配置，控制 /v1 通用透传（白名单 + 可选 model 补丁）。
type OpenAIPassthroughSettings struct {
	Enabled          bool
	AllowPrefixes    []string
	DenyPrefixes     []string
	PatchModelInJSON bool
}

func pathMatchesPrefix(rest string, prefix string) bool {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return false
	}
	if rest == p {
		return true
	}
	return strings.HasPrefix(rest, p+"/")
}

func passthroughPathAllowed(rest string, allow, deny []string) bool {
	for _, d := range deny {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if pathMatchesPrefix(rest, d) {
			return false
		}
	}
	if len(allow) == 0 {
		return false
	}
	for _, a := range allow {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		if pathMatchesPrefix(rest, a) {
			return true
		}
	}
	return false
}

// OpenAIV1Passthrough 将 /v1 下未单独注册的路径按白名单转发到 OpenAI 兼容上游（鉴权、选路、替换 Authorization、可选 JSON model 补丁；计费按 JSON usage 或预估）。
func OpenAIV1Passthrough(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := d.Passthrough
		if !cfg.Enabled {
			middleware.WriteOpenAIError(c, http.StatusNotFound, "invalid_request_error", "not_found")
			return
		}

		fullPath := strings.TrimSpace(c.Request.URL.Path)
		rest := strings.TrimPrefix(fullPath, "/v1/")
		rest = strings.Trim(rest, "/")
		if rest == "" {
			middleware.WriteOpenAIError(c, http.StatusNotFound, "invalid_request_error", "not_found")
			return
		}
		if !passthroughPathAllowed(rest, cfg.AllowPrefixes, cfg.DenyPrefixes) {
			middleware.WriteOpenAIError(c, http.StatusNotFound, "invalid_request_error", "not_found")
			return
		}

		method := strings.ToUpper(strings.TrimSpace(c.Request.Method))
		var body []byte
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
				return
			}
		}

		model := extractJSONModelField(body)
		if model == "" {
			model = routingModelForStatelessResource(c)
		}
		if model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请在 JSON 中提供 model，或使用 Query model=… / 请求头 X-TraceHub-Model")
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
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 OpenAI 兼容透传，请使用 openai / deepseek 或配置的 openai_compat_channel_types")
			return
		}

		promptEst := ""
		maxComp := 0
		if len(body) > 0 && json.Valid(body) {
			promptEst, maxComp = passthroughBillingHints(body)
		}
		if promptEst == "" {
			promptEst = model
		}
		promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptEst)
		if promptToks < 1 {
			promptToks = 1
		}
		if maxComp < 0 {
			maxComp = 0
		}
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

		upBody := body
		if cfg.PatchModelInJSON && len(body) > 0 && json.Valid(body) {
			patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
				return
			}
			upBody = patched
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()
		rawQuery := c.Request.URL.RawQuery

		resp, err := relay.ForwardOpenAIHTTP(c.Request.Context(), baseURL, secret, method, rest, upBody, rawQuery, openAIPassthroughHeaders(c))
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		ctOut := resp.Header.Get("Content-Type")
		streamLike := strings.Contains(strings.ToLower(ctOut), "text/event-stream")

		if streamLike && resp.StatusCode < 400 {
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
				metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), resp.StatusCode, &em, time.Since(start))
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), resp.StatusCode, nil, time.Since(start))
			return
		}

		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, out)
			oct := ctOut
			if oct == "" {
				oct = "application/json"
			}
			c.Data(resp.StatusCode, oct, out)
			return
		}

		pt2, ct2 := parseUsageFlexible(out)
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
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), resp.StatusCode, nil, time.Since(start))
		oct := ctOut
		if oct == "" {
			oct = "application/json"
		}
		c.Data(resp.StatusCode, oct, out)
	}
}

func passthroughBillingHints(body []byte) (promptEst string, maxComp int) {
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return "", 0
	}
	if raw, ok := m["input"]; ok {
		promptEst = rawJSONToEstimateString(raw)
	}
	if raw, ok := m["messages"]; ok {
		promptEst = string(raw)
	}
	if raw, ok := m["prompt"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			promptEst = s
		} else {
			promptEst = string(raw)
		}
	}
	if raw, ok := m["max_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxComp = mt
		}
	}
	if raw, ok := m["max_completion_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxComp = mt
		}
	}
	if raw, ok := m["max_output_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxComp = mt
		}
	}
	return promptEst, maxComp
}
