package availmodels

import (
	"context"
	"testing"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/service/upstreamprobe"
)

func TestAggregateServiceStatus_operationalAndFail(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	chOK := &ent.Channel{
		ID:     1,
		Models: []string{"m1"},
		ModelUpstreamProbe: map[string]interface{}{
			"m1": map[string]interface{}{
				"capabilities": map[string]interface{}{
					"video": map[string]interface{}{
						"status":     upstreamprobe.CapStatusOK,
						"checked_at": now,
					},
				},
			},
		},
	}
	chFail := &ent.Channel{
		ID:     2,
		Models: []string{"m1"},
		ModelUpstreamProbe: map[string]interface{}{
			"m1": map[string]interface{}{
				"capabilities": map[string]interface{}{
					"video": map[string]interface{}{
						"status":     upstreamprobe.CapStatusFail,
						"checked_at": now,
					},
				},
			},
		},
	}
	st := AggregateServiceStatusForModel(context.Background(), []*ent.Channel{chOK, chFail}, "m1", []string{"video"}, 6)
	if st != ServiceStatusDegraded {
		t.Fatalf("got %s want degraded", st)
	}
}

func TestEffectiveCapStatus_stale(t *testing.T) {
	snap := upstreamprobe.CapabilitySnapshot{
		Status:    upstreamprobe.CapStatusFail,
		CheckedAt: time.Now().Add(-7 * time.Hour),
	}
	if upstreamprobe.EffectiveCapStatus(snap, 6, time.Now()) != "stale" {
		t.Fatal("expected stale")
	}
}
