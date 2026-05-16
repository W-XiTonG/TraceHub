package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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

		model, stream, maxOut, _, err := parseChatBody(body)
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
		sel, err := router.SelectChannel(c.Request.Context(), d.RDB, d.Client, int64(u.ID), gid, model, sticky)
		if err != nil {
			middleware.WriteOpenAIError(c, 403, "permission_error", err.Error())
			return
		}

		ch := sel.Channel
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetChannel(ch.ID, sel.UpstreamModel)
		}

		promptToks, err := EstimateChatPromptTokensFromRequestBody(body, sel.UpstreamModel, d.Billing)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "messages 无效或无法估算 token")
			return
		}
		if promptToks < 1 {
			promptToks = 1
		}
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

		patched, err := router.PatchModelInBody(body, sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}
		if stream && router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			patched, _ = injectStreamUsageJSON(patched)
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)

		start := time.Now()
		switch {
		case router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras):
			if stream {
				resp, err := relay.ForwardOpenAIChat(c.Request.Context(), baseURL, secret, patched, true)
				if err != nil {
					RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 0, err)
					metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
					WriteGatewayTransportError(c, err)
					return
				}
				defer relay.DrainBody(resp.Body)
				metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

				if resp.StatusCode >= 400 {
					rawErr, _ := io.ReadAll(resp.Body)
					RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, resp.StatusCode, nil)
					out := SanitizeClientProxyBody(resp.StatusCode, "application/json", rawErr)
					c.Writer.Header().Set("Content-Type", "application/json")
					c.Writer.WriteHeader(resp.StatusCode)
					if f, ok := c.Writer.(http.Flusher); ok {
						f.Flush()
					}
					_, _ = c.Writer.Write(out)
					metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
					logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, out)
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
				cost := br.Cost(pt, comp, cached)
				if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
					em := err.Error()
					metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
					RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), &em, time.Since(start))
					return
				}
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 200, nil)
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cached, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
				return
			}

			routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
			sel, res, err := ExecuteModelForward(c.Request.Context(), routeDeps, int64(u.ID), gid, model, sticky, c.Writer.Written(), sel, body, nil, nil,
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
				WriteGatewayTransportError(c, err)
				return
			}
			ch = sel.Channel
			ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if res.StatusCode >= 400 {
				metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
				out := SanitizeClientProxyBody(res.StatusCode, "application/json", res.Body)
				logUpstreamQuotaDeclineIfLikely(c, res.StatusCode, out)
				WriteClientProxy(c, res.StatusCode, "application/json", out)
				return
			}
			usage := ParseOpenAIUsageJSON(res.Body)
			applyPromptFallback(&usage, promptToks)
			applyCompletionFallback(&usage, maxOut)
			cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
			WriteClientProxy(c, res.StatusCode, "application/json", res.Body)

		case ct == "claude":
			if stream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
				var flusher http.Flusher
				if f, ok := c.Writer.(http.Flusher); ok {
					flusher = f
				}
				pt2, ct2, err := relay.StreamClaudeMessagesAsOpenAIChatSSE(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model, c.Writer, flusher)
				metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
				if err != nil {
					var u *relay.UpstreamJSONError
					if errors.As(err, &u) {
						metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(u.StatusCode)).Inc()
						WriteClientProxy(c, u.StatusCode, "application/json", u.Body)
						return
					}
					metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
					WriteGatewayTransportError(c, err)
					return
				}
				if pt2 == 0 {
					pt2 = promptToks
				}
				if ct2 == 0 {
					ct2 = completionTokensFallback(maxOut)
				}
				cost := br.Cost(pt2, ct2, 0)
				if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
					em := err.Error()
					metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
					RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, &em, time.Since(start))
					return
				}
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, nil, time.Since(start))
				return
			}
			out, status, err := relay.CallClaudeMessages(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
				if status >= 400 {
					WriteClientProxy(c, status, "application/json", out)
					return
				}
				WriteGatewayTransportError(c, err)
				return
			}
			usage := ParseOpenAIUsageJSON(out)
			applyPromptFallback(&usage, promptToks)
			applyCompletionFallback(&usage, maxOut)
			cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), status, nil, time.Since(start))
			WriteClientProxy(c, status, "application/json", out)

		case ct == "gemini":
			if stream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
				var flusher http.Flusher
				if f, ok := c.Writer.(http.Flusher); ok {
					flusher = f
				}
				pt2, ct2, err := relay.StreamGeminiGenerateContentAsOpenAIChatSSE(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model, c.Writer, flusher)
				metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
				if err != nil {
					var u *relay.UpstreamJSONError
					if errors.As(err, &u) {
						metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(u.StatusCode)).Inc()
						WriteClientProxy(c, u.StatusCode, "application/json", u.Body)
						return
					}
					metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
					WriteGatewayTransportError(c, err)
					return
				}
				if pt2 == 0 {
					pt2 = promptToks
				}
				if ct2 == 0 {
					ct2 = completionTokensFallback(maxOut)
				}
				cost := br.Cost(pt2, ct2, 0)
				if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
					em := err.Error()
					metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
					RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, &em, time.Since(start))
					return
				}
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, nil, time.Since(start))
				return
			}
			out, status, err := relay.CallGeminiGenerateContent(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
				if status >= 400 {
					WriteClientProxy(c, status, "application/json", out)
					return
				}
				WriteGatewayTransportError(c, err)
				return
			}
			usage := ParseOpenAIUsageJSON(out)
			applyPromptFallback(&usage, promptToks)
			applyCompletionFallback(&usage, maxOut)
			cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
				return
			}
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), status, nil, time.Since(start))
			WriteClientProxy(c, status, "application/json", out)

		default:
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "暂不支持的渠道类型: "+ch.ChannelType)
		}
	}
}

