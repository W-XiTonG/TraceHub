package router

import (
	"context"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

// RoutingConfig 合并全局默认与分组覆盖后的生效路由配置。
type RoutingConfig struct {
	Strategy                       string
	FailoverEnabled                bool
	FailoverMaxAttempts            int
	StickyEnabled                  bool
	AutoHealthFromProbe            bool
	ProbeFailThreshold             int
	FailoverFirstAttemptTimeoutSec int
	FailoverRetryAttemptTimeoutSec int
	FailoverRetryAfterSec          int
	Failover4xxMode                string
	Failover4xxCodes               []int
	ModelRouting                   map[string][]int64
}

type routingCacheEntry struct {
	cfg     RoutingConfig
	expires time.Time
}

var routingCache sync.Map // groupID int -> routingCacheEntry

const routingCacheTTL = 60 * time.Second

// InvalidateRoutingCache 管理端更新分组或全局路由配置后调用。
func InvalidateRoutingCache(groupID int) {
	if groupID > 0 {
		routingCache.Delete(groupID)
	}
}

// InvalidateAllRoutingCache 全局路由配置变更后调用。
func InvalidateAllRoutingCache() {
	routingCache.Range(func(k, _ any) bool {
		routingCache.Delete(k)
		return true
	})
}

func boolFromAny(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case float64:
		return t != 0, true
	case string:
		if t == "true" {
			return true, true
		}
		if t == "false" {
			return false, true
		}
	}
	return false, false
}

func intFromAny(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	}
	return 0, false
}

func stringFromAny(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s != "" {
			return s, true
		}
	}
	return "", false
}

