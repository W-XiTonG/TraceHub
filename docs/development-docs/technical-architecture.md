# TraceHub 技术架构设计文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**参考项目**：Sub2API、New API

---

## 一、整体架构图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         客户端 / 浏览器                              │
│          HTTP/HTTPS + OpenAI-Compatible API Requests                │
└───────────────────────────────┬───────────────────────────────────┘
                                │
┌───────────────────────────────▼───────────────────────────────────┐
│                    负载均衡 / 反向代理层                             │
│                 Nginx / Caddy / Cloudflare                         │
│   ┌───────────────────────────────────────────────────────────┐  │
│   │ WAF 防护 │ HTTPS 终止 │ 流量清洗 │ 请求限速                │  │
│   └───────────────────────────────────────────────────────────┘  │
└───────────────────────────────┬───────────────────────────────────┘
                                │
┌───────────────────────────────▼───────────────────────────────────┐
│                      Go Gin HTTP 服务层                             │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │                     中间件层 (Middleware)                       │ │
│  │  CORS │ 认证 │ 限流 │ 请求日志 │ Panic 恢复 │ 请求ID         │ │
│  │  请求脱敏 │ Turnstile 验证 │ CSP 配置                       │ │
│  └──────────────────────────────────────────────────────────────┘ │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │                      路由层 (Router)                          │ │
│  │  网关 API 路由 │ Web UI 路由 │ 管理后台路由 │ Webhook 路由   │ │
│  │  健康检查路由 │ Prometheus Metrics 路由                      │ │
│  └──────────────────────────────────────────────────────────────┘ │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │                    HTTP 处理器层 (Handler)                     │ │
│  │  Gateway Handler │ User API Handler │ Admin API Handler      │ │
│  │  Webhook Handler │ Health Handler                            │ │
│  └──────────────────────────────────────────────────────────────┘ │
│  ┌───────────────┬──────────────────┬─────────────────────────┐ │
│  │   服务层       │   网关核心层      │      数据层               │ │
│  │  (Service)    │   (Relay/Core)   │     (Repository)         │ │
│  │               │                   │                           │ │
│  │  UserService   │  Relay Adapter   │  UserRepository          │ │
│  │  QuotaService  │  Router Selector │  ChannelRepository       │ │
│  │  ChannelSvc   │  Billing Engine  │  APIKeyRepository        │ │
│  │  PaymentSvc   │  RateLimiter     │  UsageLogRepository      │ │
│  │  NotifySvc    │  Tokenizer       │  GroupRepository         │ │
│  │  DashboardSvc │                   │                           │ │
│  └───────────────┴──────────────────┴─────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐      ┌────────────────┐      ┌────────────────┐
│  PostgreSQL   │      │     Redis       │      │  外部 AI 上游   │
│  (Ent ORM)    │      │ (缓存/协调/队列)│      │ Anthropic/OpenAI│
│               │      │                 │      │ Google/Gemini   │
│ 用户数据       │      │ API Key 缓存    │      │ DeepSeek/...   │
│ 渠道数据       │      │ 计费缓存         │      │ Antigravity    │
│ 调用日志       │      │ 并发槽管理       │      │                │
│ 系统配置       │      │ 会话粘性绑定     │      │                │
└───────────────┘      └────────────────┘      └────────────────┘
```

---

## 二、技术选型详细说明

### 2.1 后端技术栈

| 组件 | 技术选型 | 版本要求 | 选型理由 |
|:---|:---|:---:|:---|
| **语言** | Go | 1.23+ | 高并发原生支持，编译期依赖注入，适合网关场景 |
| **Web 框架** | Gin | latest | 高性能轻量级，中间件生态完善 |
| **ORM** | Ent | latest | 类型安全的 Go ORM，自动迁移，Schema 即代码 |
| **依赖注入** | Google Wire | latest | 编译期 DI，零运行时开销，适合大型项目 |
| **HTTP Client** | `net/http` + 连接池 | 标准库 | 复用连接池，支持超时控制、重试、熔断 |
| **日志** | Zap | latest | 高性能结构化日志，支持 JSON 输出 |
| **配置管理** | Viper | latest | 多格式配置，支持环境变量覆盖 |
| **任务队列** | Asynq | latest | Redis -backed，支持延迟任务、重试、优先级 |

> **[sub2api]** Sub2API 使用 Ent + Wire 的组合，本项目完全沿用这一成熟方案。

### 2.2 数据存储

| 组件 | 技术选型 | 版本要求 | 说明 |
|:---|:---|:---:|:---|
| **主数据库** | PostgreSQL | 16+ | 存储用户、渠道、调用日志等核心数据 |
| **缓存/协调** | Redis | 7+ | 限流计数、会话存储、计费预扣、分布式锁 |
| **任务队列** | Redis (Asynq) | 7+ | 异步任务处理（批量扣费、报表生成、邮件发送） |

### 2.3 前端技术栈

| 组件 | 技术选型 | 版本要求 | 说明 |
|:---|:---|:---:|:---|
| **框架** | Vue 3 | 3.4+ | 组合式 API，性能优秀 |
| **构建工具** | Vite | 5+ | 极速 HMR，开箱即用 |
| **样式框架** | TailwindCSS | 3+ | 原子化 CSS，快速开发 |
| **状态管理** | Pinia | latest | Vue 3 官方推荐 |
| **HTTP 客户端** | Axios | latest | 请求/响应拦截 |
| **图表** | Chart.js / ECharts | latest | 数据可视化 |

### 2.4 监控与部署

| 组件 | 技术选型 | 说明 |
|:---|:---|:---|
| **指标采集** | Prometheus | 暴露 `/metrics` 端点 |
| **可视化** | Grafana | 仪表盘展示 |
| **容器化** | Docker + Docker Compose | 一键部署 |
| **编排** | Kubernetes (可选) | 大规模部署 |
| **反向代理** | Nginx / Caddy | 负载均衡、HTTPS 终止 |

---

## 三、目录结构设计

参考 Sub2API 和 New API 的成熟项目结构，结合本项目的实际需求设计：

```
tracehub/
├── cmd/
│   └── server/
│       └── main.go                 # 程序入口（Wire 注入入口）
├── internal/
│   ├── gateway/                     # 网关核心 [sub2api 参考]
│   │   ├── router/                  # 智能路由选择器
│   │   │   ├── selector.go          # 账号选择算法（优先级/权重/粘性会话）
│   │   │   ├── sticky_session.go    # 粘性会话管理
│   │   │   └── health_check.go      # 渠道健康检查
│   │   ├── relay/                   # 请求转发适配器 [sub2api 核心参考]
│   │   │   ├── openai/              # OpenAI 适配
│   │   │   │   ├── relay.go         # 请求转发
│   │   │   │   ├── adapter.go       # 请求/响应转换
│   │   │   │   └── stream.go        # SSE 流式处理
│   │   │   ├── claude/              # Claude 适配 [sub2api 核心]
│   │   │   │   ├── relay.go
│   │   │   │   ├── adapter.go       # Messages API 格式转换
│   │   │   │   ├── oauth.go         # OAuth PKCE 流程
│   │   │   │   └── stream.go
│   │   │   ├── gemini/              # Gemini 适配 [sub2api 核心]
│   │   │   │   ├── relay.go
│   │   │   │   ├── adapter.go       # Claude→Gemini 格式转换
│   │   │   │   └── stream.go
│   │   │   ├── deepseek/            # DeepSeek 适配
│   │   │   │   ├── relay.go
│   │   │   │   └── adapter.go
│   │   │   ├── antigravity/         # Antigravity 适配 [sub2api 参考]
│   │   │   │   ├── relay.go
│   │   │   │   └── adapter.go       # v1internal 协议转换
│   │   │   └── relay_factory.go     # 适配器工厂（按平台选择 Relay）
│   │   ├── billing/                 # 计费引擎 [sub2api 参考]
│   │   │   ├── engine.go            # 计费主引擎
│   │   │   ├── tokenizer.go        # Token 统计（tiktoken）
│   │   │   ├── multimodal.go        # 多模态计费（图片/Tile/Patch）
│   │   │   ├── cache_billing.go    # Prompt Caching 折扣计费
│   │   │   ├── circuit_breaker.go  # 计费异常熔断
│   │   │   └── rate_resolver.go    # 多级倍率计算（分组/账号/管理员）
│   │   ├── ratelimit/               # 限流器 [sub2api 参考]
│   │   │   ├── user_limiter.go     # 用户级限流
│   │   │   ├── key_limiter.go      # API Key 级限流
│   │   │   ├── channel_limiter.go   # 渠道级限流
│   │   │   ├── global_limiter.go    # 全局限流
│   │   │   └── concurrency.go      # 并发槽管理
│   │   └── middleware/              # 网关专用中间件
│   │       ├── auth.go              # API Key / JWT 认证
│   │       ├── ip_whitelist.go      # IP 白名单校验
│   │       ├── request_id.go        # 请求 ID 注入
│   │       └── request_sanitizer.go # 请求脱敏
│   ├── service/                     # 业务服务层 [sub2api 参考]
│   │   ├── user/                    # 用户服务
│   │   │   └── service.go
│   │   ├── quota/                   # 配额管理服务
│   │   │   └── service.go
│   │   ├── channel/                 # 渠道管理服务
│   │   │   ├── service.go
│   │   │   └── health_check.go
│   │   ├── payment/                 # 支付服务 [sub2api 参考]
│   │   │   ├── service.go           # 支付主服务
│   │   │   ├── alipay.go            # 支付宝对接
│   │   │   ├── wxpay.go             # 微信支付对接
│   │   │   ├── stripe.go            # Stripe 对接
│   │   │   └── redeem.go            # 兑换码服务
│   │   ├── notification/            # 通知服务
│   │   │   ├── email.go             # 邮件通知（SMTP）
│   │   │   └── webhook.go           # Webhook 推送
│   │   ├── dashboard/               # 仪表盘聚合服务
│   │   │   └── service.go
│   │   └── oauth/                   # OAuth 服务 [sub2api 参考]
│   │       ├── service.go           # OAuth 通用服务
│   │       ├── claude.go            # Claude OAuth PKCE
│   │       ├── openai.go            # OpenAI OAuth
│   │       └── gemini.go           # Gemini OAuth（3类）
│   ├── handler/                     # HTTP 处理器层
│   │   ├── gateway/                 # 网关 API 处理器
│   │   │   ├── chat_completion.go
│   │   │   ├── embeddings.go
│   │   │   └── models.go
│   │   ├── user/                    # 用户 API 处理器
│   │   │   ├── auth.go             # 注册/登录
│   │   │   ├── apikey.go           # API Key 管理
│   │   │   ├── balance.go          # 余额查询
│   │   │   └── usage.go            # 用量查询
│   │   ├── admin/                   # 管理员 API 处理器
│   │   │   ├── user_manage.go
│   │   │   ├── channel_manage.go
│   │   │   ├── group_manage.go
│   │   │   ├── billing_config.go
│   │   │   ├── log_query.go
│   │   │   └── dashboard.go
│   │   ├── webhook/                 # Webhook 处理器
│   │   │   └── handler.go
│   │   └── middleware/              # HTTP 中间件
│   │       ├── cors.go
│   │       ├── auth.go
│   │       ├── ratelimit.go
│   │       ├── recovery.go
│   │       └── request_logger.go
│   ├── ent/                         # Ent ORM Schema 定义 [sub2api 参考]
│   │   ├── schema/                  # 数据模型 Schema
│   │   │   ├── user.go
│   │   │   ├── apikey.go
│   │   │   ├── group.go
│   │   │   ├── channel.go
│   │   │   ├── usage_log.go
│   │   │   ├── redeem_code.go
│   │   │   └── system_config.go
│   │   ├── client/                  # 自动生成的 Ent 客户端
│   │   └── generate.go              # Ent 代码生成入口
│   ├── repository/                  # 数据访问层（Ent 封装）
│   │   ├── user_repo.go
│   │   ├── apikey_repo.go
│   │   ├── channel_repo.go
│   │   ├── group_repo.go
│   │   ├── usage_log_repo.go
│   │   └── config_repo.go
│   ├── model/                       # 领域模型（业务实体）
│   │   ├── user.go
│   │   ├── channel.go
│   │   └── dto/                     # 数据传输对象
│   │       ├── user.go
│   │       ├── channel.go
│   │       └── billing.go
│   └── pkg/                         # 内部公共包
│       ├── cache/                   # Redis 缓存封装
│       │   ├── client.go
│       │   ├── api_key_cache.go
│       │   ├── billing_cache.go
│       │   ├── concurrency_cache.go
│       │   └── dashboard_cache.go
│       ├── config/                  # 配置管理（Viper）
│       │   ├── config.go
│       │   └── config.yaml          # 配置文件模板
│       ├── metrics/                 # Prometheus 指标
│       │   ├── collector.go
│       │   └── middleware.go        # Gin Prometheus 中间件
│       ├── encrypt/                 # 加密工具（AES-256-GCM）
│       │   └── aes.go
│       ├── httpclient/              # HTTP 客户端连接池
│       │   └── pool.go
│       └── constants/              # 常量定义
│           ├── channels.go          # 渠道类型常量
│           ├── models.go            # 模型名称常量
│           └── error_codes.go      # 错误码常量
├── web/
│   └── frontend/                    # Vue 3 前端（embed 嵌入后端）
│       ├── src/
│       │   ├── api/                 # API 调用封装
│       │   ├── components/          # 通用组件
│       │   ├── views/               # 页面组件
│       │   │   ├── home/            # 官网首页
│       │   │   ├── auth/            # 登录/注册
│       │   │   ├── dashboard/       # 用户中心仪表盘
│       │   │   ├── admin/           # 管理后台
│       │   │   └── docs/            # API 文档页
│       │   ├── router/              # Vue Router
│       │   ├── store/               # Pinia 状态管理
│       │   └── utils/               # 工具函数
│       ├── public/
│       ├── package.json
│       ├── vite.config.ts
│       └── tailwind.config.js
├── scripts/                         # 部署/运维脚本
│   ├── install.sh                   # Linux 一键安装脚本 [sub2api 参考]
│   ├── uninstall.sh
│   ├── backup.sh                    # 数据库备份脚本
│   ├── restore.sh                   # 数据库恢复脚本
│   └── migrate/                     # 数据库迁移脚本
├── deploy/                          # 部署配置
│   ├── docker-compose.yml           # 标准版（命名卷）
│   ├── docker-compose.local.yml     # 本地目录版（推荐生产）[sub2api 参考]
│   ├── Dockerfile
│   ├── .env.example                # 环境变量模板
│   └── nginx/                       # Nginx 配置模板
│       └── tracehub.conf
├── configs/                         # 配置文件模板
│   └── config.yaml.example
├── go.mod
├── go.sum
├── Makefile                         # 构建脚本
└── README.md
```

---

## 四、依赖注入设计（Google Wire）

> **[sub2api]** Sub2API 使用 Google Wire 实现编译期依赖注入，本项目完全沿用此设计。

### 4.1 注入层级

```
Application（顶层容器）
├── Config ProviderSet        // 系统配置加载器
├── Repository ProviderSet    // 数据访问层（Ent Client、Redis Client、HTTP Client）
├── Service ProviderSet      // 业务服务层（所有 Service 实例）
└── Handler ProviderSet      // HTTP 处理器层（所有 Handler 实例）
```

### 4.2 Application 结构体定义

```go
// internal/service/application.go

