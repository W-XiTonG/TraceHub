package gatewayalert

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	redisDedupPrefix = "gateway_alert:dedup:"
	redisGlobalKey   = "gateway_alert:global_external"
)

type rateLimiter struct {
	rdb *redis.Client
	mu  sync.Mutex
	mem map[string]time.Time
}

func newRateLimiter(rdb *redis.Client) *rateLimiter {
	return &rateLimiter{rdb: rdb, mem: map[string]time.Time{}}
}

// tryDedup 指纹冷却：通过返回 true。
func (r *rateLimiter) tryDedup(ctx context.Context, fp string, cooldownSec int) bool {
	if cooldownSec <= 0 {
		return true
	}
	key := redisDedupPrefix + fp
	if r.rdb != nil {
		ok, err := r.rdb.SetNX(ctx, key, "1", time.Duration(cooldownSec)*time.Second).Result()
		return err == nil && ok
	}
	return r.tryMem(key, time.Duration(cooldownSec)*time.Second)
}

// tryGlobalExternal 全局外部渠道最短间隔：通过返回 true。
func (r *rateLimiter) tryGlobalExternal(ctx context.Context, minIntervalSec int) bool {
	if minIntervalSec <= 0 {
		return true
	}
	if r.rdb != nil {
		ok, err := r.rdb.SetNX(ctx, redisGlobalKey, "1", time.Duration(minIntervalSec)*time.Second).Result()
		return err == nil && ok
	}
	return r.tryMem(redisGlobalKey, time.Duration(minIntervalSec)*time.Second)
}

func (r *rateLimiter) tryMem(key string, ttl time.Duration) bool {
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	if exp, ok := r.mem[key]; ok && now.Before(exp) {
		return false
	}
	r.mem[key] = now.Add(ttl)
	if len(r.mem) > 4096 {
		for k, exp := range r.mem {
			if now.After(exp) {
				delete(r.mem, k)
			}
		}
	}
	return true
}
