package sitecfg

import (
	"context"
	"strconv"
	"strings"

	"tracehub/internal/ent"
)

const (
	SystemConfigKeyUpstreamTimeoutStandardSec = "gateway.upstream_timeout.standard_sec"
	SystemConfigKeyUpstreamTimeoutStreamSec   = "gateway.upstream_timeout.stream_sec"
	SystemConfigKeyUpstreamTimeoutImageSec    = "gateway.upstream_timeout.image_sec"
	SystemConfigKeyUpstreamTimeoutAudioSec    = "gateway.upstream_timeout.audio_sec"
	SystemConfigKeyUpstreamTimeoutVideoSec    = "gateway.upstream_timeout.video_sec"
)

// UpstreamTimeoutDefaults 网关调用上游 HTTP 客户端超时（秒）。
type UpstreamTimeoutDefaults struct {
	StandardSec int // 对话、Embeddings 等常规请求
	StreamSec   int // SSE 流式
	ImageSec    int // 文生图
	AudioSec    int // TTS、转写等
	VideoSec    int // 视频创建/轮询/下载
}

func DefaultUpstreamTimeouts() UpstreamTimeoutDefaults {
	return UpstreamTimeoutDefaults{
		StandardSec: 120,
		StreamSec:   120,
		ImageSec:    600,
		AudioSec:    600,
		VideoSec:    1800, // 30 分钟
	}
}

func ValidateUpstreamTimeoutStandardSec(value string) error {
	return validateUpstreamTimeoutSec(value, 10, 600, errUpstreamTimeoutStandard, errUpstreamTimeoutStandardRange)
}

func ValidateUpstreamTimeoutStreamSec(value string) error {
	return validateUpstreamTimeoutSec(value, 10, 600, errUpstreamTimeoutStream, errUpstreamTimeoutStreamRange)
}

func ValidateUpstreamTimeoutImageSec(value string) error {
	return validateUpstreamTimeoutSec(value, 30, 3600, errUpstreamTimeoutImage, errUpstreamTimeoutImageRange)
}

func ValidateUpstreamTimeoutAudioSec(value string) error {
	return validateUpstreamTimeoutSec(value, 30, 3600, errUpstreamTimeoutAudio, errUpstreamTimeoutAudioRange)
}

func ValidateUpstreamTimeoutVideoSec(value string) error {
	return validateUpstreamTimeoutSec(value, 60, 7200, errUpstreamTimeoutVideo, errUpstreamTimeoutVideoRange)
}

func validateUpstreamTimeoutSec(value string, min, max int, errInt, errRange error) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return errInt
	}
	if n < min || n > max {
		return errRange
	}
	return nil
}

func EffectiveUpstreamTimeouts(ctx context.Context, client *ent.Client) UpstreamTimeoutDefaults {
	d := DefaultUpstreamTimeouts()
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamTimeoutStandardSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 10 && n <= 600 {
			d.StandardSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamTimeoutStreamSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 10 && n <= 600 {
			d.StreamSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamTimeoutImageSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 30 && n <= 3600 {
			d.ImageSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamTimeoutAudioSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 30 && n <= 3600 {
			d.AudioSec = n
		}
	}
	if v, ok, err := stringFromConfigRow(ctx, client, SystemConfigKeyUpstreamTimeoutVideoSec); err == nil && ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && n >= 60 && n <= 7200 {
			d.VideoSec = n
		}
	}
	return d
}

// IsUpstreamTimeoutConfigKey 是否为上游 HTTP 超时配置键。
func IsUpstreamTimeoutConfigKey(k string) bool {
	switch strings.TrimSpace(k) {
	case SystemConfigKeyUpstreamTimeoutStandardSec,
		SystemConfigKeyUpstreamTimeoutStreamSec,
		SystemConfigKeyUpstreamTimeoutImageSec,
		SystemConfigKeyUpstreamTimeoutAudioSec,
		SystemConfigKeyUpstreamTimeoutVideoSec:
		return true
	default:
		return false
	}
}

// UpstreamTimeoutConfigKeys 管理端「恢复默认」时清除的键。
func UpstreamTimeoutConfigKeys() []string {
	return []string{
		SystemConfigKeyUpstreamTimeoutStandardSec,
		SystemConfigKeyUpstreamTimeoutStreamSec,
		SystemConfigKeyUpstreamTimeoutImageSec,
		SystemConfigKeyUpstreamTimeoutAudioSec,
		SystemConfigKeyUpstreamTimeoutVideoSec,
	}
}
