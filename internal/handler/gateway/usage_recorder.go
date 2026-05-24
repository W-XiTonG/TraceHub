package gateway

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"tracehub/internal/ent"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/usagelogpublic"

	"github.com/gin-gonic/gin"
)

const ctxUsageRecorderKey = "tracehub_usage_recorder"

// usageCostForStatus 上游 HTTP 失败时不记非零费用（与 gatewayDeduct 一致：4xx/5xx 不扣费）。
func usageCostForStatus(status int, cost int64) int64 {
	if status >= 400 {
		return 0
	}
	return cost
}

// usageLogSkipPaths 高频元数据接口，默认不写 usage_logs（性能/噪音）。
var usageLogSkipPaths = map[string]struct{}{
	"GET /v1/models":     {},
	"GET /v1/models/:model_id": {},
}

// UsageRecorder 跟踪单次 /v1 网关请求的调用日志写入，避免成功/失败双写。
type UsageRecorder struct {
	client    *ent.Client
	userID    int
	apiKeyID  int
	channelID int
	requestID string
	model     string
	upstream  string
	ip        string
	start     time.Time

	errMsg   string
	traceRef string // 非错误追踪（如 video_job:id），写入 error_message 供前端聚合
	logged   bool
	outcomeAttempt int // 客户端最终结果在 failover 链中的序号；默认 1
	mu       sync.Mutex

	disabled bool
}

// GatewayUsageRecorder 在 GatewayAPIKeyAuth 之后注入 recorder；handler 返回且 status>=400 时异步兜底写库。
func GatewayUsageRecorder(client *ent.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		uAny, okU := c.Get(middleware.CtxGateUserKey)
		kAny, okK := c.Get(middleware.CtxGateAPIKeyKey)
		if !okU || !okK {
			c.Next()
			return
		}
		u := uAny.(*ent.User)
		k := kAny.(*ent.APIKey)
		rid, _ := c.Get("request_id")
		reqID, _ := rid.(string)

		rec := &UsageRecorder{
			client:    client,
			userID:    u.ID,
			apiKeyID:  k.ID,
			ip:        c.ClientIP(),
			requestID: reqID,
			start:     time.Now(),
			disabled:  shouldSkipUsageLog(c),
		}
		c.Set(ctxUsageRecorderKey, rec)
		c.Next()

		rec.maybeFlushFailure(c)
	}
}

func shouldSkipUsageLog(c *gin.Context) bool {
	if c.Request.Method == "GET" && strings.HasPrefix(c.Request.URL.Path, "/v1/models") {
		return true
	}
	key := c.Request.Method + " " + c.FullPath()
	_, ok := usageLogSkipPaths[key]
	return ok
}

// UsageRecorderFrom 读取当前请求的 recorder（非 /v1 或未注入时为 nil）。
func UsageRecorderFrom(c *gin.Context) *UsageRecorder {
	v, ok := c.Get(ctxUsageRecorderKey)
	if !ok {
		return nil
	}
	rec, _ := v.(*UsageRecorder)
	return rec
}

func (r *UsageRecorder) SetModel(model string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := strings.TrimSpace(model); m != "" {
		r.model = m
	}
}

func (r *UsageRecorder) SetChannel(channelID int, upstream string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if channelID > 0 {
		r.channelID = channelID
	}
	if u := strings.TrimSpace(upstream); u != "" {
		r.upstream = u
	}
}

func (r *UsageRecorder) SetUpstream(upstream string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if u := strings.TrimSpace(upstream); u != "" {
		r.upstream = u
	}
}

func (r *UsageRecorder) NoteError(msg string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := strings.TrimSpace(msg); m != "" {
		r.errMsg = m
	}
}

// SetTraceRef 写入 usage_logs.error_message 的追踪字段（如 video_job:id），供控制台聚合；真实错误优先。
func (r *UsageRecorder) SetTraceRef(ref string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if m := strings.TrimSpace(ref); m != "" {
		r.traceRef = m
	}
}

// errorMessageLocked 须在已持有 r.mu 时调用。
func (r *UsageRecorder) errorMessageLocked() *string {
	if m := strings.TrimSpace(r.errMsg); m != "" {
		return &m
	}
	if m := strings.TrimSpace(r.traceRef); m != "" {
		return &m
	}
	return nil
}

func (r *UsageRecorder) MarkLogged() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logged = true
}

func (r *UsageRecorder) IsLogged() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.logged
}

func (r *UsageRecorder) SetOutcomeAttempt(attempt int) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if attempt > 0 {
		r.outcomeAttempt = attempt
	}
}

