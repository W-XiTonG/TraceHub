package gateway

import (
	"context"
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
	"tracehub/internal/service/billing"
	"tracehub/internal/service/channelsecret"
	"tracehub/internal/service/gatewayalert"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

// RealtimeCallGatewayConfig 控制 Realtime 通话子资源（accept/reject/hangup/refer）的映射与计费策略。
type RealtimeCallGatewayConfig struct {
	MappingEnabled   bool
	MappingTTLSec    int
	BillRejectHangup bool
}

type ChatDeps struct {
	Client  *ent.Client
	RDB     *redis.Client
	AESKey  []byte
	Billing *billing.Engine
	RTCall  RealtimeCallGatewayConfig
	// OpenAICompatExtras 为除 openai/deepseek 外、仍走 OpenAI 兼容 relay 的渠道类型名（见配置 site.openai_compat_channel_types）。
	OpenAICompatExtras []string
	Passthrough        OpenAIPassthroughSettings
	KlingNative        KlingNativeSettings
}

func ChatCompletions(d ChatDeps) gin.HandlerFunc {
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

		model, stream, maxOut, maxOutExplicit, _, err := parseChatBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 messages 无效")
			return
		}
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetModel(model)
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
		var promptToks, compAssumed int
		if est, err := EstimateChatPromptTokensFromRequestBody(body, model, d.Billing); err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "messages 无效或无法估算 token")
			return
		} else {
			promptToks = est
			if promptToks < 1 {
				promptToks = 1
			}
			compAssumed = precheckCompletionAssumed(maxOut, maxOutExplicit, 512)
		}
		sel, err := SelectChannelForBillingPrecheck(c.Request.Context(), d.Client, d.RDB, d.Billing, u, gid, model, sticky,
			BillTokenEst{Prompt: promptToks, Completion: compAssumed})
		if err != nil {
			middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
			return
		}

		ch := sel.Channel
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetChannel(ch.ID, sel.UpstreamModel)
		}

		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
		if br.NeedsTokenEstimate() {
			promptToks, err = EstimateChatPromptTokensFromRequestBody(body, sel.UpstreamModel, d.Billing)
			if err != nil {
				middleware.WriteOpenAIError(c, 400, "invalid_request_error", "messages 无效或无法估算 token")
				return
			}
			if promptToks < 1 {
				promptToks = 1
			}
			compAssumed = precheckCompletionAssumed(maxOut, maxOutExplicit, 512)
		}
		if _, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
			return BillTokenEst{Prompt: promptToks, Completion: compAssumed}, nil
		}); !ok {
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

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)

		start := time.Now()
		switch {
		case router.UsesOpenAICompatRelay(ct, d.OpenAICompatExtras):
			if stream {
				routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
				sel, res, err := ExecuteModelForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, false, sel, body, nil,
					nil,
					func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
						secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
						if !ok {
							return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
						}
						resp, ferr := relay.ForwardOpenAIChat(ctx, baseURL, secret, patched, true)
						return HTTPStreamResponseToUpstreamResult(resp, ferr)
					})
				if err != nil {
					metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
					RecordUsageTransportOutcome(c, sel, res, err)
					WriteGatewayTransportErrorWithFailover(c, d.Client, gid, FailoverAttemptsFromResult(res), err)
					return
				}
				ch = sel.Channel
				ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
				metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
				if res.StatusCode >= 400 {
					metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
					out := SanitizeClientProxyBody(res.StatusCode, "application/json", res.Body)
					logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, out)
					RecordUsageProxyOutcome(c, d.Client, u.ID, k.ID, sel, res, rid, model, c.ClientIP(), start)
					WriteClientProxyWithFailover(c, d.Client, gid, res.StatusCode, "application/json", out, res)
					return
				}
				resp := res.StreamResponse
				if resp == nil || resp.Body == nil {
					metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
					middleware.WriteOpenAIError(c, 502, "api_error", "上游响应无效")
					return
				}
				defer relay.DrainBody(resp.Body)
				h := resp.Header.Get("Content-Type")
				if h != "" {
					c.Writer.Header().Set("Content-Type", h)
				}
				if c.Writer.Header().Get("X-Accel-Buffering") == "" {
					c.Writer.Header().Set("X-Accel-Buffering", "no")
				}
				c.Writer.Header().Set("X-TraceHub-SSE-Passthrough", "v3")
				c.Writer.WriteHeader(resp.StatusCode)
				if f, ok := c.Writer.(http.Flusher); ok {
					f.Flush()
				}
				capture := copyOpenAIChatSSEWithUsage(c.Writer, resp.Body)
				usage := ResolveOpenAIStreamUsage(capture, d.Billing, sel.UpstreamModel, promptToks, maxOut, maxOutExplicit)
				pt := usage.PromptTokens
				comp := usage.CompletionTokens
				cached := usage.CachedPromptTokens
				if comp < 0 {
					comp = 0
				}
				cost := GatewayFinalCost(br, pt, comp, cached)
				if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
					em := err.Error()
					metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
					noteUsageOutcomeFromUpstreamResult(c, res)
					RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
					return
				}
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 200, nil, nil)
				noteUsageOutcomeFromUpstreamResult(c, res)
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), usageNoteForLog(usage.Note), time.Since(start))
				return
			}

			routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
			sel, res, err := ExecuteModelForward(GinContextWithFailoverAttemptHook(c, model), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), sel, body, nil, nil,
				func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
					secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
					if !ok {
						return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
					}
					resp, ferr := relay.ForwardOpenAIChat(ctx, baseURL, secret, patched, false)
					return HTTPResponseToUpstreamResult(resp, ferr)
				})
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
				RecordUsageTransportOutcome(c, sel, res, err)
				WriteGatewayTransportErrorWithFailover(c, d.Client, gid, FailoverAttemptsFromResult(res), err)
				return
			}
			ch = sel.Channel
			ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if res.StatusCode >= 400 {
				metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
				out := SanitizeClientProxyBody(res.StatusCode, "application/json", res.Body)
				logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, out)
				RecordUsageProxyOutcome(c, d.Client, u.ID, k.ID, sel, res, rid, model, c.ClientIP(), start)
				WriteClientProxyWithFailover(c, d.Client, gid, res.StatusCode, "application/json", out, res)
				return
			}
			usage := ParseOpenAIUsageJSON(res.Body)
			applyPromptFallback(&usage, promptToks)
			applyCompletionFallback(&usage, maxOut, maxOutExplicit)
			cost := GatewayFinalCost(br, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			noteUsageOutcomeFromUpstreamResult(c, res)
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
			WriteClientProxy(c, res.StatusCode, "application/json", res.Body)

		case ct == "claude":
			runClaudeOrGeminiChat(c, d, u, k, gid, model, sticky, body, stream, maxOut, promptToks, br, sel, ct, start, rid, true)

		case ct == "gemini":
			runClaudeOrGeminiChat(c, d, u, k, gid, model, sticky, body, stream, maxOut, promptToks, br, sel, ct, start, rid, false)

		default:
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "暂不支持的渠道类型: "+ch.ChannelType)
		}
	}
}

