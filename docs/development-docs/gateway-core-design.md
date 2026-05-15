# TraceHub 网关核心设计文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**参考项目**：Sub2API（核心设计直接参考）

---

## 一、网关核心整体架构

网关核心是 TraceHub 的最关键模块，负责将客户端请求透明转发到各 AI 上游，并完成认证、限流、路由、计费、日志等全流程处理。

```
┌─────────────────────────────────────────────────────────────────────┐
│                      网关核心处理流程                              │
│                                                                 │
│  Request → Auth → RateLimit → Router → Relay → Billing → Response │
│                        ↓                                          │
│                   UsageLog (Async Write)                         │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 二、智能路由调度设计

> **[sub2api]** 路由策略完全参考 Sub2API，支持粘性会话、优先级/权重路由、故障转移、混合调度。

### 2.1 路由选择优先级

```
1. 强制平台上下文（专用路由前缀，如 /antigravity）
        ↓（未匹配）
2. API Key 所属分组的平台配置（group.platform）
        ↓（未配置）
3. 默认平台兜底（默认 Gemini，或自动检测）
```

### 2.2 账号选择算法

> **[sub2api]** 参考 `backend/internal/service/` 下的账号选择逻辑。

```go
// internal/gateway/router/selector.go

type Selector struct {
    channelRepo *repository.ChannelRepository
    redis       *cache.RedisClient
}

// SelectChannel 为一次请求选择最优上游渠道
func (s *Selector) SelectChannel(ctx *RelayContext) (*ent.Channel, error) {
    // 1. 获取候选渠道列表（按 model + group 过滤）
    candidates, err := s.getCandidates(ctx)
    if err != nil {
        return nil, fmt.Errorf("no available channel for model: %s", ctx.Request.Model)
    }
    
    // 2. 粘性会话检查（Sticky Session）
    if stickyChannel := s.tryStickySession(ctx); stickyChannel != nil {
        return stickyChannel, nil
    }
    
    // 3. 优先级 + 权重筛选
    candidates = s.filterByPriorityAndWeight(candidates)
    
    // 4. 健康状态过滤（排除 health_status=2 的渠道）
    candidates = s.filterByHealth(candidates)
    
    // 5. 并发容量过滤（排除达到并发上限的渠道）
    candidates = s.filterByConcurrency(ctx, candidates)
    
    // 6. 负载因子排序（load_factor 低的优先）
    sort.Slice(candidates, func(i, j int) bool {
        return candidates[i].LoadFactor < candidates[j].LoadFactor
    })
    
    // 7. 按权重随机选择一个
    selected := s.weightedRandom(candidates)
    
    // 8. 设置粘性会话（本次请求结束后绑定）
    s.setStickySession(ctx, selected)
    
    return selected, nil
}

// tryStickySession 检查粘性会话
func (s *Selector) tryStickySession(ctx *RelayContext) *ent.Channel {
    sessionKey := ctx.GetStickySessionKey()
    if sessionKey == "" {
        return nil
    }
    
    channelID, err := s.redis.Get(context.Background(), "sticky_session:"+sessionKey)
    if err != nil {
        return nil
    }
    
    // 验证该渠道仍然健康且可用
    ch, err := s.channelRepo.GetByID(ctx.Context, channelID)
    if err != nil || !ch.Enabled || ch.HealthStatus == 2 {
        s.redis.Del(context.Background(), "sticky_session:"+sessionKey)
        return nil
    }
    
    return ch
}
```

### 2.3 粘性会话管理

> **[sub2api]** 参考 Sub2API 的跨平台粘性会话管理，不同平台使用不同键前缀。

```go
// internal/gateway/router/sticky_session.go

type StickySessionManager struct {
    redis *cache.RedisClient
}

// GetStickySessionKey 根据平台和请求内容生成会话键
func (m *StickySessionManager) GetStickySessionKey(ctx *RelayContext) string {
    platform := ctx.GetPlatform()
    
    switch platform {
    case PlatformAnthropic:
        // Claude：基于请求内容摘要链哈希
        hash := m.hashMessages(ctx.Request.Messages)
        return fmt.Sprintf("sticky_session:claude:%s", hash)
    case PlatformOpenAI:
        // OpenAI：优先使用请求头中的 session_id
        if sessionID := ctx.GetHeader("X-Session-ID"); sessionID != "" {
            return fmt.Sprintf("sticky_session:openai:%s", sessionID)
        }
        return fmt.Sprintf("sticky_session:openai:%s", ctx.RequestID)
    case PlatformGemini:
        hash := m.hashMessages(ctx.Request.Messages)
        return fmt.Sprintf("sticky_session:gemini:%s", hash)
    case PlatformAntigravity:
        hash := m.hashMessages(ctx.Request.Messages)
        return fmt.Sprintf("sticky_session:antigravity:%s", hash)
    default:
        return fmt.Sprintf("sticky_session:default:%s", ctx.RequestID)
    }
}

