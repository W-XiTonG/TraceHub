package pricingsync

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

const previewTTL = 5 * time.Minute

type previewSnapshot struct {
	TargetID     int
	Direction    string
	ExpiresAt    time.Time
	SelectedKeys []string
	Payload      *Payload // pull: remote filtered; push: local to send
	Rows         []DiffRow
	Warnings     []string
}

var (
	previewMu sync.Mutex
	previews  = map[string]*previewSnapshot{}
	lastSync  = map[int]time.Time{}
)

func storePreview(s *previewSnapshot) string {
	id := uuid.NewString()
	previewMu.Lock()
	previews[id] = s
	previewMu.Unlock()
	return id
}

func takePreview(id string, targetID int, direction string) (*previewSnapshot, error) {
	previewMu.Lock()
	defer previewMu.Unlock()
	s, ok := previews[id]
	if !ok || time.Now().After(s.ExpiresAt) {
		delete(previews, id)
		return nil, errPreviewExpired
	}
	if s.TargetID != targetID || s.Direction != direction {
		return nil, errPreviewMismatch
	}
	delete(previews, id)
	return s, nil
}

func peekPreview(id string) (*previewSnapshot, bool) {
	previewMu.Lock()
	defer previewMu.Unlock()
	s, ok := previews[id]
	if !ok || time.Now().After(s.ExpiresAt) {
		if ok {
			delete(previews, id)
		}
		return nil, false
	}
	return s, true
}

func setLastManualSync(targetID int) {
	previewMu.Lock()
	lastSync[targetID] = time.Now()
	previewMu.Unlock()
}

func lastManualSyncAt(targetID int) (time.Time, bool) {
	previewMu.Lock()
	defer previewMu.Unlock()
	t, ok := lastSync[targetID]
	return t, ok
}

func checkManualCooldown(targetID int, cooldownSec int) error {
	if cooldownSec <= 0 {
		return nil
	}
	t, ok := lastManualSyncAt(targetID)
	if !ok {
		return nil
	}
	if time.Since(t) < time.Duration(cooldownSec)*time.Second {
		return errCooldown
	}
	return nil
}

const previewCleanupInterval = 2 * time.Minute

// StartPreviewCleanup 定期清理过期的预览快照，避免内存长期增长。
func StartPreviewCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(previewCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				purgeExpiredPreviews()
			}
		}
	}()
}

func purgeExpiredPreviews() {
	now := time.Now()
	previewMu.Lock()
	defer previewMu.Unlock()
	for id, s := range previews {
		if now.After(s.ExpiresAt) {
			delete(previews, id)
		}
	}
}
