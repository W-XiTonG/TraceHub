package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/pkg/channelmeta"
	"tracehub/internal/pkg/sitecfg"

	"github.com/redis/go-redis/v9"
)

type Selected struct {
	Channel       *ent.Channel
	UpstreamModel string
	Config        RoutingConfig
}

// MatchModelOnChannel 判断渠道是否支持该逻辑模型名（含别名表），并返回上游模型名。
func MatchModelOnChannel(ch *ent.Channel, requested string) (upstream string, ok bool) {
	return modelMatch(ch, requested)
}

func modelMatch(ch *ent.Channel, requested string) (upstream string, ok bool) {
	req := strings.TrimSpace(requested)
	if req == "" {
		return "", false
	}
	// 别名优先：models 与 alias 键同名时须映射到上游 ID（如火山 ep-），不能只用列表里的公开名。
	if ch.ModelAlias != nil {
		if u, ok := ch.ModelAlias[req]; ok {
			u = strings.TrimSpace(u)
			if u != "" {
				return u, true
			}
		}
	}
	for _, m := range ch.Models {
		id := strings.TrimSpace(m)
		if id == "" {
			continue
		}
		if id == req {
			return id, true
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
	return SelectChannelExcluding(ctx, rdb, client, userID, userGroupID, requestedModel, stickySession, nil)
}

// SelectChannelExcluding 在排除指定渠道 ID 后选路（供 failover 使用）。
func SelectChannelExcluding(ctx context.Context, rdb *redis.Client, client *ent.Client, userID int64, userGroupID int, requestedModel string, stickySession string, excludeIDs []int) (*Selected, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if userGroupID <= 0 {
		return nil, errors.New("用户未分配分组")
	}
	cfg, err := ResolveRoutingConfig(ctx, client, userGroupID)
	if err != nil {
		return nil, err
	}
	if !cfg.StickyEnabled {
		stickySession = ""
	}
	exclude := make(map[int]struct{}, len(excludeIDs))
	for _, id := range excludeIDs {
		exclude[id] = struct{}{}
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
	var candidates []candidate
	for _, ch := range channels {
		if _, skip := exclude[ch.ID]; skip {
			continue
		}
		if up, ok := modelMatch(ch, requestedModel); ok {
			candidates = append(candidates, candidate{ch: ch, upstream: up})
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("没有可用渠道支持模型 %s", requestedModel)
	}

	candidates, err = filterCandidatesByModelRouting(candidates, cfg.ModelRouting, requestedModel)
	if err != nil {
		return nil, err
	}

	if rdb != nil && stickySession != "" {
		key := stickyKey(userID, stickySession)
		if idStr, err := rdb.Get(ctx, key).Result(); err == nil && idStr != "" {
			var cid int
			_, _ = fmt.Sscanf(idStr, "%d", &cid)
			for _, c := range candidates {
				if c.ch.ID == cid {
					return &Selected{Channel: c.ch, UpstreamModel: c.upstream, Config: cfg}, nil
				}
			}
		}
	}

	rng := newRNG()
	chosen := applyStrategy(cfg, candidates, requestedModel, rng)

	if rdb != nil && stickySession != "" {
		_ = rdb.Set(ctx, stickyKey(userID, stickySession), strconv.Itoa(chosen.ch.ID), time.Hour).Err()
	}

	return &Selected{Channel: chosen.ch, UpstreamModel: chosen.upstream, Config: cfg}, nil
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
	return channelmeta.DefaultBaseURL(chType)
}

// StrategyName 返回生效策略名（供调试头）。
func StrategyName(cfg RoutingConfig) string {
	if cfg.Strategy == "" {
		return sitecfg.RoutingStrategyWeightedRandom
	}
	return cfg.Strategy
}
