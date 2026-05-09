package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"

	"github.com/redis/go-redis/v9"
)

type Selected struct {
	Channel       *ent.Channel
	UpstreamModel string
}

func modelMatch(ch *ent.Channel, requested string) (upstream string, ok bool) {
	for _, m := range ch.Models {
		if m == requested {
			return requested, true
		}
	}
	if ch.ModelAlias != nil {
		if u, ok := ch.ModelAlias[requested]; ok && u != "" {
			return u, true
		}
	}
	return "", false
}

func stickyKey(userID int64, session string) string {
	h := strconv.FormatInt(userID, 10)
	return fmt.Sprintf("tracehub:sticky:%s:%s", h, session)
}

// SelectChannel 根据分组、模型与粘性会话挑选渠道。
func SelectChannel(ctx context.Context, rdb *redis.Client, client *ent.Client, userID int64, userGroupID int, requestedModel string, stickySession string) (*Selected, error) {
	if userGroupID <= 0 {
		return nil, errors.New("用户未分配分组")
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HealthStatusEQ(1),
			channel.HasGroupWith(group.ID(userGroupID)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	type cand struct {
		ch       *ent.Channel
		upstream string
	}
	var candidates []cand
	for _, ch := range channels {
		if up, ok := modelMatch(ch, requestedModel); ok {
			candidates = append(candidates, cand{ch: ch, upstream: up})
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("没有可用渠道支持模型 %s", requestedModel)
	}

	// 粘性会话：命中 Redis 则固定渠道（仍需支持该模型）
	if rdb != nil && stickySession != "" {
		key := stickyKey(userID, stickySession)
		if idStr, err := rdb.Get(ctx, key).Result(); err == nil && idStr != "" {
			var cid int
			_, _ = fmt.Sscanf(idStr, "%d", &cid)
			for _, c := range candidates {
				if c.ch.ID == cid {
					return &Selected{Channel: c.ch, UpstreamModel: c.upstream}, nil
				}
			}
		}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	total := 0
	for _, c := range candidates {
		w := c.ch.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	pick := rng.Intn(total)
	acc := 0
	var chosen cand
	for _, c := range candidates {
		w := c.ch.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if pick < acc {
			chosen = c
			break
		}
	}
	if chosen.ch == nil {
		chosen = candidates[len(candidates)-1]
	}

	if rdb != nil && stickySession != "" {
		_ = rdb.Set(ctx, stickyKey(userID, stickySession), strconv.Itoa(chosen.ch.ID), time.Hour).Err()
	}

	return &Selected{Channel: chosen.ch, UpstreamModel: chosen.upstream}, nil
}

// PatchModelInBody 将请求 JSON 中的 model 替换为上游模型名。
func PatchModelInBody(body []byte, upstream string) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	b, err := json.Marshal(upstream)
	if err != nil {
		return nil, err
	}
	m["model"] = json.RawMessage(b)
	return json.Marshal(m)
}

func DefaultBaseURL(chType string) string {
	switch strings.ToLower(chType) {
	case "openai":
		return "https://api.openai.com/v1"
	case "deepseek":
		return "https://api.deepseek.com/v1"
	case "claude":
		return "https://api.anthropic.com/v1"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	default:
		return ""
	}
}
