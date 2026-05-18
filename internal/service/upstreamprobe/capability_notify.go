package upstreamprobe

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"tracehub/internal/ent"
)

type notifyMsg struct {
	ChannelID    int
	LogicalModel string
	Capability   string
	HTTPStatus   int
	Body         []byte
	Source       string
}

var (
	notifyCh   chan notifyMsg
	runtimeOn  = true
	persistOn  = true
	failThrottle time.Duration = 5 * time.Minute
)

const notifyBuf = 4096

// CapabilityRuntimeOpts 启动 capability 运行时（notify + flusher）。
type CapabilityRuntimeOpts struct {
	Client         *ent.Client
	RDB            *redis.Client
	AESKey         []byte
	RuntimeEnabled bool
	PersistEnabled bool
	RedisOverlay   bool
	FailThrottleSec int
	FlusherIntervalSec int
}

// StartCapabilityRuntime 初始化覆盖层、notify 循环与异步落库 flusher。
func StartCapabilityRuntime(ctx context.Context, opts CapabilityRuntimeOpts) {
	runtimeOn = opts.RuntimeEnabled
	persistOn = opts.PersistEnabled
	if opts.FailThrottleSec > 0 {
		failThrottle = time.Duration(opts.FailThrottleSec) * time.Second
	}
	SetOverlayRedis(opts.RDB, opts.RedisOverlay)
	if notifyCh == nil {
		notifyCh = make(chan notifyMsg, notifyBuf)
		go runNotifyLoop()
	}
	if opts.Client != nil && persistOn {
		interval := 30 * time.Second
		if opts.FlusherIntervalSec > 0 {
			interval = time.Duration(opts.FlusherIntervalSec) * time.Second
		}
		startCapabilityFlusher(ctx, opts.Client, interval)
	}
}

// NotifyCapabilityOutcome 非阻塞记录能力成败（热路径仅 channel send）。
func NotifyCapabilityOutcome(channelID int, logicalModel, capability string, httpStatus int, body []byte, source string) {
	if !runtimeOn || notifyCh == nil {
		return
	}
	logicalModel = strings.TrimSpace(logicalModel)
	capability = strings.TrimSpace(capability)
	if channelID <= 0 || logicalModel == "" || capability == "" {
		return
	}
	bodyCopy := append([]byte(nil), trimBodySnippet(body)...)
	msg := notifyMsg{
		ChannelID:    channelID,
		LogicalModel: logicalModel,
		Capability:   capability,
		HTTPStatus:   httpStatus,
		Body:         bodyCopy,
		Source:       source,
	}
	select {
	case notifyCh <- msg:
	default:
		slog.Debug("capability notify dropped", "channel_id", channelID, "model", logicalModel, "cap", capability)
	}
}

func runNotifyLoop() {
	for msg := range notifyCh {
		handleNotify(context.Background(), msg)
	}
}

func handleNotify(ctx context.Context, msg notifyMsg) {
	snap := buildSnapshot(msg)
	key := overlayRedisKey(msg.ChannelID, msg.LogicalModel, msg.Capability)
	setOverlayLocal(key, snap)
	setOverlayRedis(ctx, key, snap)
	if persistOn {
		enqueuePersist(msg.ChannelID, msg.LogicalModel, msg.Capability, snap)
	}
}

func buildSnapshot(msg notifyMsg) CapabilitySnapshot {
	now := time.Now().UTC()
	snap := CapabilitySnapshot{
		CheckedAt:  now,
		HTTPStatus: msg.HTTPStatus,
		Source:     msg.Source,
	}
	if msg.Source == "" {
		snap.Source = CapSourceRuntime
	}
	if msg.HTTPStatus >= 200 && msg.HTTPStatus < 300 {
		snap.Status = CapStatusOK
		return snap
	}
	if msg.HTTPStatus >= 400 {
		snap.Status = CapStatusFail
		code, summary := ParseUpstreamErrorBody(msg.Body)
		snap.ErrorCode = code
		snap.ErrorSummary = summary
		return snap
	}
	snap.Status = CapStatusUnknown
	return snap
}
