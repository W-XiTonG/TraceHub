package gateway

import (
	"context"
	"strings"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/gateway/router"
	"tracehub/internal/service/upstreamprobe"

	"github.com/gin-gonic/gin"
)

type failoverAttemptHookKey struct{}

// ContextWithFailoverAttemptHook 将 failover 中间尝试写库钩子挂到 context。
func ContextWithFailoverAttemptHook(ctx context.Context, hook router.FailoverAttemptHook) context.Context {
	if hook == nil {
		return ctx
	}
	return context.WithValue(ctx, failoverAttemptHookKey{}, hook)
}

func failoverAttemptHookFromContext(ctx context.Context) router.FailoverAttemptHook {
	if v := ctx.Value(failoverAttemptHookKey{}); v != nil {
		if h, ok := v.(router.FailoverAttemptHook); ok {
			return h
		}
	}
	return nil
}

// GinContextWithFailoverAttemptHook 为 ExecuteModelForward 等注入按 request 写 usage_logs 的钩子。
func GinContextWithFailoverAttemptHook(c *gin.Context, model string) context.Context {
	return ContextWithFailoverAttemptHook(c.Request.Context(), ginFailoverAttemptHook(c, model))
}

func ginFailoverAttemptHook(c *gin.Context, model string) router.FailoverAttemptHook {
	return func(ctx context.Context, ev router.FailoverAttemptEvent) {
		rec := UsageRecorderFrom(c)
		if rec == nil {
			return
		}
		if m := strings.TrimSpace(model); m != "" {
			rec.SetModel(m)
		}
		recordFailoverAttemptFromEvent(rec, ev.Selected, ev.Result, ev.Err, ev.Attempt)
	}
}

// RecordUsageTransportOutcome 记录 failover 用尽后的传输失败（客户端 outcome）。
func RecordUsageTransportOutcome(c *gin.Context, sel *router.Selected, res *router.UpstreamResult, err error) {
	if err == nil {
		return
	}
	NoteUsageOutcomeAttempt(c, FailoverAttemptsFromResult(res))
	if rec := UsageRecorderFrom(c); rec != nil {
		if sel != nil {
			rec.SetChannel(sel.Channel.ID, sel.UpstreamModel)
		}
		rec.SaveFail(502, err.Error(), 0, 0, 0)
	}
}

func noteUsageOutcomeFromUpstreamResult(c *gin.Context, res *router.UpstreamResult) {
	if res == nil {
		return
	}
	NoteUsageOutcomeAttempt(c, FailoverAttemptsFromResult(res))
}

// RecordUsageProxyOutcome 记录 failover 后仍返回给客户端的上游 4xx/5xx（计费为 0）。
func RecordUsageProxyOutcome(c *gin.Context, client *ent.Client, userID, apiKeyID int, sel *router.Selected, res *router.UpstreamResult, requestID, model, ip string, start time.Time) {
	if sel == nil || res == nil {
		return
	}
	noteUsageOutcomeFromUpstreamResult(c, res)
	if rec := UsageRecorderFrom(c); rec != nil {
		rec.SetModel(model)
		rec.SetChannel(sel.Channel.ID, sel.UpstreamModel)
		rec.SaveFail(res.StatusCode, "", 0, 0, 0)
		return
	}
	RecordUsageOKWithAttempt(c, client, userID, apiKeyID, sel.Channel.ID, requestID, model, sel.UpstreamModel,
		0, 0, 0, 0, ip, res.StatusCode, nil, time.Since(start), FailoverAttemptsFromResult(res))
}


func upstreamAttemptStatusCode(res *router.UpstreamResult, err error) int {
	if res != nil && res.StatusCode > 0 {
		return res.StatusCode
	}
	if err != nil {
		return 502
	}
	return 0
}

func upstreamAttemptErrorMessage(res *router.UpstreamResult, err error) *string {
	var msg string
	if err != nil {
		msg = err.Error()
	} else if res != nil && len(res.Body) > 0 {
		code, summary := upstreamprobe.ParseUpstreamErrorBody(res.Body)
		switch {
		case code != "" && summary != "":
			msg = code + ": " + summary
		case summary != "":
			msg = summary
		case code != "":
			msg = code
		}
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}
	return &msg
}

func recordFailoverAttemptFromEvent(rec *UsageRecorder, sel *router.Selected, res *router.UpstreamResult, err error, attempt int) {
	if rec == nil || sel == nil {
		return
	}
	status := upstreamAttemptStatusCode(res, err)
	rec.SaveFailoverAttempt(sel.Channel.ID, sel.UpstreamModel, status, upstreamAttemptErrorMessage(res, err), attempt, 0)
}
