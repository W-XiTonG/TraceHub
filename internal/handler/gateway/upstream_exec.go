package gateway

import (
	"context"
	"io"
	"net/http"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/gateway/relay"
	"tracehub/internal/gateway/router"
	"tracehub/internal/pkg/openaicompat"
	"tracehub/internal/service/channelhealth"
	"tracehub/internal/service/channelsecret"
	"tracehub/internal/service/upstreamprobe"

	"github.com/redis/go-redis/v9"
)

// RoutingExecDeps 网关选路与 failover 依赖。
type RoutingExecDeps struct {
	Client *ent.Client
	RDB    *redis.Client
	AESKey []byte
}

func (d RoutingExecDeps) selectDeps() router.SelectDeps {
	return router.SelectDeps{RDB: d.RDB, Client: d.Client}
}

// RecordUpstreamOutcome 根据上游结果更新渠道 error_count；4xx/5xx 时 body 可解析为截断摘要。
func RecordUpstreamOutcome(ctx context.Context, client *ent.Client, channelID int, statusCode int, transportErr error, body []byte) {
	if transportErr != nil || statusCode >= 400 {
		msg := ""
		if transportErr != nil {
			msg = transportErr.Error()
		} else if len(body) > 0 {
			code, summary := upstreamprobe.ParseUpstreamErrorBody(body)
			switch {
			case code != "" && summary != "":
				msg = code + ": " + summary
			case summary != "":
				msg = summary
			case code != "":
				msg = code
			}
		}
		if len(msg) > 200 {
			msg = msg[:200]
		}
		_ = channelhealth.RecordUpstreamFailure(ctx, client, channelID, msg)
		return
	}
	_ = channelhealth.RecordUpstreamSuccess(ctx, client, channelID)
}

// ChannelSecretAndBaseURL 解析渠道凭证与 API 根地址。
func ChannelSecretAndBaseURL(aesKey []byte, ch *ent.Channel) (secret string, baseURL string, ok bool) {
	baseURL = strings.TrimSpace(ch.APIBaseURL)
	if baseURL == "" {
		baseURL = router.DefaultBaseURL(ch.ChannelType)
	} else if nb, err := openaicompat.NormalizeOpenAIAPIRoot(baseURL); err == nil {
		baseURL = nb
	}
	switch ch.AuthType {
	case channel.AuthTypeAPIKey:
		secret = channelsecret.Plaintext(aesKey, ch.APIKey)
	case channel.AuthTypeOauth:
		secret = channelsecret.Plaintext(aesKey, ch.OauthToken)
	case channel.AuthTypeSetupToken:
		secret = channelsecret.Plaintext(aesKey, ch.SetupToken)
	}
	return secret, baseURL, strings.TrimSpace(secret) != ""
}

// HTTPResponseToUpstreamResult 读取非流式 HTTP 响应为 UpstreamResult。
func HTTPResponseToUpstreamResult(resp *http.Response, transportErr error) (*router.UpstreamResult, error) {
	if transportErr != nil {
		return &router.UpstreamResult{TransportErr: transportErr}, transportErr
	}
	if resp == nil {
		return nil, nil
	}
	raw, _ := io.ReadAll(resp.Body)
	relay.DrainBody(resp.Body)
	return &router.UpstreamResult{
		StatusCode: resp.StatusCode,
		Body:       raw,
		Headers:    resp.Header,
	}, nil
}

// HTTPStreamResponseToUpstreamResult 流式尝试：失败时读尽 body 并关闭；200 时保留 *http.Response 供 SSE 透传。
func HTTPStreamResponseToUpstreamResult(resp *http.Response, transportErr error) (*router.UpstreamResult, error) {
	if transportErr != nil {
		return &router.UpstreamResult{TransportErr: transportErr}, transportErr
	}
	if resp == nil {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		relay.DrainBody(resp.Body)
		return &router.UpstreamResult{
			StatusCode: resp.StatusCode,
			Body:       raw,
			Headers:    resp.Header,
		}, nil
	}
	return &router.UpstreamResult{
		StatusCode:     resp.StatusCode,
		Headers:        resp.Header,
		StreamResponse: resp,
	}, nil
}

func (d RoutingExecDeps) executeWithFailoverRecorded(
	ctx context.Context,
	userID int64,
	groupID int,
	model string,
	sticky string,
	responseWritten bool,
	firstSel *router.Selected,
	attempt func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error),
) (*router.Selected, *router.UpstreamResult, error) {
	return router.ExecuteWithFailover(ctx, d.selectDeps(), userID, groupID, model, sticky, responseWritten, firstSel,
		func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
			res, err := attempt(ctx, sel)
			if res != nil {
				var body []byte
				if res != nil {
					body = res.Body
				}
				RecordUpstreamOutcome(ctx, d.Client, sel.Channel.ID, res.StatusCode, res.TransportErr, body)
			} else if err != nil {
				RecordUpstreamOutcome(ctx, d.Client, sel.Channel.ID, 0, err, nil)
			}
			return res, err
		}, nil)
}

// ExecuteModelForward 按模型选路并支持 failover；patchFn 为 nil 时使用 PatchModelInBody，patchExtra 可为 nil。
// firstSel 为计费预检阶段已选渠道时可传入，正常路径避免重复选路；failover 关闭时行为与单次选路一致。
func ExecuteModelForward(
	ctx context.Context,
	deps RoutingExecDeps,
	userID int64,
	groupID int,
	model string,
	sticky string,
	responseWritten bool,
	firstSel *router.Selected,
	rawBody []byte,
	patchFn func([]byte, string) ([]byte, error),
	patchExtra func([]byte, *ent.Channel) []byte,
	forward func(ctx context.Context, sel *router.Selected, patched []byte) (*router.UpstreamResult, error),
) (*router.Selected, *router.UpstreamResult, error) {
	patcher := patchFn
	if patcher == nil {
		patcher = router.PatchModelInBody
	}
	return deps.executeWithFailoverRecorded(ctx, userID, groupID, model, sticky, responseWritten, firstSel,
		func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error) {
			patched, err := patcher(rawBody, sel.UpstreamModel)
			if err != nil {
				return nil, err
			}
			if patchExtra != nil {
				patched = patchExtra(patched, sel.Channel)
			}
			return forward(ctx, sel, patched)
		})
}

// ExecuteModelHTTPForward 无请求体的 HTTP 转发（GET/DELETE 等）并支持 failover。
func ExecuteModelHTTPForward(
	ctx context.Context,
	deps RoutingExecDeps,
	userID int64,
	groupID int,
	model string,
	sticky string,
	firstSel *router.Selected,
	forward func(ctx context.Context, sel *router.Selected) (*router.UpstreamResult, error),
) (*router.Selected, *router.UpstreamResult, error) {
	return deps.executeWithFailoverRecorded(ctx, userID, groupID, model, sticky, false, firstSel, forward)
}
