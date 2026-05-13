package safehttp

import (
	"fmt"
	"net"
	"net/url"
)

// EgressURLHostSafe 校验 URL 的主机名解析结果，拦截回环、私网、链路本地等常见 SSRF 目标。
func EgressURLHostSafe(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https URL")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("缺少主机名")
	}
	if ip := net.ParseIP(host); ip != nil {
		if disallowedIP(ip) {
			return fmt.Errorf("禁止访问的地址: %s", ip)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS 解析失败: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("主机无解析结果")
	}
	for _, ip := range ips {
		if disallowedIP(ip) {
			return fmt.Errorf("主机解析到禁止访问的地址: %s", ip)
		}
	}
	return nil
}

func disallowedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 0 {
			return true
		}
		// 100.64.0.0/10 (RFC 6598 CGNAT)
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}
