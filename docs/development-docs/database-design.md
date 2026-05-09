# TraceHub 数据库设计文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**数据库**：PostgreSQL 16+  
**ORM**：Ent（自动迁移）

---

## 一、ER 模型总览

```
User (用户)
  │ 1
  │ N
  ├── APIKey (API 密钥)     ← 用户持有多个 Key
  │
  │ 1
  │ N
  ├── UsageLog (调用日志)    ← 用户的所有调用记录
  │
  │ N
  │ 1
  └── Group (分组)           ← 用户归属一个分组

Group (分组)
  │ 1
  │ N
  └── GroupModelRate (分组模型倍率)  ← 分组内各模型的倍率配置
  
Channel (上游渠道/账号)
  │ N
  │ N (多对多，通过中间表关联)
  └── Group (分组)           ← 渠道归属于某个分组（同平台）

UsageLog (调用日志)
  │ N
  │ 1
  └── Channel (上游渠道)     ← 每次调用关联一个上游渠道

RedeemCode (兑换码)
  │ N
  │ 1
  └── User (用户，可选)     ← 兑换码可被某用户兑换

SystemConfig (系统配置)
  └── (KV 结构，存储全局配置项)
```

---

## 二、核心数据表结构（DDL）

> **说明**：以下 DDL 为最终表结构定义，实际项目中由 Ent Schema 生成，此处提供 DDL 便于理解字段设计。

### 2.1 用户表（`users`）

> **[sub2api]** 参考 Sub2API 的 `User` 模型设计，字段对齐。

```sql
CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255)  NOT NULL UNIQUE,
    username        VARCHAR(100),
    password_hash   VARCHAR(255)  NOT NULL,
    balance         BIGINT        DEFAULT 0,        -- 余额（最小单位：额度，1 元 = 10000 额度）
    group_id        BIGINT        REFERENCES groups(id),
    status          SMALLINT      DEFAULT 1,        -- 1:正常  2:禁用
    role            VARCHAR(50)   DEFAULT 'user',   -- user / admin / superadmin
    ip_whitelist    INET[]                     ,    -- 用户级 IP 白名单
    ip_blacklist    INET[]                     ,    -- 用户级 IP 黑名单
    concurrency     INTEGER       DEFAULT 0,        -- 用户级并发上限（0=使用分组默认值）
    rate_multiplier DECIMAL(10,4) DEFAULT 1.0000, -- 用户级费率倍率（覆盖分组）
    totp_secret    VARCHAR(255)                ,    -- TOTP 二次验证密钥
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email     ON users(email);
CREATE INDEX idx_users_group_id  ON users(group_id);
CREATE INDEX idx_users_status    ON users(status);
```

**字段说明**：

| 字段 | 说明 | 参考来源 |
|:---|:---|:---|
| `balance` | 额度单位，支持 Token 级精准计费 | 产品文档 + sub2api |
| `group_id` | 关联分组，继承分组费率和权限 | sub2api |
| `concurrency` | 用户级并发上限，参考 sub2api 的 `concurrency` 设计 | sub2api |
| `rate_multiplier` | 用户级费率倍率，可覆盖分组配置 | sub2api |
| `ip_whitelist`/`ip_blacklist` | IP 访问控制，参考 sub2api 的 APIKey 模型 | sub2api |

---

### 2.2 分组表（`groups`）

> **[sub2api]** 参考 Sub2API 的 `Group` 模型，支持订阅类型、模型路由等高级功能。

```sql
CREATE TABLE groups (
    id                  BIGSERIAL PRIMARY KEY,
    name                VARCHAR(100)   NOT NULL UNIQUE,
    description         TEXT,
    rate_multiplier     DECIMAL(10,4) DEFAULT 1.0000,  -- 分组全局费率倍率
    subscription_type   VARCHAR(50)   DEFAULT 'pay_as_you_go',  -- pay_as_you_go / subscription
    rpm_limit          INTEGER        DEFAULT 0,       -- 分组级每分钟请求数上限（0=不限制）
    model_routing      JSONB,                        -- 模型路由规则（指定模型→指定账号）
    default_model      VARCHAR(255),                 -- 默认模型
    enabled            BOOLEAN        DEFAULT TRUE,
    created_at         TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_groups_name ON groups(name);
```

**字段说明**：

