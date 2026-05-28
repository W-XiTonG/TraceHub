//go:build !cgo

package pricingsyncmigrate

import "testing"

func TestCorrectPathsConstants(t *testing.T) {
	if correctPullPath != "/api/pricing/sync/export" {
		t.Fatalf("pull=%q", correctPullPath)
	}
	if correctPushPath != "/api/pricing/sync/import" {
		t.Fatalf("push=%q", correctPushPath)
	}
}
