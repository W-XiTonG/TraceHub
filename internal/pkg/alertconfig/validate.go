package alertconfig

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	maxCustomEmails  = 8
	maxWebhookURLLen = 800
	maxEmailLen      = 255
)

// NormalizeAndValidate 清洗并校验；通过则返回可写入库的副本。
func NormalizeAndValidate(in Stored) (Stored, error) {
	out := Stored{
		DingTalkWebhook: strings.TrimSpace(in.DingTalkWebhook),
		FeishuWebhook:   strings.TrimSpace(in.FeishuWebhook),
		WeComWebhook:    strings.TrimSpace(in.WeComWebhook),
	}
	seen := map[string]struct{}{}
	for _, raw := range in.Emails {
		em := strings.TrimSpace(raw)
		if em == "" {
			continue
		}
		if len(em) > maxEmailLen {
			return Stored{}, fmt.Errorf("邮箱过长: %s", em)
		}
		if !strings.Contains(em, "@") {
			return Stored{}, fmt.Errorf("邮箱格式无效: %s", em)
		}
		key := strings.ToLower(em)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out.Emails = append(out.Emails, em)
		if len(out.Emails) > maxCustomEmails {
			return Stored{}, fmt.Errorf("自定义告警邮箱最多 %d 个", maxCustomEmails)
		}
	}
	if err := checkWebhookURL("钉钉", out.DingTalkWebhook); err != nil {
		return Stored{}, err
	}
	if err := checkWebhookURL("飞书", out.FeishuWebhook); err != nil {
		return Stored{}, err
	}
	if err := checkWebhookURL("企业微信", out.WeComWebhook); err != nil {
		return Stored{}, err
	}
	return out, nil
}

func checkWebhookURL(label, raw string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > maxWebhookURLLen {
		return fmt.Errorf("%s Webhook 过长（最多 %d 字符）", label, maxWebhookURLLen)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%s Webhook 须为有效 https URL", label)
	}
	return nil
}
