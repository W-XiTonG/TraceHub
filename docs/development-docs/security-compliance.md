# TraceHub 安全与合规文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**参考项目**：Sub2API 安全架构

---

## 一、安全架构总览

```
┌─────────────────────────────────────────────────────┐
│  第一层：网络层安全                                  │
│  WAF/CDN → HTTPS 强制 → IP 黑白名单 → DDoS 防护   │
├─────────────────────────────────────────────────────┤
│  第二层：应用层安全                                  │
│  认证 → 限流 → CORS → CSP → 请求脱敏 → 内容过滤    │
├─────────────────────────────────────────────────────┤
│  第三层：数据层安全                                  │
│  加密存储 → 最小权限 → 审计日志 → 数据保留策略      │
├─────────────────────────────────────────────────────┤
│  第四层：运维安全                                    │
│  定期备份 → 漏洞扫描 → 安全审计 → 合规检查          │
└─────────────────────────────────────────────────────┘
```

---

## 二、密钥加密存储

### 2.1 上游渠道 API Key 加密（AES-256-GCM）

> **[sub2api]** 参考 Sub2API 的密钥加密存储方案。参考产品文档 3.3 节要求。

```go
// internal/pkg/encrypt/aes.go

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

type AESGCM struct {
    key []byte // 256-bit key
}

func NewAESGCM(hexKey string) (*AESGCM, error) {
    key, err := hex.DecodeString(hexKey)
    if err != nil {
        return nil, fmt.Errorf("invalid hex key: %w", err)
    }
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes for AES-256")
    }
    return &AESGCM{key: key}, nil
}

// Encrypt 加密明文（返回 base64 编码的密文）
func (a *AESGCM) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(a.key)
    if err != nil {
        return "", err
    }
    
    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonce := make([]byte, aesGCM.NonceSize())
    if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    
    ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密密文
func (a *AESGCM) Decrypt(encrypted string) (string, error) {
    ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
    if err != nil {
        return "", err
    }
    
    block, err := aes.NewCipher(a.key)
    if err != nil {
        return "", err
    }
    
    aesGCM, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonceSize := aesGCM.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }
    
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(plaintext), nil
}
```

### 2.2 用户 API Key 哈希存储

```go
// internal/pkg/crypto/apikey.go

import (
    "crypto/sha256"
    "encoding/hex"
)

// HashAPIKey 计算 API Key 的 SHA-256 哈希
// 明文 Key 仅在创建时展示一次，数据库中只存储哈希
func HashAPIKey(apiKey string) string {
    hash := sha256.Sum256([]byte(apiKey))
    return hex.EncodeToString(hash[:])
}

// VerifyAPIKey 验证 API Key（对比哈希）
func VerifyAPIKey(apiKey string, storedHash string) bool {
    hash := sha256.Sum256([]byte(apiKey))
    return hex.EncodeToString(hash[:]) == storedHash
}
```

### 2.3 加密密钥管理

| 密钥类型 | 存储方式 | 说明 |
|:---|:---|:---|
| AES-256-GCM 主密钥 | 环境变量 `ENCRYPTION_KEY` | 用于加密上游渠道密钥 |
| JWT 签名密钥 | 环境变量 `JWT_SECRET` | 用于签发和验证 JWT |
| SMTP 密码 | 数据库加密存储 | AES-256-GCM 加密 |
| 支付私钥 | 数据库加密存储 | AES-256-GCM 加密 |

> **⚠️ 生产安全要求**：AES 主密钥和 JWT 密钥**禁止**硬编码在配置文件中，必须通过环境变量或密钥管理服务（如 HashiCorp Vault）注入。

---

## 三、传输安全

### 3.1 HTTPS 强制

> **[sub2api]** 参考 Sub2API 的安全架构，默认拒绝 HTTP 上游 URL。