// SetStickySession 绑定会话到渠道（TTL 1小时）
func (m *StickySessionManager) SetStickySession(key string, channelID int64) {
    m.redis.Set(context.Background(), key, channelID, 1*time.Hour)
}
```

### 2.4 健康检查机制

> **[sub2api]** 参考 Sub2API 的 `TokenRefreshService` 和健康检查逻辑。

```go
// internal/gateway/router/health_check.go

type HealthChecker struct {
    channelRepo *repository.ChannelRepository
    redis      *cache.RedisClient
}

// CheckChannel 检查单个渠道健康状态
func (h *HealthChecker) CheckChannel(ch *ent.Channel) HealthResult {
    // 构造轻量级探针请求
    probeReq := h.buildProbeRequest(ch)
    
    // 发送请求（设置短超时）
    resp, err := h.sendProbe(ch, probeReq)
    if err != nil {
        h.recordFailure(ch.ID)
        return HealthResult{Healthy: false, Error: err.Error()}
    }
    defer resp.Body.Close()
    
    // 解析响应
    if resp.StatusCode >= 200 && resp.StatusCode < 500 {
        h.recordSuccess(ch.ID)
        return HealthResult{Healthy: true}
    }
    
    h.recordFailure(ch.ID)
    return HealthResult{Healthy: false, Error: fmt.Sprintf("status: %d", resp.StatusCode)}
}

// recordFailure 记录失败，连续失败 N 次后自动禁用
func (h *HealthChecker) recordFailure(channelID int64) {
    key := fmt.Sprintf("health_failure_count:%d", channelID)
    count, _ := h.redis.Incr(context.Background(), key)
    if count >= 3 {
        h.channelRepo.UpdateHealthStatus(context.Background(), channelID, 2) // 2=异常
    }
}

// recordSuccess 记录成功，恢复后自动启用
func (h *HealthChecker) recordSuccess(channelID int64) {
    key := fmt.Sprintf("health_failure_count:%d", channelID)
    h.redis.Del(context.Background(), key)
    h.channelRepo.UpdateHealthStatus(context.Background(), channelID, 1) // 1=健康
}
```

---

## 三、Relay 适配器设计

> **[sub2api]** 核心设计直接参考 Sub2API 的 `backend/internal/gateway/` 目录结构，每个平台有独立的适配器。

### 3.1 适配器接口定义

```go
// internal/gateway/relay/adapter.go

type RelayAdapter interface {
    // Relay 执行完整的转发流程
    Relay(ctx *RelayContext) (*http.Response, error)
    
    // ConvertRequest 将 OpenAI 标准请求转换为上游格式
    ConvertRequest(ctx *RelayContext) (interface{}, error)
    
    // ConvertResponse 将上游响应转换为 OpenAI 标准格式
    ConvertResponse(ctx *RelayContext, upstreamResp interface{}) (*OpenAIResponse, error)
    
    // HandleStream 处理 SSE 流式响应
    HandleStream(ctx *RelayContext) (<-chan string, <-chan error, error)
}
```

### 3.2 适配器工厂

```go
// internal/gateway/relay/relay_factory.go

type Factory struct {
    adapters map[ChannelType]RelayAdapter
}

func NewFactory() *Factory {
    return &Factory{
        adapters: make(map[ChannelType]RelayAdapter),
    }
}

func (f *Factory) Register(channelType ChannelType, adapter RelayAdapter) {
    f.adapters[channelType] = adapter
}

func (f *Factory) GetAdapter(channelType ChannelType) (RelayAdapter, error) {
    adapter, ok := f.adapters[channelType]
    if !ok {
        return nil, fmt.Errorf("unsupported channel type: %s", channelType)
    }
    return adapter, nil
}

// 初始化所有适配器（Wire 注入时调用）
func (f *Factory) InitAll(
    openaiAdapter *OpenAIAdapter,
    claudeAdapter *ClaudeAdapter,
    geminiAdapter *GeminiAdapter,
    deepseekAdapter *DeepSeekAdapter,
    antigravityAdapter *AntigravityAdapter,
) {
    f.Register(ChannelTypeOpenAI, openaiAdapter)
    f.Register(ChannelTypeClaude, claudeAdapter)
    f.Register(ChannelTypeGemini, geminiAdapter)
    f.Register(ChannelTypeDeepSeek, deepseekAdapter)
    f.Register(ChannelTypeAntigravity, antigravityAdapter)
}
```

### 3.3 Claude 适配器（核心参考 sub2api）

> **[sub2api]** 参考 Sub2API 的 `ClaudeGatewayService`，原生支持 Claude Messages API 格式。

```go
// internal/gateway/relay/claude/adapter.go