func intSliceFromAny(v any) ([]int, bool) {
	switch t := v.(type) {
	case []int:
		if len(t) == 0 {
			return nil, false
		}
		return sitecfg.NormalizeFailover4xxCodeSlice(t), true
	case []int64:
		if len(t) == 0 {
			return nil, false
		}
		out := make([]int, len(t))
		for i, n := range t {
			out[i] = int(n)
		}
		return sitecfg.NormalizeFailover4xxCodeSlice(out), true
	case []float64:
		if len(t) == 0 {
			return nil, false
		}
		out := make([]int, len(t))
		for i, n := range t {
			out[i] = int(n)
		}
		return sitecfg.NormalizeFailover4xxCodeSlice(out), true
	case []any:
		if len(t) == 0 {
			return nil, false
		}
		out := make([]int, 0, len(t))
		for _, item := range t {
			if n, ok := intFromAny(item); ok {
				out = append(out, n)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return sitecfg.NormalizeFailover4xxCodeSlice(out), true
	}
	return nil, false
}

func routingDefaultsToSitecfg(d RoutingConfig) sitecfg.RoutingDefaults {
	return sitecfg.RoutingDefaults{
		Strategy:                       d.Strategy,
		FailoverEnabled:                d.FailoverEnabled,
		FailoverMaxAttempts:            d.FailoverMaxAttempts,
		StickyEnabled:                  d.StickyEnabled,
		AutoHealthFromProbe:            d.AutoHealthFromProbe,
		ProbeFailThreshold:             d.ProbeFailThreshold,
		FailoverFirstAttemptTimeoutSec: d.FailoverFirstAttemptTimeoutSec,
		FailoverRetryAttemptTimeoutSec: d.FailoverRetryAttemptTimeoutSec,
		FailoverRetryAfterSec:          d.FailoverRetryAfterSec,
		Failover4xxMode:                d.Failover4xxMode,
		Failover4xxCodes:               append([]int(nil), d.Failover4xxCodes...),
	}
}

func mergeRoutingOptions(global sitecfg.RoutingDefaults, groupOpts map[string]any) RoutingConfig {
	cfg := RoutingConfig{
		Strategy:                       global.Strategy,
		FailoverEnabled:                global.FailoverEnabled,
		FailoverMaxAttempts:            global.FailoverMaxAttempts,
		StickyEnabled:                  global.StickyEnabled,
		AutoHealthFromProbe:            global.AutoHealthFromProbe,
		ProbeFailThreshold:             global.ProbeFailThreshold,
		FailoverFirstAttemptTimeoutSec: global.FailoverFirstAttemptTimeoutSec,
		FailoverRetryAttemptTimeoutSec: global.FailoverRetryAttemptTimeoutSec,
		FailoverRetryAfterSec:          global.FailoverRetryAfterSec,
		Failover4xxMode:                sitecfg.NormalizeFailover4xxMode(global.Failover4xxMode),
		Failover4xxCodes:               append([]int(nil), global.Failover4xxCodes...),
	}
	if groupOpts == nil {
		return cfg
	}
	if v, ok := boolFromAny(groupOpts["failover_enabled"]); ok {
		cfg.FailoverEnabled = v
	}
	if v, ok := intFromAny(groupOpts["failover_max_attempts"]); ok && v >= 1 && v <= 5 {
		cfg.FailoverMaxAttempts = v
	}
	if v, ok := boolFromAny(groupOpts["sticky_enabled"]); ok {
		cfg.StickyEnabled = v
	}
	if v, ok := boolFromAny(groupOpts["auto_health_from_probe"]); ok {
		cfg.AutoHealthFromProbe = v
	}
	if v, ok := intFromAny(groupOpts["probe_fail_threshold"]); ok && v >= 1 && v <= 20 {
		cfg.ProbeFailThreshold = v
	}
	if v, ok := intFromAny(groupOpts["failover_first_attempt_timeout_sec"]); ok && v >= 5 && v <= 120 {
		cfg.FailoverFirstAttemptTimeoutSec = v
	}
	if v, ok := intFromAny(groupOpts["failover_retry_attempt_timeout_sec"]); ok && v >= 0 && v <= 600 {
		cfg.FailoverRetryAttemptTimeoutSec = v
	}
	if v, ok := intFromAny(groupOpts["failover_retry_after_sec"]); ok && v >= 1 && v <= 300 {
		cfg.FailoverRetryAfterSec = v
	}
	if v, ok := stringFromAny(groupOpts["failover_4xx_mode"]); ok {
		if sitecfg.ValidateRoutingFailover4xxMode(v) == nil {
			cfg.Failover4xxMode = sitecfg.NormalizeFailover4xxMode(v)
		}
	}
	if v, ok := intSliceFromAny(groupOpts["failover_4xx_codes"]); ok {
		cfg.Failover4xxCodes = v
	}
	return cfg
}

// ResolveRoutingConfig 读取全局默认并合并分组覆盖；结果缓存 60s。
func ResolveRoutingConfig(ctx context.Context, client *ent.Client, groupID int) (RoutingConfig, error) {
	if groupID <= 0 {
		return RoutingConfig{}, nil
	}
	if ent, ok := routingCache.Load(groupID); ok {
		e := ent.(routingCacheEntry)
		if time.Now().Before(e.expires) {
			return e.cfg, nil
		}
		routingCache.Delete(groupID)
	}
	global := sitecfg.EffectiveRoutingDefaults(ctx, client)
	cfg := mergeRoutingOptions(global, nil)
	g, err := client.Group.Get(ctx, groupID)
	if err != nil {
		return RoutingConfig{}, err
	}
	if s := g.RoutingStrategy; s != "" {
		if sitecfg.ValidateRoutingStrategy(s) == nil {
			cfg.Strategy = s
		}
	}
	cfg = mergeRoutingOptions(routingDefaultsToSitecfg(cfg), g.RoutingOptions)
	if g.ModelRouting != nil {
		cfg.ModelRouting = g.ModelRouting
	}
	routingCache.Store(groupID, routingCacheEntry{cfg: cfg, expires: time.Now().Add(routingCacheTTL)})
	return cfg, nil
}
