package channelsecret

import (
	"tracehub/internal/pkg/cryptokey"
)

// Plaintext 尝试 AES-GCM 解密；失败则视为明文兼容旧数据。
func Plaintext(aesKey []byte, stored string) string {
	if stored == "" {
		return ""
	}
	b, err := cryptokey.DecryptGCM(aesKey, stored)
	if err != nil {
		return stored
	}
	return string(b)
}