type ClaudeAdapter struct {
    httpClient *http.Client
    tokenizer *Tokenizer
}

// ConvertRequest 将 OpenAI 格式转换为 Claude Messages API 格式
func (a *ClaudeAdapter) ConvertRequest(ctx *RelayContext) (interface{}, error) {
    req := ctx.Request
    
    // 构建 Claude Messages API 请求
    claudeReq := &ClaudeRequest{
        Model:     mapModelToClaude(req.Model),
        MaxTokens: *req.MaxTokens,
        Stream:    req.Stream,
    }
    
    // 转换 Messages → Claude Messages 格式
    for _, msg := range req.Messages {
        claudeMsg := ClaudeMessage{
            Role:    mapRole(msg.Role),
            Content: a.convertContent(msg.Content),
        }
        claudeReq.Messages = append(claudeReq.Messages, claudeMsg)
    }
    
    // 转换 Tools → Claude Tool 格式
    if len(req.Tools) > 0 {
        for _, tool := range req.Tools {
            claudeReq.Tools = append(claudeReq.Tools, a.convertTool(tool))
        }
    }
    
    // 处理 System Message
    claudeReq.System = extractSystemMessage(req.Messages)
    
    return claudeReq, nil
}

// Relay 执行转发
func (a *ClaudeAdapter) Relay(ctx *RelayContext) (*http.Response, error) {
    channel := ctx.Channel
    
    // 构建上游请求
    upstreamReq, err := a.ConvertRequest(ctx)
    if err != nil {
        return nil, err
    }
    
    // 确定上游 URL
    baseURL := channel.ApiBaseURL
    if baseURL == "" {
        baseURL = "https://api.anthropic.com"
    }
    
    url := baseURL + "/v1/messages"
    
    // 构建 HTTP 请求
    body, _ := json.Marshal(upstreamReq)
    req, _ := http.NewRequestWithContext(ctx.Context, "POST", url, bytes.NewReader(body))
    
    // 设置认证头
    if channel.AuthType == "api_key" {
        req.Header.Set("x-api-key", decryptAPIKey(channel.ApiKey))
        req.Header.Set("anthropic-version", "2023-06-01")
    } else if channel.AuthType == "oauth" {
        token := getOAuthToken(channel)
        req.Header.Set("Authorization", "Bearer "+token)
    }
    
    // 发送请求
    resp, err := a.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("upstream request failed: %w", err)
    }
    
    return resp, nil
}

// HandleStream 处理 Claude SSE 流式响应
func (a *ClaudeAdapter) HandleStream(ctx *RelayContext) (<-chan string, <-chan error, error) {
    respChan := make(chan string)
    errChan := make(chan error)
    
    go func() {
        defer close(respChan)
        defer close(errChan)
        
        resp := ctx.UpstreamResp
        defer resp.Body.Close()
        
        scanner := bufio.NewScanner(resp.Body)
        for scanner.Scan() {
            line := scanner.Text()
            if !strings.HasPrefix(line, "data: ") {
                continue
            }
            
            data := strings.TrimPrefix(line, "data: ")
            if data == "[DONE]" {
                respChan <- "data: [DONE]"
                return
            }
            
            // 将 Claude SSE 格式转换为 OpenAI SSE 格式
            converted, err := a.convertSSEEvent(data)
            if err != nil {
                errChan <- err
                return
            }
            respChan <- "data: " + converted
        }
    }()
    
    return respChan, errChan, nil
}
```

### 3.4 OpenAI 适配器

> **[sub2api]** 参考 Sub2API 的 `OpenAIGatewayService`，支持 OAuth 和 API Key 两类认证。

```go
// internal/gateway/relay/openai/adapter.go

type OpenAIAdapter struct {
    httpClient *http.Client
}

