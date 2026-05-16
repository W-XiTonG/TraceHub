package gateway

import "net/http"

// 便于排查「上游报模型不存在」等：与计费无关的选路元数据（勿写入敏感信息）。
const (
	HeaderTraceHubRoutingModel  = "X-TraceHub-Routing-Model"
	HeaderTraceHubUpstreamModel = "X-TraceHub-Upstream-Model"
	HeaderTraceHubChannelID     = "X-TraceHub-Channel-Id"
)

// setTraceHubChatDebugHeaders 不向客户端暴露选路/渠道/上游 model。
func setTraceHubChatDebugHeaders(_ http.Header, _ int, _, _ string) {}