| 字段 | 说明 | 参考来源 |
|:---|:---|:---|
| `rate_multiplier` | 分组级费率乘数 | sub2api |
| `subscription_type` | 支持按量付费和订阅制两种模式 | sub2api |
| `rpm_limit` | 分组级 RPM 限制 | sub2api |
| `model_routing` | JSONB 存储模型路由规则，可指定某模型固定路由到某账号 | sub2api |

---

### 2.3 分组模型倍率表（`group_model_rates`）

> 补充 `groups` 表的模型级倍率配置，支持每个分组对每类模型设置不同倍率。

```sql
CREATE TABLE group_model_rates (
    id           BIGSERIAL PRIMARY KEY,
    group_id     BIGINT        NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    model_name   VARCHAR(255) NOT NULL,
    input_rate   DECIMAL(10,6) DEFAULT 1.0,   -- 输入 Token 倍率
    output_rate  DECIMAL(10,6) DEFAULT 1.0,   -- 输出 Token 倍率
    enabled      BOOLEAN       DEFAULT TRUE,
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE(group_id, model_name)
);

CREATE INDEX idx_gmr_group_id   ON group_model_rates(group_id);
CREATE INDEX idx_gmr_model_name ON group_model_rates(model_name);
```

---

### 2.4 上游渠道表（`channels`）

> **[sub2api]** 参考 Sub2API 的 `Account` 模型设计，字段高度对齐。支持 OAuth、API Key、Setup Token 三类认证方式。

```sql
CREATE TABLE channels (
    id                BIGSERIAL PRIMARY KEY,
    group_id          BIGINT        REFERENCES groups(id),  -- 所属分组（同平台账号池）
    channel_type      VARCHAR(50)   NOT NULL,   -- openai / claude / gemini / deepseek / antigravity
    channel_name      VARCHAR(255),
    
    -- 认证信息
    auth_type         VARCHAR(50)   DEFAULT 'api_key',  -- api_key / oauth / setup_token
    api_key           TEXT        ,                      -- AES-256-GCM 加密存储
    oauth_token       TEXT        ,                      -- OAuth Token（加密存储）
    oauth_refresh_token TEXT     ,                      -- OAuth 刷新 Token（加密存储）
    oauth_expires_at  TIMESTAMPTZ,                     -- OAuth Token 过期时间
    setup_token       TEXT        ,                      -- Setup Token（加密存储）
    
    -- 上游端点
    api_base_url      VARCHAR(500)  DEFAULT '',         -- 自定义上游端点（为空则使用官方端点）
    
    -- 模型支持
    models            JSONB         NOT NULL,          -- 支持的模型列表（数组）
    
    -- 调度相关（参考 sub2api Account 模型）
    priority          INTEGER        DEFAULT 0,         -- 优先级（数值越大越优先）
    weight            INTEGER        DEFAULT 100,       -- 权重（负载均衡比例）
    concurrency       INTEGER        DEFAULT 1,         -- 账号级并发上限
    load_factor       DECIMAL(5,2)   DEFAULT 1.00,    -- 负载因子（动态调度）
    schedulable      BOOLEAN        DEFAULT TRUE,       -- 是否可参与调度
    
    -- 状态相关
    enabled           BOOLEAN        DEFAULT TRUE,
    health_status     SMALLINT       DEFAULT 1,         -- 1:健康  2:异常  3:临时禁用
    error_count       INTEGER        DEFAULT 0,         -- 连续错误计数（用于熔断）
    last_health_check TIMESTAMPTZ,
    last_error_msg    TEXT,
    
    -- 配额相关（OAuth 类型账号）
    quota_used        BIGINT         DEFAULT 0,         -- 已用配额（OAuth 类型）
    quota_total       BIGINT         DEFAULT 0,         -- 总配额（OAuth 类型，0=无限制）
    
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_type       ON channels(channel_type);
CREATE INDEX idx_channels_group_id   ON channels(group_id);
CREATE INDEX idx_channels_enabled    ON channels(enabled);
CREATE INDEX idx_channels_health     ON channels(health_status);
```

**字段说明**：

