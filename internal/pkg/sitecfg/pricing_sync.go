package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyPricingSyncEnabled           = "site.pricing_sync.enabled"
	SystemConfigKeyPricingSyncWorkerEnabled     = "site.pricing_sync.worker_enabled"
	SystemConfigKeyPricingSyncMaxRowsPerSync    = "site.pricing_sync.max_rows_per_sync"
	SystemConfigKeyPricingSyncRequireSuperadmin = "site.pricing_sync.require_superadmin"
	SystemConfigKeyPricingSyncManualCooldownSec = "site.pricing_sync.manual_cooldown_sec"
)

func DefaultPricingSyncEnabled() bool           { return false }
func DefaultPricingSyncWorkerEnabled() bool     { return false }
func DefaultPricingSyncMaxRowsPerSync() int     { return 5000 }
func DefaultPricingSyncRequireSuperadmin() bool { return true }
func DefaultPricingSyncManualCooldownSec() int  { return 60 }

func ValidatePricingSyncBool(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidPricingSyncBool
	}
	return nil
}

func ValidatePricingSyncMaxRows(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 1 || n > 5000 {
		return errInvalidPricingSyncMaxRows
	}
	return nil
}

func ValidatePricingSyncManualCooldown(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 || n > 3600 {
		return errInvalidPricingSyncCooldown
	}
	return nil
}

func EffectivePricingSyncEnabled(ctx context.Context, client *ent.Client) bool {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncEnabled)
	if err != nil || !ok {
		return DefaultPricingSyncEnabled()
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return DefaultPricingSyncEnabled()
	}
	return b
}

func EffectivePricingSyncWorkerEnabled(ctx context.Context, client *ent.Client) bool {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncWorkerEnabled)
	if err != nil || !ok {
		return DefaultPricingSyncWorkerEnabled()
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return DefaultPricingSyncWorkerEnabled()
	}
	return b
}

func EffectivePricingSyncMaxRows(ctx context.Context, client *ent.Client) int {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncMaxRowsPerSync)
	if err != nil || !ok {
		return DefaultPricingSyncMaxRowsPerSync()
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 || n > 5000 {
		return DefaultPricingSyncMaxRowsPerSync()
	}
	return n
}

func EffectivePricingSyncRequireSuperadmin(ctx context.Context, client *ent.Client) bool {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncRequireSuperadmin)
	if err != nil || !ok {
		return DefaultPricingSyncRequireSuperadmin()
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return DefaultPricingSyncRequireSuperadmin()
	}
	return b
}

func EffectivePricingSyncManualCooldownSec(ctx context.Context, client *ent.Client) int {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyPricingSyncManualCooldownSec)
	if err != nil || !ok {
		return DefaultPricingSyncManualCooldownSec()
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 || n > 3600 {
		return DefaultPricingSyncManualCooldownSec()
	}
	return n
}

func pricingSyncMetaKey(ctx context.Context, client *ent.Client, key string) (set bool, val string, err error) {
	row, e := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(key)).Only(ctx)
	if ent.IsNotFound(e) {
		return false, "", nil
	}
	if e != nil {
		return false, "", e
	}
	return true, strings.TrimSpace(row.ConfigValue), nil
}

// PricingSyncSettingsMeta 供管理页展示 DB 覆盖状态。
func PricingSyncSettingsMeta(ctx context.Context, client *ent.Client) (map[string]any, error) {
	keys := []string{
		SystemConfigKeyPricingSyncEnabled,
		SystemConfigKeyPricingSyncWorkerEnabled,
		SystemConfigKeyPricingSyncMaxRowsPerSync,
		SystemConfigKeyPricingSyncRequireSuperadmin,
		SystemConfigKeyPricingSyncManualCooldownSec,
	}
	out := make(map[string]any)
	for _, k := range keys {
		set, val, err := pricingSyncMetaKey(ctx, client, k)
		if err != nil {
			return nil, err
		}
		out[k+"_db_set"] = set
		if set {
			out[k+"_db_value"] = val
		}
	}
	return out, nil
}
