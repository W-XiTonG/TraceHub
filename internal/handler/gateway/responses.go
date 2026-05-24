package gateway

import (
	"context"
	"encoding/json"
	"errors"
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

var errChannelNoSecret = errors.New("渠道未配置有效凭证")

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

		previewSel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
		if err != nil {
			middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
			return
		}
		ctPreview := strings.ToLower(strings.TrimSpace(previewSel.Channel.ChannelType))
		if !router.UsesOpenAICompatRelay(ctPreview, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, previewSel.Channel.ID)
		var promptToks, compAssumed int
		if br.NeedsTokenEstimate() {
			if len(inputRaw) == 0 {
				promptToks, err = d.Billing.EstimateTokens(previewSel.UpstreamModel, "")
				if err != nil {
					middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法估算 input token")
					return
				}
			} else {
				promptToks, err = EstimateResponsesInputPromptTokens(inputRaw, previewSel.UpstreamModel, d.Billing)
				if err != nil {
					middleware.WriteOpenAIError(c, 400, "invalid_request_error", "input 无效或无法估算 token")
					return
				}
			}
			if promptToks < 1 {
				promptToks = 1
			}
			compAssumed = completionTokensFallback(maxOut)
		}
		if _, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
			return BillTokenEst{Prompt: promptToks, Completion: compAssumed}, nil
		}); !ok {
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()
		routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}

		if stream {
			sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
			if err != nil {
				middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
				return
			}
			ch := sel.Channel
			ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, ch)
			if !ok {
				WriteGatewayChannelConfigError(c)
				return
			}
			patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
				return
			}
			patched = maybeInjectStreamUsageJSON(patched, ch)
			accept := "text/event-stream"
			resp, err := relay.ForwardOpenAIPostWithHeaders(c.Request.Context(), baseURL, secret, "responses", patched, openAIPassthroughHeaders(c), accept)
			if err != nil {
				RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 0, err, nil)
				metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
				WriteGatewayTransportError(c, err)
				return
			}
			defer relay.DrainBody(resp.Body)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if resp.StatusCode >= 400 {
				rawErr, _ := io.ReadAll(resp.Body)
				RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, resp.StatusCode, nil, nil)
				metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
				logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, rawErr)
				WriteClientProxy(c, resp.StatusCode, "application/json", rawErr)
				return
			}
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
			cost := GatewayFinalCost(br,pt, comp, cached)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
				em := err.Error()
				metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
				return
			}
			RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 200, nil, nil)
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
			return
		}

		sel, res, err := ExecuteModelForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), previewSel, body, nil, nil,
			func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
				secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
				if !ok {
					return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
				}
				resp, ferr := relay.ForwardOpenAIPostWithHeaders(ctx, baseURL, secret, "responses", patched, openAIPassthroughHeaders(c), "")
				return HTTPResponseToUpstreamResult(resp, ferr)
			})
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, "openai", "transport_err").Inc()
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
		applyCompletionFallback(&usage, maxOut)
		cost := GatewayFinalCost(br,usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
		WriteClientProxy(c, res.StatusCode, "application/json", res.Body)
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
