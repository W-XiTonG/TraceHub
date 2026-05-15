package router

import (
	"context"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

// RoutingConfig 合并全局默认与分组覆盖后的生效路由配置。
type RoutingConfig struct {
	Strategy            string
	FailoverEnabled     bool
	FailoverMaxAttempts int
	StickyEnabled       bool
	AutoHealthFromProbe bool
	ProbeFailThreshold  int
	ModelRouting        map[string][]int64
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

func mergeRoutingOptions(global sitecfg.RoutingDefaults, groupOpts map[string]any) RoutingConfig {
	cfg := RoutingConfig{
		Strategy:            global.Strategy,
		FailoverEnabled:     global.FailoverEnabled,
		FailoverMaxAttempts: global.FailoverMaxAttempts,
		StickyEnabled:       global.StickyEnabled,
		AutoHealthFromProbe: global.AutoHealthFromProbe,
		ProbeFailThreshold:  global.ProbeFailThreshold,
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
	cfg = mergeRoutingOptions(sitecfg.RoutingDefaults{
		Strategy:            cfg.Strategy,
		FailoverEnabled:     cfg.FailoverEnabled,
		FailoverMaxAttempts: cfg.FailoverMaxAttempts,
		StickyEnabled:       cfg.StickyEnabled,
		AutoHealthFromProbe: cfg.AutoHealthFromProbe,
		ProbeFailThreshold:  cfg.ProbeFailThreshold,
	}, g.RoutingOptions)
	if g.ModelRouting != nil {
		cfg.ModelRouting = g.ModelRouting
	}
	routingCache.Store(groupID, routingCacheEntry{cfg: cfg, expires: time.Now().Add(routingCacheTTL)})
	return cfg, nil
}
