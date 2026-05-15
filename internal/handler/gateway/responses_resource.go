package gateway

import (
	"context"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"

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
	previewSel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
	if err != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
		return
	}
	ctPreview := strings.ToLower(strings.TrimSpace(previewSel.Channel.ChannelType))
	if !router.IsOpenAICompatChannelType(ctPreview, d.OpenAICompatExtras) {
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

	rel := "responses/" + id
	if tail == "cancel" {
		rel += "/cancel"
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

	sel, res, err := ExecuteModelHTTPForward(c.Request.Context(), routeDeps, int64(u.ID), gid, model, sticky, previewSel,
		func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
			if !ok {
				return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
			}
			resp, ferr := relay.ForwardOpenAIHTTP(ctx, baseURL, secret, method, rel, nil, "", openAIPassthroughHeaders(c))
			return HTTPResponseToUpstreamResult(resp, ferr)
		})
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ctPreview, "transport_err").Inc()
		middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
		return
	}
	ch := sel.Channel
	ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
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
		c.Data(res.StatusCode, outCT, res.Body)
		return
	}
	usage := parseUsageFlexible(res.Body)
	applyPromptFallback(&usage, promptToks)
	applyCompletionFallback(&usage, maxOut)
	cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
	outCT := "application/json"
	if res.Headers != nil {
		if h := res.Headers.Get("Content-Type"); h != "" {
			outCT = h
		}
	}
	c.Data(res.StatusCode, outCT, res.Body)
}
