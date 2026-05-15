package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyRoutingStrategyDefault         = "gateway.routing.strategy_default"
	SystemConfigKeyRoutingFailoverEnabledDefault  = "gateway.routing.failover_enabled_default"
	SystemConfigKeyRoutingFailoverMaxAttempts     = "gateway.routing.failover_max_attempts_default"
	SystemConfigKeyRoutingStickyEnabledDefault    = "gateway.routing.sticky_enabled_default"
	SystemConfigKeyRoutingAutoHealthFromProbe     = "gateway.routing.auto_health_from_probe"
	SystemConfigKeyRoutingProbeFailThreshold      = "gateway.routing.probe_fail_threshold"
)

const (
	RoutingStrategyWeightedRandom = "weighted_random"
	RoutingStrategyPriority       = "priority"
	RoutingStrategyFixedOrder     = "fixed_order"
)

// RoutingDefaults 全局默认路由配置（解析分组覆盖前的基准值）。
type RoutingDefaults struct {
	Strategy            string
	FailoverEnabled     bool
	FailoverMaxAttempts int
	StickyEnabled       bool
	AutoHealthFromProbe bool
	ProbeFailThreshold  int
}

func DefaultRoutingDefaults() RoutingDefaults {
	return RoutingDefaults{
		Strategy:            RoutingStrategyWeightedRandom,
		FailoverEnabled:     false,
		FailoverMaxAttempts: 2,
		StickyEnabled:       true,
		AutoHealthFromProbe: false,
		ProbeFailThreshold:  3,
	}
}

func ValidateRoutingStrategy(value string) error {
	s := strings.TrimSpace(value)
	if s == "" {
		return nil
	}
	switch s {
	case RoutingStrategyWeightedRandom, RoutingStrategyPriority, RoutingStrategyFixedOrder:
		return nil
	default:
		return errRoutingStrategyInvalid
	}
}

func ValidateRoutingFailoverEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errRoutingFailoverBool
	}
	return nil
}

func ValidateRoutingFailoverMaxAttempts(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errRoutingFailoverAttempts
	}
	if n < 1 || n > 5 {
		return errRoutingFailoverAttemptsRange
	}
	return nil
}

func ValidateRoutingStickyEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errRoutingStickyBool
	}
	return nil
}

func ValidateRoutingAutoHealthFromProbe(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errRoutingAutoHealthBool
	}
	return nil
}

func ValidateRoutingProbeFailThreshold(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errRoutingProbeThreshold
	}
	if n < 1 || n > 20 {
		return errRoutingProbeThresholdRange
	}
	return nil
}

func EffectiveRoutingDefaults(ctx context.Context, client *ent.Client) RoutingDefaults {
	d := DefaultRoutingDefaults()
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingStrategyDefault); err == nil && ok {
		if s := strings.TrimSpace(v); s != "" {
			d.Strategy = s
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailoverEnabledDefault); err == nil && ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			d.FailoverEnabled = b
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailoverMaxAttempts); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 && n <= 5 {
			d.FailoverMaxAttempts = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingStickyEnabledDefault); err == nil && ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			d.StickyEnabled = b
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingAutoHealthFromProbe); err == nil && ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			d.AutoHealthFromProbe = b
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingProbeFailThreshold); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 && n <= 20 {
			d.ProbeFailThreshold = n
		}
	}
	if err := ValidateRoutingStrategy(d.Strategy); err != nil {
		d.Strategy = RoutingStrategyWeightedRandom
	}
	return d
}

// RoutingDBMeta 供管理端展示 DB 是否覆盖全局默认。
func RoutingDBMeta(ctx context.Context, client *ent.Client) (map[string]map[string]any, error) {
	keys := []string{
		SystemConfigKeyRoutingStrategyDefault,
		SystemConfigKeyRoutingFailoverEnabledDefault,
		SystemConfigKeyRoutingFailoverMaxAttempts,
		SystemConfigKeyRoutingStickyEnabledDefault,
		SystemConfigKeyRoutingAutoHealthFromProbe,
		SystemConfigKeyRoutingProbeFailThreshold,
	}
	out := make(map[string]map[string]any, len(keys))
	for _, k := range keys {
		row, err := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(k)).Only(ctx)
		if ent.IsNotFound(err) {
			out[k] = map[string]any{"db_set": false, "db_value": ""}
			continue
		}
		if err != nil {
			return nil, err
		}
		out[k] = map[string]any{"db_set": true, "db_value": strings.TrimSpace(row.ConfigValue)}
	}
	return out, nil
}
