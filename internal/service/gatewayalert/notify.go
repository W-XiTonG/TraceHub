package gatewayalert

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/alertconfig"
	"tracehub/internal/pkg/alertdelivery"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/mail"
	"tracehub/internal/pkg/sitecfg"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const deliverUserAgent = "TraceHub-GatewayAlert/1.0"

// Notifier 在客户网关调用异常时异步通知管理员。
type Notifier struct {
	client *ent.Client
	rdb    *redis.Client
	cfg    *config.Config
	log    *zap.Logger
	lim    *rateLimiter
}

func New(client *ent.Client, rdb *redis.Client, cfg *config.Config, log *zap.Logger) *Notifier {
	if cfg == nil {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Notifier{
		client: client,
		rdb:    rdb,
		cfg:    cfg,
		log:    log,
		lim:    newRateLimiter(rdb),
	}
}

// EventFromUsage 从 saveUsage 参数构造 Event。
func EventFromUsage(userID, apiKeyID, channelID int, requestID, model, upstream string, status int, errMsg *string, isFailoverAttempt, isClientOutcome bool) Event {
	em := ""
	if errMsg != nil {
		em = *errMsg
	}
	return Event{
		RequestID:         requestID,
		UserID:            userID,
		APIKeyID:          apiKeyID,
		ChannelID:         channelID,
		Model:             model,
		Upstream:          upstream,
		Status:            status,
		ErrMsg:            em,
		IsFailoverAttempt: isFailoverAttempt,
		IsClientOutcome:   isClientOutcome,
	}
}

// MaybeNotify 判断是否告警；限流通过后异步投递。
func (n *Notifier) MaybeNotify(evt Event) {
	if n == nil || n.client == nil {
		return
	}
	effective := sitecfg.CachedGatewayAdminAlert(context.Background(), n.client)
	if !effective.Enabled {
		return
	}
	kind, ok := Classify(evt, effective.Triggers)
	if !ok {
		return
	}
	fp := Fingerprint(kind, evt)
	ctx := context.Background()
	if !n.lim.tryDedup(ctx, fp, effective.CooldownSec) {
		return
	}
	allowExternal := true
	if effective.AnyExternalChannel() {
		allowExternal = n.lim.tryGlobalExternal(ctx, effective.MinIntervalSec)
	}
	go n.deliver(context.Background(), effective, evt, kind, allowExternal)
}

func (n *Notifier) deliver(ctx context.Context, effective sitecfg.GatewayAdminAlertEffective, evt Event, kind string, allowExternal bool) {
	title, summary, md, plain := formatMessages(kind, evt)
	if effective.Channels.InApp {
		n.persistInApp(ctx, kind, title, summary, evt)
	}
	if !allowExternal {
		return
	}
	ch := effective.Channels
	if ch.Email && smtpReady(&n.cfg.Email) && len(effective.Emails) > 0 {
		n.deliverEmail(summary, effective.Emails)
	}
	imCfg := effective.IMStored()
	imPartial := alertconfig.Stored{
		DingTalkSecret:           imCfg.DingTalkSecret,
		FeishuSecret:             imCfg.FeishuSecret,
		WeComSecret:              imCfg.WeComSecret,
		GenericWebhookSignSecret: imCfg.GenericWebhookSignSecret,
	}
	if ch.DingTalk {
		imPartial.DingTalkWebhook = imCfg.DingTalkWebhook
	}
	if ch.Feishu {
		imPartial.FeishuWebhook = imCfg.FeishuWebhook
	}
	if ch.WeCom {
		imPartial.WeComWebhook = imCfg.WeComWebhook
	}
	if ch.DingTalk || ch.Feishu || ch.WeCom {
		alertdelivery.DeliverIMWebhooks(ctx, n.log, imPartial, title, md, plain, deliverUserAgent)
	}
	if ch.GenericWebhook && strings.TrimSpace(effective.GenericWebhookURL) != "" {
		n.deliverGenericWebhook(ctx, effective, evt, kind, title)
	}
}

func (n *Notifier) deliverEmail(body string, addrs []string) {
	subj := "TraceHub 网关调用异常告警"
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		if err := mail.SendPlain(&n.cfg.Email, []string{addr}, subj, body); err != nil {
			n.log.Warn("gateway alert email failed", zap.Error(err), zap.String("to", addr))
		}
	}
}

