package gateway

import (
	"context"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/service/billing"

	"github.com/gin-gonic/gin"
)

func runClaudeOrGeminiChat(
	c *gin.Context,
	d ChatDeps,
	u *ent.User,
	k *ent.APIKey,
	gid int,
	model string,
	sticky string,
	body []byte,
	stream bool,
	maxOut int,
	promptToks int,
	br billing.ModelBillRates,
	sel *router.Selected,
	ct string,
	start time.Time,
	rid string,
	isClaude bool,
) {
	routeDeps := RoutingExecDeps{Client: d.Client, RDB: d.RDB, AESKey: d.AESKey}
	responseWritten := c.Writer.Written()
	if stream {
		responseWritten = false
	}
	sel, res, err := ExecuteModelForward(c.Request.Context(), routeDeps, int64(u.ID), gid, model, sticky, responseWritten, sel, body, nil, nil,
		func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error) {
			secret, baseURL, ok := ChannelSecretAndBaseURL(d.AESKey, sel.Channel)
			if !ok {
				return &router.UpstreamResult{StatusCode: 503}, errChannelNoSecret
			}
			if stream {
				var resp *http.Response
				var ferr error
				if isClaude {
					resp, ferr = relay.ForwardClaudeMessages(ctx, secret, baseURL, patched, sel.UpstreamModel, true)
				} else {
					resp, ferr = relay.ForwardGeminiGenerateContent(ctx, secret, baseURL, patched, sel.UpstreamModel, true)
				}
				return HTTPStreamResponseToUpstreamResult(resp, ferr)
			}
			if isClaude {
				out, status, err := relay.CallClaudeMessages(ctx, secret, baseURL, patched, sel.UpstreamModel, model)
				if err != nil && status >= 400 {
					return &router.UpstreamResult{StatusCode: status, Body: out}, nil
				}
				if err != nil {
					return nil, err
				}
				return &router.UpstreamResult{StatusCode: status, Body: out}, nil
			}
			out, status, err := relay.CallGeminiGenerateContent(ctx, secret, baseURL, patched, sel.UpstreamModel, model)
			if err != nil && status >= 400 {
				return &router.UpstreamResult{StatusCode: status, Body: out}, nil
			}
			if err != nil {
				return nil, err
			}
			return &router.UpstreamResult{StatusCode: status, Body: out}, nil
		})
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportErrorWithFailover(c, d.Client, gid, FailoverAttemptsFromResult(res), err)
		return
	}
	ch := sel.Channel
	ct = strings.ToLower(strings.TrimSpace(ch.ChannelType))
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
	if res.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(res.StatusCode)).Inc()
		WriteClientProxyWithFailover(c, d.Client, gid, res.StatusCode, "application/json", res.Body, res)
		return
	}
	if stream {
		resp := res.StreamResponse
		if resp == nil || resp.Body == nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
			middleware.WriteOpenAIError(c, 502, "api_error", "上游响应无效")
			return
		}
		defer relay.DrainBody(resp.Body)
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		var flusher http.Flusher
		if f, ok := c.Writer.(http.Flusher); ok {
			flusher = f
		}
		if c.Writer.Header().Get("X-Accel-Buffering") == "" {
			c.Writer.Header().Set("X-Accel-Buffering", "no")
		}
		c.Writer.WriteHeader(http.StatusOK)
		if flusher != nil {
			flusher.Flush()
		}
		var pt2, ct2 int
		var streamErr error
		if isClaude {
			pt2, ct2, streamErr = relay.StreamClaudeAnthropicBodyAsOpenAI(resp.Body, model, c.Writer, flusher)
		} else {
			pt2, ct2, streamErr = relay.StreamGeminiSSEBodyAsOpenAI(resp.Body, model, c.Writer, flusher)
		}
		if streamErr != nil {
			metrics.GatewayRequests.WithLabelValues(model, ct, "upstream_err").Inc()
			WriteGatewayTransportError(c, streamErr)
			return
		}
		if pt2 < 1 {
			pt2 = promptToks
		}
		if ct2 < 1 {
			ct2 = completionTokensFallback(maxOut)
		}
		cost := GatewayFinalCost(br, pt2, ct2, 0)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
			em := err.Error()
			metrics.GatewayRequests.WithLabelValues(model, ct, "deduct_err").Inc()
			RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, &em, time.Since(start))
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		RecordUpstreamOutcome(c.Request.Context(), d.Client, ch.ID, 200, nil, nil)
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, 0, cost, c.ClientIP(), 200, nil, time.Since(start))
		return
	}
	usage := ParseOpenAIUsageJSON(res.Body)
	applyPromptFallback(&usage, promptToks)
	applyCompletionFallback(&usage, maxOut)
	cost := GatewayFinalCost(br, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
	if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
		return
	}
	metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
	RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), res.StatusCode, nil, time.Since(start))
	WriteClientProxyWithFailover(c, d.Client, gid, res.StatusCode, "application/json", res.Body, res)
}