| 字段 | 说明 | 参考来源 |
|:---|:---|:---|
| `auth_type` | 支持 OAuth / API Key / Setup Token 三类 | sub2api |
| `api_key` | AES-256-GCM 加密存储，参考产品文档安全章 | 产品文档 |
| `oauth_*` 系列字段 | OAuth Token 轮转刷新，参考 sub2api `TokenRefreshService` | sub2api |
| `priority` / `weight` | 优先级+权重路由，参考 sub2api `Account` 模型 | sub2api |
| `concurrency` / `load_factor` | 账号级并发控制和动态负载因子 | sub2api |
| `schedulable` | 可调度标记，异常时自动设为 false | sub2api |
| `quota_*` 系列字段 | OAuth 类型账号的配额跟踪（参考 OpenAI Codex 场景） | sub2api |

---

### 2.5 API Key 表（`api_keys`）

> **[sub2api]** 参考 Sub2API 的 `APIKey` 模型，支持多窗口限流计数。

```sql
CREATE TABLE api_keys (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id          BIGINT        REFERENCES groups(id),  -- 可覆盖用户的分组
    
    -- Key 标识
    key_hash          VARCHAR(255)   NOT NULL UNIQUE,  -- API Key 哈希存储（SHA-256）
    key_prefix        VARCHAR(32),                     -- Key 前缀（如 `sk-abcdef`，便于识别）
    name              VARCHAR(100),                     -- 用户自定义名称
    
    -- IP 访问控制
    ip_whitelist      INET[],
    ip_blacklist      INET[],
    
    -- 配额限制
    daily_quota       BIGINT         DEFAULT -1,       -- 每日额度上限（-1=无限制）
    monthly_quota     BIGINT         DEFAULT -1,       -- 每月额度上限
    total_quota       BIGINT         DEFAULT -1,       -- 总额度上限
    
    -- 限流计数（多窗口，参考 sub2api）
    window_5h_count   INTEGER        DEFAULT 0,        -- 5 小时窗口请求计数
    window_1d_count   INTEGER        DEFAULT 0,        -- 1 天窗口请求计数
    window_7d_count   INTEGER        DEFAULT 0,        -- 7 天窗口请求计数
    window_5h_tokens  BIGINT         DEFAULT 0,        -- 5 小时窗口 Token 消耗
    window_1d_tokens  BIGINT         DEFAULT 0,        -- 1 天窗口 Token 消耗
    window_7d_tokens  BIGINT         DEFAULT 0,        -- 7 天窗口 Token 消耗
    window_reset_time  TIMESTAMPTZ,                     -- 窗口重置时间
    
    -- 状态
    enabled           BOOLEAN        DEFAULT TRUE,
    expires_at        TIMESTAMPTZ,                     -- 过期时间（NULL=永不过期）
    last_used_at      TIMESTAMPTZ,
    
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_api_keys_user_id    ON api_keys(user_id);
CREATE INDEX idx_api_keys_group_id  ON api_keys(group_id);
CREATE INDEX idx_api_keys_hash      ON api_keys(key_hash);
CREATE INDEX idx_api_keys_enabled   ON api_keys(enabled);
```

**字段说明**：

| 字段 | 说明 | 参考来源 |
|:---|:---|:---|
| `key_hash` | 哈希存储，不存储明文 Key | 产品文档安全章 |
| `key_prefix` | 支持自定义前缀，参考 sub2api | sub2api |
| `ip_whitelist`/`ip_blacklist` | Key 级 IP 访问控制 | sub2api |
| `window_*` 系列字段 | 多窗口限流计数（5h/1d/7d），参考 sub2api | sub2api |
| `daily_quota`/`monthly_quota`/`total_quota` | 多维度配额限制 | 产品文档 |

---

### 2.6 调用日志表（`usage_logs`）

> **[sub2api]** 参考 Sub2API 的 `UsageLog` 模型。**必须支持按时间分区**（参考产品文档建议）。

