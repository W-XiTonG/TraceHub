package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/gateway/router"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"

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
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetModel(model)
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
		pathUnderV1 := strings.TrimPrefix(strings.TrimSuffix(c.Request.URL.Path, "/"), "/v1/")
		if pathUnderV1 == "" {
			pathUnderV1 = "realtime"
		}

		dialer := websocket.Dialer{
			HandshakeTimeout: 45 * time.Second,
			Subprotocols:     parseSecWebSocketProtocols(c.GetHeader("Sec-WebSocket-Protocol")),
		}
		sel, upConn, dialErr := dialRealtimeUpstreamWithFailover(
			c.Request.Context(), d, int64(u.ID), gid, model, sticky, pathUnderV1, q, c.Request.Header, dialer)
		if dialErr != nil {
			middleware.WriteOpenAIError(c, 503, "api_error", MsgServiceUnavailable)
			return
		}
		if upConn == nil {
			middleware.WriteOpenAIError(c, 403, "permission_error", "没有可用渠道支持该模型")
			return
		}
		defer upConn.Close()

		ch := sel.Channel
		if rec := UsageRecorderFrom(c); rec != nil {
			rec.SetChannel(ch.ID, sel.UpstreamModel)
		}
		chType := strings.ToLower(strings.TrimSpace(ch.ChannelType))
		if !router.UsesOpenAICompatRelay(chType, d.OpenAICompatExtras) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", MsgCapabilityUnsupported)
			return
		}

		br, _ := d.Billing.ModelBillRates(c.Request.Context(), u, model, ch.ID)
		const promptToks = 4096
		const compToks = 4096
		var compAssumed int
		if br.NeedsTokenEstimate() {
			compAssumed = completionTokensFallback(compToks)
		}
		if _, ok := GatewayBillingPrecheck(c, u, br, func() (BillTokenEst, error) {
			return BillTokenEst{Prompt: promptToks, Completion: compAssumed}, nil
		}); !ok {
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		clientConn, err := realtimeUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer clientConn.Close()

		cost := GatewayFinalCost(br, promptToks, compToks, 0)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, true); err != nil {
			slog.Warn("realtime ws deduct failed", "user_id", u.ID, "err", err)
		}
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, promptToks, compToks, 0, cost, c.ClientIP(), 200, nil, time.Since(start))

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
