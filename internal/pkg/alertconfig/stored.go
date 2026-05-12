package alertconfig

import "strings"

// Stored 为用户自定义告警目标，持久化于 users.custom_alert_config（JSON）。
// 加签密钥仅通过 PUT /api/user/bind 的 custom_alert_secrets 更新，勿在 custom_alert_config 中提交。
type Stored struct {
	Emails          []string `json:"emails,omitempty"`
	DingTalkWebhook string   `json:"dingtalk_webhook,omitempty"`
	FeishuWebhook   string   `json:"feishu_webhook,omitempty"`
	WeComWebhook    string   `json:"wecom_webhook,omitempty"`
	// 钉钉 / 企业微信：与开放平台一致的 URL 查询参数加签（timestamp 毫秒 + sign）。
	DingTalkSecret string `json:"dingtalk_secret,omitempty"`
	WeComSecret    string `json:"wecom_secret,omitempty"`
	// 飞书：签名校验时 JSON 根级 timestamp（秒）+ sign（见飞书自定义机器人文档）。
	FeishuSecret string `json:"feishu_secret,omitempty"`
	// 账户「通用 Webhook」若需 URL 加签（与钉钉算法相同），填写密钥；仅对 user.webhook_url 生效。
	GenericWebhookSignSecret string `json:"generic_webhook_sign_secret,omitempty"`
}

// Empty 表示未配置任何渠道。
func (s Stored) Empty() bool {
	if len(s.Emails) > 0 {
		return false
	}
	return s.DingTalkWebhook == "" && s.FeishuWebhook == "" && s.WeComWebhook == ""
}

// ReallyEmpty 表示渠道与加签密钥均为空（可删除 JSON 列）。
func (s Stored) ReallyEmpty() bool {
	if !s.Empty() {
		return false
	}
	sec := s.DingTalkSecret + s.FeishuSecret + s.WeComSecret + s.GenericWebhookSignSecret
	return strings.TrimSpace(sec) == ""
}
