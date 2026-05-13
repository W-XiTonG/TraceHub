package gateway

import (
	"net/http"
	"strconv"
)

// 便于排查「上游报模型不存在」等：与计费无关的选路元数据（勿写入敏感信息）。
const (
	HeaderTraceHubRoutingModel  = "X-TraceHub-Routing-Model"
	HeaderTraceHubUpstreamModel = "X-TraceHub-Upstream-Model"
	HeaderTraceHubChannelID     = "X-TraceHub-Channel-Id"
)

func setTraceHubChatDebugHeaders(h http.Header, channelID int, routingModel, upstreamModel string) {
	h.Set(HeaderTraceHubRoutingModel, routingModel)
	h.Set(HeaderTraceHubUpstreamModel, upstreamModel)
	h.Set(HeaderTraceHubChannelID, strconv.Itoa(channelID))
}
