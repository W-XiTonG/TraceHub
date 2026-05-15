package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	// SystemConfigKeyUpstreamModelProbeEnabled 是否启用后台周期性上游 /v1/models 探测。
	SystemConfigKeyUpstreamModelProbeEnabled = "upstream_model_probe.enabled"
	// SystemConfigKeyUpstreamModelProbeIntervalMinutes 探测周期间隔（分钟），建议 15–1440。
	SystemConfigKeyUpstreamModelProbeIntervalMinutes = "upstream_model_probe.interval_minutes"
)

// DefaultUpstreamModelProbeEnabled 默认关闭，避免未预期外呼。
func DefaultUpstreamModelProbeEnabled() bool { return false }

// DefaultUpstreamModelProbeIntervalMinutes 默认 6 小时。
func DefaultUpstreamModelProbeIntervalMinutes() int { return 360 }

// ValidateUpstreamModelProbeEnabled 校验 true/false。
func ValidateUpstreamModelProbeEnabled(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidUpstreamProbeEnabled
	}
	return nil
}

// ValidateUpstreamModelProbeIntervalMinutes 校验 15–1440。
func ValidateUpstreamModelProbeIntervalMinutes(value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errInvalidUpstreamProbeInterval
	}
	if n < 15 || n > 1440 {
		return errUpstreamProbeIntervalRange
	}
	return nil
}

// EffectiveUpstreamModelProbeEnabled 读取 DB 覆盖，缺省为默认关闭。
func EffectiveUpstreamModelProbeEnabled(ctx context.Context, client *ent.Client) bool {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamModelProbeEnabled)
	if err != nil || !ok {
		return DefaultUpstreamModelProbeEnabled()
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return DefaultUpstreamModelProbeEnabled()
	}
	return b
}

// EffectiveUpstreamModelProbeIntervalMinutes 读取 DB 覆盖，缺省 360；非法则回退默认。
func EffectiveUpstreamModelProbeIntervalMinutes(ctx context.Context, client *ent.Client) int {
	v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamModelProbeIntervalMinutes)
	if err != nil || !ok {
		return DefaultUpstreamModelProbeIntervalMinutes()
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 15 || n > 1440 {
		return DefaultUpstreamModelProbeIntervalMinutes()
	}
	return n
}

// UpstreamModelProbeDBMeta 返回是否曾在 DB 中设置过（供管理页展示）。
func UpstreamModelProbeDBMeta(ctx context.Context, client *ent.Client) (enabledSet bool, enabledVal string, intervalSet bool, intervalVal string, err error) {
	row, e1 := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyUpstreamModelProbeEnabled)).Only(ctx)
	if ent.IsNotFound(e1) {
		enabledSet = false
	} else if e1 != nil {
		err = e1
		return
	} else {
		enabledSet = true
		enabledVal = strings.TrimSpace(row.ConfigValue)
	}
	row2, e2 := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(SystemConfigKeyUpstreamModelProbeIntervalMinutes)).Only(ctx)
	if ent.IsNotFound(e2) {
		intervalSet = false
	} else if e2 != nil {
		err = e2
		return
	} else {
		intervalSet = true
		intervalVal = strings.TrimSpace(row2.ConfigValue)
	}
	return
}
