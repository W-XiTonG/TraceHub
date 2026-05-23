package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyRoutingStrategyDefault              = "gateway.routing.strategy_default"
	SystemConfigKeyRoutingFailoverEnabledDefault         = "gateway.routing.failover_enabled_default"
	SystemConfigKeyRoutingFailoverMaxAttempts            = "gateway.routing.failover_max_attempts_default"
	SystemConfigKeyRoutingStickyEnabledDefault           = "gateway.routing.sticky_enabled_default"
	SystemConfigKeyRoutingAutoHealthFromProbe            = "gateway.routing.auto_health_from_probe"
	SystemConfigKeyRoutingProbeFailThreshold               = "gateway.routing.probe_fail_threshold"
	SystemConfigKeyRoutingFailoverFirstAttemptTimeoutSec   = "gateway.routing.failover_first_attempt_timeout_sec"
	SystemConfigKeyRoutingFailoverRetryAttemptTimeoutSec   = "gateway.routing.failover_retry_attempt_timeout_sec"
	SystemConfigKeyRoutingFailoverRetryAfterSec            = "gateway.routing.failover_retry_after_sec"
	SystemConfigKeyRoutingFailover4xxModeDefault           = "gateway.routing.failover_4xx_mode_default"
)

const (
	Failover4xxModeOff        = "off"
	Failover4xxModeSelective  = "selective"
	Failover4xxModeAggressive = "aggressive"
	Failover4xxModeCustom     = "custom"
)

const (
	RoutingStrategyWeightedRandom = "weighted_random"
	RoutingStrategyPriority       = "priority"
	RoutingStrategyFixedOrder     = "fixed_order"
)

// RoutingDefaults 全局默认路由配置（解析分组覆盖前的基准值）。
type RoutingDefaults struct {
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
}

func DefaultRoutingDefaults() RoutingDefaults {
	return RoutingDefaults{
		Strategy:                       RoutingStrategyWeightedRandom,
		FailoverEnabled:                true,
		FailoverMaxAttempts:            2,
		StickyEnabled:                  true,
		AutoHealthFromProbe:            false,
		ProbeFailThreshold:             3,
		FailoverFirstAttemptTimeoutSec: 30,
		FailoverRetryAttemptTimeoutSec: 0,
		FailoverRetryAfterSec:          15,
		Failover4xxMode:                Failover4xxModeOff,
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

func ValidateRoutingFailoverFirstAttemptTimeoutSec(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errRoutingFailoverFirstTimeout
	}
	if n < 5 || n > 120 {
		return errRoutingFailoverFirstTimeoutRange
	}
	return nil
}

func ValidateRoutingFailoverRetryAttemptTimeoutSec(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errRoutingFailoverRetryTimeout
	}
	if n < 0 || n > 600 {
		return errRoutingFailoverRetryTimeoutRange
	}
	return nil
}

func ValidateRoutingFailoverRetryAfterSec(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errRoutingFailoverRetryAfter
	}
	if n < 1 || n > 300 {
		return errRoutingFailoverRetryAfterRange
	}
	return nil
}

func ValidateRoutingFailover4xxMode(value string) error {
	switch strings.TrimSpace(value) {
	case "", Failover4xxModeOff, Failover4xxModeSelective, Failover4xxModeAggressive, Failover4xxModeCustom:
		return nil
	default:
		return errRoutingFailover4xxMode
	}
}

func NormalizeFailover4xxMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case Failover4xxModeSelective, Failover4xxModeAggressive, Failover4xxModeCustom:
		return strings.TrimSpace(mode)
	default:
		return Failover4xxModeOff
	}
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
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailoverFirstAttemptTimeoutSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 5 && n <= 120 {
			d.FailoverFirstAttemptTimeoutSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailoverRetryAttemptTimeoutSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 0 && n <= 600 {
			d.FailoverRetryAttemptTimeoutSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailoverRetryAfterSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 1 && n <= 300 {
			d.FailoverRetryAfterSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailover4xxModeDefault); err == nil && ok {
		if ValidateRoutingFailover4xxMode(v) == nil {
			d.Failover4xxMode = NormalizeFailover4xxMode(v)
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyRoutingFailover4xxCodesDefault); err == nil && ok {
		if codes, err := ParseFailover4xxCodes(v); err == nil {
			d.Failover4xxCodes = codes
		}
	}
	if err := ValidateRoutingFailover4xxCodesForMode(d.Failover4xxMode, d.Failover4xxCodes); err != nil {
		d.Failover4xxCodes = nil
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
		SystemConfigKeyRoutingFailoverFirstAttemptTimeoutSec,
		SystemConfigKeyRoutingFailoverRetryAttemptTimeoutSec,
		SystemConfigKeyRoutingFailoverRetryAfterSec,
		SystemConfigKeyRoutingFailover4xxModeDefault,
		SystemConfigKeyRoutingFailover4xxCodesDefault,
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
