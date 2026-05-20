package gateway

import (
	"strings"

	"tracehub/internal/gateway/router"
)

func supportsVideoChannel(ct string, extras []string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return router.UsesOpenAICompatRelay(ct, extras)
}

func isVolcengineVideoChannel(ct string) bool {
	return strings.ToLower(strings.TrimSpace(ct)) == "volcengine"
}
