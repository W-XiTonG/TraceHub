package alertdelivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tracehub/internal/pkg/alertconfig"

	"go.uber.org/zap"
)

const defaultUserAgent = "TraceHub-Alert/1.0"

// PostJSON 向 Webhook POST JSON，超时 12s。
func PostJSON(ctx context.Context, log *zap.Logger, label, webhookURL string, body []byte, userAgent string) {
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		log.Warn("alert webhook build failed", zap.String("channel", label), zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn("alert webhook post failed", zap.String("channel", label), zap.Error(err))
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("alert webhook non-2xx", zap.String("channel", label), zap.Int("status", resp.StatusCode))
	}
}

func DingTalkMarkdownPayload(title, md string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  md,
		},
	})
}

func FeishuTextPayload(text string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": text,
		},
	})
}

func FeishuSignedTextJSON(secret, text string) ([]byte, error) {
	ts, sign, err := feishuRobotSign(secret)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"timestamp": ts,
		"sign":      sign,
		"msg_type":  "text",
		"content": map[string]string{
			"text": text,
		},
	})
}

func WeComMarkdownPayload(md string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": md,
		},
	})
}

// DeliverIMWebhooks 按 Stored 配置投递钉钉/飞书/企微。
func DeliverIMWebhooks(ctx context.Context, log *zap.Logger, cfg alertconfig.Stored, title, md, plain string, userAgent string) {
	if cfg.DingTalkWebhook != "" {
		pl, err := DingTalkMarkdownPayload(title, md)
		if err != nil {
			log.Warn("dingtalk payload", zap.Error(err))
		} else {
			postURL := cfg.DingTalkWebhook
			if strings.TrimSpace(cfg.DingTalkSecret) != "" {
				var e error
				postURL, e = AppendDingTalkStyleSignQuery(cfg.DingTalkWebhook, cfg.DingTalkSecret)
				if e != nil {
					log.Warn("dingtalk sign url", zap.Error(e))
				} else {
					PostJSON(ctx, log, "dingtalk", postURL, pl, userAgent)
				}
			} else {
				PostJSON(ctx, log, "dingtalk", postURL, pl, userAgent)
			}
		}
	}
	if cfg.FeishuWebhook != "" {
		var pl []byte
		var err error
		if strings.TrimSpace(cfg.FeishuSecret) != "" {
			pl, err = FeishuSignedTextJSON(cfg.FeishuSecret, plain)
		} else {
			pl, err = FeishuTextPayload(plain)
		}
		if err != nil {
			log.Warn("feishu payload", zap.Error(err))
		} else {
			PostJSON(ctx, log, "feishu", cfg.FeishuWebhook, pl, userAgent)
		}
	}
	if cfg.WeComWebhook != "" {
		pl, err := WeComMarkdownPayload(md)
		if err != nil {
			log.Warn("wecom payload", zap.Error(err))
		} else {
			postURL := cfg.WeComWebhook
			if strings.TrimSpace(cfg.WeComSecret) != "" {
				var e error
				postURL, e = AppendDingTalkStyleSignQuery(cfg.WeComWebhook, cfg.WeComSecret)
				if e != nil {
					log.Warn("wecom sign url", zap.Error(e))
				} else {
					PostJSON(ctx, log, "wecom", postURL, pl, userAgent)
				}
			} else {
				PostJSON(ctx, log, "wecom", postURL, pl, userAgent)
			}
		}
	}
}

// AppendDingTalkStyleSignQuery 钉钉 / 企业微信 URL 加签。
func AppendDingTalkStyleSignQuery(webhookURL, secret string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return webhookURL, nil
	}
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := ts + "\n" + secret
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write([]byte(stringToSign)); err != nil {
		return "", err
	}
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	enc := url.QueryEscape(sig)
	u, err := url.Parse(webhookURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("timestamp", ts)
	q.Set("sign", enc)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func feishuRobotSign(secret string) (timestamp, sign string, err error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", "", fmt.Errorf("empty secret")
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	stringToSign := ts + "\n" + secret
	mac := hmac.New(sha256.New, []byte(stringToSign))
	if _, err = mac.Write([]byte{}); err != nil {
		return "", "", err
	}
	return ts, base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