// Relay 执行转发（OpenAI 格式与原生格式一致，转换开销最小）
func (a *OpenAIAdapter) Relay(ctx *RelayContext) (*http.Response, error) {
    channel := ctx.Channel
    
    baseURL := channel.ApiBaseURL
    if baseURL == "" {
        // 根据 auth_type 选择端点
        if channel.AuthType == "oauth" {
            baseURL = "https://chatgpt.com/backend-api/codex/responses"
        } else {
            baseURL = "https://api.openai.com/v1"
        }
    }
    
    url := baseURL + "/chat/completions"
    
    body, _ := json.Marshal(ctx.Request)
    req, _ := http.NewRequestWithContext(ctx.Context, "POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    // 认证
    if channel.AuthType == "api_key" {
        req.Header.Set("Authorization", "Bearer "+decryptAPIKey(channel.ApiKey))
    } else if channel.AuthType == "oauth" {
        token := getOAuthToken(channel)
        req.Header.Set("Authorization", "Bearer "+token)
        
        // 解析上游响应头中的配额信息（Codex 场景）
        // 如：x-ratelimit-remaining-requests / x-ratelimit-remaining-tokens
    }
    
    return a.httpClient.Do(req)
}
```

### 3.5 Gemini 适配器（重点参考 sub2api）

> **[sub2api]** 参考 Sub2API 的 `GeminiMessagesCompatService`，实现 Claude Messages API → Gemini REST API 格式转换。

```go
// internal/gateway/relay/gemini/adapter.go

type GeminiAdapter struct {
    httpClient *http.Client
}

// ConvertRequest 将 OpenAI/Claude 格式转换为 Gemini API 格式
func (a *GeminiAdapter) ConvertRequest(ctx *RelayContext) (interface{}, error) {
    req := ctx.Request
    
    geminiReq := &GeminiRequest{
        Contents: a.convertMessages(req.Messages),
    }
    
    // 注入 thoughtSignature（工具调用兼容性）
    if len(req.Tools) > 0 {
        geminiReq.GenerationConfig.ThinkingConfig = &ThinkingConfig{
            IncludeThoughts: true,
        }
    }
    
    // 处理网页搜索等功能保留
    geminiReq.Tools = a.convertTools(req.Tools)
    
    return geminiReq, nil
}

// Relay 执行转发
func (a *GeminiAdapter) Relay(ctx *RelayContext) (*http.Response, error) {
    channel := ctx.Channel
    
    baseURL := channel.ApiBaseURL
    if baseURL == "" {
        baseURL = "https://generativelanguage.googleapis.com"
    }
    
    model := mapModelToGemini(ctx.Request.Model)
    url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", baseURL, model)
    
    // Gemini 使用 API Key 作为 URL 参数
    if channel.AuthType == "api_key" {
        url += "?key=" + decryptAPIKey(channel.ApiKey)
    }
    
    body, _ := json.Marshal(upstreamReq)
    req, _ := http.NewRequestWithContext(ctx.Context, "POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    return a.httpClient.Do(req)
}
```

---

## 四、计费引擎设计

> **[sub2api]** 计费引擎完全参考 Sub2API 的 `BillingService` 和 `BillingCacheService` 设计。

### 4.1 计费流程

```
请求开始
    │
    ▼
预扣费（基于输入 Token 预估）
    │
    ▼
调用上游 API
    │
    ▼
统计实际 Token 使用量（上游响应）
    │
    ▼
计算实际费用（多级倍率）
    │
    ▼
修正预扣费（多退少补）
    │
    ▼
写入 UsageLog（异步）
```

### 4.2 多级倍率计算

> **[sub2api]** 参考 Sub2API 的 `model_pricing_resolver.go`，层级乘数计算最终扣费。

```go
// internal/gateway/billing/engine.go

type Engine struct {
    tokenizer *Tokenizer
    cache     *cache.BillingCache
}

// CalculateCost 计算请求的实际费用
func (e *Engine) CalculateCost(ctx *RelayContext, promptTokens, completionTokens int) (int64, error) {
    // 1. 获取基准价格（从 LiteLLM 官方定价表或本地配置）
    inputPrice, outputPrice, err := e.getModelPrice(ctx.ModelUpstream)
    if err != nil {
        return 0, err
    }
    
    // 2. 分组级倍率
    groupRate := ctx.Group.RateMultiplier
    
    // 3. 模型级倍率（分组配置）
    var modelInputRate, modelOutputRate decimal.Decimal
    modelRate := e.getGroupModelRate(ctx.Group.ID, ctx.ModelUpstream)
    if modelRate != nil {
        modelInputRate = modelRate.InputRate
        modelOutputRate = modelRate.OutputRate
    } else {
        modelInputRate = decimal.NewFromFloat(1.0)
        modelOutputRate = decimal.NewFromFloat(1.0)
    }
    
    // 4. 账号级成本乘数
    accountMultiplier := decimal.NewFromFloat(ctx.Channel.CostMultiplier)
    
    // 5. 管理员自定义分组费率（用户级覆盖）
    userRate := decimal.NewFromFloat(1.0)
    if ctx.User.GroupRates != nil {
        if r, ok := ctx.User.GroupRates[ctx.ModelUpstream]; ok {
            userRate = decimal.NewFromFloat(r)
        }
    }
    
    // 6. 计算最终费用（额度单位）
    inputCost := inputPrice.
        Mul(decimal.NewFromInt(int64(promptTokens))).
        Mul(groupRate).
        Mul(modelInputRate).
        Mul(accountMultiplier).
        Mul(userRate)
    
    outputCost := outputPrice.
        Mul(decimal.NewFromInt(int64(completionTokens))).
        Mul(groupRate).
        Mul(modelOutputRate).
        Mul(accountMultiplier).
        Mul(userRate)
    
    totalCost := inputCost.Add(outputCost)
    
    return int64(totalCost.InexactFloat64()), nil
}

// PreDeduct 预扣费（基于输入 Token 预估）
func (e *Engine) PreDeduct(ctx *RelayContext) error {
    // 预估输入 Token
    promptTokens := e.tokenizer.CountTokens(ctx.Request.Model, ctx.Request.Messages)
    
    // 预估最大输出 Token
    maxTokens := 1024
    if ctx.Request.MaxTokens != nil {
        maxTokens = *ctx.Request.MaxTokens
    }
    
    // 预估费用
    estimatedCost, _ := e.CalculateCost(ctx, promptTokens, maxTokens)
    
    // 检查余额
    balance := e.cache.GetBalance(ctx.User.ID)
    if balance < estimatedCost {
        return fmt.Errorf("insufficient balance: have %d, need %d", balance, estimatedCost)
    }
    
    // 预扣费
    err := e.cache.PreDeduct(ctx.User.ID, ctx.APIKey.ID, estimatedCost)
    if err != nil {
        return err
    }
    
    ctx.PreCost = estimatedCost
    return nil
}

// Finalize 修正预扣费（多退少补）
func (e *Engine) Finalize(ctx *RelayContext) error {
    actualCost := ctx.ActualCost
    
    // 多退少补
    delta := ctx.PreCost - actualCost
    if delta > 0 {
        // 实际费用少于预估，退还差额
        e.cache.Refund(ctx.User.ID, ctx.APIKey.ID, delta)
    } else if delta < 0 {
        // 实际费用超出预估，扣除差额
        e.cache.Deduct(ctx.User.ID, ctx.APIKey.ID, -delta)
    }
    
    // 更新数据库余额（异步）
    go e.updateUserBalance(ctx.User.ID, actualCost)
    
    return nil
}
```

### 4.2.1 模型不计费（全局列表 + 分组 `billable`，分组优先）

现行实现位于 `internal/service/billing/engine.go`（`ModelBillRates`、`Cost`、`modelBillable` 等），与网关 `gatewayDeduct`（`cost <= 0` 跳过扣费）配合。

| 条件 | 行为 |
|:---|:---|
| 分组存在 `group_model_rates` 行且 `billable = false` | 该用户在该分组下对该逻辑模型 **不计费**（`Cost` 为 0）。 |
| 分组存在行且 `billable = true` | **正常计费**；即使模型出现在全局免计费列表中也仍计费（分组优先）。 |
| 分组无对应行 | 若逻辑模型（含定价表别名等候选名）命中全局列表，则 **不计费**；否则按内置单价与倍率计费。 |

- **全局列表**：`internal/pkg/config/config.go` 中 `site.no_bill_models`（YAML）与 `system_configs` 键 `site.no_bill_models`（JSON 字符串数组）；库内非空配置优先于 YAML。管理后台「系统配置」页可编辑该列表；更新后计费引擎会失效内存缓存（短 TTL）。
- **`enabled` 与 `billable` 区别**：`enabled=false` 仅表示不采用该行自定义输入/输出倍率，会回退默认定价，**不表示免计费**；免计费须 `billable=false` 或命中全局列表。
- **用量日志**：不计费时仍写入 `usage_logs`，`total_cost=0`（与既有零费用路径一致）。

### 4.3 计费异常熔断

> **[sub2api]** 参考 Sub2API 的 `billing.circuit_breaker` 配置，异常时 fail-closed。

```go
// internal/gateway/billing/circuit_breaker.go

type CircuitBreaker struct {
    redis *cache.RedisClient
    threshold float64  // 异常费用阈值
}

// CheckAbnormalBilling 检查是否存在异常计费（防止上游返回异常 Token 数）
func (cb *CircuitBreaker) CheckAbnormalBilling(ctx *RelayContext) error {
    // 单次请求费用异常检查
    if float64(ctx.ActualCost) > cb.threshold {
        // 记录异常，触发熔断
        cb.redis.Incr(context.Background(), "billing_abnormal_count")
        return fmt.Errorf("abnormal billing detected: cost=%d, threshold=%.2f", 
            ctx.ActualCost, cb.threshold)
    }
    
    // 用户短时间内费用激增检查
    // ...
    
    return nil
}
```

### 4.4 多模态计费

> **[sub2api]** 参考 Sub2API 对图片/Tile/Patch 的计费逻辑。

```go
// internal/gateway/billing/multimodal.go

// CountMultimodalTokens 计算多模态输入的 Token 数
func (e *Engine) CountMultimodalTokens(model string, messages []Message) int {
    total := 0
    for _, msg := range messages {
        switch content := msg.Content.(type) {
        case string:
            total += e.tokenizer.CountTokens(model, content)
        case []ContentPart:
            for _, part := range content {
                if part.Type == "text" {
                    total += e.tokenizer.CountTokens(model, part.Text)
                } else if part.Type == "image_url" {
                    // 图片按 Tile/Patch 计费
                    tiles := calculateImageTiles(part.ImageURL.URL, part.ImageURL.Detail)
                    total += tiles * getImageTokenCost(model)
                }
            }
        }
    }
    return total
}

// calculateImageTiles 计算图片的 Tile 数量（参考 OpenAI 计费规则）
func calculateImageTiles(imageURL string, detail string) int {
    // low 细节：固定 85 tokens
    if detail == "low" {
        return 85
    }
    
    // high 细节：按 512x512 tile 计算
    // 每张图片最多 1105 tokens
    // ...
    return 0
}
```

---

## 五、限流器设计

> **[sub2api]** 参考 Sub2API 的三层限流体系：用户级 + API Key 级 + 渠道级 + 全局级。

### 5.1 限流中间件（Gin 中间件）

```go
// internal/handler/middleware/ratelimit.go

func RateLimitMiddleware(limiter *ratelimit.Limiter) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. 全局限流
        if !limiter.GlobalAllow() {
            c.JSON(429, OpenAIErrorResponse{
                Error: &OpenAIError{
                    Message: "global rate limit exceeded",
                    Type:    "rate_limit_error",
                    Code:    "rate_limit_exceeded",
                },
            })
            c.Abort()
            return
        }
        
        // 2. 用户级限流（从 context 中获取用户信息）
        user, exists := c.Get("user")
        if exists {
            if !limiter.UserAllow(user.(*ent.User).ID) {
                c.JSON(429, OpenAIErrorResponse{...})
                c.Abort()
                return
            }
        }
        
        // 3. API Key 级限流
        apiKey, exists := c.Get("api_key")
        if exists {
            if !limiter.KeyAllow(apiKey.(*ent.APIKey).ID) {
                c.JSON(429, OpenAIErrorResponse{...})
                c.Abort()
                return
            }
        }
        
        c.Next()
    }
}
```

### 5.2 用户级限流器

```go
// internal/gateway/ratelimit/user_limiter.go

