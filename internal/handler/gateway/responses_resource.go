package gateway

import (
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

// routingModelForStatelessResource 用于 GET/DELETE Responses 等无法从路径推断模型的请求。
func routingModelForStatelessResource(c *gin.Context) string {
	m := strings.TrimSpace(c.Query("model"))
	if m != "" {
		return m
	}
	return strings.TrimSpace(c.GetHeader("X-TraceHub-Model"))
}

func normalizeResponseIDParam(c *gin.Context) (id string, ok bool) {
	id = strings.TrimSpace(c.Param("response_id"))
	if id == "" || strings.ContainsAny(id, "/\\") {
		return "", false
	}
	return id, true
}

// ResponsesGet GET /v1/responses/{response_id}。选路需 Query「model」或请求头 X-TraceHub-Model。
func ResponsesGet(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		responsesResourceHTTP(c, d, http.MethodGet, "")
	}
}

// ResponsesDelete DELETE /v1/responses/{response_id}。选路同 ResponsesGet。
func ResponsesDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		responsesResourceHTTP(c, d, http.MethodDelete, "")
	}
}

// ResponsesCancel POST /v1/responses/{response_id}/cancel。选路同 ResponsesGet。
func ResponsesCancel(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		responsesResourceHTTP(c, d, http.MethodPost, "cancel")
	}
}

func responsesResourceHTTP(c *gin.Context, d ChatDeps, method, tail string) {
	id, ok := normalizeResponseIDParam(c)
	if !ok {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid response id")
		return
	}
	model := routingModelForStatelessResource(c)
	if model == "" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请使用 Query 参数 model=…，或设置请求头 X-TraceHub-Model")
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
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 Responses 资源接口，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
		return
	}

	promptToks := 64
	maxOut := 512
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
		middleware.WriteOpenAIError(c, 503, "upstream_error", "渠道未配置有效凭证")
		return
	}

	rel := "responses/" + id
	if tail == "cancel" {
		rel += "/cancel"
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()

	resp, err := relay.ForwardOpenAIHTTP(c.Request.Context(), baseURL, secret, method, rel, nil, "", openAIPassthroughHeaders(c))
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
	usage := parseUsageFlexible(raw)
	applyPromptFallback(&usage, promptToks)
	applyCompletionFallback(&usage, maxOut)
	cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
	outCT := resp.Header.Get("Content-Type")
	if outCT == "" {
		outCT = "application/json"
	}
	c.Data(resp.StatusCode, outCT, raw)
}
