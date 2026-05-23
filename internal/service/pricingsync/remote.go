package pricingsync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/modelpricingsynctarget"
	"tracehub/internal/service/channelsecret"

	"github.com/google/uuid"
)

const maxBodyBytes = 8 << 20

type RemoteOpts struct {
	AESKey            []byte
	AllowInsecureHTTP bool
}

func httpClient() *http.Client {
	return &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			return ValidateOutboundURL(req.URL.String(), true)
		},
	}
}

func buildAuthRequest(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, method, url string, body io.Reader, opts RemoteOpts) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	switch t.CredentialMode {
	case modelpricingsynctarget.CredentialModeNone:
	case modelpricingsynctarget.CredentialModeStatic:
		key := channelsecret.Plaintext(opts.AESKey, t.APIKeyEncrypted)
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	case modelpricingsynctarget.CredentialModeChannel:
		if t.ChannelID == 0 {
			return nil, fmt.Errorf("未绑定渠道")
		}
		ch, err := client.Channel.Get(ctx, t.ChannelID)
		if err != nil {
			return nil, fmt.Errorf("渠道不存在: %w", err)
		}
		key := channelsecret.Plaintext(opts.AESKey, ch.APIKey)
		if key == "" {
			return nil, fmt.Errorf("渠道未配置 API Key")
		}
		req.Header.Set("Authorization", "Bearer "+key)
	default:
		return nil, fmt.Errorf("未知凭证模式")
	}
	for k, v := range t.ExtraHeaders {
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	return req, nil
}

// FetchRemotePull 从外部拉取 pricing payload。
func FetchRemotePull(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, opts RemoteOpts) (*Payload, error) {
	if !t.Enabled {
		return nil, errTargetDisabled
	}
	url := joinURL(t.BaseURL, t.PullPath)
	if err := ValidateOutboundURL(url, opts.AllowInsecureHTTP); err != nil {
		return nil, err
	}
	req, err := buildAuthRequest(ctx, client, t, http.MethodGet, url, nil, opts)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	var p Payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, fmt.Errorf("解析 JSON: %w", err)
	}
	if p.Version != VersionTraceHubPricingV1 {
		return nil, errVersionMismatch
	}
	return &p, nil
}

// PostRemotePush 推送 pricing payload 到外部。
func PostRemotePush(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, p *Payload, opts RemoteOpts) error {
	if !t.Enabled {
		return errTargetDisabled
	}
	url := joinURL(t.BaseURL, t.PushPath)
	if err := ValidateOutboundURL(url, opts.AllowInsecureHTTP); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := buildAuthRequest(ctx, client, t, http.MethodPost, url, strings.NewReader(string(raw)), opts)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