func parseChatBody(body []byte) (model string, stream bool, maxOut int, maxOutExplicit bool, promptText string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", false, 0, false, "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
		model = strings.TrimSpace(model)
	}
	if raw, ok := m["stream"]; ok {
		_ = json.Unmarshal(raw, &stream)
	}
	maxOut = 0
	maxOutExplicit = false
	if raw, ok := m["max_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
			maxOutExplicit = true
		}
	}
	if raw, ok := m["max_completion_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
			maxOutExplicit = true
		}
	}
	msgsRaw, ok := m["messages"]
	if !ok {
		return strings.TrimSpace(model), stream, maxOut, maxOutExplicit, "", nil
	}
	var messages []map[string]any
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return strings.TrimSpace(model), stream, maxOut, maxOutExplicit, "", err
	}
	var b strings.Builder
	for _, msg := range messages {
		if content, ok := msg["content"]; ok {
			switch v := content.(type) {
			case string:
				b.WriteString(v)
				b.WriteByte('\n')
			default:
				// 预检 token 由 EstimateChatPromptTokensFromRequestBody 单独解析；此处勿序列化含 base64 的 content
				b.WriteByte('\n')
			}
		}
	}
	return strings.TrimSpace(model), stream, maxOut, maxOutExplicit, b.String(), nil
}

// effectiveChannelBaseURL 解析渠道实际上游根地址（含留空时用类型默认）。
func effectiveChannelBaseURL(ch *ent.Channel) string {
	if ch == nil {
		return ""
	}
	base := strings.TrimSpace(ch.APIBaseURL)
	if base == "" {
		base = router.DefaultBaseURL(ch.ChannelType)
	} else if nb, err := openaicompat.NormalizeOpenAIAPIRoot(base); err == nil {
		base = nb
	}
	return strings.ToLower(base)
}

