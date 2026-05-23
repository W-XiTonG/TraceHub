package router

import (
	"context"
	"errors"
	"net/http"
	"time"

	"tracehub/internal/ent"

	"github.com/redis/go-redis/v9"
)

// UpstreamResult 上游 HTTP 转发结果。
type UpstreamResult struct {
	StatusCode       int
	Body             []byte
	Headers          http.Header
	TransportErr     error
	StreamResponse   *http.Response // 仅流式 200 且未读 body 时使用；调用方负责关闭
	FailoverAttempts int            // 本次请求上游尝试次数（含成功）
}

// UpstreamAttemptFunc 执行一次上游请求。
type UpstreamAttemptFunc func(ctx context.Context, sel *Selected) (*UpstreamResult, error)

// SelectDeps 选路与 failover 依赖。
type SelectDeps struct {
	RDB    *redis.Client
	Client *ent.Client
}

func attemptContext(parent context.Context, cfg RoutingConfig, attemptN int) (context.Context, context.CancelFunc) {
	sec := 0
	if attemptN == 0 {
		sec = cfg.FailoverFirstAttemptTimeoutSec
		if sec < 5 {
			sec = 30
		}
	} else if cfg.FailoverRetryAttemptTimeoutSec > 0 {
		sec = cfg.FailoverRetryAttemptTimeoutSec
	}
	if sec <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, time.Duration(sec)*time.Second)
}

func annotateAttempts(res *UpstreamResult, attempts int) {
	if res != nil {
		res.FailoverAttempts = attempts
	}
}

// ExecuteWithFailover 按配置选路并在可重试错误时换渠道重试。
// firstSel 非 nil 时首轮复用该选路结果，避免计费预检与转发重复查库（failover 关闭时仅一次选路）。
// meta 非 nil 时写入尝试次数等信息。
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
	meta *FailoverExecMeta,
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
				if meta != nil {
					meta.Attempts = attemptN
				}
				return lastSel, lastRes, selErr
			}
		}
		lastSel = sel
		cfg = sel.Config

		attemptCtx, cancel := attemptContext(ctx, cfg, attemptN)
		res, err := attempt(attemptCtx, sel)
		cancel()

		attemptsUsed := attemptN + 1
		annotateAttempts(res, attemptsUsed)
		lastRes = res
		lastErr = err

		body := []byte(nil)
		if res != nil {
			body = res.Body
		}
		afterExclude := append(append([]int(nil), exclude...), sel.Channel.ID)
		remaining := CountModelCandidates(ctx, deps.Client, groupID, model, afterExclude)
		retryable := IsRetryableUpstreamForFailover(cfg, statusCodeFrom(res, err), err, body, remaining)

		if err == nil && res != nil && !retryable {
			if meta != nil {
				meta.Attempts = attemptsUsed
			}
			return sel, res, nil
		}
		if err == nil && res == nil {
			if meta != nil {
				meta.Attempts = attemptsUsed
			}
			return sel, nil, nil
		}
		if !retryable || attemptN+1 >= maxAttempts {
			if meta != nil {
				meta.Attempts = attemptsUsed
			}
			if err != nil {
				return sel, res, err
			}
			return sel, res, nil
		}

		if cfg.StickyEnabled && sticky != "" {
			ClearStickySession(ctx, deps.RDB, userID, sticky)
		}
		exclude = append(exclude, sel.Channel.ID)
	}
	if meta != nil {
		meta.Attempts = maxAttempts
	}
	if lastErr != nil {
		return lastSel, lastRes, lastErr
	}
	return lastSel, lastRes, errors.New("failover exhausted")
}

func statusCodeFrom(res *UpstreamResult, err error) int {
	if res != nil && res.StatusCode > 0 {
		return res.StatusCode
	}
	return 0
}
