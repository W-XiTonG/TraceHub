package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/billing"
	"tracehub/internal/service/channelsecret"

	"github.com/gin-gonic/gin"
)

// RealtimeCallAccept POST /v1/realtime/calls/{call_id}/accept
func RealtimeCallAccept(d ChatDeps) gin.HandlerFunc {
	return realtimeCallSubresource(d, "accept")
}

// RealtimeCallReject POST /v1/realtime/calls/{call_id}/reject
func RealtimeCallReject(d ChatDeps) gin.HandlerFunc {
	return realtimeCallSubresource(d, "reject")
}

// RealtimeCallHangup POST /v1/realtime/calls/{call_id}/hangup
func RealtimeCallHangup(d ChatDeps) gin.HandlerFunc {
	return realtimeCallSubresource(d, "hangup")
}

// RealtimeCallRefer POST /v1/realtime/calls/{call_id}/refer
func RealtimeCallRefer(d ChatDeps) gin.HandlerFunc {
	return realtimeCallSubresource(d, "refer")
}

func realtimeCallSubresource(d ChatDeps, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		callID := strings.TrimSpace(c.Param("call_id"))
		if callID == "" || strings.ContainsAny(callID, "/\\") {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid call id")
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
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

		var mapped *realtimeCallMapping
		if d.RTCall.MappingEnabled && d.RDB != nil {
			var mErr error
			mapped, mErr = getRealtimeCallMapping(c.Request.Context(), d.RDB, k.ID, callID)
			if mErr != nil {
				middleware.WriteOpenAIError(c, 503, "api_error", "实时通话映射读取失败")
				return
			}
		}
		explicitModel := pickRealtimeCallControlModel(c, body)
		route, err := resolveRealtimeCallSubresourceRoute(c.Request.Context(), d, int64(u.ID), gid, sticky, explicitModel, mapped)
		if err != nil {
			if re, ok := err.(*realtimeCallRouteError); ok {
				middleware.WriteOpenAIError(c, re.Code, re.Type, re.Message)
				return
			}
			middleware.WriteOpenAIError(c, 500, "api_error", err.Error())
			return
		}

		ch := route.Channel
		ct := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.IsOpenAICompatChannelType(ct, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 Realtime 通话控制，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
			return
		}

		billMinor := (action == "reject" || action == "hangup") && d.RTCall.BillRejectHangup
		promptToks := 1
		maxComp := 2048
		promptEst := string(normalizeRealtimeJSONBody(body))
		if action == "reject" || action == "hangup" {
			maxComp = 0
			if !billMinor {
				promptToks = 0
			}
		}
		if promptToks > 0 {
			if promptEst == "" || promptEst == "{}" {
				promptEst = action
			}
			promptToks, _ = d.Billing.EstimateTokens(route.UpstreamModel, promptEst)
			if promptToks < 1 {
				promptToks = 1
			}
		}

		var costPre int64
		var br billing.ModelBillRates
		if promptToks > 0 {
			br, _ = d.Billing.ModelBillRates(c.Request.Context(), u, route.LogicalModel)
			compAssumed := completionTokensFallback(maxComp)
			costPre = br.Cost(promptToks, compAssumed, 0)
			if u.Balance < costPre {
				WriteInsufficientBalance402Precheck(c, u.Balance, costPre, promptToks, compAssumed)
				return
			}
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

		patched, err := patchRealtimeSubresourceBody(action, body, route.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "请求 JSON 无效")
			return
		}

		subpath := "realtime/calls/" + callID + "/" + action
		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		resp, err := relay.ForwardOpenAIPostWithHeaders(c.Request.Context(), baseURL, secret, subpath, patched, openAIPassthroughHeaders(c), "")
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(route.LogicalModel, ct, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(route.LogicalModel, ct, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			outCT := resp.Header.Get("Content-Type")
			if outCT == "" {
				outCT = "application/json"
			}
			c.Data(resp.StatusCode, outCT, raw)
			return
		}

		if promptToks > 0 {
			usage := parseUsageFlexible(raw)
			applyPromptFallback(&usage, promptToks)
			if maxComp > 0 {
				applyCompletionFallback(&usage, maxComp)
			}
			cost := br.Cost(usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
			if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
				return
			}
			go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, route.LogicalModel, route.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
		}

		metrics.GatewayRequests.WithLabelValues(route.LogicalModel, ct, "200").Inc()
		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/json"
		}
		c.Data(resp.StatusCode, outCT, raw)
	}
}

