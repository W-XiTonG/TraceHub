package gateway

import (
	"strings"

	"tracehub/internal/gateway/router"
)

func supportsVideoChannel(ct string, extras []string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if isKlingVideoChannel(ct) {
		return true
	}
	return router.UsesOpenAICompatRelay(ct, extras)
}

func isVolcengineVideoChannel(ct string) bool {
	return strings.ToLower(strings.TrimSpace(ct)) == "volcengine"
}

func isKlingVideoChannel(ct string) bool {
	return strings.ToLower(strings.TrimSpace(ct)) == "kling"
}
