package payment

import "strings"

// sanitizePostgresText 移除 PostgreSQL text 不允许的 NUL(0x00) 等控制字符。
func sanitizePostgresText(s string, maxLen int) string {
	if s == "" {
		return s
	}
	s = strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}
