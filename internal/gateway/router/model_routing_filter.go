package router

import (
	"fmt"
	"strings"
)

// filterCandidatesByModelRouting 若分组配置了 model_routing[model]，仅保留列表中的渠道（保持配置顺序）。
// 过滤后无可用候选时返回错误，避免静默漂移到未绑定渠道。
func filterCandidatesByModelRouting(candidates []candidate, modelRouting map[string][]int64, requestedModel string) ([]candidate, error) {
	if len(modelRouting) == 0 || len(candidates) == 0 {
		return candidates, nil
	}
	order, ok := modelRouting[strings.TrimSpace(requestedModel)]
	if !ok || len(order) == 0 {
		return candidates, nil
	}
	byID := make(map[int]candidate, len(candidates))
	for _, c := range candidates {
		byID[c.ch.ID] = c
	}
	filtered := make([]candidate, 0, len(order))
	for _, id := range order {
		if c, ok := byID[int(id)]; ok {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("模型 %s 绑定的渠道当前不可用", requestedModel)
	}
	return filtered, nil
}
