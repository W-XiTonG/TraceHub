package upstreamprobe

import (
	"context"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/pkg/sitecfg"

	"go.uber.org/zap"
)

// StartWorker 在后台按 system_configs 周期探测渠道上游模型列表；单协程串行按渠道请求。
func StartWorker(ctx context.Context, log *zap.Logger, client *ent.Client, aesKey []byte) {
	if log == nil {
		log = zap.NewNop()
	}
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			enabled := sitecfg.EffectiveUpstreamModelProbeEnabled(ctx, client)
			intervalMin := sitecfg.EffectiveUpstreamModelProbeIntervalMinutes(ctx, client)
			if !enabled {
				select {
				case <-time.After(5 * time.Minute):
				case <-ctx.Done():
					return
				}
				continue
			}
			channels, err := client.Channel.Query().
				Where(channel.Enabled(true), channel.Schedulable(true)).
				All(ctx)
			if err != nil {
				log.Warn("upstream_probe list channels", zap.Error(err))
			} else {
				for _, ch := range channels {
					if ctx.Err() != nil {
						return
					}
					if err := ProbeChannel(ctx, client, aesKey, ch); err != nil {
						log.Warn("upstream_probe channel failed",
							zap.Int("channel_id", ch.ID),
							zap.Error(err))
					}
					select {
					case <-time.After(400 * time.Millisecond):
					case <-ctx.Done():
						return
					}
				}
				if sitecfg.EffectiveUpstreamCapabilityProbeEnabled(ctx, client) {
					for _, ch := range channels {
						if ctx.Err() != nil {
							return
						}
						if err := ProbeChannelCapabilities(ctx, client, aesKey, ch, CapabilityProbeMaxPerRound); err != nil {
							log.Warn("upstream_capability_probe channel failed",
								zap.Int("channel_id", ch.ID),
								zap.Error(err))
						}
						select {
						case <-time.After(400 * time.Millisecond):
						case <-ctx.Done():
							return
						}
					}
				}
			}
			wait := time.Duration(intervalMin) * time.Minute
			if wait < 15*time.Minute {
				wait = 15 * time.Minute
			}
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
	}()
}