```sql
-- 主表（分区表父表）
CREATE TABLE usage_logs (
    id                BIGSERIAL,
    user_id           BIGINT        REFERENCES users(id),
    api_key_id        BIGINT        REFERENCES api_keys(id),
    channel_id        BIGINT        REFERENCES channels(id),
    
    request_id        VARCHAR(64),                     -- 请求唯一 ID
    model             VARCHAR(255)  NOT NULL,
    model_upstream    VARCHAR(255),                    -- 实际上游模型名称（映射后）
    
    -- Token 用量
    prompt_tokens     INTEGER        DEFAULT 0,
    completion_tokens INTEGER        DEFAULT 0,
    total_tokens      INTEGER        DEFAULT 0,
    
    -- 计费
    input_cost        BIGINT         DEFAULT 0,        -- 输入费用（额度）
    output_cost       BIGINT         DEFAULT 0,        -- 输出费用（额度）
    total_cost        BIGINT         DEFAULT 0,        -- 总费用（额度）
    
    -- 请求信息
    request_time      TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    response_time_ms  INTEGER,                         -- 上游响应耗时（毫秒）
    status_code       SMALLINT,
    error_message     TEXT,
    
    ip_address        INET,
    user_agent        TEXT,
    is_cached         BOOLEAN        DEFAULT FALSE,    -- 是否命中语义缓存
    cached_tokens     INTEGER        DEFAULT 0,        -- 缓存 Token 数量（用于折扣计费）
    
    PRIMARY KEY (id, request_time)
) PARTITION BY RANGE (request_time);

-- 创建索引（分区表索引需在每个分区上单独创建，此处为模板）
CREATE INDEX idx_usage_logs_user_time    ON usage_logs(user_id, request_time);
CREATE INDEX idx_usage_logs_api_key     ON usage_logs(api_key_id);
CREATE INDEX idx_usage_logs_channel     ON usage_logs(channel_id);
CREATE INDEX idx_usage_logs_model       ON usage_logs(model);
CREATE INDEX idx_usage_logs_request_time ON usage_logs(request_time DESC);
```

**分区策略**（参考产品文档建议，使用 TimescaleDB 或原生分区）：

```sql
-- 按月分区（自动创建下月分区）
CREATE TABLE usage_logs_202605 PARTITION OF usage_logs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');

CREATE TABLE usage_logs_202606 PARTITION OF usage_logs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- 自动分区管理（推荐 timescaledb）
SELECT create_hypertable('usage_logs', 'request_time', chunk_time_interval => INTERVAL '1 month');
```

**字段说明**：

| 字段 | 说明 | 参考来源 |
|:---|:---|:---|
| `model_upstream` | 模型映射后的实际上游模型名 | 产品文档 |
| `input_cost`/`output_cost` | 分离计费，支持不同倍率 | sub2api |
| `is_cached`/`cached_tokens` | 语义缓存折扣计费 | 产品文档 |
| `status_code` | HTTP 状态码，用于日志筛选 | 产品文档 |

---

### 2.7 兑换码表（`redeem_codes`）

> **[sub2api]** 参考 Sub2API 内置支付系统的兑换码功能。

```sql
CREATE TABLE redeen_codes (
    id             BIGSERIAL PRIMARY KEY,
    code           VARCHAR(255)   NOT NULL UNIQUE,
    code_type      VARCHAR(50)    NOT NULL,  -- fixed（固定额度）/ discount（折扣）/ full_reduction（满减）
    
    -- 额度相关
    amount         BIGINT         DEFAULT 0,     -- 固定额度（额度单位）
    discount_rate  DECIMAL(5,4)   DEFAULT 1.0,  -- 折扣率（0.8 = 8折）
    min_amount     BIGINT         DEFAULT 0,     -- 满减最低消费（额度）
    
    -- 状态
    status         SMALLINT       DEFAULT 1,     -- 1:未兑换  2:已兑换  3:已过期  4:已作废
    used_by        BIGINT         REFERENCES users(id),
    used_at        TIMESTAMPTZ,
    
    -- 限制
    max_uses       INTEGER        DEFAULT 1,     -- 最大兑换次数（1=仅限1人兑换）
    used_count     INTEGER        DEFAULT 0,     -- 已兑换次数
    valid_from     TIMESTAMPTZ,
    valid_until    TIMESTAMPTZ,
    
    created_by     BIGINT         REFERENCES users(id),  -- 创建者（管理员）
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_redeem_codes_code    ON redeen_codes(code);
CREATE INDEX idx_redeem_codes_status ON redeen_codes(status);
```

---

### 2.8 系统配置表（`system_configs`）

```sql
CREATE TABLE system_configs (
    id           BIGSERIAL PRIMARY KEY,
    config_key   VARCHAR(255)   NOT NULL UNIQUE,
    config_value TEXT,
    description  TEXT,
    updated_at   TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
```

**预置配置项**：

