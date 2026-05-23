package pricingsync

import (
	"testing"
	"time"

	"tracehub/internal/ent"
)

func TestNormalizeScheduleIntervalMin(t *testing.T) {
	if got := NormalizeScheduleIntervalMin(30); got != 60 {
		t.Fatalf("got %d", got)
	}
	if got := NormalizeScheduleIntervalMin(120); got != 120 {
		t.Fatalf("got %d", got)
	}
}

func TestTargetDueForScheduledPull(t *testing.T) {
	now := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	tgt := &ent.ModelPricingSyncTarget{
		Enabled:             true,
		ScheduleEnabled:     true,
		ScheduleIntervalMin: 60,
		LastPullAt:          now.Add(-61 * time.Minute),
	}
	if !TargetDueForScheduledPull(tgt, now) {
		t.Fatal("expected due")
	}
	tgt.LastPullAt = now.Add(-30 * time.Minute)
	if TargetDueForScheduledPull(tgt, now) {
		t.Fatal("expected not due")
	}
}
