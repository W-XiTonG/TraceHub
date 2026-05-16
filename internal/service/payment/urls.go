package payment

import "strings"

func successURL(cfg RuntimeConfig, orderNo string) string {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.SiteBaseURL), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return base + "/console/billing?paid=1&order_no=" + orderNo
}

func cancelURL(cfg RuntimeConfig, orderNo string) string {
	base := strings.TrimSuffix(strings.TrimSpace(cfg.SiteBaseURL), "/")
	if base == "" {
		base = "http://localhost:5173"
	}
	return base + "/console/billing?cancelled=1&order_no=" + orderNo
}
