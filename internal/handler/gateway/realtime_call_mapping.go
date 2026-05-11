package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const realtimeCallMappingKeyPrefix = "tracehub:rtcall:"

// realtimeCallMapping Redis 中存储的 call_id → 选路快照（按 API Key 隔离）。
type realtimeCallMapping struct {
	ChannelID     int    `json:"channel_id"`
	UpstreamModel string `json:"upstream_model"`
	LogicalModel  string `json:"logical_model"`
}

func realtimeCallRedisKey(apiKeyID int, callID string) string {
	return fmt.Sprintf("%s%d:%s", realtimeCallMappingKeyPrefix, apiKeyID, callID)
}

func getRealtimeCallMapping(ctx context.Context, rdb *redis.Client, apiKeyID int, callID string) (*realtimeCallMapping, error) {
	if rdb == nil {
		return nil, nil
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return nil, nil
	}
	s, err := rdb.Get(ctx, realtimeCallRedisKey(apiKeyID, callID)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m realtimeCallMapping
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, nil
	}
	if m.ChannelID <= 0 || strings.TrimSpace(m.LogicalModel) == "" {
		return nil, nil
	}
	return &m, nil
}

func setRealtimeCallMapping(ctx context.Context, rdb *redis.Client, apiKeyID int, callID string, m realtimeCallMapping, ttlSec int) error {
	if rdb == nil {
		return nil
	}
	callID = strings.TrimSpace(callID)
	if callID == "" || m.ChannelID <= 0 || strings.TrimSpace(m.LogicalModel) == "" {
		return nil
	}
	if ttlSec <= 0 {
		ttlSec = 86400
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, realtimeCallRedisKey(apiKeyID, callID), b, time.Duration(ttlSec)*time.Second).Err()
}
