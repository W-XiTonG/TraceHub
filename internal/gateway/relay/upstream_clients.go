package relay

import (
	"context"
	"net/http"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

type upstreamTimeoutStore struct {
	standard time.Duration
	stream   time.Duration
	image    time.Duration
	audio    time.Duration
	video    time.Duration
}

var (
	upstreamTOMu   sync.RWMutex
	upstreamTO     = DefaultUpstreamTimeoutStore()
	clientCache    sync.Map // string -> *http.Client
	streamTransport *http.Transport
)

func init() {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableCompression = true
	streamTransport = t
}

func DefaultUpstreamTimeoutStore() upstreamTimeoutStore {
	d := sitecfg.DefaultUpstreamTimeouts()
	return upstreamTimeoutStore{
		standard: time.Duration(d.StandardSec) * time.Second,
		stream:   time.Duration(d.StreamSec) * time.Second,
		image:    time.Duration(d.ImageSec) * time.Second,
		audio:    time.Duration(d.AudioSec) * time.Second,
		video:    time.Duration(d.VideoSec) * time.Second,
	}
}

// ApplyUpstreamTimeouts 从站点配置刷新上游 HTTP 客户端超时（管理端保存后调用）。
func ApplyUpstreamTimeouts(d sitecfg.UpstreamTimeoutDefaults) {
	upstreamTOMu.Lock()
	upstreamTO = upstreamTimeoutStore{
		standard: time.Duration(d.StandardSec) * time.Second,
		stream:   time.Duration(d.StreamSec) * time.Second,
		image:    time.Duration(d.ImageSec) * time.Second,
		audio:    time.Duration(d.AudioSec) * time.Second,
		video:    time.Duration(d.VideoSec) * time.Second,
	}
	clientCache = sync.Map{}
	upstreamTOMu.Unlock()
}

// ApplyUpstreamTimeoutsFromDB 读取 system_configs 并刷新上游超时。
func ApplyUpstreamTimeoutsFromDB(ctx context.Context, client *ent.Client) {
	ApplyUpstreamTimeouts(sitecfg.EffectiveUpstreamTimeouts(ctx, client))
}

func currentUpstreamTO() upstreamTimeoutStore {
	upstreamTOMu.RLock()
	defer upstreamTOMu.RUnlock()
	return upstreamTO
}

func clientFor(cacheKey string, timeout time.Duration, stream bool) *http.Client {
	if v, ok := clientCache.Load(cacheKey); ok {
		return v.(*http.Client)
	}
	var c *http.Client
	if stream {
		c = &http.Client{Timeout: timeout, Transport: streamTransport}
	} else {
		c = &http.Client{Timeout: timeout}
	}
	actual, _ := clientCache.LoadOrStore(cacheKey, c)
	return actual.(*http.Client)
}

func httpUpstream() *http.Client {
	to := currentUpstreamTO()
	return clientFor("standard:"+to.standard.String(), to.standard, false)
}

func httpStreamUpstream() *http.Client {
	to := currentUpstreamTO()
	return clientFor("stream:"+to.stream.String(), to.stream, true)
}

func httpImageUpstream() *http.Client {
	to := currentUpstreamTO()
	return clientFor("image:"+to.image.String(), to.image, false)
}

func httpAudioUpstream() *http.Client {
	to := currentUpstreamTO()
	return clientFor("audio:"+to.audio.String(), to.audio, false)
}

func httpVideoUpstream() *http.Client {
	to := currentUpstreamTO()
	return clientFor("video:"+to.video.String(), to.video, false)
}
