package pricingsync

import (
	"sync"
)

type targetLock struct {
	mu    sync.Mutex
	busy  bool
}

var targetLockMap sync.Map

func withTargetLock(targetID int, fn func() error) error {
	v, _ := targetLockMap.LoadOrStore(targetID, &targetLock{})
	lk := v.(*targetLock)
	lk.mu.Lock()
	defer lk.mu.Unlock()
	if lk.busy {
		return errSyncInProgress
	}
	lk.busy = true
	defer func() { lk.busy = false }()
	return fn()
}