```go
// internal/gateway/relay/adapter.go

// validateUpstreamURL 验证上游 URL 是否为 HTTPS
func validateUpstreamURL(rawURL string) error {
    u, err := url.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid upstream URL: %w", err)
    }
    
    if u.Scheme != "https" {
        if !config.GetBool("allow_insecure_http") {
            return fmt.Errorf(
                "insecure HTTP upstream URL not allowed in production. " +
                "Set ALLOW_INSECURE_HTTP=true only for development/internal networks"
            )
        }
        // 开发环境警告
        log.Warn("using insecure HTTP upstream URL", "url", rawURL)
    }
    
    return nil
}
```

### 3.2 双向 TLS（mTLS，企业级）

```go
// 企业接入场景支持双向 TLS
func loadMTLSConfig() *tls.Config {
    cert, _ := tls.LoadX509KeyPair("client.crt", "client.key")
    caCert, _ := os.ReadFile("ca.crt")
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)
    
    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS12,
    }
}
```

### 3.3 安全响应头

```go
// internal/handler/middleware/security_headers.go

func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        // CSP（Content-Security-Policy）
        c.Header("Content-Security-Policy", "default-src 'self'")
        
        // 防止 MIME 类型嗅探
        c.Header("X-Content-Type-Options", "nosniff")
        
        // 防止点击劫持
        c.Header("X-Frame-Options", "DENY")
        
        // XSS 防护
        c.Header("X-XSS-Protection", "1; mode=block")
        
        // 严格传输安全（HSTS）
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        
        // 引用策略
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        
        c.Next()
    }
}
```

---

## 四、访问控制

### 4.1 IP 白名单（多级）

> **[sub2api]** 参考 Sub2API 的 IP 白名单机制，支持用户级、API Key 级、全局级。

```go
// internal/gateway/middleware/ip_whitelist.go

func IPWhitelistMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        clientIP := c.ClientIP()
        
        // 获取当前 API Key
        apiKey, exists := c.Get("api_key")
        if !exists {
            c.Next()
            return
        }
        
        key := apiKey.(*ent.APIKey)
        
        // 1. 检查 API Key 级 IP 白名单
        if len(key.IPWhitelist) > 0 {
            if !matchIPList(clientIP, key.IPWhitelist) {
                c.JSON(403, OpenAIErrorResponse{
                    Error: &OpenAIError{
                        Message: "IP address not in API Key whitelist",
                        Type:    "permission_error",
                        Code:    "ip_not_allowed",
                    },
                })
                c.Abort()
                return
            }
        }
        
        // 2. 检查 API Key 级 IP 黑名单
        if len(key.IPBlacklist) > 0 {
            if matchIPList(clientIP, key.IPBlacklist) {
                c.JSON(403, OpenAIErrorResponse{...})
                c.Abort()
                return
            }
        }
        
        // 3. 检查用户级 IP 白名单（如果有配置）
        user := c.MustGet("user").(*ent.User)
        if len(user.IPWhitelist) > 0 {
            if !matchIPList(clientIP, user.IPWhitelist) {
                c.JSON(403, OpenAIErrorResponse{...})
                c.Abort()
                return
            }
        }
        
        // 4. 检查全局 IP 黑名单
        if globalBlacklist.Match(clientIP) {
            c.JSON(403, OpenAIErrorResponse{...})
            c.Abort()
            return
        }
        
        c.Next()
    }
}

// matchIPList 检查 IP 是否在 IP 列表中（支持 CIDR）
func matchIPList(ip string, cidrs []string) bool {
    parsedIP := net.ParseIP(ip)
    for _, cidr := range cidrs {
        _, network, err := net.ParseCIDR(cidr)
        if err == nil && network.Contains(parsedIP) {
            return true
        }
        // 也支持精确匹配（非 CIDR 格式）
        if err != nil && ip == cidr {
            return true
        }
    }
    return false
}
```

### 4.2 CORS 白名单配置

> **[sub2api]** 参考 Sub2API 的 CORS 配置。

