package relay

import "context"

// LongRunningUpstreamContext 供文生图、视频、长音频、SSE 等耗时上游请求使用：从 failover
// attempt 的短超时 context（默认 30s）中解耦，避免 attemptCtx 被 cancel 后 client.Do 或 body 读取提前失败。
func LongRunningUpstreamContext(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}

// StreamUpstreamContext 同 LongRunningUpstreamContext，专用于 SSE 流式透传。
func StreamUpstreamContext(ctx context.Context) context.Context {
	return LongRunningUpstreamContext(ctx)
}
