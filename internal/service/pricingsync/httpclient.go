package pricingsync

import (
	"net"
	"net/url"
	"strings"
)

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if path == "" {
		return base
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

// ValidateOutboundURL 校验出站 URL，拒绝 SSRF 目标。
func ValidateOutboundURL(raw string, allowInsecureHTTP bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return errInvalidURL
	}
	_ = allowInsecureHTTP // 保留参数兼容；HTTP/HTTPS 均可
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return errInvalidURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return errSSRFBlocked
		}
		return nil
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return errSSRFBlocked
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return errSSRFBlocked
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	if ip.Equal(net.IPv4(169, 254, 169, 254)) {
		return true
	}
	return false
}