```go
// internal/handler/middleware/cors.go

func CORS() gin.HandlerFunc {
    origins := config.GetStringSlice("cors_allowed_origins")
    
    return func(c *gin.Context) {
        origin := c.GetHeader("Origin")
        allowed := false
        
        for _, o := range origins {
            if o == "*" || o == origin {
                allowed = true
                break
            }
        }
        
        if allowed {
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, X-Sticky-Session")
            c.Header("Access-Control-Allow-Credentials", "true")
            c.Header("Access-Control-Max-Age", "86400")
        }
        
        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        
        c.Next()
    }
}
```

### 4.3 响应头过滤

> **[sub2api]** 参考 Sub2API 的响应头过滤能力，防止敏感信息泄露。

```go
// internal/handler/middleware/response_filter.go

func ResponseHeaderFilter() gin.HandlerFunc {
    // 需要过滤的上游响应头
    filteredHeaders := map[string]bool{
        "x-ratelimit-limit-tokens":    true,
        "x-ratelimit-remaining-tokens": true,
        "x-primary-used-percent":      true, // OpenAI Codex 配额信息
    }
    
    return func(c *gin.Context) {
        c.Next()
        
        // 过滤上游响应中可能泄露内部信息的响应头
        for header := range filteredHeaders {
            c.Writer.Header().Del(header)
        }
    }
}
```

---

## 五、内容安全过滤

> 参考产品文档 3.2 节"内容安全配置"要求。

### 5.1 请求/响应内容过滤中间件

```go
// internal/gateway/middleware/content_filter.go

type ContentFilter struct {
    sensitiveWords []string // 敏感词库
    enabled        bool
}

func (cf *ContentFilter) FilterContent(text string) (bool, string) {
    if !cf.enabled {
        return false, text
    }
    
    // 检查敏感词
    for _, word := range cf.sensitiveWords {
        if strings.Contains(strings.ToLower(text), strings.ToLower(word)) {
            return true, "" // 返回 true 表示触发过滤
        }
    }
    
    return false, text
}

// ContentFilterMiddleware 内容安全过滤中间件（异步检查）
func (cf *ContentFilter) Middleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if !cf.enabled {
            c.Next()
            return
        }
        
        // 仅对聊天补全接口启用内容过滤
        if c.Request.URL.Path != "/v1/chat/completions" {
            c.Next()
            return
        }
        
        var req OpenAIChatRequest
        if err := c.ShouldBindJSON(&req); err != nil {
            c.Next()
            return
        }
        
        // 检查请求内容
        for _, msg := range req.Messages {
            content := extractTextContent(msg.Content)
            filtered, _ := cf.FilterContent(content)
            if filtered {
                c.JSON(400, OpenAIErrorResponse{
                    Error: &OpenAIError{
                        Message: "请求内容包含敏感信息，已被拦截",
                        Type:    "content_filter_error",
                        Code:    "content_blocked",
                    },
                })
                c.Abort()
                return
            }
        }
        
        // 保存原始请求体（供后续中间件使用）
        c.Set("request_body", req)
        c.Next()
        
        // 异步检查响应内容（不阻塞请求）
        go func() {
            // ... 记录日志，触发告警
        }()
    }
}
```

---

## 六、审计日志

### 6.1 操作审计（管理员操作）

> 参考产品文档 3.2 节"操作日志"要求。

```go
// internal/service/audit/audit_log.go

type AuditLog struct {
    ID         int64     `json:"id"`
    OperatorID int64     `json:"operator_id"`
    OperatorIP string    `json:"operator_ip"`
    Action     string    `json:"action"`     // create / update / delete / login / export
    Resource   string    `json:"resource"`   // user / channel / group / config
    ResourceID int64     `json:"resource_id"`
    Detail     string    `json:"detail"`     // 变更内容 JSON
    Timestamp  time.Time `json:"timestamp"`
}

type AuditService struct {
    repo *repository.AuditLogRepository
}

// Log 记录管理员操作
func (s *AuditService) Log(ctx context.Context, log *AuditLog) error {
    return s.repo.Create(ctx, log)
}

// LogUserCreated 记录管理员创建用户
func (s *AuditService) LogUserCreated(ctx context.Context, adminID int64, userID int64, detail string) {
    s.Log(ctx, &AuditLog{
        OperatorID: adminID,
        OperatorIP: GetClientIP(ctx),
        Action:     "create",
        Resource:   "user",
        ResourceID: userID,
        Detail:     detail,
    })
}
```