| config_key | 说明 | 默认值 |
|:---|:---|:---|
| `site_name` | 站点名称 | `TraceHub` |
| `site_logo` | 站点 Logo URL | |
| `default_user_balance` | 新用户默认余额（额度） | `0` |
| `default_user_concurrency` | 新用户默认并发上限 | `1` |
| `api_key_prefix` | API Key 默认前缀 | `sk-th-` |
| `rate_multiplier` | 全局费率倍率 | `1.0` |
| `billing.circuit_breaker` | 计费异常熔断 | `true` |
| `allow_insecure_http` | 允许 HTTP 上游（**生产禁止**） | `false` |
| `cors_allowed_origins` | CORS 白名单 | `*` |
| `smtp_host` | SMTP 服务器地址 | |
| `smtp_port` | SMTP 端口 | `587` |
| `smtp_username` | SMTP 用户名 | |
| `smtp_password` | SMTP 密码（加密存储） | |
| `stripe_publishable_key` | Stripe 公钥 | |
| `stripe_secret_key` | Stripe 私钥（加密存储） | |
| `alipay_app_id` | 支付宝 App ID | |
| `alipay_private_key` | 支付宝私钥（加密存储） | |
| `wxpay_mch_id` | 微信支付商户号 | |
| `wxpay_api_key` | 微信支付 API Key（加密存储） | |

---

### 2.9 仪表盘聚合表（`dashboard_aggregations`）

> **[sub2api]** 参考 Sub2API 的 `DashboardAggregationRepository`，使用原生 SQL 做聚合查询。

```sql
CREATE TABLE dashboard_aggregations (
    id                  BIGSERIAL PRIMARY KEY,
    agg_date            DATE           NOT NULL,       -- 聚合日期
    agg_type            VARCHAR(50)    NOT NULL,       -- daily / weekly / monthly
    
    -- 调用统计
    total_requests      BIGINT         DEFAULT 0,
    total_tokens        BIGINT         DEFAULT 0,
    total_cost          BIGINT         DEFAULT 0,       -- 总成本（额度）
    
    -- 用户统计
    active_users        INTEGER        DEFAULT 0,       -- 活跃用户数
    new_users          INTEGER        DEFAULT 0,       -- 新增用户数
    
    -- 模型统计（JSONB 存储各模型的调用量和费用）
    model_stats         JSONB,
    
    -- 渠道统计（JSONB 存储各渠道的调用量和费用）
    channel_stats       JSONB,
    
    created_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    UNIQUE(agg_date, agg_type)
);
```

---

## 三、索引策略

### 3.1 高频查询索引

```sql
-- 用户相关
CREATE INDEX idx_users_email     ON users(email);
CREATE INDEX idx_users_group_id  ON users(group_id);

-- API Key 相关
CREATE INDEX idx_api_keys_hash     ON api_keys(key_hash);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

-- 调用日志（最高频查询）
CREATE INDEX idx_usage_logs_user_time    ON usage_logs(user_id, request_time DESC);
CREATE INDEX idx_usage_logs_api_key     ON usage_logs(api_key_id);
CREATE INDEX idx_usage_logs_channel     ON usage_logs(channel_id);
CREATE INDEX idx_usage_logs_model_time ON usage_logs(model, request_time DESC);

-- 渠道相关
CREATE INDEX idx_channels_type      ON channels(channel_type);
CREATE INDEX idx_channels_health   ON channels(health_status);
```

### 3.2 分区表索引说明

> **重要**：PostgreSQL 分区表的索引需要在**每个分区**上单独创建。推荐使用 `pg_partman` 扩展自动管理分区和索引。

```sql
-- 使用 pg_partman 自动管理分区
CREATE EXTENSION IF NOT EXISTS pg_partman;

-- 配置自动分区（按月分区）
SELECT partman.create_parent(
    'public.usage_logs',
    'request_time',
    'native',
    'monthly'
);
```

---

## 四、Ent Schema 设计（代码层）

> **[sub2api]** 参考 Sub2API 的 `backend/ent/schema/` 目录结构，使用 Ent 定义 Schema，自动生成数据库表结构。

### 4.1 Schema 文件结构

```
internal/ent/schema/
├── user.go              # User Schema
├── apikey.go            # APIKey Schema
├── group.go             # Group Schema
├── groupmodelrate.go    # GroupModelRate Schema
├── channel.go           # Channel Schema
├── usagelog.go         # UsageLog Schema
├── redeen_code.go     # RedeemCode Schema
├── systemconfig.go      # SystemConfig Schema
└── dashboardagg.go     # DashboardAggregation Schema
```

