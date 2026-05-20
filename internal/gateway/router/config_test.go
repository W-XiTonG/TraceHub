package router

import (
	"context"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

func TestResolveRoutingConfig_inheritGlobal(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:resolve?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().SetName("g1").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ResolveRoutingConfig(ctx, client, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Strategy != sitecfg.RoutingStrategyWeightedRandom {
		t.Fatalf("strategy=%q", cfg.Strategy)
	}
	if !cfg.FailoverEnabled {
		t.Fatal("expected failover on by default")
	}
	if !cfg.StickyEnabled {
		t.Fatal("expected sticky on by default")
	}
}

func TestResolveRoutingConfig_groupOverride(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:resolve2?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("g2").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		SetRoutingOptions(map[string]any{
			"failover_enabled":      true,
			"failover_max_attempts": float64(3),
		}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)
	cfg, err := ResolveRoutingConfig(ctx, client, g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Strategy != sitecfg.RoutingStrategyPriority {
		t.Fatalf("strategy=%q", cfg.Strategy)
	}
	if !cfg.FailoverEnabled {
		t.Fatal("expected failover enabled")
	}
	if cfg.FailoverMaxAttempts != 3 {
		t.Fatalf("max=%d", cfg.FailoverMaxAttempts)
	}
}
