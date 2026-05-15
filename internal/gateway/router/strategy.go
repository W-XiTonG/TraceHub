package router

import (
	"math/rand"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/pkg/sitecfg"
)

type candidate struct {
	ch       *ent.Channel
	upstream string
}

func poolMinErrorCount(candidates []candidate) []candidate {
	minErr := -1
	for _, c := range candidates {
		ec := c.ch.ErrorCount
		if minErr < 0 || ec < minErr {
			minErr = ec
		}
	}
	var pool []candidate
	for _, c := range candidates {
		if c.ch.ErrorCount == minErr {
			pool = append(pool, c)
		}
	}
	if len(pool) == 0 {
		return candidates
	}
	return pool
}

func pickWeightedRandom(pool []candidate, rng *rand.Rand) candidate {
	total := 0
	for _, c := range pool {
		w := c.ch.Weight
		if w <= 0 {
			w = 1
		}
		total += w
	}
	if total <= 0 {
		return pool[len(pool)-1]
	}
	pick := rng.Intn(total)
	acc := 0
	for _, c := range pool {
		w := c.ch.Weight
		if w <= 0 {
			w = 1
		}
		acc += w
		if pick < acc {
			return c
		}
	}
	return pool[len(pool)-1]
}

func pickByWeightedRandom(candidates []candidate, rng *rand.Rand) candidate {
	pool := poolMinErrorCount(candidates)
	return pickWeightedRandom(pool, rng)
}

func pickByPriority(candidates []candidate, rng *rand.Rand) candidate {
	maxP := candidates[0].ch.Priority
	for _, c := range candidates[1:] {
		if c.ch.Priority > maxP {
			maxP = c.ch.Priority
		}
	}
	var pool []candidate
	for _, c := range candidates {
		if c.ch.Priority == maxP {
			pool = append(pool, c)
		}
	}
	if len(pool) == 1 {
		return pool[0]
	}
	return pickWeightedRandom(pool, rng)
}

func pickByFixedOrder(candidates []candidate, modelRouting map[string][]int64, requestedModel string, rng *rand.Rand) candidate {
	if modelRouting != nil {
		if order, ok := modelRouting[requestedModel]; ok && len(order) > 0 {
			byID := make(map[int]candidate, len(candidates))
			for _, c := range candidates {
				byID[c.ch.ID] = c
			}
			for _, id := range order {
				if c, ok := byID[int(id)]; ok {
					return c
				}
			}
		}
	}
	return pickByPriority(candidates, rng)
}

func applyStrategy(cfg RoutingConfig, candidates []candidate, requestedModel string, rng *rand.Rand) candidate {
	switch cfg.Strategy {
	case sitecfg.RoutingStrategyPriority:
		return pickByPriority(candidates, rng)
	case sitecfg.RoutingStrategyFixedOrder:
		return pickByFixedOrder(candidates, cfg.ModelRouting, requestedModel, rng)
	default:
		return pickByWeightedRandom(candidates, rng)
	}
}

func newRNG() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}
