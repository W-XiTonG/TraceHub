package sitecfg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

// SystemConfigKeyGatewayIPBlacklist 全局 OpenAI 网关（/v1）客户端 IP 黑名单：JSON 字符串数组；
// 非空时列表内 IP/CIDR 不可访问 /v1；空数组或未配置表示不启用。
const SystemConfigKeyGatewayIPBlacklist = "gateway.ip_blacklist"

// SystemConfigKeyGatewayIPWhitelist 已废弃：历史版本曾用 gateway.ip_whitelist 作白名单；恢复默认时会删除。
const SystemConfigKeyGatewayIPWhitelist = "gateway.ip_whitelist"

const gatewayIPBlacklistCacheTTL = 5 * time.Second

var (
	gwIPBLCacheMu    sync.Mutex
	gwIPBLCacheList  []string
	gwIPBLCacheUntil time.Time
)

// InvalidateGatewayIPBlacklistCache 管理端更新 gateway.ip_blacklist 后调用，使下次请求重新读库。
func InvalidateGatewayIPBlacklistCache() {
	gwIPBLCacheMu.Lock()
	gwIPBLCacheList = nil
	gwIPBLCacheUntil = time.Time{}
	gwIPBLCacheMu.Unlock()
}

// CachedGatewayIPBlacklist 返回当前生效的全局网关 IP 黑名单（可能来自短 TTL 缓存）。
func CachedGatewayIPBlacklist(ctx context.Context, client *ent.Client) []string {
	now := time.Now()
	gwIPBLCacheMu.Lock()
	defer gwIPBLCacheMu.Unlock()
	if !gwIPBLCacheUntil.IsZero() && now.Before(gwIPBLCacheUntil) {
		return gwIPBLCacheList
	}
	list, err := loadGatewayIPBlacklistFromDB(ctx, client)
	if err != nil {
		// 读库失败时短时退避；不启用黑名单以免误伤全站。
		gwIPBLCacheList = nil
		gwIPBLCacheUntil = now.Add(2 * time.Second)
		return nil
	}
	gwIPBLCacheList = list
	gwIPBLCacheUntil = now.Add(gatewayIPBlacklistCacheTTL)
	return list
}

func loadGatewayIPBlacklistFromDB(ctx context.Context, client *ent.Client) ([]string, error) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyGatewayIPBlacklist)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return normalizeGatewayIPBlacklistList(parseGatewayIPBlacklistJSON(row.ConfigValue)), nil
}

// GatewayIPBlacklistDBMeta 管理端：是否库中有行、原始 JSON、解析后的列表。
func GatewayIPBlacklistDBMeta(ctx context.Context, client *ent.Client) (hasRow bool, raw string, list []string) {
	row, err := client.SystemConfig.Query().
		Where(systemconfig.ConfigKeyEQ(SystemConfigKeyGatewayIPBlacklist)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return false, "", nil
		}
		return false, "", nil
	}
	list = normalizeGatewayIPBlacklistList(parseGatewayIPBlacklistJSON(row.ConfigValue))
	return true, row.ConfigValue, list
}

// ValidateGatewayIPBlacklistJSON 管理端写入 gateway.ip_blacklist 前校验（JSON 数组，元素为 IPv4/IPv6 或 CIDR）。
func ValidateGatewayIPBlacklistJSON(value string) error {
	v := strings.TrimSpace(value)
	if v == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(v), &arr); err != nil {
		return errGatewayIPBlacklistJSON
	}
	for _, s := range arr {
		s = strings.TrimSpace(s)
		if s == "" {
			return errGatewayIPBlacklistEmptyEntry
		}
		if err := validateGatewayIPBlacklistEntry(s); err != nil {
			return err
		}
	}
	return nil
}

func validateGatewayIPBlacklistEntry(s string) error {
	if strings.Contains(s, "/") {
		_, _, err := net.ParseCIDR(s)
		if err != nil {
			return fmt.Errorf("%w: %q", errGatewayIPBlacklistBadCIDR, s)
		}
		return nil
	}
	if net.ParseIP(s) == nil {
		return fmt.Errorf("%w: %q", errGatewayIPBlacklistBadIP, s)
	}
	return nil
}

func parseGatewayIPBlacklistJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err != nil {
		return nil
	}
	return arr
}

func normalizeGatewayIPBlacklistList(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var (
	errGatewayIPBlacklistJSON    = errors.New("gateway.ip_blacklist 须为 JSON 字符串数组")
	errGatewayIPBlacklistEmptyEntry = errors.New("gateway.ip_blacklist 数组元素不能为空字符串")
	errGatewayIPBlacklistBadCIDR = errors.New("无效 CIDR")
	errGatewayIPBlacklistBadIP   = errors.New("无效 IP")
)
