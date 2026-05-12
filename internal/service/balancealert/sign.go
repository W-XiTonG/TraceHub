package balancealert

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// appendDingTalkStyleSignQuery 钉钉 / 企业微信自定义机器人 URL 加签：timestamp 毫秒 + sign（Base64(HMAC-SHA256) 后 URL 编码）。
func appendDingTalkStyleSignQuery(webhookURL, secret string) (string, error) {
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

// feishuRobotSign 飞书自定义机器人「签名校验」：timestamp 为秒；签名为 Base64(HMAC-SHA256(key= timestamp+"\n"+secret, msg=空))。
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
