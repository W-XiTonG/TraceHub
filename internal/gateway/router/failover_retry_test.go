package router

import (
	"net/http"
	"testing"

	"tracehub/internal/pkg/sitecfg"
)

func TestIsRetryableUpstreamForFailover_selective401(t *testing.T) {
	cfg := RoutingConfig{Failover4xxMode: sitecfg.Failover4xxModeSelective}
	body := []byte(`{"error":{"message":"invalid api key"}}`)
	if !IsRetryableUpstreamForFailover(cfg, http.StatusUnauthorized, nil, body, 2) {
		t.Fatal("expected retryable")
	}
}

func TestIsRetryableUpstreamForFailover_selective401CodeUnchecked(t *testing.T) {
	cfg := RoutingConfig{
		Failover4xxMode:  sitecfg.Failover4xxModeSelective,
		Failover4xxCodes: []int{400, 403},
	}
	body := []byte(`{"error":{"message":"invalid api key"}}`)
	if IsRetryableUpstreamForFailover(cfg, http.StatusUnauthorized, nil, body, 2) {
		t.Fatal("expected not retryable when 401 not selected")
	}
}

func TestIsRetryableUpstreamForFailover_custom403(t *testing.T) {
	cfg := RoutingConfig{
		Failover4xxMode:  sitecfg.Failover4xxModeCustom,
		Failover4xxCodes: []int{403},
	}
	if !IsRetryableUpstreamForFailover(cfg, http.StatusForbidden, nil, []byte(`forbidden`), 2) {
		t.Fatal("expected retryable")
	}
}

func TestIsRetryableUpstreamForFailover_off4xx(t *testing.T) {
	cfg := RoutingConfig{Failover4xxMode: sitecfg.Failover4xxModeOff}
	if IsRetryableUpstreamForFailover(cfg, http.StatusUnauthorized, nil, []byte(`invalid api key`), 2) {
		t.Fatal("expected not retryable")
	}
}

func TestIsRetryableUpstreamForFailover_blacklistQuota(t *testing.T) {
	cfg := RoutingConfig{Failover4xxMode: sitecfg.Failover4xxModeAggressive}
	body := []byte(`{"error":"insufficient_quota"}`)
	if IsRetryableUpstreamForFailover(cfg, http.StatusBadRequest, nil, body, 2) {
		t.Fatal("expected not retryable")
	}
}
