package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
)

// NotifyBalanceUpdated 向用户 webhook_url 推送 balance.updated。
func NotifyBalanceUpdated(ctx context.Context, user *ent.User, event string, data map[string]any) {
	url := strings.TrimSpace(user.WebhookURL)
	if url == "" {
		return
	}
	payload := map[string]any{
		"event":      event,
		"created_at": time.Now().UTC().Format(time.RFC3339),
		"data":       data,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	sig := signWebhook(body, []byte(user.Email))
	req.Header.Set("X-TraceHub-Signature", "sha256="+sig)
	go func() {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
}

func signWebhook(body, secret []byte) string {
	if len(secret) == 0 {
		secret = []byte("tracehub")
	}
	m := hmac.New(sha256.New, secret)
	_, _ = m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}
