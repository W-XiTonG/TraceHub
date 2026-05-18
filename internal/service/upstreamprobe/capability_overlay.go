package upstreamprobe

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	overlayMu sync.RWMutex
	overlay   = make(map[string]CapabilitySnapshot)

	overlayRDB       *redis.Client
	overlayRedisOn   bool
	overlayRedisTTL  = 24 * time.Hour
)

// SetOverlayRedis 配置可选 Redis 覆盖层（多实例一致）。
func SetOverlayRedis(rdb *redis.Client, enabled bool) {
	overlayMu.Lock()
	defer overlayMu.Unlock()
	overlayRDB = rdb
	overlayRedisOn = enabled && rdb != nil
}

func setOverlayLocal(key string, snap CapabilitySnapshot) {
	overlayMu.Lock()
	overlay[key] = snap
	overlayMu.Unlock()
}

func getOverlayLocal(key string) (CapabilitySnapshot, bool) {
	overlayMu.Lock()
	defer overlayMu.Unlock()
	s, ok := overlay[key]
	return s, ok
}

func setOverlayRedis(ctx context.Context, key string, snap CapabilitySnapshot) {
	if !overlayRedisOn || overlayRDB == nil {
		return
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	_ = overlayRDB.Set(ctx, key, b, overlayRedisTTL).Err()
}

func getOverlayRedis(ctx context.Context, key string) (CapabilitySnapshot, bool) {
	if !overlayRedisOn || overlayRDB == nil {
		return CapabilitySnapshot{}, false
	}
	b, err := overlayRDB.Get(ctx, key).Bytes()
	if err != nil || len(b) == 0 {
		return CapabilitySnapshot{}, false
	}
	var snap CapabilitySnapshot
	if json.Unmarshal(b, &snap) != nil {
		return CapabilitySnapshot{}, false
	}
	return snap, snap.Status != ""
}

// GetCapabilityOverlay 读取覆盖层（本地优先，Redis 补充）。
func GetCapabilityOverlay(ctx context.Context, channelID int, logicalModel, capability string) (CapabilitySnapshot, bool) {
	key := overlayRedisKey(channelID, logicalModel, capability)
	if s, ok := getOverlayLocal(key); ok {
		return s, true
	}
	return getOverlayRedis(ctx, key)
}

// MergeCapabilityIntoModelProbe 将覆盖层合并进 model 行 map（用于读 DB probe）。
func MergeCapabilityIntoModelProbe(ctx context.Context, channelID int, logicalModel string, row map[string]interface{}) {
	if row == nil {
		return
	}
	caps, _ := row["capabilities"].(map[string]interface{})
	if caps == nil {
		caps = make(map[string]interface{})
		row["capabilities"] = caps
	}
	for _, cap := range []string{CapabilityVideo, CapabilityImage, CapabilityAudio} {
		ov, ok := GetCapabilityOverlay(ctx, channelID, logicalModel, cap)
		if !ok {
			continue
		}
		existing, _ := capRowFromMap(asMap(caps[cap]))
		if !existing.CheckedAt.IsZero() && !ov.CheckedAt.IsZero() && existing.CheckedAt.After(ov.CheckedAt) {
			continue
		}
		caps[cap] = capRowToMap(ov)
	}
}

func asMap(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}
