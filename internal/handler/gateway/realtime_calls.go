package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

// RealtimeCallsWebRTC POST /v1/realtime/calls（OpenAI WebRTC 统一接口：multipart，字段 sdp + session）。
// 参见 https://developers.openai.com/api/docs/guides/realtime-webrtc
func RealtimeCallsWebRTC(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := strings.ToLower(c.GetHeader("Content-Type"))
		if !strings.HasPrefix(ct, "multipart/form-data") {
			middleware.WriteOpenAIError(c, 415, "invalid_request_error", "Content-Type must be multipart/form-data with sdp and session fields (OpenAI WebRTC unified interface)")
			return
		}
		if err := c.Request.ParseMultipartForm(maxAudioMultipartMemory); err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
			return
		}
		form := c.Request.MultipartForm
		defer func() {
			if form != nil {
				_ = form.RemoveAll()
			}
		}()

		sessionStr := readFirstFormString(form, "session")
		if strings.TrimSpace(sessionStr) == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing session field")
			return
		}
		sdpStr := readFirstFormString(form, "sdp")
		if strings.TrimSpace(sdpStr) == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing sdp field")
			return
		}

		model, err := modelFromRealtimeSessionJSON([]byte(sessionStr), c.Request.URL.Query())
		if err != nil || model == "" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法在 session JSON 中解析 model，请在 session 内设置 model 或 Query 增加 model=")
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
		if chType != "openai" && chType != "deepseek" {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "当前渠道类型不支持 Realtime calls，请使用 openai / deepseek 渠道")
			return
		}

		promptToks, _ := d.Billing.EstimateTokens(sel.UpstreamModel, sessionStr+"\n"+sdpStr)
		if promptToks < 1 {
			promptToks = 1
		}
		maxComp := 4096
		costPre, _ := d.Billing.Charge(c.Request.Context(), u, model, promptToks, maxComp)
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

		patchedSession, err := router.PatchModelInBody([]byte(sessionStr), sel.UpstreamModel)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "session 不是合法 JSON")
			return
		}

		body, contentType, err := rebuildRealtimeCallsMultipart(form, sdpStr, string(patchedSession))
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart")
			return
		}

		reqID, _ := c.Get("request_id")
		rid, _ := reqID.(string)
		start := time.Now()

		resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, "realtime/calls", contentType, body, openAIPassthroughHeaders(c), "")
		if err != nil {
			metrics.GatewayRequests.WithLabelValues(model, chType, "transport_err").Inc()
			middleware.WriteOpenAIError(c, 503, "upstream_error", err.Error())
			return
		}
		defer relay.DrainBody(resp.Body)
		metrics.GatewayLatency.WithLabelValues(chType).Observe(time.Since(start).Seconds())

		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			metrics.GatewayRequests.WithLabelValues(model, chType, http.StatusText(resp.StatusCode)).Inc()
			logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
			outCT := resp.Header.Get("Content-Type")
			if outCT == "" {
				outCT = "application/json"
			}
			c.Data(resp.StatusCode, outCT, raw)
			return
		}
		pt2, ct2 := parseUsageFlexible(raw)
		if pt2 == 0 {
			pt2 = promptToks
		}
		if ct2 == 0 {
			ct2 = maxComp
		}
		cost, _ := d.Billing.Charge(c.Request.Context(), u, model, pt2, ct2)
		_ = d.Billing.DeductBalance(c.Request.Context(), int64(u.ID), cost)
		metrics.GatewayRequests.WithLabelValues(model, chType, "200").Inc()
		go saveUsage(d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, pt2, ct2, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))

		outCT := resp.Header.Get("Content-Type")
		if outCT == "" {
			outCT = "application/sdp"
		}
		c.Data(resp.StatusCode, outCT, raw)
	}
}

func readFirstFormString(form *multipart.Form, name string) string {
	if form == nil {
		return ""
	}
	if vs := form.Value[name]; len(vs) > 0 {
		return strings.Join(vs, "")
	}
	if fhs, ok := form.File[name]; ok && len(fhs) > 0 {
		f, err := fhs[0].Open()
		if err != nil {
			return ""
		}
		defer f.Close()
		b, _ := io.ReadAll(f)
		return string(b)
	}
	return ""
}

func modelFromRealtimeSessionJSON(sessionBytes []byte, q url.Values) (string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(sessionBytes, &m); err != nil {
		return "", err
	}
	if raw, ok := m["model"]; ok {
		var s string
		_ = json.Unmarshal(raw, &s)
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s), nil
		}
	}
	if s := strings.TrimSpace(q.Get("model")); s != "" {
		return s, nil
	}
	return "", nil
}

func rebuildRealtimeCallsMultipart(form *multipart.Form, sdpFinal, sessionFinal string) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, name := range sortedFormValueKeys(form.Value) {
		if name == "sdp" || name == "session" {
			continue
		}
		for _, v := range form.Value[name] {
			if err := w.WriteField(name, v); err != nil {
				return nil, "", err
			}
		}
	}
	for _, name := range sortedFormFileKeys(form.File) {
		if name == "sdp" || name == "session" {
			continue
		}
		for _, fh := range form.File[name] {
			fn := fh.Filename
			if fn == "" {
				fn = name
			}
			part, err := w.CreateFormFile(name, fn)
			if err != nil {
				return nil, "", err
			}
			f, err := fh.Open()
			if err != nil {
				return nil, "", err
			}
			_, err = io.Copy(part, f)
			_ = f.Close()
			if err != nil {
				return nil, "", err
			}
		}
	}
	if err := w.WriteField("sdp", sdpFinal); err != nil {
		return nil, "", err
	}
	if err := w.WriteField("session", sessionFinal); err != nil {
		return nil, "", err
	}
	ct := w.FormDataContentType()
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return []byte(buf.String()), ct, nil
}
