package pricingsync

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"tracehub/internal/ent"
)

// TestConnectionResult 连接测试结果。
type TestConnectionResult struct {
	OK          bool           `json:"ok"`
	Direction   string         `json:"direction"`
	LatencyMS   int64          `json:"latency_ms"`
	ModelCounts map[string]int `json:"model_counts,omitempty"`
	Message     string         `json:"message,omitempty"`
	HTTPStatus  int              `json:"http_status,omitempty"`
}

// TestTargetConnection 测试出站目标连通性（只读，不写入定价）。
func TestTargetConnection(ctx context.Context, client *ent.Client, targetID int, direction string, opts RemoteOpts) (TestConnectionResult, error) {
	direction, err := normalizeDirection(direction)
	if err != nil {
		return TestConnectionResult{}, err
	}
	t, err := client.ModelPricingSyncTarget.Get(ctx, targetID)
	if err != nil {
		return TestConnectionResult{}, err
	}
	if !directionAllowed(t, direction) {
		return TestConnectionResult{}, errDirection
	}
	switch direction {
	case DirectionPull:
		return testPullConnection(ctx, client, t, opts)
	case DirectionPush:
		return testPushConnection(ctx, client, t, opts)
	default:
		return TestConnectionResult{}, errDirection
	}
}

func testPullConnection(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, opts RemoteOpts) (TestConnectionResult, error) {
	start := time.Now()
	url := joinURL(t.BaseURL, t.PullPath)
	if err := ValidateOutboundURL(url, opts.AllowInsecureHTTP); err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPull, Message: err.Error()}, nil
	}
	req, err := buildAuthRequest(ctx, client, t, http.MethodGet, url, nil, opts)
	if err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPull, Message: err.Error()}, nil
	}
	resp, err := httpClient().Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPull, LatencyMS: latency, Message: err.Error()}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TestConnectionResult{
			OK: false, Direction: DirectionPull, LatencyMS: latency,
			HTTPStatus: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 300)),
		}, nil
	}
	p, err := ParsePayloadJSON(body)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, errVersionMismatch) {
			msg = errVersionMismatch.Error()
		} else {
			msg = "解析 JSON: " + msg
		}
		return TestConnectionResult{OK: false, Direction: DirectionPull, LatencyMS: latency, Message: msg}, nil
	}
	counts := payloadModelCounts(p)
	return TestConnectionResult{
		OK: true, Direction: DirectionPull, LatencyMS: latency,
		ModelCounts: counts,
		Message:     fmt.Sprintf("连接成功：全站 %d 条，渠道块 %d，分组块 %d", counts["global"], counts["channels"], counts["groups"]),
	}, nil
}

func testPushConnection(ctx context.Context, client *ent.Client, t *ent.ModelPricingSyncTarget, opts RemoteOpts) (TestConnectionResult, error) {
	start := time.Now()
	url := joinURL(t.BaseURL, t.PushPath)
	if err := ValidateOutboundURL(url, opts.AllowInsecureHTTP); err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPush, Message: err.Error()}, nil
	}
	req, err := buildAuthRequest(ctx, client, t, http.MethodGet, url, nil, opts)
	if err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPush, Message: err.Error()}, nil
	}
	resp, err := httpClient().Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return TestConnectionResult{OK: false, Direction: DirectionPush, LatencyMS: latency, Message: err.Error()}, nil
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch resp.StatusCode {
	case http.StatusOK, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusMethodNotAllowed, http.StatusBadRequest:
		return TestConnectionResult{
			OK: true, Direction: DirectionPush, LatencyMS: latency, HTTPStatus: resp.StatusCode,
			Message: fmt.Sprintf("端点可达（HTTP %d）", resp.StatusCode),
		}, nil
	case http.StatusNotFound:
		return TestConnectionResult{
			OK: false, Direction: DirectionPush, LatencyMS: latency, HTTPStatus: resp.StatusCode,
			Message: "HTTP 404：请检查 push_path 是否正确",
		}, nil
	default:
		if resp.StatusCode >= 200 && resp.StatusCode < 500 {
			return TestConnectionResult{
				OK: true, Direction: DirectionPush, LatencyMS: latency, HTTPStatus: resp.StatusCode,
				Message: fmt.Sprintf("端点有响应（HTTP %d）", resp.StatusCode),
			}, nil
		}
		return TestConnectionResult{
			OK: false, Direction: DirectionPush, LatencyMS: latency, HTTPStatus: resp.StatusCode,
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		}, nil
	}
}

func payloadModelCounts(p *Payload) map[string]int {
	if p == nil {
		return map[string]int{"global": 0, "channels": 0, "groups": 0}
	}
	chItems := 0
	for _, b := range p.Channels {
		chItems += len(b.Items)
	}
	grItems := 0
	for _, b := range p.Groups {
		grItems += len(b.Items)
	}
	return map[string]int{
		"global":   len(p.Global),
		"channels": chItems,
		"groups":   grItems,
	}
}
