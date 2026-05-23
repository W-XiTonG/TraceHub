package gateway

import (
	"context"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/ent/group"
	"tracehub/internal/gateway/router"
	"tracehub/internal/service/billing"

	"github.com/redis/go-redis/v9"
)

// SelectChannelForBillingPrecheck 选路供计费预检；failover 开启且同模型多渠道时选用预估扣费最高的渠道。
func SelectChannelForBillingPrecheck(
	ctx context.Context,
	client *ent.Client,
	rdb *redis.Client,
	eng *billing.Engine,
	u *ent.User,
	gid int,
	model string,
	sticky string,
	tokenEst BillTokenEst,
) (*router.Selected, error) {
	cfg, err := router.ResolveRoutingConfig(ctx, client, gid)
	if err != nil {
		return nil, err
	}
	if !cfg.FailoverEnabled || router.CountModelCandidates(ctx, client, gid, model, nil) < 2 {
		return router.SelectChannel(ctx, rdb, client, int64(u.ID), gid, model, sticky)
	}
	channels, err := client.Channel.Query().
		Where(
			channel.Enabled(true),
			channel.Schedulable(true),
			channel.HealthStatusEQ(1),
			channel.HasGroupWith(group.ID(gid)),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var best *router.Selected
	var bestPre int64 = -1
	for _, ch := range channels {
		up, ok := router.MatchModelOnChannel(ch, model)
		if !ok {
			continue
		}
		br, err := eng.ModelBillRates(ctx, u, model, ch.ID)
		if err != nil {
			continue
		}
		pre, _ := br.PrecheckCost(tokenEst.Prompt, tokenEst.Completion, tokenEst.Cached)
		if pre > bestPre {
			bestPre = pre
			best = &router.Selected{Channel: ch, UpstreamModel: up, Config: cfg}
		}
	}
	if best != nil {
		return best, nil
	}
	return router.SelectChannel(ctx, rdb, client, int64(u.ID), gid, model, sticky)
}