type UserLimiter struct {
    redis *cache.RedisClient
}

// UserAllow 检查用户是否允许发起请求（多窗口限流）
func (l *UserLimiter) UserAllow(userID int64) bool {
    now := time.Now()
    
    // 5 小时窗口
    window5h := now.Format("2006-01-02-15") // 每 5 小时一个窗口
    key5h := fmt.Sprintf("rate_limit:user:%d:5h:%s", userID, window5h)
    count5h, _ := l.redis.Incr(context.Background(), key5h)
    l.redis.Expire(context.Background(), key5h, 5*time.Hour)
    if count5h > getUserLimit5h(userID) {
        return false
    }
    
    // 1 天窗口
    // ...
    
    // 7 天窗口
    // ...
    
    return true
}
```

### 5.3 渠道级限流器

```go
// internal/gateway/ratelimit/channel_limiter.go

type ChannelLimiter struct {
    redis *cache.RedisClient
}

// ChannelAllow 检查渠道是否达到并发/QPS 上限
func (l *ChannelLimiter) ChannelAllow(channelID int64, concurrency int) bool {
    key := fmt.Sprintf("concurrency:channel:%d", channelID)
    
    current, _ := l.redis.Get(context.Background(), key)
    currentVal, _ := strconv.Atoi(current)
    
    if currentVal >= concurrency {
        return false  // 达到并发上限
    }
    
    // 获取并发槽
    l.redis.Incr(context.Background(), key)
    return true
}

