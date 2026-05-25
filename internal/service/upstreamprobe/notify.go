package upstreamprobe

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

const reconcileNotifyUserAgent = "TraceHub-UpstreamReconcile/1.0"

var reconcileNotifyDeps struct {
	cfg *config.Config
	log *zap.Logger
	rdb *redis.Client
}

// SetReconcileNotifyDeps 在 server 启动时注入邮件/Webhook 投递依赖（可选）。
func SetReconcileNotifyDeps(cfg *config.Config, log *zap.Logger, rdb *redis.Client) {
	reconcileNotifyDeps.cfg = cfg
	reconcileNotifyDeps.log = log
	reconcileNotifyDeps.rdb = rdb
}

func maybeNotifyReconcile(
	ctx context.Context,
	client *ent.Client,
	ch *ent.Channel,
	diff reconcileDiff,
	cfg sitecfg.UpstreamModelProbeReconcileEffective,
	autoSynced bool,
	notifyAdded, notifyRemoved bool,
	probeWithMeta map[string]interface{},
) {
	oldProbe := ch.ModelUpstreamProbe
	probeDirty := false
	if notifyAdded && shouldNotifyReconcile(oldProbe, diff.AddedUpstream, "last_notify_add_fp") {
		go deliverReconcileNotify(context.Background(), client, ch, "upstream_model_added", diff.AddedUpstream, autoSynced, cfg.Scope)
		updateNotifyFingerprint(probeWithMeta, "last_notify_add_fp", diff.AddedUpstream)
		probeDirty = true
	}
	if notifyRemoved && shouldNotifyReconcile(oldProbe, diff.RemovedUpstream, "last_notify_remove_fp") {
		go deliverReconcileNotify(context.Background(), client, ch, "upstream_model_removed", diff.RemovedUpstream, autoSynced, cfg.Scope)
		updateNotifyFingerprint(probeWithMeta, "last_notify_remove_fp", diff.RemovedUpstream)
		probeDirty = true
	}
	if probeDirty {
		_ = client.Channel.UpdateOneID(ch.ID).SetModelUpstreamProbe(probeWithMeta).Exec(ctx)
	}
}

func updateNotifyFingerprint(probe map[string]interface{}, key string, ids []string) {
	raw, ok := probe[ProbeReconcileMetaKey]
	meta, ok := raw.(map[string]interface{})
	if !ok || meta == nil {
		meta = map[string]interface{}{}
		probe[ProbeReconcileMetaKey] = meta
	}
	meta[key] = idsFingerprint(ids)
}

func deliverReconcileNotify(
	ctx context.Context,
	client *ent.Client,
	ch *ent.Channel,
	kind string,
	ids []string,
	autoSynced bool,
	scope string,
) {
	effective := sitecfg.CachedGatewayAdminAlert(ctx, client)
	if !effective.Enabled {
		return
	}
	if !effective.Channels.InApp && !effective.AnyExternalChannel() {
		return
	}

	chName := strings.TrimSpace(ch.ChannelName)
	if chName == "" {
		chName = fmt.Sprintf("#%d", ch.ID)
	}
	title := formatReconcileNotifyTitle(kind, chName, len(ids))
	summary := formatReconcileSummary(kind, ch, ids, autoSynced, scope)
	md, plain := formatReconcileMarkdown(title, summary, ch, ids, autoSynced, scope)

	if effective.Channels.InApp {
		persistReconcileInApp(ctx, client, kind, title, summary, ch, ids, autoSynced, scope)
	}

	cfg := reconcileNotifyDeps.cfg
	log := reconcileNotifyDeps.log
	if log == nil {
		log = zap.NewNop()
	}
	if cfg == nil {
		return
	}

	channels := effective.Channels
	if channels.Email && smtpReadyReconcile(&cfg.Email) && len(effective.Emails) > 0 {
		subj := "TraceHub 上游模型列表变更"
		for _, addr := range effective.Emails {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			if err := mail.SendPlain(&cfg.Email, []string{addr}, subj, plain); err != nil {
				log.Warn("upstream reconcile alert email failed", zap.Error(err), zap.String("to", addr))
			}
		}
	}
	imCfg := effective.IMStored()
	imPartial := alertconfig.Stored{
		DingTalkSecret:           imCfg.DingTalkSecret,
		FeishuSecret:             imCfg.FeishuSecret,
		WeComSecret:              imCfg.WeComSecret,
		GenericWebhookSignSecret: imCfg.GenericWebhookSignSecret,
	}
	if channels.DingTalk {
		imPartial.DingTalkWebhook = imCfg.DingTalkWebhook
	}
	if channels.Feishu {
		imPartial.FeishuWebhook = imCfg.FeishuWebhook
	}
	if channels.WeCom {
		imPartial.WeComWebhook = imCfg.WeComWebhook
	}
	if channels.DingTalk || channels.Feishu || channels.WeCom {
		alertdelivery.DeliverIMWebhooks(ctx, log, imPartial, title, md, plain, reconcileNotifyUserAgent)
	}
	if channels.GenericWebhook && strings.TrimSpace(effective.GenericWebhookURL) != "" {
		deliverReconcileGenericWebhook(ctx, log, effective, kind, title, ch, ids, autoSynced, scope)
	}
}

