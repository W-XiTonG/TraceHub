package sitecfg

import (
	"context"
	"crypto/tls"
	"os"
	"strconv"
	"strings"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/config"
)

// TLSMaterialStatus 内置 TLS 所需证书/私钥是否已配置且可用。
type TLSMaterialStatus struct {
	CertPath        string
	KeyPath         string
	CertConfigured  bool
	KeyConfigured   bool
	CertReadable    bool
	KeyReadable     bool
	PairOK          bool
	PairError       string
	MaterialReady   bool // 路径已配置、文件可读且证书与私钥配对成功
}

// TLSMaterialStatus 评估当前生效路径下的证书材料（DB 覆盖 YAML）。
func EvaluateTLSMaterial(ctx context.Context, client *ent.Client, cfg *config.Config) TLSMaterialStatus {
	return evaluateTLSMaterialPaths(EffectiveTLSCertKeyPaths(ctx, client, cfg))
}

// TLSMaterialStatusWithOverrides 批量保存前用请求体中的路径覆盖评估。
func TLSMaterialStatusWithOverrides(ctx context.Context, client *ent.Client, cfg *config.Config, certOverride, keyOverride string) TLSMaterialStatus {
	paths := EffectiveTLSCertKeyPaths(ctx, client, cfg)
	if c := strings.TrimSpace(certOverride); c != "" {
		paths.CertFile = c
	}
	if k := strings.TrimSpace(keyOverride); k != "" {
		paths.KeyFile = k
	}
	return evaluateTLSMaterialPaths(paths)
}

func evaluateTLSMaterialPaths(paths TLSCertKeyPaths) TLSMaterialStatus {
	st := TLSMaterialStatus{
		CertPath: strings.TrimSpace(paths.CertFile),
		KeyPath:  strings.TrimSpace(paths.KeyFile),
	}
	st.CertConfigured = st.CertPath != ""
	st.KeyConfigured = st.KeyPath != ""
	if st.CertConfigured {
		if _, err := os.Stat(st.CertPath); err == nil {
			st.CertReadable = true
		}
	}
	if st.KeyConfigured {
		if _, err := os.Stat(st.KeyPath); err == nil {
			st.KeyReadable = true
		}
	}
	if st.CertReadable && st.KeyReadable {
		if _, err := tls.LoadX509KeyPair(st.CertPath, st.KeyPath); err != nil {
			st.PairError = err.Error()
		} else {
			st.PairOK = true
		}
	}
	st.MaterialReady = st.CertReadable && st.KeyReadable && st.PairOK
	return st
}

// ValidateBuiltInTLSEnabled 写入 server.tls.enabled=true 前校验证书材料齐备。
func ValidateBuiltInTLSEnabled(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	return validateTLSMaterial(EvaluateTLSMaterial(ctx, client, cfg))
}

// ValidateTLSMaterialStatus 证书材料未齐备时返回可展示给管理端的错误。
func ValidateTLSMaterialStatus(st TLSMaterialStatus) error {
	return validateTLSMaterial(st)
}

func validateTLSMaterial(st TLSMaterialStatus) error {
	if !st.CertConfigured || !st.KeyConfigured {
		return errTLSMaterialNotConfigured
	}
	if !st.CertReadable || !st.KeyReadable {
		return errTLSMaterialNotReadable
	}
	if !st.PairOK {
		if st.PairError != "" {
			return errTLSCertKeyPairMismatch
		}
		return errTLSCertKeyPairMismatch
	}
	return nil
}

// ValidateTLSEnabledConfigValue 管理端写入 server.tls.enabled 时的值校验。
func ValidateTLSEnabledConfigValue(ctx context.Context, client *ent.Client, cfg *config.Config, value string) error {
	b, err := parseBoolConfigValue(value)
	if err != nil {
		return errInvalidTLSEnabled
	}
	if !b {
		return nil
	}
	return ValidateBuiltInTLSEnabled(ctx, client, cfg)
}

func parseBoolConfigValue(value string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(value))
}
