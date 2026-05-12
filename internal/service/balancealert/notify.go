package balancealert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/mail"

	"go.uber.org/zap"
)

// Notifier 在用户余额因扣费从高于阈值降至不高于阈值时，异步发送邮件与/或 Webhook。
type Notifier struct {
	cfg *config.Config
	log *zap.Logger
}

func New(cfg *config.Config, log *zap.Logger) *Notifier {
	if cfg == nil {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Notifier{cfg: cfg, log: log}
}

func (n *Notifier) MaybeNotifyCrossBelow(_ context.Context, u *ent.User, oldBal, newBal int64) {
	if n == nil || u == nil {
		return
	}
	t := u.BalanceAlertThreshold
	if t <= 0 {
		return
	}
	if !(oldBal > t && newBal <= t) {
		return
	}
	go n.deliver(context.Background(), u, newBal, t)
}

func (n *Notifier) deliver(ctx context.Context, u *ent.User, newBal, threshold int64) {
	unit := n.cfg.Site.CurrencyQuotaPerYuan
	if unit <= 0 {
		unit = 10000
	}
	yuan := float64(newBal) / float64(unit)
	thYuan := float64(threshold) / float64(unit)

	subj := "TraceHub 余额告警"
	mailBody := fmt.Sprintf(
		"您好，\n\n您的 TraceHub 账户余额已降至告警线以下。\n\n"+
			"当前余额（内部额度）: %d\n"+
			"告警阈值: %d\n"+
			"约合余额: %.4f 元（按站点配置 1 元=%d 额度估算）\n"+
			"约合阈值: %.4f 元\n\n"+
			"请及时充值或兑换额度，以免后续请求因余额不足失败。\n\n"+
			"（本邮件在余额首次跌破阈值时发送一次；若余额回升至阈值以上后再次跌破，会再次提醒。）\n",
		newBal, threshold, yuan, unit, thYuan,
	)

	if smtpReady(&n.cfg.Email) && strings.TrimSpace(u.Email) != "" {
		if err := mail.SendPlain(&n.cfg.Email, []string{u.Email}, subj, mailBody); err != nil {
			n.log.Warn("balance alert email failed", zap.Error(err), zap.Int("user_id", u.ID))
		}
	}
	n.deliverCustomEmails(ctx, u, subj, mailBody)

	url := strings.TrimSpace(u.WebhookURL)
	if url != "" {
		payload := map[string]any{
			"event":                     "balance.low",
			"user_id":                   u.ID,
			"email":                     u.Email,
			"balance":                   newBal,
			"threshold":                 threshold,
			"currency_quota_per_yuan":   unit,
			"balance_yuan_approx":       yuan,
			"threshold_yuan_approx":     thYuan,
			"message":                   "Balance crossed below configured alert threshold after a successful deduction.",
		}
		b, err := json.Marshal(payload)
		if err != nil {
			n.log.Warn("balance alert webhook marshal failed", zap.Error(err), zap.Int("user_id", u.ID))
			return
		}
		postURL := url
		if sec := strings.TrimSpace(u.CustomAlertConfig.GenericWebhookSignSecret); sec != "" {
			var e error
			postURL, e = appendDingTalkStyleSignQuery(url, sec)
			if e != nil {
				n.log.Warn("balance alert generic webhook sign url failed", zap.Error(e), zap.Int("user_id", u.ID))
				return
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, bytes.NewReader(b))
		if err != nil {
			n.log.Warn("balance alert webhook request build failed", zap.Error(err), zap.Int("user_id", u.ID))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "TraceHub-BalanceAlert/1.0")
		client := &http.Client{Timeout: 12 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			n.log.Warn("balance alert webhook post failed", zap.Error(err), zap.Int("user_id", u.ID))
			return
		}
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			n.log.Warn("balance alert webhook non-2xx", zap.Int("status", resp.StatusCode), zap.Int("user_id", u.ID))
		}
	}

	n.deliverCustomIMWebhooks(ctx, u, newBal, threshold, unit, yuan, thYuan)
}

func (n *Notifier) deliverCustomEmails(ctx context.Context, u *ent.User, subj, mailBody string) {
	cfg := u.CustomAlertConfig
	if cfg.Empty() || !smtpReady(&n.cfg.Email) {
		return
	}
	sent := map[string]struct{}{}
	if em := strings.ToLower(strings.TrimSpace(u.Email)); em != "" {
		sent[em] = struct{}{}
	}
	for _, addr := range cfg.Emails {
		key := strings.ToLower(strings.TrimSpace(addr))
		if key == "" {
			continue
		}
		if _, dup := sent[key]; dup {
			continue
		}
		sent[key] = struct{}{}
		if err := mail.SendPlain(&n.cfg.Email, []string{addr}, subj, mailBody); err != nil {
			n.log.Warn("balance alert custom email failed", zap.Error(err), zap.Int("user_id", u.ID), zap.String("to", addr))
		}
	}
}

func (n *Notifier) deliverCustomIMWebhooks(ctx context.Context, u *ent.User, newBal, threshold, unit int64, yuan, thYuan float64) {
	cfg := u.CustomAlertConfig
	if cfg.Empty() {
		return
	}
	md := imMarkdownLines(u.ID, u.Email, newBal, threshold, unit, yuan, thYuan)
	plain := imPlainText(u.ID, u.Email, newBal, threshold, unit, yuan, thYuan)

	if cfg.DingTalkWebhook != "" {
		pl, err := dingTalkMarkdownPayload("TraceHub 余额告警", md)
		if err != nil {
			n.log.Warn("dingtalk payload", zap.Error(err), zap.Int("user_id", u.ID))
		} else {
			postURL := cfg.DingTalkWebhook
			if strings.TrimSpace(cfg.DingTalkSecret) != "" {
				var e error
				postURL, e = appendDingTalkStyleSignQuery(cfg.DingTalkWebhook, cfg.DingTalkSecret)
				if e != nil {
					n.log.Warn("dingtalk sign url", zap.Error(e), zap.Int("user_id", u.ID))
				} else {
					postJSON(ctx, n.log, u.ID, "dingtalk", postURL, pl)
				}
			} else {
				postJSON(ctx, n.log, u.ID, "dingtalk", postURL, pl)
			}
		}
	}
	if cfg.FeishuWebhook != "" {
		var pl []byte
		var err error
		if strings.TrimSpace(cfg.FeishuSecret) != "" {
			pl, err = feishuSignedTextJSON(cfg.FeishuSecret, plain)
		} else {
			pl, err = feishuTextPayload(plain)
		}
		if err != nil {
			n.log.Warn("feishu payload", zap.Error(err), zap.Int("user_id", u.ID))
		} else {
			postJSON(ctx, n.log, u.ID, "feishu", cfg.FeishuWebhook, pl)
		}
	}
	if cfg.WeComWebhook != "" {
		pl, err := weComMarkdownPayload(md)
		if err != nil {
			n.log.Warn("wecom payload", zap.Error(err), zap.Int("user_id", u.ID))
		} else {
			postURL := cfg.WeComWebhook
			if strings.TrimSpace(cfg.WeComSecret) != "" {
				var e error
				postURL, e = appendDingTalkStyleSignQuery(cfg.WeComWebhook, cfg.WeComSecret)
				if e != nil {
					n.log.Warn("wecom sign url", zap.Error(e), zap.Int("user_id", u.ID))
				} else {
					postJSON(ctx, n.log, u.ID, "wecom", postURL, pl)
				}
			} else {
				postJSON(ctx, n.log, u.ID, "wecom", postURL, pl)
			}
		}
	}
}

func smtpReady(e *config.EmailConfig) bool {
	return e != nil && strings.TrimSpace(e.SMTPHost) != "" && strings.TrimSpace(e.From) != ""
}
