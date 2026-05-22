package sitecfg

import (
	"path/filepath"
	"strings"
)

const maxTLSPathLen = 500

// ValidateTLSCertFilePath 管理端写入 server.tls.cert_file 前校验。
func ValidateTLSCertFilePath(value string) error {
	return validateTLSFilePath(value, "server.tls.cert_file", errTLSCertFileEmpty, errTLSCertFileTooLong, errTLSCertFileInvalid)
}

// ValidateTLSKeyFilePath 管理端写入 server.tls.key_file 前校验。
func ValidateTLSKeyFilePath(value string) error {
	return validateTLSFilePath(value, "server.tls.key_file", errTLSKeyFileEmpty, errTLSKeyFileTooLong, errTLSKeyFileInvalid)
}

func validateTLSFilePath(value string, _ string, errEmpty, errLong, errInvalid error) error {
	s := strings.TrimSpace(value)
	if s == "" {
		return errEmpty
	}
	if len(s) > maxTLSPathLen {
		return errLong
	}
	if strings.Contains(s, "..") {
		return errInvalid
	}
	if !filepath.IsAbs(s) {
		return errInvalid
	}
	return nil
}