// ReleaseChannelConcurrency 释放渠道并发槽
func (l *ChannelLimiter) ReleaseChannelConcurrency(channelID int64) {
    key := fmt.Sprintf("concurrency:channel:%d", channelID)
    l.redis.Decr(context.Background(), key)
}
```

### 5.4 并发槽管理

> **[sub2api]** 参考 Sub2API 的 `ConcurrencyService`，使用 Redis 信号量实现分布式并发控制。

```go
// internal/gateway/ratelimit/concurrency.go

type ConcurrencyManager struct {
    redis *cache.RedisClient
}

// AcquireUserSlot 获取用户并发槽
func (m *ConcurrencyManager) AcquireUserSlot(userID int64, maxConcurrency int) (bool, error) {
    key := fmt.Sprintf("concurrency:user:%d", userID)
    current, err := m.redis.Incr(context.Background(), key)
    if err != nil {
        return false, err
    }
    
    if int(current) > maxConcurrency {
        m.redis.Decr(context.Background(), key) // 回滚
        return false, nil
    }
    
    // 设置过期时间（防止死锁）
    m.redis.Expire(context.Background(), key, 10*time.Minute)
    return true, nil
}

// ReleaseUserSlot 释放用户并发槽
func (m *ConcurrencyManager) ReleaseUserSlot(userID int64) {
    key := fmt.Sprintf("concurrency:user:%d", userID)
    m.redis.Decr(context.Background(), key)
}
```

---

## 六、Token 统计器设计

> **[sub2api]** 参考 Sub2API 使用 `tiktoken` 进行 Token 统计，支持多模态。

```go
// internal/gateway/billing/tokenizer.go

