package upstreamprobe

import (
	"testing"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

func TestComputeReconcileDiff_modelsOnly(t *testing.T) {
	ch := &ent.Channel{
		Models: []string{"a", "b", "gone"},
		ModelAlias: map[string]string{
			"my-a": "a",
		},
	}
	upstream := map[string]struct{}{"a": {}, "b": {}, "new-up": {}}
	oldProbe := map[string]interface{}{
		ProbeUpstreamSnapshotKey: map[string]interface{}{
			"ids": []interface{}{"a", "b"},
		},
	}
	diff := computeReconcileDiff(ch, upstream, oldProbe, []string{"a", "b", "new-up"}, sitecfg.UpstreamModelProbeReconcileScopeModelsOnly)

	if len(diff.AddedUpstream) != 1 || diff.AddedUpstream[0] != "new-up" {
		t.Fatalf("added upstream: %#v", diff.AddedUpstream)
	}
	if len(diff.ToAddModels) != 1 || diff.ToAddModels[0] != "new-up" {
		t.Fatalf("to add: %#v", diff.ToAddModels)
	}
	if len(diff.ToRemoveLocal) != 1 || diff.ToRemoveLocal[0] != "gone" {
		t.Fatalf("to remove: %#v", diff.ToRemoveLocal)
	}
}

func TestComputeReconcileDiff_firstSnapshotNoUpstreamNotify(t *testing.T) {
	ch := &ent.Channel{Models: []string{"a"}}
	upstream := map[string]struct{}{"a": {}, "b": {}}
	diff := computeReconcileDiff(ch, upstream, nil, []string{"a", "b"}, sitecfg.UpstreamModelProbeReconcileScopeModelsOnly)
	if len(diff.AddedUpstream) != 0 {
		t.Fatalf("first snapshot should not treat all as added: %#v", diff.AddedUpstream)
	}
	if len(diff.ToAddModels) != 1 || diff.ToAddModels[0] != "b" {
		t.Fatalf("to add from local diff: %#v", diff.ToAddModels)
	}
}

func TestAppendModelsDedup(t *testing.T) {
	out := appendModelsDedup([]string{"a"}, []string{"a", "b"})
	if len(out) != 2 || out[1] != "b" {
		t.Fatalf("got %#v", out)
	}
}

func TestShouldNotifyReconcile_dedup(t *testing.T) {
	ids := []string{"m1", "m2"}
	old := map[string]interface{}{
		ProbeReconcileMetaKey: map[string]interface{}{
			"last_notify_add_fp": idsFingerprint(ids),
		},
	}
	if shouldNotifyReconcile(old, ids, "last_notify_add_fp") {
		t.Fatal("expected dedup")
	}
	if !shouldNotifyReconcile(old, []string{"m3"}, "last_notify_add_fp") {
		t.Fatal("expected notify on change")
	}
}

func TestIdsFingerprint_stable(t *testing.T) {
	a := idsFingerprint([]string{"b", "a"})
	b := idsFingerprint([]string{"a", "b"})
	if a != b {
		t.Fatalf("%s vs %s", a, b)
	}
}
