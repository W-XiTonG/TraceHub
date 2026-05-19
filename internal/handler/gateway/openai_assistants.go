package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"tracehub/internal/gateway/relay"
	"tracehub/internal/middleware"
	"tracehub/internal/pkg/metrics"

	"github.com/gin-gonic/gin"
)

// assistantsPickRoutingModel 优先使用 JSON 中的 model，否则使用 Query「model」或头 X-TraceHub-Model（与 Files 等资源一致）。
func assistantsPickRoutingModel(c *gin.Context, body []byte) (string, bool) {
	var m string
	if len(body) > 0 {
		m = extractJSONModelField(body)
	}
	if m != "" {
		return m, true
	}
	m = routingModelForStatelessResource(c)
	if m == "" {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "缺少用于选路的 model：请在 JSON 中提供 model，或使用 Query 参数 model=…，或设置请求头 X-TraceHub-Model")
		return "", false
	}
	return m, true
}

func parseJSONStreamFlag(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(body, &m) != nil {
		return false
	}
	raw, ok := m["stream"]
	if !ok {
		return false
	}
	var b bool
	_ = json.Unmarshal(raw, &b)
	return b
}

func assistantsForwardJSON(c *gin.Context, d ChatDeps, method, relPath string, body []byte, prePrompt, preComp int, patchModel, useLong bool) {
	model, ok := assistantsPickRoutingModel(c, body)
	if !ok {
		return
	}
	q := upstreamQueryStripModel(c)
	enterpriseForwardJSON(c, d, model, prePrompt, preComp, method, relPath, q, body, patchModel, useLong)
}

func assistantsForwardMaybeStreamPost(c *gin.Context, d ChatDeps, relPath string, body []byte, prePrompt, preComp int, useLong bool) {
	model, ok := assistantsPickRoutingModel(c, body)
	if !ok {
		return
	}
	q := upstreamQueryStripModel(c)
	if parseJSONStreamFlag(body) {
		enterpriseForwardPostSSE(c, d, model, prePrompt, preComp, relPath, q, body)
		return
	}
	enterpriseForwardJSON(c, d, model, prePrompt, preComp, http.MethodPost, relPath, q, body, true, useLong)
}

// --- Assistants ---

func OpenAIAssistantsCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "assistants", body, 256, 2048, true, false)
	}
}

func OpenAIAssistantsList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		assistantsForwardJSON(c, d, http.MethodGet, "assistants", nil, 64, 512, false, false)
	}
}

func OpenAIAssistantsRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("assistant_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid assistant id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodGet, "assistants/"+id, nil, 64, 512, false, false)
	}
}

func OpenAIAssistantsModify(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("assistant_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid assistant id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "assistants/"+id, body, 128, 1024, true, false)
	}
}

func OpenAIAssistantsDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("assistant_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid assistant id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodDelete, "assistants/"+id, nil, 32, 256, false, false)
	}
}

// --- Threads ---

func OpenAIThreadsCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "threads", body, 128, 1024, true, false)
	}
}

func OpenAIThreadsRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodGet, "threads/"+id, nil, 64, 512, false, false)
	}
}

func OpenAIThreadsModify(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "threads/"+id, body, 128, 1024, true, false)
	}
}

func OpenAIThreadsDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodDelete, "threads/"+id, nil, 32, 256, false, false)
	}
}

// --- Thread messages ---

func OpenAIThreadMessagesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(tid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "threads/" + tid + "/messages"
		assistantsForwardJSON(c, d, http.MethodPost, rel, body, 256, 1536, true, false)
	}
}

func OpenAIThreadMessagesList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(tid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		rel := "threads/" + tid + "/messages"
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, false)
	}
}

func OpenAIThreadMessagesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		mid := strings.TrimSpace(c.Param("message_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(mid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or message id")
			return
		}
		rel := "threads/" + tid + "/messages/" + mid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, false)
	}
}

func OpenAIThreadMessagesModify(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		mid := strings.TrimSpace(c.Param("message_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(mid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or message id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "threads/" + tid + "/messages/" + mid
		assistantsForwardJSON(c, d, http.MethodPost, rel, body, 128, 1024, true, false)
	}
}

// --- Thread runs ---

func OpenAIThreadRunsCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(tid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "threads/" + tid + "/runs"
		assistantsForwardMaybeStreamPost(c, d, rel, body, 512, 2048, true)
	}
}

