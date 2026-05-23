package adminhandler

import (
	"context"
	"fmt"
	"strings"

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
	if v, ok := opts["failover_first_attempt_timeout_sec"]; ok {
		if n, ok := routerIntFromAny(v); ok {
			if n < 5 || n > 120 {
				return fmt.Errorf("failover_first_attempt_timeout_sec 须在 5–120 之间")
			}
		}
	}
	if v, ok := opts["failover_retry_attempt_timeout_sec"]; ok {
		if n, ok := routerIntFromAny(v); ok {
			if n < 0 || n > 600 {
				return fmt.Errorf("failover_retry_attempt_timeout_sec 须在 0–600 之间")
			}
		}
	}
	if v, ok := opts["failover_retry_after_sec"]; ok {
		if n, ok := routerIntFromAny(v); ok {
			if n < 1 || n > 300 {
				return fmt.Errorf("failover_retry_after_sec 须在 1–300 之间")
			}
		}
	}
	if v, ok := opts["failover_4xx_mode"]; ok {
		if s, ok := v.(string); ok {
			if sitecfg.ValidateRoutingFailover4xxMode(s) != nil {
				return fmt.Errorf("failover_4xx_mode 须为 off、selective、aggressive 或 custom")
			}
		}
	}
	mode := sitecfg.Failover4xxModeOff
	if v, ok := opts["failover_4xx_mode"].(string); ok && strings.TrimSpace(v) != "" {
		mode = sitecfg.NormalizeFailover4xxMode(v)
	}
	var codes []int
	if v, ok := opts["failover_4xx_codes"]; ok {
		switch t := v.(type) {
		case []any:
			for _, item := range t {
				switch n := item.(type) {
				case float64:
					codes = append(codes, int(n))
				case int:
					codes = append(codes, n)
				case int64:
					codes = append(codes, int(n))
				}
			}
		case []int:
			codes = append(codes, t...)
		}
	}
	if err := sitecfg.ValidateRoutingFailover4xxCodesForMode(mode, codes); err != nil {
		return fmt.Errorf("failover_4xx_codes 在 custom 模式下须至少选择一个状态码")
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