import "github.com/pkoukk/tiktoken-go"

type Tokenizer struct {
    encoders map[string]*tiktoken.Tiktoken
    mu       sync.RWMutex
}

// GetEncoder 获取指定模型的 tiktoken 编码器
func (t *Tokenizer) GetEncoder(model string) (*tiktoken.Tiktoken, error) {
    t.mu.RLock()
    enc, ok := t.encoders[model]
    t.mu.RUnlock()
    
    if ok {
        return enc, nil
    }
    
    t.mu.Lock()
    defer t.mu.Unlock()
    
    // 根据模型名称选择对应的编码器
    var encoding string
    if strings.HasPrefix(model, "gpt-4") {
        encoding = "cl100k_base"
    } else if strings.HasPrefix(model, "gpt-3.5") {
        encoding = "cl100k_base"
    } else if strings.HasPrefix(model, "claude") {
        encoding = "cl100k_base"  // Claude 使用近似编码
    } else if strings.HasPrefix(model, "gemini") {
        encoding = "cl100k_base"  // Gemini 使用近似编码
    } else {
        encoding = "cl100k_base"  // 默认
    }
    
    enc, err := tiktoken.GetEncoding(encoding)
    if err != nil {
        return nil, err
    }
    
    t.encoders[model] = enc
    return enc, nil
}

// CountTokens 统计消息列表的 Token 数
func (t *Tokenizer) CountTokens(model string, messages []Message) int {
    enc, err := t.GetEncoder(model)
    if err != nil {
        // 降级：按字符数估算（1 Token ≈ 4 字符）
        return estimateByChars(messages)
    }
    
    total := 0
    for _, msg := range messages {
        switch content := msg.Content.(type) {
        case string:
            tokens := enc.Encode(content, nil, nil)
            total += len(tokens)
        case []ContentPart:
            for _, part := range content {
                if part.Type == "text" {
                    tokens := enc.Encode(part.Text, nil, nil)
                    total += len(tokens)
                }
                // 图片不通过 tiktoken 统计，由上游返回或按规则估算
            }
        }
    }
    
    return total
}

// CountTokensFromResponse 从上游响应中解析实际 Token 使用量
func (t *Tokenizer) CountTokensFromResponse(ctx *RelayContext, resp interface{}) (int, int, error) {
    // 优先使用上游响应中的 usage 字段
    // 如果上游不返回 usage，则本地统计
    // ...
}
```

---

## 七、协议转换详解

### 7.1 Claude → OpenAI 响应转换

```go
// internal/gateway/relay/claude/converter.go

// ConvertClaudeResponseToOpenAI 将 Claude 响应转换为 OpenAI 格式
func ConvertClaudeResponseToOpenAI(claudeResp *ClaudeResponse) *OpenAIResponse {
    openaiResp := &OpenAIResponse{
        ID:      "chatcmpl-" + shortUUID(),
        Object:  "chat.completion",
        Created: time.Now().Unix(),
        Model:   claudeResp.Model,
        Choices: []Choice{
            {
                Index: 0,
                Message: &Message{
                    Role:    "assistant",
                    Content: extractTextFromClaudeContent(claudeResp.Content),
                },
                FinishReason: mapFinishReason(claudeResp.StopReason),
            },
        },
        Usage: Usage{
            PromptTokens:     claudeResp.Usage.InputTokens,
            CompletionTokens: claudeResp.Usage.OutputTokens,
            TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
        },
    }
    
    return openaiResp
}
```

### 7.2 Gemini → OpenAI 响应转换

```go
// internal/gateway/relay/gemini/converter.go

