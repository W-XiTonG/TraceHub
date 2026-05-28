package pricingsync

import (
	"testing"
	"time"
)

func TestPurgeExpiredPreviews(t *testing.T) {
	previewMu.Lock()
	previews = map[string]*previewSnapshot{
		"expired": {ExpiresAt: time.Now().Add(-time.Minute)},
		"active":  {ExpiresAt: time.Now().Add(5 * time.Minute)},
	}
	previewMu.Unlock()

	purgeExpiredPreviews()

	previewMu.Lock()
	defer previewMu.Unlock()
	if _, ok := previews["expired"]; ok {
		t.Fatal("expected expired preview removed")
	}
	if _, ok := previews["active"]; !ok {
		t.Fatal("expected active preview kept")
	}
	delete(previews, "active")
}
