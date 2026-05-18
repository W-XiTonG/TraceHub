package upstreamprobe

// 上游能力探测路径模板（P2：探测与运维解析用；网关热路径仅 Notify，不走此表）。
const (
	PathTemplateOpenAIVideos         = "openai_videos"
	PathTemplateXAIVideosGenerations = "xai_videos_generations"
)

// VideoProbePathForTemplate 返回探测用相对路径。
func VideoProbePathForTemplate(tmpl string) string {
	switch tmpl {
	case PathTemplateXAIVideosGenerations:
		return "/v1/videos/generations"
	default:
		return "/videos"
	}
}