### 4.2 示例：Channel Schema（Ent 版本）

```go
// internal/ent/schema/channel.go

package schema

import (
    "time"
    "github.com/weaviate/weaviate/entities/schema"
    "entgo.io/ent"
    "entgo.io/ent/schema/field"
    "entgo.io/ent/schema/index"
)

type Channel struct {
    ent.Schema
}

func (Channel) Fields() []ent.Field {
    return []ent.Field{
        field.String("channel_type").NotEmpty(),
        field.String("channel_name").Optional(),
        
        // 认证
        field.Enum("auth_type").
            Values("api_key", "oauth", "setup_token").
            Default("api_key"),
        field.Text("api_key").Optional().Sensitive(),
        field.Text("oauth_token").Optional().Sensitive(),
        field.Text("oauth_refresh_token").Optional().Sensitive(),
        field.Time("oauth_expires_at").Optional(),
        field.Text("setup_token").Optional().Sensitive(),
        
        // 上游端点
        field.String("api_base_url").Default("").Optional(),
        
        // 模型支持（JSON 数组）
        field.JSON("models", []string{}).Default("[]"),
        
        // 调度
        field.Int("priority").Default(0),
        field.Int("weight").Default(100),
        field.Int("concurrency").Default(1),
        field.Float("load_factor").Default(1.0),
        field.Bool("schedulable").Default(true),
        
        // 状态
        field.Bool("enabled").Default(true),
        field.Int("health_status").Default(1),  // 1:健康 2:异常
        field.Int("error_count").Default(0),
        field.Time("last_health_check").Optional(),
        field.Text("last_error_msg").Optional(),
        
        // OAuth 配额
        field.Int64("quota_used").Default(0),
        field.Int64("quota_total").Default(0),
        
        field.Time("created_at").Default(time.Now),
        field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
    }
}

func (Channel) Edges() []ent.Edge {
    return []ent.Edge{
        edge.From("group", Group.Type).
            Ref("channels").
            Field("group_id").
            Unique(),  // 一个分组有多个渠道，一个渠道属于一个分组
    }
}

func (Channel) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("channel_type"),
        index.Fields("enabled"),
        index.Fields("health_status"),
    }
}
```

---

## 五、迁移管理

### 5.1 Ent 自动迁移

```go
// cmd/server/main.go

func main() {
    client, err := ent.Open("postgres", dsn)
    if err != nil {
        log.Fatalf("failed opening connection: %v", err)
    }
    defer client.Close()
    
    // 自动迁移（开发环境）
    if err := client.Schema.Create(context.Background()); err != nil {
        log.Fatalf("failed creating schema resources: %v", err)
    }
    
    // 生产环境建议使用版本化迁移文件
    // 执行: go generate ./ent 生成迁移 SQL
}
```

### 5.2 版本化迁移（生产推荐）

```
scripts/migrate/
├── 001_init_schema.up.sql        # 初始 Schema
├── 001_init_schema.down.sql
├── 002_add_channel_models.up.sql  # 新增字段迁移
├── 002_add_channel_models.down.sql
└── ...
```

---

## 六、Redis 缓存/协调设计

> **[sub2api]** 参考 Sub2API 的 Redis 使用方式，分为缓存类和协调类两类。

### 6.1 缓存键设计

| 键名模式 | 类型 | 说明 | 过期时间 |
|:---|:---:|:---|:---:|
| `api_key:{hash}` | String | API Key 认证信息缓存 | 1 小时 |
| `user:{id}:balance` | String | 用户余额缓存 | 5 分钟 |
| `billing:{user_id}` | String | 计费预扣缓存 | 请求生命周期 |
| `dashboard:{type}:{date}` | String | 仪表盘统计数据缓存 | 1 小时 |
| `sticky_session:{key}` | String | 粘性会话绑定（账号 ID） | 1 小时 |

### 6.2 协调键设计

| 键名模式 | 类型 | 说明 |
|:---|:---:|:---|
| `concurrency:{user_id}` | Semaphore | 用户级并发槽 |
| `concurrency:{channel_id}` | Semaphore | 渠道级并发槽 |
| `rate_limit:{api_key_id}:{window}` | String | API Key 级限流计数 |
| `temp_unsched:{channel_id}` | String | 临时禁用调度锁 |
