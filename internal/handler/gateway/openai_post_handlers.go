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
	sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
	if err != nil {
		middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
		return
	}
	ch := sel.Channel
	ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
	if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持该 OpenAI 接口，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
		return
	}

	promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, promptEst)
	if promptToks < 1 {
		promptToks = 1
	}
	preComp := maxCompFallback
	if preComp < 0 {
		preComp = 0
	}
	costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, preComp)
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
	patched, err := patch(body, sel.UpstreamModel)
	if err != nil {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
		return
	}

	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()

	resp, err := relay.ForwardOpenAIPostWithHeaders(c.Request.Context(), baseURL, secret, subpath, patched, openAIPassthroughHeaders(c), "")
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
		pt2 = promptToks
	}
	if ct2 == 0 && maxCompFallback > 0 {
		ct2 = maxCompFallback
	}
	cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
	c.Data(resp.StatusCode, "application/json", raw)
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