type Application struct {
    // HTTP 服务
    Engine *gin.Engine
    
    // 核心服务
    UserService       *user.Service
    QuotaService      *quota.Service
    ChannelService    *channel.Service
    BillingService    *billing.Engine
    PaymentService    *payment.Service
    NotifyService    *notification.Service
    DashboardService *dashboard.Service
    OAuthService     *oauth.Service
    
    // 网关核心
    RelayFactory *relay.Factory
    Router       *router.Selector
    
    // 数据层
    DB     *ent.Client
    Redis  *redis.Client
    
    // 清理函数（优雅关闭）
    CleanupFuncs []func()
}

func (app *Application) Shutdown(ctx context.Context) error {
    for _, cleanup := range app.CleanupFuncs {
        cleanup()
    }
    return nil
}
```

### 4.3 Wire 注入文件

```go
// cmd/server/wire.go

//go:build wireinject

package main

import "github.com/google/wire"

func InitApplication() (*Application, error) {
    wire.Build(
        config.ProviderSet,
        repository.ProviderSet,
        service.ProviderSet,
        handler.ProviderSet,
        NewApplication,
    )
    return &Application{}, nil
}
```

---

## 五、请求处理全流程

```
客户端请求
    │
    ▼
┌─────────────────────────────────────────────┐
│ Step 1: Nginx 层                            │
│  - HTTPS 终止                              │
│  - WAF 防护                                │
│  - 请求限速（基础）                         │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 2: Gin 中间件链                        │
│  - CORS 处理                               │
│  - 请求 ID 注入 (X-Request-ID)             │
│  - IP 白名单校验                            │
│  - 认证中间件 (API Key / JWT 解析)          │
│  - 限流中间件 (用户级 → Key 级 → 全局)     │
│  - 请求日志 (结构化 JSON)                   │
│  - Panic 恢复                               │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 3: 路由分发                            │
│  - 根据路径选择 Handler                     │
│  - /v1/chat/completions → GatewayHandler   │
│  - /api/user/* → UserAPIHandler            │
│  - /api/admin/* → AdminAPIHandler          │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 4: 网关核心处理 (Relay)                │
│  - 解析请求体，提取 model 参数              │
│  - Token 预估（输入 Token 计数）            │
│  - 预扣费（基于预估 Token）                 │
│  - 智能路由选择上游渠道                      │
│    ├── 分组路由（优先）                     │
│    ├── 粘性会话路由                         │
│    ├── 优先级 + 权重路由                    │
│    └── 故障转移                             │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 5: Relay 适配器                        │
│  - 请求格式转换（OpenAI 标准 → 上游格式）   │
│  - 调用上游 AI API                         │
│  - 响应格式转换（上游格式 → OpenAI 标准）   │
│  - SSE 流式转发                             │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 6: 计费修正                            │
│  - 统计实际 Token 使用量                    │
│  - 计算实际费用（多级倍率）                 │
│  - 修正预扣费（多退少补）                   │
│  - 写入调用日志（异步）                     │
└──────────────────────┬──────────────────────┘
                       │
    ▼
┌─────────────────────────────────────────────┐
│ Step 7: 响应返回                            │
│  - 封装标准 OpenAI 响应格式                 │
│  - 注入响应头（X-Request-ID 等）            │
│  - 记录响应日志                             │
└─────────────────────────────────────────────┘
```

---

## 六、关键设计决策

### 6.1 为什么选择 Ent 而非 GORM？

> **[sub2api]** Sub2API 使用 Ent，本项目沿用。

| 对比维度 | Ent | GORM |
|:---|:---:|:---:|
| 类型安全 | ✅ 编译期检查 | ❌ 运行时反射 |
| Schema 即代码 | ✅ | ⚠️ 部分支持 |
| 迁移管理 | ✅ 自动生成迁移 SQL | ✅ AutoMigrate |
| 性能 | ✅ 更高（代码生成） | ⚠️ 反射开销 |
| 学习曲线 | ⚠️ 较陡 | ✅ 较平缓 |

### 6.2 为什么使用 Google Wire？

> **[sub2api]** 编译期依赖注入，零运行时开销，大型 Go 项目最佳实践。

- 依赖关系在编译期确定，避免运行时 DI 的性能损耗
- 依赖图可视化，便于理解系统架构
- 与 Ent 的代码生成机制天然契合

### 6.3 为什么前端使用 embed 嵌入后端二进制？

> **[sub2api]** 单个二进制文件部署，极大简化运维。

```go
//go:embed all:../../web/frontend/dist
var FrontendDist embed.FS

// 在 Gin 中挂载静态文件服务
distFS, _ := fs.Sub(FrontendDist, "web/frontend/dist")
engine.StaticFS("/assets", http.FS(distFS))
```

- 部署时只需分发单个二进制文件
- 前端版本与后端版本天然对齐
- 适合 Docker 单容器部署模式

### 6.4 多渠道适配设计

参考 Sub2API 的 `Relay Factory` 模式：

```go
// internal/gateway/relay/relay_factory.go

type Factory struct {
    adapters map[ChannelType]RelayAdapter
}

func (f *Factory) GetAdapter(channelType ChannelType) (RelayAdapter, error) {
    adapter, ok := f.adapters[channelType]
    if !ok {
        return nil, fmt.Errorf("unsupported channel type: %s", channelType)
    }
    return adapter, nil
}

type RelayAdapter interface {
    Relay(ctx *RelayContext) (*http.Response, error)
    ConvertRequest(req *OpenAIRequest) (interface{}, error)
    ConvertResponse(resp interface{}) (*OpenAIResponse, error)
    HandleStream(ctx *RelayContext) (<-chan string, error)
}
```

---

## 七、配置管理设计

### 7.1 配置加载优先级

```
环境变量（最高）
    ↓
命令行参数
    ↓
配置文件（config.yaml）
    ↓
默认值（最低）
```

### 7.2 核心配置项

```yaml
# config.yaml 核心配置结构

server:
  port: 8080
  mode: "release"  # debug / release

database:
  driver: "postgres"
  dsn: "host=localhost user=tracehub password=xxx dbname=tracehub port=5432 sslmode=disable"
  max_idle_conns: 10
  max_open_conns: 100

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  pool_size: 10

jwt:
  secret: "your-jwt-secret-key"
  expire_hours: 72

billing:
  rate_multiplier: 1.0        # 全局费率倍率
  circuit_breaker: true        # 计费异常熔断
  default_user_balance: 0      # 默认用户余额（额度）
  currency_unit: 10000         # 1 元 = 10000 额度

security:
  allow_insecure_http: false   # 是否允许 HTTP 上游（⚠️ 仅开发环境）
  cors_allowed_origins: ["*"]
  turnstile_site_key: ""
  turnstile_secret_key: ""

simple_mode: false             # 简易模式（隐藏 SaaS 功能）
```

---

## 八、总结

本架构设计完全参考 Sub2API 的成熟实践，在以下方面保持功能对等或增强：

1. **Ent + Wire 架构**：类型安全 ORM + 编译期依赖注入
2. **多层限流体系**：用户级 + Key 级 + 渠道级 + 全局级
3. **智能路由调度**：粘性会话 + 优先级/权重 + 故障转移 + 混合调度
4. **Token 级精准计费**：多级倍率 + 离线兜底 + 计费熔断
5. **多平台协议适配**：Claude/OpenAI/Gemini/DeepSeek/Antigravity
6. **单二进制部署**：前端 embed 嵌入，简化运维
7. **内置支付系统**：支付宝/微信/Stripe/EasyPay
