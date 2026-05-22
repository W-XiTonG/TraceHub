package sitecfg

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
	"tracehub/internal/pkg/config"
)

const (
	SystemConfigKeyHTTPSForceRedirect = "site.https.force_redirect"
	SystemConfigKeyHTTPSHSTSEnabled     = "site.https.hsts_enabled"
	SystemConfigKeyHTTPSHSTSMaxAge      = "site.https.hsts_max_age_seconds"
	SystemConfigKeyTLSEnabled           = "server.tls.enabled"
	SystemConfigKeyTLSCertFile          = "server.tls.cert_file"
	SystemConfigKeyTLSKeyFile           = "server.tls.key_file"
)

// TLSCertKeyPaths 内置 TLS 证书与私钥的生效路径（DB 覆盖 YAML）。
type TLSCertKeyPaths struct {
	CertFile string
	KeyFile  string
}

const httpsPolicyCacheTTL = 10 * time.Second

const defaultHSTSMaxAgeSeconds = 31536000

// HTTPSPolicy 运行时 HTTPS 策略快照（由短 TTL 缓存提供，热路径不查库）。
type HTTPSPolicy struct {
	ForceRedirect     bool
	HSTSEnabled       bool
	HSTSMaxAgeSeconds int
}

var (
	httpsPolicyCacheMu sync.Mutex
	httpsPolicyCache   *httpsPolicyCacheEntry
)

type httpsPolicyCacheEntry struct {
	policy  HTTPSPolicy
	expires time.Time
}

// InvalidateHTTPSPolicyCache 管理端写入 site.https.* 后调用。
func InvalidateHTTPSPolicyCache() {
	httpsPolicyCacheMu.Lock()
	httpsPolicyCache = nil
	httpsPolicyCacheMu.Unlock()
}

// CachedHTTPSPolicy 返回当前生效的 HTTPS 策略（带短 TTL 缓存）。
func CachedHTTPSPolicy(ctx context.Context, client *ent.Client, cfg *config.Config) HTTPSPolicy {
	now := time.Now()
	httpsPolicyCacheMu.Lock()
	if httpsPolicyCache != nil && now.Before(httpsPolicyCache.expires) {
		p := httpsPolicyCache.policy
		httpsPolicyCacheMu.Unlock()
		return p
	}
	httpsPolicyCacheMu.Unlock()

	p := loadHTTPSPolicy(ctx, client, cfg)

	httpsPolicyCacheMu.Lock()
	httpsPolicyCache = &httpsPolicyCacheEntry{policy: p, expires: now.Add(httpsPolicyCacheTTL)}
	httpsPolicyCacheMu.Unlock()
	return p
}

func loadHTTPSPolicy(ctx context.Context, client *ent.Client, cfg *config.Config) HTTPSPolicy {
	file := fileHTTPSPolicy(cfg)
	if client == nil {
		return file
	}
	fr, okFR, errFR := boolFromConfigRow(ctx, client, SystemConfigKeyHTTPSForceRedirect)
	if errFR != nil {
		return file
	}
	he, okHE, errHE := boolFromConfigRow(ctx, client, SystemConfigKeyHTTPSHSTSEnabled)
	if errHE != nil {
		return file
	}
	ma, okMA, errMA := intFromConfigRow(ctx, client, SystemConfigKeyHTTPSHSTSMaxAge)
	if errMA != nil {
		return file
	}
	out := file
	if okFR {
		out.ForceRedirect = fr
	}
	if okHE {
		out.HSTSEnabled = he
	}
	if okMA && ma > 0 {
		out.HSTSMaxAgeSeconds = ma
	}
	return out
}

func fileHTTPSPolicy(cfg *config.Config) HTTPSPolicy {
	if cfg == nil {
		return HTTPSPolicy{HSTSMaxAgeSeconds: defaultHSTSMaxAgeSeconds}
	}
	ma := cfg.Site.HTTPS.HSTSMaxAgeSeconds
	if ma <= 0 {
		ma = defaultHSTSMaxAgeSeconds
	}
	return HTTPSPolicy{
		ForceRedirect:     cfg.Site.HTTPS.ForceRedirect,
		HSTSEnabled:       cfg.Site.HTTPS.HSTSEnabled,
		HSTSMaxAgeSeconds: ma,
	}
}

// TLSEnabledAtStartup 进程启动时读取是否开启内置 TLS（DB 覆盖 YAML）。
func TLSEnabledAtStartup(ctx context.Context, client *ent.Client, cfg *config.Config) bool {
	file := false
	if cfg != nil {
		file = cfg.Server.TLS.Enabled
	}
	if client == nil {
		return file
	}
	b, ok, err := boolFromConfigRow(ctx, client, SystemConfigKeyTLSEnabled)
	if err != nil || !ok {
		return file
	}
	return b
}

// HTTPSForceRedirectDBMeta 是否已在库中写入 force_redirect。
func HTTPSForceRedirectDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyHTTPSForceRedirect)
}

func HTTPSHSTSEnabledDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyHTTPSHSTSEnabled)
}

func HTTPSHSTSMaxAgeDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyHTTPSHSTSMaxAge)
}

func TLSEnabledDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyTLSEnabled)
}

func TLSCertFileDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyTLSCertFile)
}

func TLSKeyFileDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string) {
	return configRowMeta(ctx, client, SystemConfigKeyTLSKeyFile)
}

// EffectiveTLSCertKeyPaths 返回生效的证书/私钥路径（库中非空路径覆盖 YAML）。
func EffectiveTLSCertKeyPaths(ctx context.Context, client *ent.Client, cfg *config.Config) TLSCertKeyPaths {
	out := TLSCertKeyPaths{}
	if cfg != nil {
		out.CertFile = strings.TrimSpace(cfg.Server.TLS.CertFile)
		out.KeyFile = strings.TrimSpace(cfg.Server.TLS.KeyFile)
	}
	if client == nil {
		return out
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyTLSCertFile); err == nil && ok && strings.TrimSpace(v) != "" {
		out.CertFile = strings.TrimSpace(v)
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyTLSKeyFile); err == nil && ok && strings.TrimSpace(v) != "" {
		out.KeyFile = strings.TrimSpace(v)
	}
	return out
}

func configRowMeta(ctx context.Context, client *ent.Client, key string) (hasRow bool, raw string) {
	if client == nil {
		return false, ""
	}
	row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
	if err != nil {
		return false, ""
	}
	return true, row.ConfigValue
}

func boolFromConfigRow(ctx context.Context, client *ent.Client, key string) (bool, bool, error) {
	v, ok, err := stringFromConfigRow(ctx, client, key)
	if err != nil || !ok {
		return false, false, err
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return false, true, nil
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, false, err
	}
	return b, true, nil
}

func intFromConfigRow(ctx context.Context, client *ent.Client, key string) (int, bool, error) {
	v, ok, err := stringFromConfigRow(ctx, client, key)
	if err != nil || !ok {
		return 0, false, err
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return 0, true, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

// ValidateHTTPSForceRedirect 写入 site.https.force_redirect 前校验。
func ValidateHTTPSForceRedirect(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidHTTPSForceRedirect
	}
	return nil
}

func ValidateHTTPSHSTSEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidHTTPSHSTSEnabled
	}
	return nil
}

func ValidateHTTPSHSTSMaxAge(value string) error {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return errInvalidHTTPSHSTSMaxAge
	}
	if n < 0 || n > 63072000 {
		return errHTTPSHSTSMaxAgeRange
	}
	return nil
}

func ValidateTLSEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidTLSEnabled
	}
	return nil
}