### 6.2 请求审计（调用日志）

> 调用日志表 `usage_logs` 本身即为审计数据，记录每个请求的完整信息。

参考数据库设计文档中的 `usage_logs` 表结构，包含：
- 请求唯一 ID
- 用户 ID / API Key ID / 渠道 ID
- 模型名称
- Token 用量
- 费用
- 源 IP
- 状态码 / 错误信息
- 时间戳

---

## 七、数据保留策略

> 参考产品文档 3.2 节"数据保留策略"要求。

### 7.1 日志保留配置

| 数据类型 | 默认保留时长 | 可配置范围 | 说明 |
|:---|:---:|:---|:---|
| 调用日志 | 90 天 | 30~180 天 | 超期自动清理 |
| 审计日志 | 180 天 | 90~365 天 | 满足合规审计需求 |
| 登录日志 | 180 天 | 90~365 天 | 安全审计需要 |
| Prometheus 指标 | 15 天 | 7~30 天 | 监控数据滚动 |

### 7.2 自动清理实现

```go
// internal/service/log/log_cleanup.go

type LogCleanupService struct {
    db    *ent.Client
    redis *redis.Client
    config *config.Config
}

// CleanupExpiredLogs 清理过期日志
func (s *LogCleanupService) CleanupExpiredLogs(ctx context.Context) error {
    retentionDays := s.config.GetInt("log_retention_days") // 默认 90
    
    // 清理调用日志（按分区删除，高效）
    cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
    _, err := s.db.ExecContext(ctx, 
        "DELETE FROM usage_logs WHERE request_time < $1",
        cutoffDate,
    )
    if err != nil {
        return fmt.Errorf("failed to cleanup usage logs: %w", err)
    }
    
    // 清理审计日志
    auditRetentionDays := s.config.GetInt("audit_log_retention_days") // 默认 180
    auditCutoffDate := time.Now().AddDate(0, 0, -auditRetentionDays)
    s.db.ExecContext(ctx,
        "DELETE FROM audit_logs WHERE timestamp < $1",
        auditCutoffDate,
    )
    
    return nil
}
```

### 7.3 合规审计数据保留要求

| 合规标准 | 最低保留要求 | 推荐配置 |
|:---|:---:|:---|
| **等保二级** | 90 天 | 180 天 |
| **等保三级** | 180 天 | 365 天 |
| **ISO 27001** | 180 天 | 365 天 |

---

## 八、TOTP 二次验证（可选增强）

> **[sub2api]** 参考 Sub2API 的 TOTP 支持。

```go
// internal/service/auth/totp.go

import (
    "github.com/pquerna/otp"
    "github.com/pquerna/otp/totp"
)

type TOTPService struct{}

// GenerateSecret 生成 TOTP 密钥
func (s *TOTPService) GenerateSecret(userEmail string) (*otp.Key, error) {
    return totp.Generate(totp.GenerateOpts{
        Issuer:      "TraceHub",
        AccountName: userEmail,
        SecretSize:  32,
        Period:      30,
        Digits:      otp.DigitsSix,
        Algorithm:   otp.AlgorithmSHA1,
    })
}

// Validate 验证 TOTP 验证码
func (s *TOTPService) Validate(secret string, passcode string) bool {
    return totp.Validate(passcode, secret)
}
```

---

## 九、安全配置清单

### 9.1 生产环境必检项

