package gateway

import (
	"strings"

	"tracehub/internal/gateway/router"
)

func supportsVideoChannel(ct string, extras []string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return router.IsOpenAICompatChannelType(ct, extras) || ct == "volcengine"
}

func isVolcengineVideoChannel(ct string) bool {
	return strings.ToLower(strings.TrimSpace(ct)) == "volcengine"
}
