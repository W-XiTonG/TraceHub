package gateway

import (
	"context"
	"encoding/json"
	"io"
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

// Embeddings OpenAI 兼容 POST /v1/embeddings（OpenAI / DeepSeek 等渠道）。
func Embeddings(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, est, err := parseEmbeddingsBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 input 无效")
			return
		}
		openAIForwardJSONCustomPatch(c, d, body, model, est, 0, "embeddings", router.PatchModelInBody)
	}
}

// Completions OpenAI 兼容 POST /v1/completions（Legacy 文本补全）。
func Completions(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, promptText, maxOut, err := parseCompletionsBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 prompt 无效")
			return
		}
		openAIForwardJSONCustomPatch(c, d, body, model, promptText, maxOut, "completions", router.PatchModelInBody)
	}
}

// Moderations OpenAI 兼容 POST /v1/moderations。
func Moderations(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		model, est, err := parseModerationsBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 input 无效")
			return
		}
		openAIForwardJSONCustomPatch(c, d, body, model, est, 0, "moderations", router.PatchModelInBody)
	}
}

// openAIForwardJSONCustomPatch 将 JSON POST 转发至上游；patch 负责把请求体中的模型名替换为上游名（可为 router.PatchModelInBody 或 Realtime 专用补丁）。
func openAIForwardJSONCustomPatch(c *gin.Context, d ChatDeps, body []byte, model, promptEst string, maxCompFallback int, subpath string, patch func([]byte, string) ([]byte, error)) {
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
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
		return
	}

	promptToks, _ := d.Billing.EstimateTokens(previewSel.UpstreamModel, promptEst)
	if promptToks < 1 {
		promptToks = 1
	}
	preComp := maxCompFallback
	if preComp < 0 {
		preComp = 0
	}
	preChargeComp := preComp
	if preChargeComp > 0 {
		preChargeComp = completionTokensFallback(preChargeComp)
	}
	br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model)
	costPre := br.Cost(promptToks, preChargeComp, 0)
	if u.Balance < costPre {
		WriteInsufficientBalance402Precheck(c, u.Balance, costPre, promptToks, preChargeComp)
		return
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

	sel, res, err := ExecuteModelForward(c.Request.Context(), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), previewSel, body, patch, nil,
		func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
			if !ok {
				return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
			}
			resp, ferr := relay.ForwardOpenAIPostWithHeaders(ctx, baseURL, secret, subpath, patched, openAIPassthroughHeaders(c), "")
			return HTTPResponseToUpstreamResult(resp, ferr)
		})
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ctPreview, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	ch := sel.Channel
	ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

	if res.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, res.Body)
		WriteClientProxy(c, res.StatusCode, "application/json", res.Body)
		return
	}
	usage := parseUsageFlexible(res.Body)
	applyPromptFallback(&usage, promptToks)
	if maxCompFallback > 0 {
		applyCompletionFallback(&usage, maxCompFallback)
	}
	cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
	WriteClientProxy(c, res.StatusCode, "application/json", res.Body)
}

func parseEmbeddingsBody(body []byte) (model string, inputEst string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	if raw, ok := m["input"]; ok {
		inputEst = rawJSONToEstimateString(raw)
	}
	return strings.TrimSpace(model), inputEst, nil
}

func parseModerationsBody(body []byte) (model string, inputEst string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	if raw, ok := m["input"]; ok {
		inputEst = rawJSONToEstimateString(raw)
	}
	return strings.TrimSpace(model), inputEst, nil
}

func rawJSONToEstimateString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		return strings.Join(ss, "\n")
	}
	var nums []int
	if json.Unmarshal(raw, &nums) == nil {
		if len(nums) == 0 {
			return ""
		}
		return strings.Repeat("n ", len(nums))
	}
	return string(raw)
}

func parseCompletionsBody(body []byte) (model string, promptText string, maxOut int, err error) {
	maxOut = 512
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", "", 0, err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
	}
	if raw, ok := m["max_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
		}
	}
	if raw, ok := m["prompt"]; ok {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			promptText = s
		} else {
			var ss []string
			if json.Unmarshal(raw, &ss) == nil {
				promptText = strings.Join(ss, "")
			} else {
				promptText = string(raw)
			}
		}
	}
	return strings.TrimSpace(model), promptText, maxOut, nil
}
