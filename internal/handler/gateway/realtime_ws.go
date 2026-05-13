package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/channelsecret"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var realtimeUpgrader = websocket.Upgrader{
	ReadBufferSize:    4096,
	WriteBufferSize:   4096,
	EnableCompression: true,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// RealtimeWebSocket 代理 OpenAI 兼容 Realtime WebSocket（常见：wss://…/v1/realtime?model=…）。
// 同时支持子路径，如 /v1/realtime/translations（与上游路径一致）。
func RealtimeWebSocket(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !websocket.IsWebSocketUpgrade(c.Request) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "WebSocket upgrade required")
			return
		}

		q := c.Request.URL.Query()
		model := strings.TrimSpace(q.Get("model"))
		if model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing model query parameter")
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
		chType := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.IsOpenAICompatChannelType(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 Realtime WebSocket，请使用 OpenAI 兼容渠道（openai / deepseek 或 site.openai_compat_channel_types）")
			return
		}

		// Realtime 会话按固定占位计费（上游 usage 在 WS 帧内，不在此解析）。
		const promptToks = 4096
		const compToks = 4096
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, compToks)
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

		pathUnderV1 := strings.TrimPrefix(strings.TrimSuffix(c.Request.URL.Path, "/"), "/v1/")
		if pathUnderV1 == "" {
			pathUnderV1 = "realtime"
		}

		q2 := url.Values{}
		for k, vals := range q {
			if strings.EqualFold(k, "model") {
				continue
			}
			for _, v := range vals {
				q2.Add(k, v)
			}
		}
		q2.Set("model", sel.UpstreamModel)

		wsURL, err := openaicompat.WebSocketURLFromOpenAIRoot(baseURL, pathUnderV1, q2)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法构造上游 WebSocket URL")
			return
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: 45 * time.Second,
			Subprotocols:     parseSecWebSocketProtocols(c.GetHeader("Sec-WebSocket-Protocol")),
		}
		upHdr := http.Header{}
		upHdr.Set("Authorization", "Bearer "+secret)
		for _, k := range []string{"OpenAI-Beta", "OpenAI-Organization", "OpenAI-Project", "OpenAI-Safety-Identifier", "User-Agent"} {
			if v := strings.TrimSpace(c.GetHeader(k)); v != "" {
				upHdr.Set(k, v)
			}
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		clientConn, err := realtimeUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer clientConn.Close()

		upConn, upResp, err := dialer.DialContext(c.Request.Context(), wsURL, upHdr)
		if err != nil {
			if upResp != nil && upResp.Body != nil {
				_ = upResp.Body.Close()
			}
			_ = clientConn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "upstream connection failed"),
				time.Now().Add(5*time.Second))
			return
		}
		if upResp != nil && upResp.Body != nil {
			_ = upResp.Body.Close()
		}
		defer upConn.Close()

		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, compToks)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
			slog.Warn("realtime ws deduct failed", "user_id", u.ID, "err", err)
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, promptToks, compToks, cost, c.ClientIP(), 200, nil, time.Since(start))

		t0 := time.Now()
		bridgeWebSockets(c.Request.Context(), clientConn, upConn)
		metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(t0).Seconds())
	}
}

func parseSecWebSocketProtocols(h string) []string {
	if strings.TrimSpace(h) == "" {
		return nil
	}
	parts := strings.Split(h, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func bridgeWebSockets(ctx context.Context, client, upstream *websocket.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	closeOnce := sync.Once{}
	closeBoth := func() {
		closeOnce.Do(func() {
			deadline := time.Now().Add(2 * time.Second)
			_ = client.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				deadline)
			_ = upstream.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
				deadline)
			_ = client.Close()
			_ = upstream.Close()
		})
	}

	pump := func(from, to *websocket.Conn) {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				closeBoth()
				return
			}
			_ = from.SetReadDeadline(time.Now().Add(120 * time.Second))
			mt, r, err := from.ReadMessage()
			if err != nil {
				closeBoth()
				return
			}
			_ = to.SetWriteDeadline(time.Now().Add(120 * time.Second))
			if err := to.WriteMessage(mt, r); err != nil {
				closeBoth()
				return
			}
		}
	}

	go pump(client, upstream)
	go pump(upstream, client)
	wg.Wait()
}
