package adminhandler

import (
	"context"
	"fmt"

	"tracehub/internal/ent"
	"tracehub/internal/ent/channel"
	"tracehub/internal/gateway/router"
	"tracehub/internal/pkg/sitecfg"
)

// validateGroupModelRouting 校验 model_routing 中渠道 ID 均属于该分组。
func validateGroupModelRouting(ctx context.Context, client *ent.Client, groupID int, modelRouting map[string][]int64) error {
	if len(modelRouting) == 0 {
		return nil
	}
	chs, err := client.Channel.Query().Where(channel.GroupIDEQ(groupID)).All(ctx)
	if err != nil {
		return err
	}
	chByID := make(map[int]*ent.Channel, len(chs))
	for _, ch := range chs {
		chByID[ch.ID] = ch
	}
	for model, ids := range modelRouting {
		if model == "" {
			return fmt.Errorf("model_routing 模型名不能为空")
		}
		for _, id := range ids {
			ch, ok := chByID[int(id)]
			if !ok {
				return fmt.Errorf("model_routing[%s] 含不属于该分组的渠道 ID %d", model, id)
			}
			if _, ok := router.MatchModelOnChannel(ch, model); !ok {
				return fmt.Errorf("model_routing[%s] 渠道 %d 未声明该模型", model, id)
			}
		}
	}
	return nil
}

func validateGroupRoutingStrategy(s string) error {
	if s == "" {
		return nil
	}
	return sitecfg.ValidateRoutingStrategy(s)
}

func validateGroupRoutingOptions(opts map[string]any) error {
	if len(opts) == 0 {
		return nil
	}
	if v, ok := opts["failover_max_attempts"]; ok {
		if n, ok := routerIntFromAny(v); ok {
			if n < 1 || n > 5 {
				return fmt.Errorf("failover_max_attempts 须在 1–5 之间")
			}
		}
	}
	if v, ok := opts["probe_fail_threshold"]; ok {
		if n, ok := routerIntFromAny(v); ok {
			if n < 1 || n > 20 {
				return fmt.Errorf("probe_fail_threshold 须在 1–20 之间")
			}
		}
	}
	return nil
}

func routerIntFromAny(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	}
	return 0, false
}
