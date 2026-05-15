package router

import (
	"context"
	"errors"
	"net/http"

	"tracehub/internal/ent"

	"github.com/redis/go-redis/v9"
)

// UpstreamResult 上游 HTTP 转发结果。
type UpstreamResult struct {
	StatusCode   int
	Body         []byte
	Headers      http.Header
	TransportErr error
}

// UpstreamAttemptFunc 执行一次上游请求。
type UpstreamAttemptFunc func(ctx context.Context, sel *Selected) (*UpstreamResult, error)

// IsRetryableUpstream 判断上游错误是否可 failover 重试。
func IsRetryableUpstream(statusCode int, transportErr error) bool {
	if transportErr != nil {
		return true
	}
	if statusCode >= 500 {
		return true
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	return false
}

// SelectDeps 选路与 failover 依赖。
type SelectDeps struct {
	RDB    *redis.Client
	Client *ent.Client
}

// ExecuteWithFailover 按配置选路并在可重试错误时换渠道重试。
// firstSel 非 nil 时首轮复用该选路结果，避免计费预检与转发重复查库（failover 关闭时仅一次选路）。
func ExecuteWithFailover(
	ctx context.Context,
	deps SelectDeps,
	userID int64,
	groupID int,
	model string,
	sticky string,
	responseWritten bool,
	firstSel *Selected,
	attempt UpstreamAttemptFunc,
) (*Selected, *UpstreamResult, error) {
	var exclude []int
	var lastSel *Selected
	var lastRes *UpstreamResult
	var lastErr error

	cfg, err := ResolveRoutingConfig(ctx, deps.Client, groupID)
	if err != nil {
		return nil, nil, err
	}
	maxAttempts := 1
	if cfg.FailoverEnabled && !responseWritten {
		maxAttempts = cfg.FailoverMaxAttempts
		if maxAttempts < 1 {
			maxAttempts = 1
		}
		if maxAttempts > 5 {
			maxAttempts = 5
		}
	}

	for attemptN := 0; attemptN < maxAttempts; attemptN++ {
		var sel *Selected
		if attemptN == 0 && firstSel != nil {
			sel = firstSel
		} else {
			var selErr error
			sel, selErr = SelectChannelExcluding(ctx, deps.RDB, deps.Client, userID, groupID, model, sticky, exclude)
			if selErr != nil {
				return lastSel, lastRes, selErr
			}
		}
		lastSel = sel

		res, err := attempt(ctx, sel)
		lastRes = res
		lastErr = err
		if err == nil && res != nil && !IsRetryableUpstream(res.StatusCode, nil) {
			return sel, res, nil
		}
		if err == nil && res == nil {
			return sel, nil, nil
		}
		retryable := false
		if err != nil {
			retryable = IsRetryableUpstream(0, err)
		} else if res != nil {
			retryable = IsRetryableUpstream(res.StatusCode, nil)
		}
		if !retryable || attemptN+1 >= maxAttempts {
			if err != nil {
				return sel, res, err
			}
			return sel, res, nil
		}
		exclude = append(exclude, sel.Channel.ID)
	}
	if lastErr != nil {
		return lastSel, lastRes, lastErr
	}
	return lastSel, lastRes, errors.New("failover exhausted")
}
