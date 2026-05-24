package gateway

import (
	"net/http"
	"net/url"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/gateway/router"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/channelsecret"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func dialRealtimeUpstreamWithFailover(
	c *gin.Context,
	d ChatDeps,
	userID int64,
	gid int,
	model string,
	sticky string,
	pathUnderV1 string,
	clientQuery url.Values,
	clientHeaders http.Header,
	dialer websocket.Dialer,
) (*router.Selected, *websocket.Conn, int, error) {
	ctx := c.Request.Context()
	cfg, err := router.ResolveRoutingConfig(ctx, d.Client, gid)
	if err != nil {
		return nil, nil, 0, err
	}
	maxAttempts := 1
	if cfg.FailoverEnabled {
		maxAttempts = cfg.FailoverMaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		if maxAttempts > 5 {
			maxAttempts = 5
		}
	}
	var exclude []int
	var lastSel *router.Selected
	var lastErr error
	attemptsUsed := 0
	for attemptN := 0; attemptN < maxAttempts; attemptN++ {
		sel, err := router.SelectChannelExcluding(ctx, d.RDB, d.Client, userID, gid, model, sticky, exclude)
		if err != nil {
			return lastSel, nil, attemptsUsed, err
		}
		lastSel = sel
		ch := sel.Channel
		attemptsUsed = attemptN + 1
		secret, baseURL, ok := channelSecretAndBase(d, ch)
		if !ok {
			lastErr = errChannelNoSecret
			after := append(append([]int(nil), exclude...), ch.ID)
			willRetry := attemptN+1 < maxAttempts && router.CountModelCandidates(ctx, d.Client, gid, model, after) >= 1
			if willRetry {
				recordRealtimeFailoverAttempt(c, model, sel, 503, errChannelNoSecret, attemptsUsed)
			}
			exclude = append(exclude, ch.ID)
			if !willRetry {
				break
			}
			continue
		}
		q2 := cloneQueryWithoutModel(clientQuery)
		q2.Set("model", sel.UpstreamModel)
		wsURL, err := openaicompat.WebSocketURLFromOpenAIRoot(baseURL, pathUnderV1, q2)
		if err != nil {
			return sel, nil, attemptsUsed, err
		}
		upHdr := realtimeUpstreamHeaders(secret, clientHeaders)
		upConn, upResp, err := dialer.DialContext(ctx, wsURL, upHdr)
		if upResp != nil && upResp.Body != nil {
			_ = upResp.Body.Close()
		}
		if err == nil {
			return sel, upConn, attemptsUsed, nil
		}
		lastErr = err
		RecordUpstreamOutcome(ctx, d.Client, ch.ID, 0, err, nil)
		if cfg.StickyEnabled && sticky != "" {
			router.ClearStickySession(ctx, d.RDB, userID, sticky)
		}
		after := append(append([]int(nil), exclude...), ch.ID)
		willRetry := attemptN+1 < maxAttempts &&
			router.IsRetryableUpstream(0, err) &&
			router.CountModelCandidates(ctx, d.Client, gid, model, after) >= 1
		if willRetry {
			recordRealtimeFailoverAttempt(c, model, sel, 502, err, attemptsUsed)
		}
		if !willRetry {
			break
		}
		exclude = append(exclude, ch.ID)
	}
	return lastSel, nil, attemptsUsed, lastErr
}

func recordRealtimeFailoverAttempt(c *gin.Context, model string, sel *router.Selected, status int, err error, attempt int) {
	rec := UsageRecorderFrom(c)
	if rec == nil || sel == nil {
		return
	}
	if m := strings.TrimSpace(model); m != "" {
		rec.SetModel(m)
	}
	var em *string
	if err != nil {
		msg := err.Error()
		if len(msg) > 200 {
			msg = msg[:200]
		}
		em = &msg
	}
	rec.SaveFailoverAttempt(sel.Channel.ID, sel.UpstreamModel, status, em, attempt, 0)
}

func channelSecretAndBase(d ChatDeps, ch *ent.Channel) (secret, baseURL string, ok bool) {
	baseURL = strings.TrimSpace(ch.APIBaseURL)
	if baseURL == "" {
		baseURL = router.DefaultBaseURL(ch.ChannelType)
	} else if nb, err := openaicompat.NormalizeOpenAIAPIRoot(baseURL); err == nil {
		baseURL = nb
	}
	switch ch.AuthType {
	case channel.AuthTypeAPIKey:
		secret = channelsecret.Plaintext(d.AESKey, ch.APIKey)
	case channel.AuthTypeOauth:
		secret = channelsecret.Plaintext(d.AESKey, ch.OauthToken)
	case channel.AuthTypeSetupToken:
		secret = channelsecret.Plaintext(d.AESKey, ch.SetupToken)
	}
	return secret, baseURL, strings.TrimSpace(secret) != ""
}

func cloneQueryWithoutModel(q url.Values) url.Values {
	q2 := url.Values{}
	for k, vals := range q {
		if strings.EqualFold(k, "model") {
			continue
		}
		for _, v := range vals {
			q2.Add(k, v)
		}
	}
	return q2
}

func realtimeUpstreamHeaders(secret string, clientHeaders http.Header) http.Header {
	upHdr := http.Header{}
	upHdr.Set("Authorization", "Bearer "+secret)
	for _, k := range []string{"OpenAI-Beta", "OpenAI-Organization", "OpenAI-Project", "OpenAI-Safety-Identifier", "User-Agent"} {
		if v := strings.TrimSpace(clientHeaders.Get(k)); v != "" {
			upHdr.Set(k, v)
		}
	}
	return upHdr
}
