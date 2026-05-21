package gateway

import (
	"testing"

	"tracehub/internal/pkg/klingvideo"
)

func TestKlingNative_upstreamPath(t *testing.T) {
	got := klingvideo.UpstreamRelativePath("videos/effects")
	if got != "v1/videos/effects" {
		t.Fatalf("%q", got)
	}
}
