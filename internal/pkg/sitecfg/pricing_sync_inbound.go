package sitecfg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyPricingSyncInboundTokenSHA256 = "site.pricing_sync.inbound_token_sha256"
	SystemConfigKeyPricingSyncInboundTokenAt     = "site.pricing_sync.inbound_token_at"
	inboundTokenPrefix                           = "thps_"
)

// GenerateInboundSyncToken 生成新入站同步密钥（明文仅返回一次）；哈希写入 system_configs。
func GenerateInboundSyncToken(ctx context.Context, client *ent.Client) (plain string, err error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	plain = inboundTokenPrefix + hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	hashHex := hex.EncodeToString(sum[:])
	now := time.Now().UTC().Format(time.RFC3339)
	if err := upsertPricingSyncConfig(ctx, client, SystemConfigKeyPricingSyncInboundTokenSHA256, hashHex); err != nil {
		return "", err
	}
	if err := upsertPricingSyncConfig(ctx, client, SystemConfigKeyPricingSyncInboundTokenAt, now); err != nil {
		return "", err
	}
	return plain, nil
}

func upsertPricingSyncConfig(ctx context.Context, client *ent.Client, key, value string) error {
	existing, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(key)).
		Only(ctx)
	if ent.IsNotFound(err) {
		_, err = client.SystemConfig.Create().SetConfigKey(key).SetConfigValue(value).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	return client.SystemConfig.UpdateOneID(existing.ID).SetConfigValue(value).Exec(ctx)
}

// HasInboundSyncToken 是否已配置入站同步密钥。
func HasInboundSyncToken(ctx context.Context, client *ent.Client) bool {
	h, ok, _ := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncInboundTokenSHA256)
	return ok && strings.TrimSpace(h) != ""
}

// InboundSyncTokenMeta 入站密钥元数据（不含明文）。
func InboundSyncTokenMeta(ctx context.Context, client *ent.Client) (configured bool, createdAt string) {
	h, ok, _ := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncInboundTokenSHA256)
	if !ok || strings.TrimSpace(h) == "" {
		return false, ""
	}
	at, _, _ := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncInboundTokenAt)
	return true, at
}

// ValidateInboundSyncToken 校验 Bearer 入站同步密钥。
func ValidateInboundSyncToken(ctx context.Context, client *ent.Client, bearer string) bool {
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return false
	}
	stored, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncInboundTokenSHA256)
	if err != nil || !ok || stored == "" {
		return false
	}
	sum := sha256.Sum256([]byte(bearer))
	got := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(stored)) == 1
}

// GenerateOutboundSyncAPIKey 生成出站访问外部系统用的随机密钥（不落库）。
func GenerateOutboundSyncAPIKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return inboundTokenPrefix + hex.EncodeToString(b), nil
}

// InboundPricingPaths 对外定价同步路径（相对站点根）。
func InboundPricingPaths() (exportPath, importPath string) {
	return "/api/pricing/sync/export", "/api/pricing/sync/import"
}

// BuildInboundSyncURLs 构造供他人访问的完整 URL。
func BuildInboundSyncURLs(ctx context.Context, client *ent.Client, requestScheme, requestHost string) (exportURL, importURL string, base string) {
	exportPath, importPath := InboundPricingPaths()
	pub := EffectiveAPIPublicBaseURL(ctx, client)
	if pub != "" {
		pub = strings.TrimRight(pub, "/")
		if i := strings.Index(pub, "/v1"); i > 0 {
			base = pub[:i]
		} else {
			base = pub
		}
	} else if requestHost != "" {
		sch := requestScheme
		if sch == "" {
			sch = "https"
		}
		base = fmt.Sprintf("%s://%s", sch, requestHost)
	}
	if base == "" {
		return "", "", ""
	}
	return base + exportPath, base + importPath, base
}