// upstreamSupportsOpenAIStreamOptions 是否可安全注入 OpenAI 专有 stream_options.include_usage。
// 仅官方 OpenAI/Azure 等明确支持该字段的上游需要注入；硅基、DeepSeek、自建反代等第三方兼容网关
// 往往在 chunk 内自带 usage，注入可能导致 SSE 提前断开（仅 1～2 个 chunk）。
func upstreamSupportsOpenAIStreamOptions(ch *ent.Channel) bool {
	base := effectiveChannelBaseURL(ch)
	if base == "" {
		return false
	}
	for _, host := range []string{"api.openai.com", "openai.azure.com"} {
		if strings.Contains(base, host) {
			return true
		}
	}
	return false
}

func maybeInjectStreamUsageJSON(body []byte, ch *ent.Channel) []byte {
	if !upstreamSupportsOpenAIStreamOptions(ch) {
		return body
	}
	out, err := injectStreamUsageJSON(body)
	if err != nil {
		return body
	}
	return out
}

func injectStreamUsageJSON(body []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return body, err
	}
	so, _ := json.Marshal(map[string]bool{"include_usage": true})
	m["stream_options"] = json.RawMessage(so)
	return json.Marshal(m)
}

func saveUsage(client *ent.Client, userID, apiKeyID, channelID int, requestID, model, upstream string, pt, ct, cachedPrompt int, cost int64, ip string, status int, errMsg *string, elapsed time.Duration, meta UsageLogMeta) {
	cost = usageCostForStatus(status, cost)
	if !meta.IsClientOutcome {
		cost = 0
		pt, ct, cachedPrompt = 0, 0, 0
	}
	ctx := context.Background()
	ms := int(elapsed.Milliseconds())
	attempt := meta.UpstreamAttempt
	if attempt < 1 {
		attempt = 1
	}
	creator := client.UsageLog.Create().
		SetUserID(userID).
		SetNillableAPIKeyID(&apiKeyID).
		SetModel(model).
		SetPromptTokens(pt).
		SetCompletionTokens(ct).
		SetTotalTokens(pt + ct).
		SetTotalCost(cost).
		SetResponseTimeMs(ms).
		SetNillableStatusCode(&status).
		SetNillableIPAddress(&ip).
		SetIsCached(cachedPrompt > 0).
		SetCachedTokens(cachedPrompt).
		SetUpstreamAttempt(attempt).
		SetIsClientOutcome(meta.IsClientOutcome).
		SetIsFailoverAttempt(meta.IsFailoverAttempt)
	if channelID > 0 {
		creator.SetChannelID(channelID)
	}
	if strings.TrimSpace(upstream) != "" {
		creator.SetModelUpstream(upstream)
	}
	if requestID != "" {
		creator.SetRequestID(requestID)
	}
	if errMsg != nil {
		creator.SetNillableErrorMessage(errMsg)
	}
	ulog, err := creator.Save(ctx)
	logArgs := []any{
		"user_id", userID,
		"api_key_id", apiKeyID,
		"channel_id", channelID,
		"routing_model", model,
		"upstream_model", upstream,
		"status_code", status,
		"prompt_tokens", pt,
		"completion_tokens", ct,
		"total_cost", cost,
		"duration_ms", ms,
	}
	if requestID != "" {
		logArgs = append(logArgs, "request_id", requestID)
	}
	logArgs = append(logArgs, "upstream_attempt", attempt, "is_client_outcome", meta.IsClientOutcome, "is_failover_attempt", meta.IsFailoverAttempt)
	if errMsg != nil {
		logArgs = append(logArgs, "bill_error", *errMsg)
	}
	if err != nil {
		slog.Warn("gateway usage log persist failed", append(logArgs, "err", err)...)
		return
	}
	if n := gatewayalert.Global(); n != nil {
		n.MaybeNotify(gatewayalert.EventFromUsage(
			userID, apiKeyID, channelID, requestID, model, upstream, status, errMsg,
			meta.IsFailoverAttempt, meta.IsClientOutcome,
		))
	}
	if channelID <= 0 {
		slog.Warn("gateway usage recorded without channel_id", append(logArgs, "usage_log_id", ulog.ID)...)
		return
	}
	slog.Info("gateway usage recorded", append(logArgs, "usage_log_id", ulog.ID)...)
}
