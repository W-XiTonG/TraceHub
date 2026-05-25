package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/ent/systemconfig"
)

const (
	SystemConfigKeyUpstreamModelProbeReconcileEnabled    = "upstream_model_probe.reconcile_enabled"
	SystemConfigKeyUpstreamModelProbeSyncOnIncrease      = "upstream_model_probe.sync_on_increase"
	SystemConfigKeyUpstreamModelProbeSyncOnDecrease      = "upstream_model_probe.sync_on_decrease"
	SystemConfigKeyUpstreamModelProbeNotifyOnIncrease   = "upstream_model_probe.notify_on_increase"
	SystemConfigKeyUpstreamModelProbeNotifyOnDecrease    = "upstream_model_probe.notify_on_decrease"
	SystemConfigKeyUpstreamModelProbeReconcileScope      = "upstream_model_probe.reconcile_scope"

	UpstreamModelProbeReconcileScopeModelsOnly      = "models_only"
	UpstreamModelProbeReconcileScopeModelsAndAlias  = "models_and_alias"
)

// UpstreamModelProbeReconcileEffective 探测成功后可选联动可用模型与通知。
type UpstreamModelProbeReconcileEffective struct {
	Enabled          bool
	SyncOnIncrease   bool
	SyncOnDecrease   bool
	NotifyOnIncrease bool
	NotifyOnDecrease bool
	Scope            string
}

func DefaultUpstreamModelProbeReconcileEnabled() bool   { return false }
func DefaultUpstreamModelProbeSyncOnIncrease() bool     { return false }
func DefaultUpstreamModelProbeSyncOnDecrease() bool     { return false }
func DefaultUpstreamModelProbeNotifyOnIncrease() bool   { return false }
func DefaultUpstreamModelProbeNotifyOnDecrease() bool   { return false }
func DefaultUpstreamModelProbeReconcileScope() string {
	return UpstreamModelProbeReconcileScopeModelsOnly
}

func ValidateUpstreamModelProbeReconcileBool(value string) error {
	if _, err := strconv.ParseBool(strings.TrimSpace(value)); err != nil {
		return errInvalidUpstreamProbeReconcileBool
	}
	return nil
}

func ValidateUpstreamModelProbeReconcileScope(value string) error {
	s := strings.TrimSpace(value)
	if s == UpstreamModelProbeReconcileScopeModelsOnly || s == UpstreamModelProbeReconcileScopeModelsAndAlias {
		return nil
	}
	return errInvalidUpstreamProbeReconcileScope
}

func effectiveBoolConfig(ctx context.Context, client *ent.Client, key string, def bool) bool {
	v, ok, err := stringFromConfigRow(ctx, client, key)
	if err != nil || !ok {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
}

// EffectiveUpstreamModelProbeReconcile 读取探测联动配置；缺省均为关闭。
func EffectiveUpstreamModelProbeReconcile(ctx context.Context, client *ent.Client) UpstreamModelProbeReconcileEffective {
	scope, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamModelProbeReconcileScope)
	if err != nil || !ok {
		scope = DefaultUpstreamModelProbeReconcileScope()
	} else {
		scope = strings.TrimSpace(scope)
		if ValidateUpstreamModelProbeReconcileScope(scope) != nil {
			scope = DefaultUpstreamModelProbeReconcileScope()
		}
	}
	return UpstreamModelProbeReconcileEffective{
		Enabled:          effectiveBoolConfig(ctx, client, SystemConfigKeyUpstreamModelProbeReconcileEnabled, DefaultUpstreamModelProbeReconcileEnabled()),
		SyncOnIncrease:   effectiveBoolConfig(ctx, client, SystemConfigKeyUpstreamModelProbeSyncOnIncrease, DefaultUpstreamModelProbeSyncOnIncrease()),
		SyncOnDecrease:   effectiveBoolConfig(ctx, client, SystemConfigKeyUpstreamModelProbeSyncOnDecrease, DefaultUpstreamModelProbeSyncOnDecrease()),
		NotifyOnIncrease: effectiveBoolConfig(ctx, client, SystemConfigKeyUpstreamModelProbeNotifyOnIncrease, DefaultUpstreamModelProbeNotifyOnIncrease()),
		NotifyOnDecrease: effectiveBoolConfig(ctx, client, SystemConfigKeyUpstreamModelProbeNotifyOnDecrease, DefaultUpstreamModelProbeNotifyOnDecrease()),
		Scope:            scope,
	}
}

func (r UpstreamModelProbeReconcileEffective) AnyAction() bool {
	return r.NotifyOnIncrease || r.NotifyOnDecrease ||
		(r.Enabled && (r.SyncOnIncrease || r.SyncOnDecrease))
}

// UpstreamModelProbeReconcileDBMeta 各 reconcile 键是否曾在 DB 中设置。
func UpstreamModelProbeReconcileDBMeta(ctx context.Context, client *ent.Client) (
	reconcileEnSet bool, reconcileEnVal string,
	syncIncSet bool, syncIncVal string,
	syncDecSet bool, syncDecVal string,
	notifyIncSet bool, notifyIncVal string,
	notifyDecSet bool, notifyDecVal string,
	scopeSet bool, scopeVal string,
	err error,
) {
	keys := []struct {
		key       string
		setPtr    *bool
		valPtr    *string
	}{
		{SystemConfigKeyUpstreamModelProbeReconcileEnabled, &reconcileEnSet, &reconcileEnVal},
		{SystemConfigKeyUpstreamModelProbeSyncOnIncrease, &syncIncSet, &syncIncVal},
		{SystemConfigKeyUpstreamModelProbeSyncOnDecrease, &syncDecSet, &syncDecVal},
		{SystemConfigKeyUpstreamModelProbeNotifyOnIncrease, &notifyIncSet, &notifyIncVal},
		{SystemConfigKeyUpstreamModelProbeNotifyOnDecrease, &notifyDecSet, &notifyDecVal},
		{SystemConfigKeyUpstreamModelProbeReconcileScope, &scopeSet, &scopeVal},
	}
	for _, item := range keys {
		row, e := client.SystemConfig.Query().Where(systemconfig.ConfigKeyEQ(item.key)).Only(ctx)
		if ent.IsNotFound(e) {
			*item.setPtr = false
			continue
		}
		if e != nil {
			err = e
			return
		}
		*item.setPtr = true
		*item.valPtr = strings.TrimSpace(row.ConfigValue)
	}
	return
}