func OpenAIThreadRunsList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		if !safeOpenAIResourceID(tid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread id")
			return
		}
		rel := "threads/" + tid + "/runs"
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 128, 1024, false, true)
	}
}

func OpenAIThreadRunsRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or run id")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 128, 1024, false, true)
	}
}

func OpenAIThreadRunsModify(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or run id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid
		assistantsForwardJSON(c, d, http.MethodPost, rel, body, 128, 1024, true, true)
	}
}

func OpenAIThreadRunsCancel(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or run id")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid + "/cancel"
		assistantsForwardJSON(c, d, http.MethodPost, rel, nil, 64, 512, false, true)
	}
}

func OpenAIThreadRunsSubmitToolOutputs(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or run id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid + "/submit_tool_outputs"
		assistantsForwardMaybeStreamPost(c, d, rel, body, 256, 2048, true)
	}
}

func OpenAIThreadRunStepsList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread or run id")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid + "/steps"
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 128, 1024, false, true)
	}
}

func OpenAIThreadRunStepsRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tid := strings.TrimSpace(c.Param("thread_id"))
		rid := strings.TrimSpace(c.Param("run_id"))
		sid := strings.TrimSpace(c.Param("step_id"))
		if !safeOpenAIResourceID(tid) || !safeOpenAIResourceID(rid) || !safeOpenAIResourceID(sid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid thread, run or step id")
			return
		}
		rel := "threads/" + tid + "/runs/" + rid + "/steps/" + sid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, true)
	}
}

// --- Vector stores ---

func OpenAIVectorStoresCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "vector_stores", body, 256, 2048, true, false)
	}
}

func OpenAIVectorStoresList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		assistantsForwardJSON(c, d, http.MethodGet, "vector_stores", nil, 64, 512, false, false)
	}
}

func OpenAIVectorStoresRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodGet, "vector_stores/"+id, nil, 64, 512, false, false)
	}
}

func OpenAIVectorStoresModify(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		assistantsForwardJSON(c, d, http.MethodPost, "vector_stores/"+id, body, 128, 1024, true, false)
	}
}

func OpenAIVectorStoresDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(id) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		assistantsForwardJSON(c, d, http.MethodDelete, "vector_stores/"+id, nil, 32, 256, false, false)
	}
}

func OpenAIVectorStoreFilesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(vsID) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		ctHdr := strings.ToLower(c.GetHeader("Content-Type"))
		if strings.HasPrefix(ctHdr, "multipart/form-data") {
			openAIVectorStoreFilesCreateMultipart(c, d, vsID)
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "vector_stores/" + vsID + "/files"
		assistantsForwardJSON(c, d, http.MethodPost, rel, body, 256, 2048, true, false)
	}
}

