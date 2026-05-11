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
	MappingEnabled       bool
	MappingTTLSec        int
	BillRejectHangup     bool
}

type ChatDeps struct {
	Client  *ent.Client
	RDB     *redis.Client
	AESKey  []byte
	Billing *billing.Engine
	RTCall  RealtimeCallGatewayConfig
	// OpenAICompatExtras 为除 openai/deepseek 外、仍走 OpenAI 兼容 relay 的渠道类型名（见配置 site.openai_compat_channel_types）。
	OpenAICompatExtras []string
	Passthrough          OpenAIPassthroughSettings
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

		model, stream, maxOut, promptText, err := parseChatBody(body)
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少 model 或 messages 无效")
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
		if stream && router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			patched, _ = injectStreamUsageJSON(patched)
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)

		start := time.Now()
		switch {
		case router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras):
			resp, err := relay.ForwardOpenAIChat(c.Request.Context(), baseURL, secret, patched, stream)
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
				_, _ = copyStreamFlushing(c.Writer, resp.Body)
				pt := promptToks
				comp := maxOut
				cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt, comp)
				_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt, comp, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
				return
			}

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
						c.Data(u.StatusCode, "application/json", u.Body)
						return
					}
					metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
					middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
					return
				}
				if pt2 == 0 {
					pt2 = promptToks
				}
				if ct2 == 0 {
					ct2 = maxOut
				}
				cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
				_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), 200, nil, time.Since(start))
				return
			}
			out, status, err := relay.CallClaudeMessages(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
				if status >= 400 {
					c.Data(status, "application/json", out)
					return
				}
				middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
				return
			}
			pt2, ct2 := parseUsageFromOpenAIResponse(out)
			cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
			_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), status, nil, time.Since(start))
			c.Data(status, "application/json", out)

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
						c.Data(u.StatusCode, "application/json", u.Body)
						return
					}
					metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
					middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
					return
				}
				if pt2 == 0 {
					pt2 = promptToks
				}
				if ct2 == 0 {
					ct2 = maxOut
				}
				cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
				_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
				metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
				go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), 200, nil, time.Since(start))
				return
			}
			out, status, err := relay.CallGeminiGenerateContent(c.Request.Context(), secret, baseURL, patched, sel.UpstreamModel, model)
			metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
			if err != nil {
				metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
				if status >= 400 {
					c.Data(status, "application/json", out)
					return
				}
				middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
				return
			}
			pt2, ct2 := parseUsageFromOpenAIResponse(out)
			cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
			_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
			metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), status, nil, time.Since(start))
			c.Data(status, "application/json", out)

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
		return model, stream, maxOut, "", nil
	}
	var messages []map[string]any
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return model, stream, maxOut, "", err
	}
	var b strings.Builder
	for _, msg := range messages {
		if content, ok := msg["content"]; ok {
			switch v := content.(type) {
			case string:
				b.WriteString(v)
				b.WriteByte('\n')
			default:
				bytes, _ := json.Marshal(v)
				b.Write(bytes)
				b.WriteByte('\n')
			}
		}
	}
	return model, stream, maxOut, b.String(), nil
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

func parseUsageFromOpenAIResponse(raw []byte) (promptTokens, completionTokens int) {
	var wrap struct {
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &wrap) == nil {
		return wrap.Usage.PromptTokens, wrap.Usage.CompletionTokens
	}
	return 0, 0
}

func saveUsage(client *ent.Client, userID, apiKeyID, channelID int, requestID, model, upstream string, pt, ct int, cost int64, ip string, status int, errMsg *string, elapsed time.Duration) {
	ctx := context.Background()
	ms := int(elapsed.Milliseconds())
	creator := client.UsageLog.Create().
		SetUserID(userID).
		SetNillableAPIKeyID(&apiKeyID).
		SetNillableChannelID(&channelID).
		SetModel(model).
		SetNillableModelUpstream(&upstream).
		SetPromptTokens(pt).
		SetCompletionTokens(ct).
		SetTotalTokens(pt + ct).
		SetTotalCost(cost).
		SetResponseTimeMs(ms).
		SetNillableStatusCode(&status).
		SetNillableIPAddress(&ip)
	if requestID != "" {
		creator.SetRequestID(requestID)
	}
	if errMsg != nil {
		creator.SetNillableErrorMessage(errMsg)
	}
	_, _ = creator.Save(ctx)
}