func (r *UsageRecorder) OutcomeAttempt() int {
	if r == nil {
		return 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.outcomeAttempt > 0 {
		return r.outcomeAttempt
	}
	return 1
}

// SaveFailoverAttempt 记录 failover 中间上游尝试；不触发 MarkLogged。
func (r *UsageRecorder) SaveFailoverAttempt(channelID int, upstream string, status int, errMsg *string, attempt int, _ time.Duration) {
	if r == nil || r.disabled {
		return
	}
	r.mu.Lock()
	model := r.model
	client := r.client
	userID := r.userID
	apiKeyID := r.apiKeyID
	requestID := r.requestID
	ip := r.ip
	start := r.start
	if channelID > 0 {
		r.channelID = channelID
	}
	if u := strings.TrimSpace(upstream); u != "" {
		r.upstream = u
	}
	up := r.upstream
	chID := r.channelID
	r.mu.Unlock()

	if model == "" {
		model = "gateway:unknown"
	}
	go saveUsage(client, userID, apiKeyID, chID, requestID, model, up,
		0, 0, 0, 0, ip, status, errMsg, time.Since(start), metaForFailoverAttempt(attempt))
}

// SaveOK 记录成功（或已产生计费用量）的调用，异步写库。
func (r *UsageRecorder) SaveOK(pt, ct, cachedPrompt int, cost int64, status int, errMsg *string) {
	if r == nil || r.disabled {
		return
	}
	r.MarkLogged()
	r.persist(pt, ct, cachedPrompt, cost, status, errMsg, metaForClientOutcome(r.OutcomeAttempt()))
}

// SaveFail 显式记录失败（如 WebSocket dial 失败）；异步写库。
func (r *UsageRecorder) SaveFail(status int, errMsg string, pt, ct int, cost int64) {
	if r == nil || r.disabled {
		return
	}
	r.MarkLogged()
	var em *string
	if errMsg != "" {
		em = &errMsg
	}
	r.persist(pt, ct, 0, cost, status, em, metaForClientOutcome(r.OutcomeAttempt()))
}

func (r *UsageRecorder) maybeFlushFailure(c *gin.Context) {
	if r == nil || r.disabled {
		return
	}
	r.mu.Lock()
	logged := r.logged
	model := r.model
	em := r.errorMessageLocked()
	channelID := r.channelID
	upstream := r.upstream
	r.mu.Unlock()

	if logged {
		return
	}
	status := c.Writer.Status()
	if status < 400 {
		return
	}
	r.MarkLogged()
	if model == "" {
		model = fallbackModel(c)
	}
	if detail := gatewayErrMsgFromContext(c); detail != nil {
		slog.Info("gateway usage denied",
			"request_id", r.requestID,
			"user_id", r.userID,
			"status", status,
			"detail", *detail,
		)
		wrapped := usagelogpublic.WrapInternal(*detail)
		em = &wrapped
	}
	go saveUsage(r.client, r.userID, r.apiKeyID, channelID, r.requestID, model, upstream,
		0, 0, 0, 0, r.ip, status, em, time.Since(r.start), metaForClientOutcome(r.OutcomeAttempt()))
}

func gatewayErrMsgFromContext(c *gin.Context) *string {
	v, ok := c.Get(middleware.CtxGatewayErrMsgKey)
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// noteGatewayLogicalModel 在选路失败等场景写入真实逻辑模型名（避免日志仅显示 route:/v1/...）。
func noteGatewayLogicalModel(c *gin.Context, model string) {
	if rec := UsageRecorderFrom(c); rec != nil {
		if m := strings.TrimSpace(model); m != "" {
			rec.SetModel(m)
		}
	}
}

func (r *UsageRecorder) persist(pt, ct, cachedPrompt int, cost int64, status int, errMsg *string, meta UsageLogMeta) {
	r.mu.Lock()
	model := r.model
	channelID := r.channelID
	upstream := r.upstream
	client := r.client
	userID := r.userID
	apiKeyID := r.apiKeyID
	requestID := r.requestID
	ip := r.ip
	start := r.start
	r.mu.Unlock()

	if model == "" {
		model = "gateway:unknown"
	}
	go saveUsage(client, userID, apiKeyID, channelID, requestID, model, upstream,
		pt, ct, cachedPrompt, cost, ip, status, errMsg, time.Since(start), meta)
}

func fallbackModel(c *gin.Context) string {
	if m := routingModelForStatelessResource(c); m != "" {
		return m
	}
	if p := strings.TrimSpace(c.FullPath()); p != "" {
		return "route:" + p
	}
	path := strings.TrimSpace(c.Request.URL.Path)
	if path != "" {
		return "route:" + path
	}
	return "gateway:unknown"
}

// RecordUsageOK 在 recorder 存在时用 SaveOK，否则回退为裸 saveUsage（测试/非 gate 路由）。
func RecordUsageOK(c *gin.Context, client *ent.Client, userID, apiKeyID, channelID int, requestID, model, upstream string, pt, ct, cachedPrompt int, cost int64, ip string, status int, errMsg *string, elapsed time.Duration) {
	RecordUsageOKWithAttempt(c, client, userID, apiKeyID, channelID, requestID, model, upstream, pt, ct, cachedPrompt, cost, ip, status, errMsg, elapsed, 0)
}

// RecordUsageOKWithAttempt 写入客户端最终结果；attempt<=0 时从 UsageRecorder 读取。
func RecordUsageOKWithAttempt(c *gin.Context, client *ent.Client, userID, apiKeyID, channelID int, requestID, model, upstream string, pt, ct, cachedPrompt int, cost int64, ip string, status int, errMsg *string, elapsed time.Duration, attempt int) {
	if rec := UsageRecorderFrom(c); rec != nil {
		rec.SetModel(model)
		rec.SetChannel(channelID, upstream)
		if attempt > 0 {
			rec.SetOutcomeAttempt(attempt)
		}
		rec.SaveOK(pt, ct, cachedPrompt, cost, status, errMsg)
		return
	}
	meta := metaForClientOutcome(attempt)
	if attempt < 1 {
		meta = defaultUsageLogMeta()
	}
	go saveUsage(client, userID, apiKeyID, channelID, requestID, model, upstream, pt, ct, cachedPrompt, cost, ip, status, errMsg, elapsed, meta)
}

// NoteUsageOutcomeAttempt 在 RecordUsageOK 前设置 failover 最终尝试序号。
func NoteUsageOutcomeAttempt(c *gin.Context, attempt int) {
	if rec := UsageRecorderFrom(c); rec != nil && attempt > 0 {
		rec.SetOutcomeAttempt(attempt)
	}
}

// noteUsageErrorFromContext 供 billing / OpenAI 错误写出路径挂接。
func noteUsageErrorFromContext(c *gin.Context, msg string) {
	if rec := UsageRecorderFrom(c); rec != nil {
		rec.NoteError(msg)
	}
}
