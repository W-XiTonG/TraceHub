package balancealert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

func postJSON(ctx context.Context, log *zap.Logger, userID int, label, url string, body []byte) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		log.Warn("alert webhook build failed", zap.String("channel", label), zap.Error(err), zap.Int("user_id", userID))
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "TraceHub-BalanceAlert/1.0")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Warn("alert webhook post failed", zap.String("channel", label), zap.Error(err), zap.Int("user_id", userID))
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Warn("alert webhook non-2xx", zap.String("channel", label), zap.Int("status", resp.StatusCode), zap.Int("user_id", userID))
	}
}

func dingTalkMarkdownPayload(title, md string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  md,
		},
	})
}

func feishuTextPayload(text string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msg_type": "text",
		"content": map[string]string{
			"text": text,
		},
	})
}

// feishuSignedTextJSON 飞书开启「签名校验」时，根级需带 timestamp（秒）与 sign。
func feishuSignedTextJSON(secret, text string) ([]byte, error) {
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

func weComMarkdownPayload(md string) ([]byte, error) {
	return json.Marshal(map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": md,
		},
	})
}

func imMarkdownLines(userID int, email string, newBal, threshold, unit int64, yuan, thYuan float64) string {
	return fmt.Sprintf(
		"### TraceHub 余额告警\n\n"+
			"- **用户 ID**: %d\n"+
			"- **注册邮箱**: %s\n"+
			"- **当前余额（额度）**: %d\n"+
			"- **告警阈值**: %d\n"+
			"- **约合余额**: %.4f 元（1 元=%d 额度）\n"+
			"- **约合阈值**: %.4f 元\n\n"+
			"请及时充值或兑换额度，以免后续 API 因余额不足失败。",
		userID, email, newBal, threshold, yuan, unit, thYuan,
	)
}

func imPlainText(userID int, email string, newBal, threshold, unit int64, yuan, thYuan float64) string {
	return fmt.Sprintf(
		"[TraceHub 余额告警] 用户ID=%d 邮箱=%s 当前余额=%d 阈值=%d 约合余额=%.4f元(1元=%d额度) 约合阈值=%.4f元。请及时充值。",
		userID, email, newBal, threshold, yuan, unit, thYuan,
	)
}