func parseChatBody(body []byte) (model string, stream bool, maxOut int, promptText string, err error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", false, 0, "", err
	}
	if raw, ok := m["model"]; ok {
		_ = json.Unmarshal(raw, &model)
		model = strings.TrimSpace(model)
	}
	if raw, ok := m["stream"]; ok {
		_ = json.Unmarshal(raw, &stream)
	}
	maxOut = 512
	if raw, ok := m["max_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
		}
	}
	if raw, ok := m["max_completion_tokens"]; ok {
		var mt int
		if json.Unmarshal(raw, &mt) == nil && mt > 0 {
			maxOut = mt
		}
	}
	msgsRaw, ok := m["messages"]
	if !ok {
		return strings.TrimSpace(model), stream, maxOut, "", nil
	}
	var messages []map[string]any
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return strings.TrimSpace(model), stream, maxOut, "", err
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
	return strings.TrimSpace(model), stream, maxOut, b.String(), nil
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

// copyOpenAIChatSSEWithUsage 透传 OpenAI 兼容 SSE，并解析各 data 行中最后一个非空 usage（依赖 stream_options.include_usage）。
// 写入经 WriterFlushWrap，与 copyStreamFlushing 一致，避免缓冲导致流式卡顿。
func copyOpenAIChatSSEWithUsage(w http.ResponseWriter, r io.Reader) (promptTokens, completionTokens, cachedPromptTokens int) {
	out := WriterFlushWrap(w)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if _, werr := io.WriteString(out, line); werr != nil {
			return promptTokens, completionTokens, cachedPromptTokens
		}
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(s, "data:"))
			if payload != "" && payload != "[DONE]" {
				rawPayload := []byte(payload)
				u := ParseOpenAIUsageJSON(rawPayload)
				if u.PromptTokens > 0 || u.CompletionTokens > 0 || u.CachedPromptTokens > 0 {
					promptTokens, completionTokens, cachedPromptTokens = u.PromptTokens, u.CompletionTokens, u.CachedPromptTokens
				} else {
					flex := parseUsageFlexible(rawPayload)
					if flex.PromptTokens > 0 || flex.CompletionTokens > 0 || flex.CachedPromptTokens > 0 {
						promptTokens, completionTokens, cachedPromptTokens = flex.PromptTokens, flex.CompletionTokens, flex.CachedPromptTokens
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return promptTokens, completionTokens, cachedPromptTokens
			}
			return promptTokens, completionTokens, cachedPromptTokens
		}
	}
}

func saveUsage(client *ent.Client, userID, apiKeyID, channelID int, requestID, model, upstream string, pt, ct, cachedPrompt int, cost int64, ip string, status int, errMsg *string, elapsed time.Duration) {
	ctx := context.Background()
	ms := int(elapsed.Milliseconds())
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
		SetCachedTokens(cachedPrompt)
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
	if errMsg != nil {
		logArgs = append(logArgs, "bill_error", *errMsg)
	}
	if err != nil {
		slog.Warn("gateway usage log persist failed", append(logArgs, "err", err)...)
		return
	}
	if channelID <= 0 {
		slog.Warn("gateway usage recorded without channel_id", append(logArgs, "usage_log_id", ulog.ID)...)
		return
	}
	slog.Info("gateway usage recorded", append(logArgs, "usage_log_id", ulog.ID)...)
}
