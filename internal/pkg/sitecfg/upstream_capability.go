package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyUpstreamCapabilityFailStaleHours = "upstream_capability.fail_stale_hours"
	SystemConfigKeyUpstreamCapabilityProbeEnabled   = "upstream_capability_probe.enabled"
)

func DefaultUpstreamCapabilityRuntimeEnabled() bool   { return true }
func DefaultUpstreamCapabilityPersistEnabled() bool   { return true }
func DefaultUpstreamCapabilityRedisOverlay() bool     { return true }
func DefaultUpstreamCapabilityFailThrottleSec() int   { return 300 }
func DefaultUpstreamCapabilityFlusherIntervalSec() int { return 30 }
func DefaultUpstreamCapabilityFailStaleHours() int    { return 6 }
func DefaultUpstreamCapabilityProbeEnabled() bool       { return false }

func ValidateUpstreamCapabilityFailStaleHours(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errInvalidUpstreamCapabilityFailStale
	}
	if n < 0 || n > 168 {
		return errUpstreamCapabilityFailStaleRange
	}
	return nil
}

func ValidateUpstreamCapabilityProbeEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidUpstreamCapabilityProbeEnabled
	}
	return nil
}

func EffectiveUpstreamCapabilityFailStaleHours(ctx context.Context, client *ent.Client) int {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamCapabilityFailStaleHours)
	if err != nil || !ok {
		return DefaultUpstreamCapabilityFailStaleHours()
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || n > 168 {
		return DefaultUpstreamCapabilityFailStaleHours()
	}
	return n
}

func EffectiveUpstreamCapabilityProbeEnabled(ctx context.Context, client *ent.Client) bool {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamCapabilityProbeEnabled)
	if err != nil || !ok {
		return DefaultUpstreamCapabilityProbeEnabled()
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return DefaultUpstreamCapabilityProbeEnabled()
	}
	return b
}

func UpstreamCapabilityFailStaleHoursDBMeta(ctx context.Context, client *ent.Client) (set bool, val string, err error) {
	row, e := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyUpstreamCapabilityFailStaleHours)).Only(ctx)
	if ent.IsNotFound(e) {
		return false, "", nil
	}
	if e != nil {
		return false, "", e
	}
	return true, strings.TrimSpace(row.ConfigValue), nil
}