func (n *Notifier) deliverGenericWebhook(ctx context.Context, effective sitecfg.GatewayAdminAlertEffective, evt Event, kind, title string) {
	url := strings.TrimSpace(effective.GenericWebhookURL)
	if url == "" {
		return
	}
	payload := map[string]any{
		"event":       "gateway.client_error",
		"kind":        kind,
		"title":       title,
		"request_id":  evt.RequestID,
		"user_id":     evt.UserID,
		"api_key_id":  evt.APIKeyID,
		"channel_id":  evt.ChannelID,
		"model":       evt.Model,
		"upstream":    evt.Upstream,
		"status_code": evt.Status,
		"message":     truncate(evt.ErrMsg, 500),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		n.log.Warn("gateway alert webhook marshal failed", zap.Error(err))
		return
	}
	postURL := url
	if sec := strings.TrimSpace(effective.Secrets.GenericWebhookSignSecret); sec != "" {
		var e error
		postURL, e = alertdelivery.AppendDingTalkStyleSignQuery(url, sec)
		if e != nil {
			n.log.Warn("gateway alert generic webhook sign failed", zap.Error(e))
			return
		}
	}
	alertdelivery.PostJSON(ctx, n.log, "generic_webhook", postURL, b, deliverUserAgent)
}

func (n *Notifier) persistInApp(ctx context.Context, kind, title, summary string, evt Event) {
	payload := map[string]any{
		"request_id":          evt.RequestID,
		"user_id":             evt.UserID,
		"api_key_id":          evt.APIKeyID,
		"channel_id":          evt.ChannelID,
		"model":               evt.Model,
		"upstream":            evt.Upstream,
		"status_code":         evt.Status,
		"is_failover_attempt": evt.IsFailoverAttempt,
	}
	if em := strings.TrimSpace(evt.ErrMsg); em != "" {
		payload["error_message"] = truncate(em, 500)
	}
	sev := "warn"
	if evt.Status >= 500 || kind == "transport" {
		sev = "error"
	}
	creator := n.client.AdminGatewayAlert.Create().
		SetEventKind(kind).
		SetSeverity(sev).
		SetTitle(title).
		SetSummary(summary).
		SetPayloadJSON(payload)
	if evt.RequestID != "" {
		creator.SetRequestID(evt.RequestID)
	}
	if evt.UserID > 0 {
		creator.SetUserID(evt.UserID)
	}
	if evt.Status > 0 {
		creator.SetStatusCode(evt.Status)
	}
	if m := strings.TrimSpace(evt.Model); m != "" {
		creator.SetModel(m)
	}
	if evt.ChannelID > 0 {
		creator.SetChannelID(evt.ChannelID)
	}
	if _, err := creator.Save(ctx); err != nil {
		n.log.Warn("gateway alert in-app persist failed", zap.Error(err))
	}
}

func formatMessages(kind string, evt Event) (title, summary, md, plain string) {
	title = "TraceHub 网关调用异常"
	kindLabel := kindLabel(kind)
	statusStr := "—"
	if evt.Status > 0 {
		statusStr = fmt.Sprintf("%d", evt.Status)
	}
	summary = fmt.Sprintf("[%s] 用户 %d · 模型 %s · HTTP %s",
		kindLabel, evt.UserID, displayOrDash(evt.Model), statusStr)
	if rid := strings.TrimSpace(evt.RequestID); rid != "" {
		summary += " · request_id=" + rid
	}
	if em := strings.TrimSpace(evt.ErrMsg); em != "" {
		summary += "\n" + truncate(em, 400)
	}
	md = fmt.Sprintf("### %s\n\n"+
		"- **类型**: %s\n"+
		"- **用户 ID**: %d\n"+
		"- **模型**: %s\n"+
		"- **上游**: %s\n"+
		"- **HTTP**: %s\n"+
		"- **request_id**: %s\n",
		title, kindLabel, evt.UserID, displayOrDash(evt.Model), displayOrDash(evt.Upstream), statusStr, displayOrDash(evt.RequestID))
	if em := strings.TrimSpace(evt.ErrMsg); em != "" {
		md += fmt.Sprintf("- **错误**: %s\n", truncate(em, 400))
	}
	plain = fmt.Sprintf("[%s] 类型=%s 用户=%d 模型=%s 上游=%s HTTP=%s request_id=%s",
		title, kindLabel, evt.UserID, displayOrDash(evt.Model), displayOrDash(evt.Upstream), statusStr, displayOrDash(evt.RequestID))
	if em := strings.TrimSpace(evt.ErrMsg); em != "" {
		plain += " 错误=" + truncate(em, 200)
	}
	return title, summary, md, plain
}

func kindLabel(kind string) string {
	switch kind {
	case "failover":
		return "故障转移"
	case "5xx":
		return "5xx"
	case "4xx":
		return "4xx"
	case "402":
		return "402 余额不足"
	case "transport":
		return "传输失败"
	default:
		return kind
	}
}

func displayOrDash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "—"
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func smtpReady(e *config.EmailConfig) bool {
	return e != nil && strings.TrimSpace(e.SMTPHost) != "" && strings.TrimSpace(e.From) != ""
}
