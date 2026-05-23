package pricingsync

import (
	"context"
	"time"

	"tracehub/internal/ent/modelpricingsynctarget"
	"tracehub/internal/pkg/sitecfg"

	"go.uber.org/zap"
)

// StartWorker 后台定时同步 pricing targets（按各目标 schedule_interval_min 拉取）。
func StartWorker(ctx context.Context, log *zap.Logger, svc ServiceOpts) {
	if log == nil {
		log = zap.NewNop()
	}
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			if !sitecfg.EffectivePricingSyncWorkerEnabled(ctx, svc.Client) || !sitecfg.EffectivePricingSyncEnabled(ctx, svc.Client) {
				select {
				case <-time.After(5 * time.Minute):
				case <-ctx.Done():
					return
				}
				continue
			}
			now := time.Now()
			targets, err := svc.Client.ModelPricingSyncTarget.Query().
				Where(
					modelpricingsynctarget.Enabled(true),
					modelpricingsynctarget.ScheduleEnabled(true),
				).
				All(ctx)
			if err != nil {
				log.Warn("pricing_sync list targets", zap.Error(err))
			} else {
				for _, t := range targets {
					if ctx.Err() != nil {
						return
					}
					if !TargetDueForScheduledPull(t, now) {
						continue
					}
					if err := RunScheduledTarget(ctx, svc, t.ID); err != nil {
						log.Warn("pricing_sync target failed", zap.Int("target_id", t.ID), zap.Error(err))
					}
					select {
					case <-time.After(400 * time.Millisecond):
					case <-ctx.Done():
						return
					}
				}
			}
			select {
			case <-time.After(workerPollInterval):
			case <-ctx.Done():
				return
			}
		}
	}()
}