type realtimeCallRouteError struct {
	Code    int
	Type    string
	Message string
}

func (e *realtimeCallRouteError) Error() string { return e.Message }

type realtimeCallRoute struct {
	LogicalModel  string
	Channel       *ent.Channel
	UpstreamModel string
}

func pickRealtimeCallControlModel(c *gin.Context, body []byte) string {
	if m := extractJSONModelField(body); m != "" {
		return m
	}
	if m := strings.TrimSpace(c.Query("model")); m != "" {
		return m
	}
	return strings.TrimSpace(c.GetHeader("X-TraceHub-Model"))
}

func patchRealtimeSubresourceBody(action string, body []byte, upstream string) ([]byte, error) {
	b := normalizeRealtimeJSONBody(body)
	switch action {
	case "accept":
		return router.PatchModelInBody(b, upstream)
	default:
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		if _, ok := m["model"]; ok {
			m["model"] = upstream
			return json.Marshal(m)
		}
		return b, nil
	}
}

func resolveRealtimeCallSubresourceRoute(ctx context.Context, d ChatDeps, userID int64, gid int, sticky, explicitModel string, mapped *realtimeCallMapping) (*realtimeCallRoute, error) {
	if mapped != nil {
		if explicitModel != "" && explicitModel != mapped.LogicalModel {
			return nil, &realtimeCallRouteError{
				Code:    400,
				Type:    "invalid_request_error",
				Message: "Query / 请求头 / JSON 中的 model 与当前 call 映射不一致，请移除 model 或改为与创建通话时一致",
			}
		}
		ch, err := d.Client.Channel.Query().
			Where(
				channel.IDEQ(mapped.ChannelID),
				channel.Enabled(true),
				channel.Schedulable(true),
				channel.HealthStatusEQ(1),
				channel.HasGroupWith(group.ID(gid)),
			).
			Only(ctx)
		if err != nil {
			return nil, &realtimeCallRouteError{
				Code:    400,
				Type:    "invalid_request_error",
				Message: "实时通话映射已失效，请使用 Query 参数 model=…、请求头 X-TraceHub-Model 或在 JSON 中提供 model 后重试",
			}
		}
		if _, ok := router.MatchModelOnChannel(ch, mapped.LogicalModel); !ok {
			return nil, &realtimeCallRouteError{
				Code:    400,
				Type:    "invalid_request_error",
				Message: "渠道已不再支持该通话映射中的模型，请携带 model 重新选路",
			}
		}
		up := strings.TrimSpace(mapped.UpstreamModel)
		if up == "" {
			if u2, ok := router.MatchModelOnChannel(ch, mapped.LogicalModel); ok {
				up = u2
			}
		}
		return &realtimeCallRoute{
			LogicalModel:  mapped.LogicalModel,
			Channel:       ch,
			UpstreamModel: up,
		}, nil
	}

	if explicitModel == "" {
		return nil, &realtimeCallRouteError{
			Code:    400,
			Type:    "invalid_request_error",
			Message: "缺少用于选路的 model：请使用 Query 参数 model=…、请求头 X-TraceHub-Model、JSON 中的 model，或启用 Redis 通话映射（site.realtime_call_mapping_enabled）",
		}
	}

	sel, err := router.SelectChannel(ctx, d.RDB, d.Client, userID, gid, explicitModel, sticky)
	if err != nil {
		return nil, &realtimeCallRouteError{403, "permission_error", err.Error()}
	}
	return &realtimeCallRoute{
		LogicalModel:  explicitModel,
		Channel:       sel.Channel,
		UpstreamModel: sel.UpstreamModel,
	}, nil
}
