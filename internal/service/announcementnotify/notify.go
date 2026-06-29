package announcementnotify

import (
	"context"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/user"
	"tracehub/internal/pkg/announcementaudience"
	"tracehub/internal/pkg/config"
	"tracehub/internal/pkg/mail"

	"go.uber.org/zap"
)

// Result 邮件发送统计（同步探测 SMTP 可用性；实际发送异步进行）。
type Result struct {
	Requested      bool   `json:"requested"`
	Sent           int    `json:"sent,omitempty"`
	Failed         int    `json:"failed,omitempty"`
	SkippedReason  string `json:"skipped_reason,omitempty"`
	RecipientCount int    `json:"recipient_count,omitempty"`
}

// Notifier 在公告发布时异步发送邮件通知。
type Notifier struct {
	cfg *config.Config
	client *ent.Client
	log *zap.Logger
}

func New(cfg *config.Config, client *ent.Client, log *zap.Logger) *Notifier {
	if cfg == nil || client == nil {
		return nil
	}
	if log == nil {
		log = zap.NewNop()
	}
	return &Notifier{cfg: cfg, client: client, log: log}
}

// NotifyOnPublish 若 requested 为 true，异步发送邮件；返回同步结果（含 skip 原因）。
func (n *Notifier) NotifyOnPublish(ctx context.Context, a *ent.Announcement, requested bool) Result {
	if !requested || n == nil || a == nil {
		return Result{Requested: requested}
	}
	if !smtpReady(&n.cfg.Email) {
		return Result{Requested: true, SkippedReason: "smtp_not_configured"}
	}
	userIDs, err := announcementaudience.ResolveRecipientUserIDs(ctx, n.client, a)
	if err != nil {
		n.log.Warn("announcement email resolve recipients failed", zap.Error(err), zap.Int("announcement_id", a.ID))
		return Result{Requested: true, SkippedReason: "resolve_recipients_failed"}
	}
	if len(userIDs) == 0 {
		return Result{Requested: true, SkippedReason: "no_recipients"}
	}
	go n.deliver(context.Background(), a, userIDs)
	return Result{Requested: true, RecipientCount: len(userIDs)}
}

func (n *Notifier) deliver(ctx context.Context, a *ent.Announcement, userIDs []int) {
	us, err := n.client.User.Query().
		Where(user.IDIn(userIDs...)).
		Select(user.FieldID, user.FieldEmail).
		All(ctx)
	if err != nil {
		n.log.Warn("announcement email load users failed", zap.Error(err), zap.Int("announcement_id", a.ID))
		return
	}
	consoleURL := ""
	subject, body := mail.AnnouncementEmail(a.Title, a.Content, consoleURL)
	sent, failed := 0, 0
	for _, u := range us {
		em := strings.TrimSpace(u.Email)
		if em == "" {
			continue
		}
		if err := mail.SendPlain(&n.cfg.Email, []string{em}, subject, body); err != nil {
			failed++
			n.log.Warn("announcement email send failed",
				zap.Error(err),
				zap.Int("announcement_id", a.ID),
				zap.Int("user_id", u.ID),
			)
			continue
		}
		sent++
	}
	n.log.Info("announcement email delivered",
		zap.Int("announcement_id", a.ID),
		zap.Int("sent", sent),
		zap.Int("failed", failed),
	)
}

func smtpReady(e *config.EmailConfig) bool {
	return e != nil && strings.TrimSpace(e.SMTPHost) != "" && strings.TrimSpace(e.From) != ""
}
