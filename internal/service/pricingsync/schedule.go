package pricingsync

import (
	"time"

	"tracehub/internal/ent"
)

const (
	minScheduleIntervalMin = 60
	maxScheduleIntervalMin = 10080 // 7 天
	workerPollInterval     = time.Minute
)

// NormalizeScheduleIntervalMin 将定时间隔规范到 60–10080 分钟。
func NormalizeScheduleIntervalMin(minutes int) int {
	if minutes < minScheduleIntervalMin {
		return minScheduleIntervalMin
	}
	if minutes > maxScheduleIntervalMin {
		return maxScheduleIntervalMin
	}
	return minutes
}

// TargetDueForScheduledPull 是否已到该目标的定时拉取时间。
func TargetDueForScheduledPull(t *ent.ModelPricingSyncTarget, now time.Time) bool {
	if t == nil || !t.Enabled || !t.ScheduleEnabled {
		return false
	}
	interval := time.Duration(NormalizeScheduleIntervalMin(t.ScheduleIntervalMin)) * time.Minute
	if t.LastPullAt.IsZero() {
		return true
	}
	return !t.LastPullAt.Add(interval).After(now)
}
