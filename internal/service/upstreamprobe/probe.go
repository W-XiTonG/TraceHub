package upstreamprobe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/pkg/sitecfg"
	"tracehub/internal/service/channelhealth"
	"tracehub/internal/service/channelsecret"
)

// ProbeMetaChannelKey 写入 model_upstream_probe 中表示整次拉取失败元信息。
const ProbeMetaChannelKey = "_channel"

// modelInUpstreamList 判定本地模型键是否出现在上游列表：键本身或别名指向的上游 id 任一命中即 true。
func modelInUpstreamList(localKey string, upstream map[string]struct{}, alias map[string]string) bool {
	k := strings.TrimSpace(localKey)
	if k == "" {
		return false
	}
	if _, ok := upstream[k]; ok {
		return true
	}
	if alias == nil {
		return false
	}
	if tgt, ok := alias[k]; ok {
		t := strings.TrimSpace(tgt)
		if t != "" {
			if _, ok2 := upstream[t]; ok2 {
				return true
			}
		}
	}
	return false
}

func collectLocalModelKeys(models []string, alias map[string]string) []string {
	seen := make(map[string]struct{})
	var keys []string
	for _, m := range models {
		id := strings.TrimSpace(m)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			keys = append(keys, id)
		}
	}
	if alias != nil {
		for k := range alias {
			id := strings.TrimSpace(k)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				keys = append(keys, id)
			}
		}
	}
	return keys
}

func bearerForChannel(aesKey []byte, ch *ent.Channel) (string, error) {
	switch ch.AuthType {
	case channel.AuthTypeAPIKey:
		s := channelsecret.Plaintext(aesKey, ch.APIKey)
		if s == "" {
			return "", fmt.Errorf("api_key 为空")
		}
		return s, nil
	case channel.AuthTypeOauth:
		s := channelsecret.Plaintext(aesKey, ch.OauthToken)
		if s == "" {
			return "", fmt.Errorf("oauth_token 为空")
		}
		return s, nil
	case channel.AuthTypeSetupToken:
		s := channelsecret.Plaintext(aesKey, ch.SetupToken)
		if s == "" {
			return "", fmt.Errorf("setup_token 为空")
		}
		return s, nil
	default:
		return "", fmt.Errorf("不支持的 auth_type")
	}
}

func cloneProbeForKeys(old map[string]interface{}, keepKeys map[string]struct{}) map[string]interface{} {
	out := make(map[string]interface{})
	if old == nil {
		return out
	}
	for k, v := range old {
		if k == ProbeMetaChannelKey {
			continue
		}
		if _, keep := keepKeys[k]; keep {
			out[k] = v
		}
	}
	return out
}

// ProbeChannel 对单条渠道拉取上游 /v1/models 并更新 model_upstream_probe。
// 拉取失败时不覆盖已有模型行的 in_upstream_list，仅写入 ProbeMetaChannelKey。
func ProbeChannel(ctx context.Context, client *ent.Client, aesKey []byte, ch *ent.Channel) error {
	localKeys := collectLocalModelKeys(ch.Models, ch.ModelAlias)
	keep := make(map[string]struct{})
	for _, k := range localKeys {
		keep[k] = struct{}{}
	}
	oldProbe := ch.ModelUpstreamProbe
	now := time.Now().UTC().Format(time.RFC3339)

	if !ch.Enabled || !ch.Schedulable {
		return nil
	}
	if len(localKeys) == 0 {
		// 无模型可标注：清空探测结果（保留 _channel 无意义）
		return client.Channel.UpdateOneID(ch.ID).ClearModelUpstreamProbe().Exec(ctx)
	}

	bearer, err := bearerForChannel(aesKey, ch)
	if err != nil {
		merged := cloneProbeForKeys(oldProbe, keep)
		merged[ProbeMetaChannelKey] = map[string]interface{}{
			"error":      err.Error(),
			"checked_at": now,
		}
		if e := client.Channel.UpdateOneID(ch.ID).SetModelUpstreamProbe(merged).Exec(ctx); e != nil {
			return e
		}
		return applyProbeHealthIfEnabled(ctx, client, ch.ID, false)
	}

	ids, _, err := FetchUpstreamModelIDs(ctx, ch.ChannelType, ch.APIBaseURL, bearer)
	if err != nil {
		merged := cloneProbeForKeys(oldProbe, keep)
		merged[ProbeMetaChannelKey] = map[string]interface{}{
			"error":      err.Error(),
			"checked_at": now,
		}
		if e := client.Channel.UpdateOneID(ch.ID).SetModelUpstreamProbe(merged).Exec(ctx); e != nil {
			return e
		}
		return applyProbeHealthIfEnabled(ctx, client, ch.ID, false)
	}

	upstream := make(map[string]struct{})
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			upstream[id] = struct{}{}
		}
	}

	alias := ch.ModelAlias
	if alias == nil {
		alias = map[string]string{}
	}
	out := make(map[string]interface{})
	for _, k := range localKeys {
		inList := modelInUpstreamList(k, upstream, alias)
		out[k] = map[string]interface{}{
			"in_upstream_list": inList,
			"checked_at":       now,
		}
	}
	reconcileCfg := sitecfg.EffectiveUpstreamModelProbeReconcile(ctx, client)
	if reconcileCfg.AnyAction() {
		if e := maybeReconcileAfterProbe(ctx, client, ch, upstream, out, oldProbe, now); e != nil {
			return e
		}
		return applyProbeHealthIfEnabled(ctx, client, ch.ID, true)
	}
	if e := client.Channel.UpdateOneID(ch.ID).SetModelUpstreamProbe(out).Exec(ctx); e != nil {
		return e
	}
	return applyProbeHealthIfEnabled(ctx, client, ch.ID, true)
}

func applyProbeHealthIfEnabled(ctx context.Context, client *ent.Client, channelID int, probeOK bool) error {
	defaults := sitecfg.EffectiveRoutingDefaults(ctx, client)
	if !defaults.AutoHealthFromProbe {
		return nil
	}
	return channelhealth.ApplyProbeHealth(ctx, client, channelID, probeOK, defaults.ProbeFailThreshold)
}
