package router

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"tracehub/internal/ent/enttest"
	"tracehub/internal/pkg/sitecfg"

	_ "github.com/mattn/go-sqlite3"
)

func TestExecuteWithFailover_retriesOn5xx(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failover?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-g").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		SetRoutingOptions(map[string]any{
			"failover_enabled":      true,
			"failover_max_attempts": float64(2),
		}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	chHigh, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(10).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chLow, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	sel, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(ctx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			if sel.Channel.ID == chHigh.ID {
				return &UpstreamResult{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":"boom"}`)}, nil
			}
			return &UpstreamResult{StatusCode: http.StatusOK, Body: []byte(`{"ok":true}`)}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
	if sel.Channel.ID != chLow.ID {
		t.Fatalf("expected channel %d, got %d", chLow.ID, sel.Channel.ID)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
}

func TestExecuteWithFailover_noRetryOn4xx(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failover4xx?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-4xx").
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

	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(10).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	_, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(ctx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			return &UpstreamResult{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":"bad"}`)}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("attempts=%d", attempts)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", res.StatusCode)
	}
}

func TestExecuteWithFailover_disabledSingleAttempt(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failoveroff?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().SetName("failover-off").Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(10).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	_, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(ctx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			return &UpstreamResult{StatusCode: http.StatusInternalServerError}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("attempts=%d", attempts)
	}
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status=%d", res.StatusCode)
	}
}

func TestExecuteWithFailover_transportErrorRetries(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failovertr?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-tr").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		SetRoutingOptions(map[string]any{
			"failover_enabled":      true,
			"failover_max_attempts": float64(2),
		}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	chHigh, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(10).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chLow, err := client.Channel.Create().
		SetGroupID(g.ID).
		SetChannelType("openai").
		SetPriority(1).
		SetModels([]string{"gpt-4o"}).
		SetEnabled(true).
		SetSchedulable(true).
		SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	sel, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(ctx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			if sel.Channel.ID == chHigh.ID {
				return &UpstreamResult{TransportErr: errors.New("dial failed")}, errors.New("dial failed")
			}
			return &UpstreamResult{StatusCode: http.StatusOK, Body: []byte(`{}`)}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
	if sel.Channel.ID != chLow.ID {
		t.Fatalf("expected channel %d, got %d", chLow.ID, sel.Channel.ID)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", res.StatusCode)
	}
}
