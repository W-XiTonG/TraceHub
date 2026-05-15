package channelhealth

import (
	"context"
	"strings"

	"tracehub/internal/ent"
)

// RecordUpstreamFailure 上游失败时递增 error_count 并记录错误信息。
func RecordUpstreamFailure(ctx context.Context, client *ent.Client, channelID int, errMsg string) error {
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil {
		return err
	}
	msg := strings.TrimSpace(errMsg)
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	up := client.Channel.UpdateOne(ch).AddErrorCount(1)
	if msg != "" {
		up.SetLastErrorMsg(msg)
	}
	return up.Exec(ctx)
}

// RecordUpstreamSuccess 上游成功时递减 error_count（不低于 0）。
func RecordUpstreamSuccess(ctx context.Context, client *ent.Client, channelID int) error {
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.ErrorCount <= 0 {
		return nil
	}
	return client.Channel.UpdateOne(ch).AddErrorCount(-1).Exec(ctx)
}

// ResetErrorCount 管理端手动重置错误计数。
func ResetErrorCount(ctx context.Context, client *ent.Client, channelID int) error {
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil {
		return err
	}
	return client.Channel.UpdateOne(ch).SetErrorCount(0).ClearLastErrorMsg().Exec(ctx)
}

// ApplyProbeHealth 根据探测结果更新 health_status；manualDisabled 为 true（status=3）时不覆盖。
func ApplyProbeHealth(ctx context.Context, client *ent.Client, channelID int, probeOK bool, failThreshold int) error {
	if failThreshold < 1 {
		failThreshold = 3
	}
	ch, err := client.Channel.Get(ctx, channelID)
	if err != nil {
		return err
	}
	if ch.HealthStatus == 3 {
		return nil
	}
	if probeOK {
		if ch.HealthStatus != 1 {
			return client.Channel.UpdateOne(ch).SetHealthStatus(1).ClearLastErrorMsg().Exec(ctx)
		}
		return nil
	}
	next := ch.ErrorCount + 1
	if next < failThreshold {
		return client.Channel.UpdateOne(ch).SetErrorCount(next).Exec(ctx)
	}
	return client.Channel.UpdateOne(ch).SetHealthStatus(2).SetErrorCount(next).Exec(ctx)
}