func formatReconcileSummary(kind string, ch *ent.Channel, ids []string, autoSynced bool, scope string) string {
	action := "已检测"
	if autoSynced {
		action = "已自动同步到渠道模型列表"
	}
	verb := "新增"
	if kind == "upstream_model_removed" {
		verb = "移除"
	}
	name := strings.TrimSpace(ch.ChannelName)
	if name == "" {
		name = fmt.Sprintf("渠道 #%d", ch.ID)
	}
	return fmt.Sprintf("%s：上游 %s %d 个模型（%s）\n范围：%s\n模型：%s",
		name, verb, len(ids), action, scope, strings.Join(ids, ", "))
}

func formatReconcileMarkdown(title, summary string, ch *ent.Channel, ids []string, autoSynced bool, scope string) (md, plain string) {
	syncNote := "仅通知"
	if autoSynced {
		syncNote = "已自动写入渠道 models"
	}
	md = fmt.Sprintf("### %s\n\n%s\n\n- **渠道 ID**: %d\n- **范围**: %s\n- **处理**: %s\n- **模型**: %s\n",
		title, summary, ch.ID, scope, syncNote, strings.Join(ids, ", "))
	plain = fmt.Sprintf("%s | channel_id=%d scope=%s synced=%v ids=%s",
		title, ch.ID, scope, autoSynced, strings.Join(ids, ", "))
	return md, plain
}

func persistReconcileInApp(ctx context.Context, client *ent.Client, kind, title, summary string, ch *ent.Channel, ids []string, autoSynced bool, scope string) {
	payload := map[string]any{
		"channel_id":   ch.ID,
		"channel_name": strings.TrimSpace(ch.ChannelName),
		"model_ids":    ids,
		"auto_synced":  autoSynced,
		"scope":        scope,
	}
	_, err := client.AdminGatewayAlert.Create().
		SetEventKind(kind).
		SetSeverity("warn").
		SetTitle(title).
		SetSummary(summary).
		SetPayloadJSON(payload).
		SetChannelID(ch.ID).
		Save(ctx)
	if err != nil && reconcileNotifyDeps.log != nil {
		reconcileNotifyDeps.log.Warn("upstream reconcile in-app persist failed", zap.Error(err))
	}
}

func deliverReconcileGenericWebhook(
	ctx context.Context,
	log *zap.Logger,
	effective sitecfg.GatewayAdminAlertEffective,
	kind, title string,
	ch *ent.Channel,
	ids []string,
	autoSynced bool,
	scope string,
) {
	url := strings.TrimSpace(effective.GenericWebhookURL)
	if url == "" {
		return
	}
	payload := map[string]any{
		"event":        "upstream_model.reconcile",
		"kind":         kind,
		"title":        title,
		"channel_id":   ch.ID,
		"channel_name": strings.TrimSpace(ch.ChannelName),
		"model_ids":    ids,
		"auto_synced":  autoSynced,
		"scope":        scope,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		log.Warn("upstream reconcile webhook marshal failed", zap.Error(err))
		return
	}
	postURL := url
	if sec := strings.TrimSpace(effective.Secrets.GenericWebhookSignSecret); sec != "" {
		var e error
		postURL, e = alertdelivery.AppendDingTalkStyleSignQuery(url, sec)
		if e != nil {
			log.Warn("upstream reconcile generic webhook sign failed", zap.Error(err))
			return
		}
	}
	alertdelivery.PostJSON(ctx, log, "generic_webhook", postURL, b, reconcileNotifyUserAgent)
}

func smtpReadyReconcile(e *config.EmailConfig) bool {
	return e != nil && strings.TrimSpace(e.SMTPHost) != "" && strings.TrimSpace(e.From) != ""
}