// ConvertGeminiResponseToOpenAI 将 Gemini 响应转换为 OpenAI 格式
func ConvertGeminiResponseToOpenAI(geminiResp *GeminiResponse) *OpenAIResponse {
    // Gemini 的 candidates[0].content.parts[0].text → OpenAI 的 choices[0].message.content
    text := ""
    if len(geminiResp.Candidates) > 0 {
        for _, part := range geminiResp.Candidates[0].Content.Parts {
            text += part.Text
        }
    }
    
    return &OpenAIResponse{
        ID:      "chatcmpl-" + shortUUID(),
        Object:  "chat.completion",
        Created: time.Now().Unix(),
        Model:   geminiResp.ModelVersion,
        Choices: []Choice{
            {
                Index: 0,
                Message: &Message{
                    Role:    "assistant",
                    Content: text,
                },
                FinishReason: mapGeminiFinishReason(geminiResp.Candidates[0].FinishReason),
            },
        },
        Usage: Usage{
            PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
            CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
            TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
        },
    }
}
```

---

## 八、OAuth Token 管理

> **[sub2api]** 参考 Sub2API 的 `TokenRefreshService`，实现 OAuth Token 自动轮转刷新。

```go
// internal/service/oauth/token_refresh.go

type TokenRefreshService struct {
    channelRepo *repository.ChannelRepository
    httpClient  *http.Client
}

// RefreshIfNeeded 检查并在必要时刷新 OAuth Token
func (s *TokenRefreshService) RefreshIfNeeded(ctx context.Context, ch *ent.Channel) error {
    if ch.AuthType != "oauth" {
        return nil
    }
    
    // 如果 Token 还有超过 5 分钟才过期，不需要刷新
    if ch.OAuthExpiresAt != nil && time.Until(*ch.OAuthExpiresAt) > 5*time.Minute {
        return nil
    }
    
    // 使用 refresh_token 获取新 token
    newToken, newRefreshToken, expiresAt, err := s.doRefresh(ch)
    if err != nil {
        return fmt.Errorf("failed to refresh oauth token: %w", err)
    }
    
    // 更新数据库
    return s.channelRepo.UpdateOAuthToken(ctx, ch.ID, newToken, newRefreshToken, expiresAt)
}

// doRefresh 执行 OAuth 刷新流程（各平台不同）
func (s *TokenRefreshService) doRefresh(ch *ent.Channel) (string, string, time.Time, error) {
    switch ch.ChannelType {
    case "claude":
        return s.refreshClaudeToken(ch)
    case "openai":
        return s.refreshOpenAIToken(ch)
    case "gemini":
        return s.refreshGeminiToken(ch)
    default:
        return "", "", time.Time{}, fmt.Errorf("unsupported oauth platform: %s", ch.ChannelType)
    }
}
```

---

## 九、语义缓存设计（高级功能）

> **产品文档**提到语义缓存作为高级功能，降低重复调用的成本和延迟。

```go
// internal/gateway/cache/semantic_cache.go

type SemanticCache struct {
    redis *cache.RedisClient
}

// GetCachedResponse 查询语义缓存（基于请求内容哈希）
func (c *SemanticCache) GetCachedResponse(ctx *RelayContext) (*OpenAIResponse, bool) {
    // 计算请求内容的语义哈希
    hash := c.computeSemanticHash(ctx.Request)
    
    key := fmt.Sprintf("semantic_cache:%s:%s", ctx.Request.Model, hash)
    cached, err := c.redis.Get(context.Background(), key)
    if err != nil {
        return nil, false
    }
    
    var resp OpenAIResponse
    json.Unmarshal([]byte(cached), &resp)
    return &resp, true
}

// SetCachedResponse 设置语义缓存
func (c *SemanticCache) SetCachedResponse(ctx *RelayContext, resp *OpenAIResponse) {
    hash := c.computeSemanticHash(ctx.Request)
    key := fmt.Sprintf("semantic_cache:%s:%s", ctx.Request.Model, hash)
    
    data, _ := json.Marshal(resp)
    // 缓存 1 小时，折扣计费（50% 费用）
    c.redis.Set(context.Background(), key, string(data), 1*time.Hour)
    
    // 标记此次响应使用了缓存（用于计费折扣）
    ctx.IsCached = true
    ctx.CachedTokens = ctx.InputTokens  // 缓存命中，不重复计费输入 Token
}
```

---

## 十、总结

网关核心模块完全参考 Sub2API 的成熟设计，在以下方面保持功能对等：

| 功能 | Sub2API 对应实现 | TraceHub 实现 |
|:---|:---:|:---|
| 智能路由 | `Selector` + `sticky_session` | 完全参考，增加混合调度 |
| 适配器体系 | `GatewayService` 系列 | 完全参考，增加 DeepSeek 适配器 |
| 计费引擎 | `BillingService` + 多级倍率 | 完全参考 |
| OAuth 管理 | `TokenRefreshService` | 完全参考 |
| 限流体系 | 三层限流 + 并发槽 | 完全参考 |
| 语义缓存 | 离线兜底计费 | 增强实现 |