| # | 检查项 | 要求 | 验证方法 |
|:-:|:---|:---|:---|
| 1 | HTTPS 强制 | 所有通信使用 HTTPS | curl -v 检查 |
| 2 | JWT 密钥强度 | 至少 32 字符随机字符串 | 检查环境变量 |
| 3 | AES 主密钥强度 | 64 字符十六进制字符串 | 检查环境变量 |
| 4 | 数据库密码 | 至少 16 字符，含特殊字符 | 检查 DSN |
| 5 | Redis 密码 | 至少 16 字符 | 检查 Redis 配置 |
| 6 | CORS 白名单 | 不使用 `*`，列出允许的域名 | 检查配置 |
| 7 | 上游 URL | 仅允许 HTTPS | 检查 `allow_insecure_http=false` |
| 8 | 日志脱敏 | API Key、密码等敏感信息不记录明文 | 检查日志 |
| 9 | CSP 配置 | 启用 Content-Security-Policy | 检查响应头 |
| 10 | 日志保留 | 至少 90 天（等保二级） | 检查配置 |

### 9.2 定期安全检查

| 检查类型 | 频率 | 说明 |
|:---|:---:|:---|
| 弱密码扫描 | 每周 | 自动扫描用户弱密码并提示修改 |
| 异常调用模式 | 实时 | 单用户短时间内大量调用触发告警 |
| 依赖漏洞扫描 | 每月 | `go mod` 依赖 CVE 扫描 |
| SSL 证书检查 | 每天 | 自动检查 SSL 证书有效期 |
| 日志完整性 | 每周 | 验证审计日志无篡改 |

### 9.3 安全响应流程

```
发现安全事件
    │
    ▼
立即评估影响范围
    │
    ├─ 数据泄露 → 通知受影响用户 + 修改泄露密钥 + 保留证据
    ├─ 异常调用 → 封禁相关账号 + 审计日志排查
    ├─ 渠道故障 → 自动切换备用渠道 + 通知管理员
    └─ 服务异常 → 回滚到上一稳定版本 + 根因分析
```

---

## 十、Turnstile 验证码（反爬虫）

> **[sub2api]** 参考 Sub2API 的 Turnstile 集成。

```go
// internal/handler/middleware/turnstile.go

type TurnstileMiddleware struct {
    siteKey   string
    secretKey string
    enabled   bool
}

func (t *TurnstileMiddleware) Verify(c *gin.Context) {
    if !t.enabled {
        c.Next()
        return
    }
    
    token := c.PostForm("cf-turnstile-response")
    if token == "" {
        c.JSON(403, gin.H{"error": "turnstile verification required"})
        c.Abort()
        return
    }
    
    // 向 Cloudflare 验证
    resp, err := http.PostForm(
        "https://challenges.cloudflare.com/turnstile/v0/siteverify",
        url.Values{
            "secret":   {t.secretKey},
            "response": {token},
            "remoteip": {c.ClientIP()},
        },
    )
    // ... 处理验证结果
    
    c.Next()
}
```

---

## 十一、总结

安全与合规设计参考 Sub2API 的安全架构，在以下方面保持功能对等或增强：

| 安全能力 | Sub2API 对应 | TraceHub 实现 |
|:---|:---:|:---|
| 密钥加密存储 | AES-256-GCM | 完全参考 |
| HTTPS 强制 | 默认拒绝 HTTP 上游 | 完全参考 |
| CORS 白名单 | 可配置 | 完全参考 |
| 响应头过滤 | 支持 CSP/安全头 | 完全参考 |
| 内容安全过滤 | 基础支持 | 增强实现 |
| 审计日志 | 操作审计 | 增强实现 |
| 数据保留策略 | 支持 | 增强实现（自动清理） |
| TOTP | 支持 | 完全参考 |
| Turnstile | 支持 | 完全参考 |
| IP 黑白名单 | 支持 | 增强（三级：全局/用户/Key） |
