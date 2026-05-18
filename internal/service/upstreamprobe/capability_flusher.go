package upstreamprobe

import (
	"context"
	"sync"
	"time"

	"tracehub/internal/ent"
)

type persistKey struct {
	channelID    int
	logicalModel string
	capability   string
}

type persistEntry struct {
	snap CapabilitySnapshot
}

var (
	persistMu      sync.Mutex
	persistPending = make(map[persistKey]persistEntry)
	lastPersisted  = make(map[persistKey]CapabilitySnapshot)
	lastFailWrite  = make(map[persistKey]time.Time)

	flusherClient *ent.Client
)

func enqueuePersist(channelID int, logicalModel, capability string, snap CapabilitySnapshot) {
	if flusherClient == nil {
		return
	}
	k := persistKey{channelID: channelID, logicalModel: logicalModel, capability: capability}
	persistMu.Lock()
	defer persistMu.Unlock()
	last, hasLast := lastPersisted[k]
	if snap.Status == CapStatusOK {
		if hasLast && last.Status == CapStatusOK {
			return
		}
		persistPending[k] = persistEntry{snap: snap}
		return
	}
	if snap.Status == CapStatusFail && hasLast && last.Status == CapStatusFail {
		if last.ErrorCode == snap.ErrorCode && last.ErrorSummary == snap.ErrorSummary {
			if t, ok := lastFailWrite[k]; ok && time.Since(t) < failThrottle {
				return
			}
		}
	}
	persistPending[k] = persistEntry{snap: snap}
}

func startCapabilityFlusher(ctx context.Context, client *ent.Client, interval time.Duration) {
	flusherClient = client
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				flushPersistBatch(context.Background())
				return
			case <-ticker.C:
				flushPersistBatch(ctx)
			}
		}
	}()
}

func flushPersistBatch(ctx context.Context) {
	if flusherClient == nil {
		return
	}
	persistMu.Lock()
	if len(persistPending) == 0 {
		persistMu.Unlock()
		return
	}
	batch := make(map[persistKey]persistEntry, len(persistPending))
	for k, v := range persistPending {
		batch[k] = v
	}
	persistPending = make(map[persistKey]persistEntry)
	persistMu.Unlock()

	byChannel := make(map[int][]persistKey)
	for k := range batch {
		byChannel[k.channelID] = append(byChannel[k.channelID], k)
	}
	for chID, keys := range byChannel {
		ch, err := flusherClient.Channel.Get(ctx, chID)
		if err != nil {
			continue
		}
		probe := ch.ModelUpstreamProbe
		if probe == nil {
			probe = make(map[string]interface{})
		}
		changed := false
		for _, k := range keys {
			ent := batch[k]
			row, ok := probe[k.logicalModel].(map[string]interface{})
			if !ok {
				row = map[string]interface{}{}
			}
			caps, _ := row["capabilities"].(map[string]interface{})
			if caps == nil {
				caps = make(map[string]interface{})
				row["capabilities"] = caps
			}
			caps[k.capability] = capRowToMap(ent.snap)
			if _, has := row["checked_at"]; !has {
				row["checked_at"] = ent.snap.CheckedAt.UTC().Format(time.RFC3339)
			}
			probe[k.logicalModel] = row
			changed = true
			persistMu.Lock()
			lastPersisted[k] = ent.snap
			if ent.snap.Status == CapStatusFail {
				lastFailWrite[k] = time.Now()
			}
			persistMu.Unlock()
		}
		if !changed {
			continue
		}
		if err := flusherClient.Channel.UpdateOneID(chID).SetModelUpstreamProbe(probe).Exec(ctx); err == nil {
			MaybeApplyCapabilityProbeHealth(ctx, flusherClient, chID)
		}
	}
}
