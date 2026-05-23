package router

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const stickySessionTTL = time.Hour

// ClearStickySession 上游可恢复失败并即将 failover 时清除粘性绑定。
func ClearStickySession(ctx context.Context, rdb *redis.Client, userID int64, stickySession string) {
	if rdb == nil || stickySession == "" {
		return
	}
	_ = rdb.Del(ctx, stickyKey(userID, stickySession)).Err()
}

// UpdateStickySession 成功选路后写入粘性会话（TTL 1h）。
func UpdateStickySession(ctx context.Context, rdb *redis.Client, userID int64, stickySession string, channelID int) {
	if rdb == nil || stickySession == "" || channelID <= 0 {
		return
	}
	_ = rdb.Set(ctx, stickyKey(userID, stickySession), strconv.Itoa(channelID), stickySessionTTL).Err()
}
