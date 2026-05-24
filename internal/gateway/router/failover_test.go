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
		}, nil, nil)
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

func TestExecuteWithFailover_attemptHook_onIntermediateRetry(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failoverhook?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-hook").
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

	hookCalls := 0
	hook := func(_ context.Context, ev FailoverAttemptEvent) {
		if !ev.WillRetry {
			t.Fatal("expected WillRetry")
		}
		if ev.Attempt != 1 {
			t.Fatalf("attempt=%d", ev.Attempt)
		}
		hookCalls++
	}

	attempts := 0
	_, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(_ context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			if sel.Channel.ID == chHigh.ID {
				return &UpstreamResult{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":"boom"}`)}, nil
			}
			return &UpstreamResult{StatusCode: http.StatusOK, Body: []byte(`{"ok":true}`)}, nil
		}, nil, hook)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || hookCalls != 1 {
		t.Fatalf("attempts=%d hookCalls=%d", attempts, hookCalls)
	}
	if res.FailoverAttempts != 2 {
		t.Fatalf("FailoverAttempts=%d", res.FailoverAttempts)
	}
	_ = chLow
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
		}, nil, nil)
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

func TestExecuteWithFailover_retriesOn401Selective(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failover401?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-401").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		SetRoutingOptions(map[string]any{
			"failover_enabled":      true,
			"failover_max_attempts": float64(2),
			"failover_4xx_mode":     sitecfg.Failover4xxModeSelective,
		}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	chHigh, err := client.Channel.Create().
		SetGroupID(g.ID).SetChannelType("openai").SetPriority(10).
		SetModels([]string{"gpt-4o"}).SetEnabled(true).SetSchedulable(true).SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	chLow, err := client.Channel.Create().
		SetGroupID(g.ID).SetChannelType("openai").SetPriority(1).
		SetModels([]string{"gpt-4o"}).SetEnabled(true).SetSchedulable(true).SetHealthStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}

	attempts := 0
	sel, res, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(ctx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			if sel.Channel.ID == chHigh.ID {
				return &UpstreamResult{StatusCode: http.StatusUnauthorized, Body: []byte(`{"error":{"message":"invalid api key"}}`)}, nil
			}
			return &UpstreamResult{StatusCode: http.StatusOK, Body: []byte(`{}`)}, nil
		}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || sel.Channel.ID != chLow.ID || res.StatusCode != http.StatusOK {
		t.Fatalf("attempts=%d sel=%d status=%d", attempts, sel.Channel.ID, res.StatusCode)
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
		}, nil, nil)
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
		}, nil, nil)
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

func TestExecuteWithFailover_firstAttemptContextDeadline(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:failoverdeadline?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	ctx := context.Background()

	g, err := client.Group.Create().
		SetName("failover-deadline").
		SetRoutingStrategy(sitecfg.RoutingStrategyPriority).
		SetRoutingOptions(map[string]any{
			"failover_enabled":                   true,
			"failover_max_attempts":              float64(2),
			"failover_first_attempt_timeout_sec": float64(1),
		}).
		Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	InvalidateRoutingCache(g.ID)

	chHigh, _ := client.Channel.Create().SetGroupID(g.ID).SetChannelType("openai").SetPriority(10).
		SetModels([]string{"gpt-4o"}).SetEnabled(true).SetSchedulable(true).SetHealthStatus(1).Save(ctx)
	chLow, _ := client.Channel.Create().SetGroupID(g.ID).SetChannelType("openai").SetPriority(1).
		SetModels([]string{"gpt-4o"}).SetEnabled(true).SetSchedulable(true).SetHealthStatus(1).Save(ctx)

	attempts := 0
	sel, _, err := ExecuteWithFailover(ctx, SelectDeps{Client: client}, 1, g.ID, "gpt-4o", "", false, nil,
		func(attemptCtx context.Context, sel *Selected) (*UpstreamResult, error) {
			attempts++
			if sel.Channel.ID == chHigh.ID {
				<-attemptCtx.Done()
				return nil, attemptCtx.Err()
			}
			return &UpstreamResult{StatusCode: http.StatusOK}, nil
		}, nil, nil)
	if err != nil || attempts != 2 || sel.Channel.ID != chLow.ID {
		t.Fatalf("err=%v attempts=%d sel=%d", err, attempts, sel.Channel.ID)
	}
}