func openAIVectorStoreFilesCreateMultipart(c *gin.Context, d ChatDeps, vsID string) {
	if err := c.Request.ParseMultipartForm(maxOpenAIFileMultipartMemory); err != nil {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid multipart body")
		return
	}
	form := c.Request.MultipartForm
	defer func() {
		if form != nil {
			_ = form.RemoveAll()
		}
	}()
	if len(form.File["file"]) == 0 {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "missing file")
		return
	}
	model, ok := enterpriseRoutingModel(c, nil, false)
	if !ok {
		return
	}
	var totalFile int64
	for _, fhs := range form.File {
		for _, fh := range fhs {
			totalFile += fh.Size
		}
	}
	imgTok := int(totalFile / 256)
	if imgTok < 128 {
		imgTok = 128
	}
	u, k, sel, secret, baseURL, ct, ok2, br := enterpriseResolveUpstream(c, d, model, imgTok, 2048)
	if !ok2 {
		return
	}
	ch := sel.Channel
	body, contentType, err := rebuildMultipartWithUpstreamModel(form, sel.UpstreamModel)
	if err != nil {
		middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法重组 multipart 请求")
		return
	}
	rel := "vector_stores/" + vsID + "/files"
	reqID, _ := c.Get("request_id")
	rid, _ := reqID.(string)
	start := time.Now()
	resp, err := relay.ForwardOpenAIMultipart(c.Request.Context(), baseURL, secret, rel, contentType, body, openAIPassthroughHeaders(c), "")
	if err != nil {
		metrics.GatewayRequests.WithLabelValues(model, ct, "transport_err").Inc()
		WriteGatewayTransportError(c, err)
		return
	}
	defer relay.DrainBody(resp.Body)
	metrics.GatewayLatency.WithLabelValues(ct).Observe(time.Since(start).Seconds())
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		metrics.GatewayRequests.WithLabelValues(model, ct, http.StatusText(resp.StatusCode)).Inc()
		logUpstreamQuotaDeclineIfLikely(c, resp.StatusCode, raw)
		WriteClientProxy(c, resp.StatusCode, "application/json", raw)
		return
	}
		usage := parseUsageFlexible(raw)
		if usage.PromptTokens == 0 {
			usage.PromptTokens = imgTok
		}
		applyCompletionFallback(&usage, 2048)
		cost := GatewayFinalCost(br,usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens)
		if err := gatewayDeduct(c, d.Billing, int64(u.ID), cost, false); err != nil {
			return
		}
		metrics.GatewayRequests.WithLabelValues(model, ct, "200").Inc()
		RecordUsageOK(c, d.Client, u.ID, k.ID, ch.ID, rid, model, sel.UpstreamModel, usage.PromptTokens, usage.CompletionTokens, usage.CachedPromptTokens, cost, c.ClientIP(), int(resp.StatusCode), nil, time.Since(start))
	WriteClientProxy(c, resp.StatusCode, "application/json", raw)
}

func OpenAIVectorStoreFilesList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(vsID) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		rel := "vector_stores/" + vsID + "/files"
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, false)
	}
}

func OpenAIVectorStoreFilesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		fid := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(fid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store or file id")
			return
		}
		rel := "vector_stores/" + vsID + "/files/" + fid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, false)
	}
}

func OpenAIVectorStoreFilesDelete(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		fid := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(fid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store or file id")
			return
		}
		rel := "vector_stores/" + vsID + "/files/" + fid
		assistantsForwardJSON(c, d, http.MethodDelete, rel, nil, 32, 256, false, false)
	}
}

func OpenAIVectorStoreFileBatchesCreate(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		if !safeOpenAIResourceID(vsID) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store id")
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "无法读取请求体")
			return
		}
		rel := "vector_stores/" + vsID + "/file_batches"
		assistantsForwardJSON(c, d, http.MethodPost, rel, body, 256, 2048, true, false)
	}
}

func OpenAIVectorStoreFileBatchesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		bid := strings.TrimSpace(c.Param("batch_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(bid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store or batch id")
			return
		}
		rel := "vector_stores/" + vsID + "/file_batches/" + bid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 128, 1024, false, true)
	}
}

func OpenAIVectorStoreFileBatchesCancel(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		bid := strings.TrimSpace(c.Param("batch_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(bid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store or batch id")
			return
		}
		rel := "vector_stores/" + vsID + "/file_batches/" + bid + "/cancel"
		assistantsForwardJSON(c, d, http.MethodPost, rel, nil, 64, 512, false, true)
	}
}

func OpenAIVectorStoreFileBatchFilesList(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		bid := strings.TrimSpace(c.Param("batch_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(bid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid vector store or batch id")
			return
		}
		rel := "vector_stores/" + vsID + "/file_batches/" + bid + "/files"
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, true)
	}
}

func OpenAIVectorStoreFileBatchFilesRetrieve(d ChatDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		vsID := strings.TrimSpace(c.Param("vector_store_id"))
		bid := strings.TrimSpace(c.Param("batch_id"))
		fid := strings.TrimSpace(c.Param("file_id"))
		if !safeOpenAIResourceID(vsID) || !safeOpenAIResourceID(bid) || !safeOpenAIResourceID(fid) {
			middleware.WriteOpenAIError(c, 400, "invalid_request_error", "invalid id")
			return
		}
		rel := "vector_stores/" + vsID + "/file_batches/" + bid + "/files/" + fid
		assistantsForwardJSON(c, d, http.MethodGet, rel, nil, 64, 512, false, true)
	}
}
