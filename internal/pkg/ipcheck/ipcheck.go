package ipcheck

import (
	"net"
	"strings"
)

// Allowed 在白名单为空时放行；否则要求客户端 IP 命中任一条目（支持 CIDR）。
func Allowed(clientIP string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(clientIP))
	if ip == nil {
		return false
	}
	for _, entry := range whitelist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err == nil && cidr.Contains(ip) {
				return true
			}
			continue
		}
		if pip := net.ParseIP(entry); pip != nil && pip.Equal(ip) {
			return true
		}
	}
	return false
}
