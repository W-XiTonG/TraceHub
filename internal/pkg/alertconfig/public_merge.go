package alertconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxSignSecretLen = 256

// PublicView 供 API 返回：不包含密钥，仅返回是否已配置加签。
func PublicView(s Stored) map[string]any {
	return map[string]any{
		"emails":                         s.Emails,
		"dingtalk_webhook":               s.DingTalkWebhook,
		"feishu_webhook":                 s.FeishuWebhook,
		"wecom_webhook":                  s.WeComWebhook,
		"dingtalk_sign_configured":       strings.TrimSpace(s.DingTalkSecret) != "",
		"feishu_sign_configured":         strings.TrimSpace(s.FeishuSecret) != "",
		"wecom_sign_configured":          strings.TrimSpace(s.WeComSecret) != "",
		"generic_webhook_sign_configured": strings.TrimSpace(s.GenericWebhookSignSecret) != "",
	}
}

// MergeChannelFields 用请求体中的渠道字段覆盖，并保留原有加签密钥（请求体应已清除密钥字段）。
func MergeChannelFields(existing, patch Stored) Stored {
	out := existing
	out.Emails = patch.Emails
	out.DingTalkWebhook = patch.DingTalkWebhook
	out.FeishuWebhook = patch.FeishuWebhook
	out.WeComWebhook = patch.WeComWebhook
	return out
}

// StripSecretsForWire 反序列化客户端 JSON 后调用，防止通过 custom_alert_config 写入密钥。
func StripSecretsForWire(s *Stored) {
	s.DingTalkSecret = ""
	s.FeishuSecret = ""
	s.WeComSecret = ""
	s.GenericWebhookSignSecret = ""
}

// ApplySecretPatch 根据 custom_alert_secrets 合并密钥。patchJSON 为对象，键：dingtalk、feishu、wecom、generic_webhook；
// 值为字符串（空字符串表示清除该密钥）；未出现的键保持不变。
func ApplySecretPatch(existing Stored, patchJSON []byte) (Stored, error) {
	if len(strings.TrimSpace(string(patchJSON))) == 0 || string(patchJSON) == "null" {
		return existing, nil
	}
	var m map[string]any
	if err := json.Unmarshal(patchJSON, &m); err != nil {
		return Stored{}, fmt.Errorf("custom_alert_secrets 不是合法 JSON 对象")
	}
	out := existing
	for _, key := range []string{"dingtalk", "feishu", "wecom", "generic_webhook"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		if raw == nil {
			switch key {
			case "dingtalk":
				out.DingTalkSecret = ""
			case "feishu":
				out.FeishuSecret = ""
			case "wecom":
				out.WeComSecret = ""
			case "generic_webhook":
				out.GenericWebhookSignSecret = ""
			}
			continue
		}
		str, ok := raw.(string)
		if !ok {
			return Stored{}, fmt.Errorf("custom_alert_secrets.%s 须为字符串", key)
		}
		str = strings.TrimSpace(str)
		if len(str) > maxSignSecretLen {
			return Stored{}, fmt.Errorf("加签密钥过长（最多 %d 字符）: %s", maxSignSecretLen, key)
		}
		switch key {
		case "dingtalk":
			out.DingTalkSecret = str
		case "feishu":
			out.FeishuSecret = str
		case "wecom":
			out.WeComSecret = str
		case "generic_webhook":
			out.GenericWebhookSignSecret = str
		}
	}
	return out, nil
}

// ValidateSignSecretsWithChannels 校验「有密钥必有 Webhook」等。
func ValidateSignSecretsWithChannels(s Stored, webhookURL string) error {
	if strings.TrimSpace(s.DingTalkSecret) != "" && strings.TrimSpace(s.DingTalkWebhook) == "" {
		return fmt.Errorf("已配置钉钉加签密钥时须填写钉钉 Webhook")
	}
	if strings.TrimSpace(s.FeishuSecret) != "" && strings.TrimSpace(s.FeishuWebhook) == "" {
		return fmt.Errorf("已配置飞书加签密钥时须填写飞书 Webhook")
	}
	if strings.TrimSpace(s.WeComSecret) != "" && strings.TrimSpace(s.WeComWebhook) == "" {
		return fmt.Errorf("已配置企业微信加签密钥时须填写企业微信 Webhook")
	}
	if strings.TrimSpace(s.GenericWebhookSignSecret) != "" && strings.TrimSpace(webhookURL) == "" {
		return fmt.Errorf("已配置通用 Webhook 加签密钥时须填写上方「回调 URL」")
	}
	return nil
}
