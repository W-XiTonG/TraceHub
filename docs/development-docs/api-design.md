# TraceHub API 接口设计文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**Base URL**：`https://api.tracehub.com/v1`（可配置）  
**认证方式**：`Authorization: Bearer <token_or_api_key>`

---

## 一、认证机制

### 1.1 API Key 认证（推荐使用）

```
Authorization: Bearer <api_key>
```

- API Key 通过用户中心生成
- 支持 IP 白名单限制
- 支持多窗口限流（5h/1d/7d）

### 1.2 JWT 认证（管理后台使用）

```
Authorization: Bearer <jwt_token>
```

- 登录后返回 `access_token` 和 `refresh_token`
- `access_token` 有效期 72 小时（可配）
- `refresh_token` 用于刷新令牌

---

## 二、网关 API（对外提供 AI 服务）

> **[sub2api]** 这些端点参考 Sub2API 的网关 API 设计，保持 OpenAI 兼容格式。

### 2.1 聊天补全 `POST /v1/chat/completions`

**描述**：OpenAI 兼容的聊天补全接口，支持流式和非流式，支持多模型切换。

**请求示例**：

```json
POST /v1/chat/completions
Authorization: Bearer sk-th-abc123
Content-Type: application/json

{
  "model": "gpt-4o",
  "messages": [
    {"role": "user", "content": "你好，请介绍一下自己"}
  ],
  "stream": false,
  "temperature": 0.7,
  "max_tokens": 1024
}
```

**关键请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|:---|:---:|:---:|:---|
| `model` | string | ✅ | 模型名称（如 `gpt-4o`, `claude-3-5-sonnet-20241022`, `gemini-2.0-flash`） |
| `messages` | array | ✅ | 对话消息列表 |
| `stream` | boolean | ❌ | 是否流式返回（SSE），默认 `false` |
| `temperature` | number | ❌ | 温度参数，0-2，默认 1 |
| `max_tokens` | integer | ❌ | 最大输出 Token |
| `tools` | array | ❌ | 工具定义（Function Calling） |
| `tool_choice` | string/object | ❌ | 工具选择策略 |
| `response_format` | object | ❌ | 响应格式（如 `{"type": "json_object"}`） |

**SSE 流式响应示例**：

```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1715123456,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"你好"}}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1715123456,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"！"}}]}

data: [DONE]
```

**响应示例**（非流式）：

```json
{
  "id": "chatcmpl-xxx",
  "object": "chat.completion",
  "created": 1715123456,
  "model": "gpt-4o",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "你好！我是 AI 助手..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 128,
    "total_tokens": 143
  }
}
```

> **[sub2api]** 支持粘性会话：同一会话的连续请求会自动路由到同一上游账号，通过请求头 `X-Sticky-Session: true` 启用。

---

### 2.2 文本补全 `POST /v1/completions`（Legacy）

```json
POST /v1/completions
{
  "model": "gpt-3.5-turbo-instruct",
  "prompt": "Once upon a time",
  "max_tokens": 50
}
```

> 保留接口，兼容旧版 OpenAI API。

---

### 2.3 获取模型列表 `GET /v1/models`

**描述**：返回当前 API Key 可用的模型列表。

**响应示例**：

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-4o",
      "object": "model",
      "created": 1715123456,
      "owned_by": "openai",
      "permission": []
    },
    {
      "id": "claude-3-5-sonnet-20241022",
      "object": "model",
      "created": 1715123456,
      "owned_by": "anthropic",
      "permission": []
    }
  ]
}
```

> **[sub2api]** 模型列表由分组配置和渠道配置共同决定，仅返回用户有权访问的模型。

---

### 2.4 文本向量 `POST /v1/embeddings`

```json
POST /v1/embeddings
{
  "model": "text-embedding-3-small",
  "input": "The food was delicious and the waiter..."
}
```

**响应示例**：

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "embedding": [0.0023064255, -0.009327292, ...],
      "index": 0
    }
  ],
  "model": "text-embedding-3-small",
  "usage": {
    "prompt_tokens": 8,
    "total_tokens": 8
  }
}
```

---

### 2.5 图片生成 `POST /v1/images/generations`（可选扩展）

```json
POST /v1/images/generations
{
  "model": "dall-e-3",
  "prompt": "A cute cat sitting on a sofa",
  "n": 1,
  "size": "1024x1024"
}
```

---

### 2.6 视频生成（OpenAI Videos API）

与 [OpenAI Video generation](https://developers.openai.com/api/docs/guides/video-generation) 对齐。选路模型名：创建请求体中的 `model`；查询/下载/删除需 Query `model=…` 或请求头 `X-TraceHub-Model`。

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/videos` | 创建异步视频任务（JSON 或 `multipart/form-data`，含 `input_reference` 图生视频） |
| GET | `/v1/videos/{video_id}` | 查询任务状态（`queued` / `in_progress` / `completed` / `failed`） |
| GET | `/v1/videos/{video_id}/content` | 下载成品视频（`video/mp4` 等） |
| DELETE | `/v1/videos/{video_id}` | 删除任务 |
| GET | `/v1/videos` | 列表（需 `model` 选路） |

**渠道**：`openai` / `deepseek` / `site.openai_compat_channel_types` 直转上游 `/videos`；`volcengine`（火山方舟）经网关字段适配后转发，默认 `api_base_url` 为 `https://ark.cn-beijing.volces.com/api/v3`。

**计费**：`POST /v1/videos` 按模型+时长预扣；`GET /v1/videos/{id}` 在 `status=completed` 时读取上游 `usage.seconds` 或 `seconds`/`duration` 精调（补扣/退还差额）；Redis 键 `tracehub:video:job:{user_id}:{video_id}` 保证同一任务只结算一次；轮询与 `content` 下载不重复扣费。

```json
POST /v1/videos
{
  "model": "sora-2",
  "prompt": "A cat walking in the rain",
  "seconds": "8",
  "size": "1280x720"
}
```

**查询 / 下载 / 删除示例**：

```http
GET /v1/videos/{video_id}?model=sora-2
Authorization: Bearer sk-th-xxx
X-TraceHub-Model: sora-2

GET /v1/videos/{video_id}/content?model=sora-2
DELETE /v1/videos/{video_id}?model=sora-2
```

创建响应与 OpenAI Video 对象类似（含 `id`、`status` 等）；`volcengine` 渠道下网关会将上游方舟任务响应规范化（如 `task_id` → `id`）。

#### 2.6.1 渠道类型与协议对照（管理端配置）

选路依据请求中的 **逻辑 `model` 名** 与渠道 `models` / `model_alias`，**不依据** `api_base_url`。`api_base_url` 留空时使用下表默认根地址（可在渠道上覆盖）。

| `channel_type` | 默认 Base URL（可覆盖） | 对话 `chat/completions` | 视频 `/v1/videos` | 上游视频协议 |
|----------------|-------------------------|-------------------------|-------------------|--------------|
| `openai` | `https://api.openai.com/v1` | OpenAI 兼容转发 | 直转 `/videos` | OpenAI Videos |
| `deepseek` | `https://api.deepseek.com/v1` | OpenAI 兼容转发 | 直转 `/videos`（若上游支持） | OpenAI Videos |
| `claude` | `https://api.anthropic.com/v1` | Messages API 适配 | 不支持 | — |
| `gemini` | `https://generativelanguage.googleapis.com` | generateContent 适配 | 不支持 | — |
| `volcengine` | `https://ark.cn-beijing.volces.com/api/v3` | OpenAI 兼容转发（`/api/v3/chat/completions` 等） | 网关转换为方舟 `contents/generations/tasks` | 方舟任务 API |
| `openai_compat`（`site.openai_compat_channel_types`） | 须在渠道填写 | OpenAI 兼容转发 | 直转 `/videos`（取决于上游） | 取决于上游 |

管理端：`GET /api/admin/channel-types` 返回各类型元数据（默认 URL、能力标签、运营提示）；`GET /api/admin/channel-model-presets?channel_type=` 含相同字段及推荐模型列表。

---

### 2.7 音频（OpenAI Audio API）

与 [OpenAI Audio](https://platform.openai.com/docs/api-reference/audio) 对齐，由网关按渠道类型转发至上游对应路径。

| 方法 | 路径 | Content-Type | 说明 |
|------|------|----------------|------|
| POST | `/v1/audio/speech` | `application/json` | 语音合成（TTS），请求体含 `model`、`input`（文本）、`voice` 等 |
| POST | `/v1/audio/transcriptions` | `multipart/form-data` | 语音转文字，字段含 `file`、`model`，可选 `prompt`、`language` |
| POST | `/v1/audio/translations` | `multipart/form-data` | 语音翻译为英文文本，字段与转写类似 |

**选路**：请求 JSON 或 multipart 中的 `model`；无 `model` 时可用 Query `?model=` 或头 `X-TraceHub-Model`。

**示例（TTS）**：

```json
POST /v1/audio/speech
{
  "model": "tts-1",
  "input": "你好，世界",
  "voice": "alloy"
}
```

成功时响应体为音频二进制（如 `audio/mpeg`），非 JSON。

**示例（转写）**：

```http
POST /v1/audio/transcriptions
Content-Type: multipart/form-data

model=whisper-1
file=@sample.mp3
```

**计费**：按部署与分组模型倍率规则从余额扣费；具体单价以管理端计费配置为准。

---

### 2.8 火山方舟：视频与 3D（`volcengine` 渠道）

当请求选路至 **`channel_type = volcengine`** 时，`/v1/videos` 在网关内转换为方舟 **`POST /api/v3/contents/generations/tasks`**（非 OpenAI 原生 `/videos`）。对话、`embeddings` 等其它 OpenAI 兼容接口则直转方舟 **`/api/v3/*`**（与 `openai` 渠道相同 relay 实现）。对外仍使用 **§2.6** 的路径与 API Key，无需单独「火山专用」Base URL。Realtime、Responses 等是否可用取决于方舟账号与模型开通情况。

#### 2.8.1 模型 ID

- 请求与渠道配置请使用方舟 **Model ID**（如 `doubao-seedance-2-0-260128`），与 `GET https://ark.cn-beijing.volces.com/api/v3/models` 返回的 `id` 一致。
- 上游 JSON 中的短字段 **`name`**（如 `doubao-seedance-2-0`）仅作展示；若要用短名调用，请在渠道配置 **`model_alias`**（键为逻辑名，值为完整 `id` 或 `ep-` 接入点）。

#### 2.8.2 Seedance 视频（文生视频 / 图生视频）

**OpenAI 形态（网关自动转换）**：

```json
POST /v1/videos
{
  "model": "doubao-seedance-1-5-pro-251215",
  "prompt": "一只猫在雨中行走",
  "seconds": "8",
  "size": "1280x720"
}
```

**方舟原生 `content[]`（推荐 Seedance 2.0 多模态）**：若 body 已含非空 `content` 数组，网关透传并补全兼容字段，例如：

```json
{
  "model": "doubao-seedance-2-0-260128",
  "content": [
    { "type": "text", "text": "镜头缓慢推进" },
    {
      "type": "image_url",
      "image_url": { "url": "https://example.com/ref.jpg" },
      "role": "first_frame"
    }
  ],
  "duration": 11,
  "ratio": "16:9",
  "generate_audio": true
}
```

**图生视频（参考图）**：
- **JSON**：在 `image_url` 或 `content` 内提供公网可访问的图片 URL（推荐）。
- **multipart `input_reference`（试玩等）**：网关将本地上传编码后写入请求；普通 Seedance 转为 `image_url`（data URL），Seed3D 转为 `content` 内 `image_url`。部分模型仍建议优先使用公网 URL。

#### 2.8.3 Seed3D 图生 3D（`doubao-seed3d-*`）

3D 生成**必须提供参考图**；若携带 `duration`、`resolution`、`ratio`、`seconds`、`size` 等视频参数，上游可能报错，请按模型说明 omit。

对外仍调用 **`POST /v1/videos`**，`model` 填 Seed3D 的 Model ID，例如 `doubao-seed3d-2-0-260328`。

**方式 A：JSON + 公网图片 URL**

```json
{
  "model": "doubao-seed3d-2-0-260328",
  "prompt": "生成卡通小狗 --fileformat glb",
  "image_url": "https://example.com/reference.png"
}
```

**方式 B：JSON + 原生 content**

```json
{
  "model": "doubao-seed3d-2-0-260328",
  "content": [
    { "type": "text", "text": "生成卡通小狗 --fileformat glb" },
    {
      "type": "image_url",
      "image_url": { "url": "https://example.com/reference.png" }
    }
  ]
}
```

**方式 C：multipart 本地上传**（试玩 / 部分客户端）

```http
POST /v1/videos
Content-Type: multipart/form-data

model=doubao-seed3d-2-0-260328
prompt=生成卡通小狗 --fileformat glb
input_reference=@local.jpg
```

网关将文件编码为 data URL 并写入方舟要求的 `content`（`text` + `image_url`）。

**prompt 可选参数**（写入 `text` 或 `prompt` 字符串，以方舟文档为准）：如 `--fileformat glb|obj|usd|usdz`、`--subdivisionlevel` 等。

#### 2.8.4 任务查询与下载

与 §2.6 相同：`GET /v1/videos/{id}`、`GET .../content`、`DELETE`；Query 或头中须带选路 `model`。

完成后 `content` 下载行为依赖上游返回（视频为 `content.video_url`；3D 产物类型以方舟为准）。

#### 2.8.5 常见错误

| 上游提示 | 含义 | 处理 |
|----------|------|------|
| `not activated` | 账号未开通该 Model ID 的 API | 在方舟控制台开通；确认渠道 API Key 与 curl 测试为同一账号 |
| `duration` / `resolution` must be empty | 对 Seed3D 误传了视频参数 | 去掉 `seconds`、`size` 等 |
| missing `content` | Seed3D body 格式不正确 | 使用 §2.8.3 的 `content` 或带图 JSON |
| `image_data` / 参考图相关 | 未提供图片或格式不对 | 提供公网 URL、base64，或 multipart `input_reference` |

网关错误响应可能含 `X-TraceHub-Routing-Model`、`X-TraceHub-Upstream-Model`、`X-TraceHub-Channel-Id` 便于排查。

---

## 三、用户 API（用户中心接口）

> **认证方式**：JWT（`Authorization: Bearer <jwt_token>`）或 API Key

### 3.1 认证相关

#### 3.1.1 用户注册 `POST /api/user/register`

```json
POST /api/user/register
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "username": "testuser"  // 可选
}
```

**响应**：

```json
{
  "code": 0,
  "message": "注册成功，请登录",
  "data": null
}
```

#### 3.1.2 用户登录 `POST /api/user/login`

```json
POST /api/user/login
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**响应**：

```json
{
  "code": 0,
  "message": "登录成功",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_in": 259200,  // 72 小时（秒）
    "user": {
      "id": 1,
      "email": "user@example.com",
      "username": "testuser",
      "balance": 50000,  // 额度
      "group_name": "基础分组"
    }
  }
}
```

#### 3.1.3 刷新 Token `POST /api/user/refresh`

```json
POST /api/user/refresh
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIs..."
}
```

#### 3.1.4 启用 TOTP `POST /api/user/totp/enable`（可选增强）

支持二次验证，参考 sub2api 的 TOTP 实现。

---

### 3.2 API Key 管理

> **[sub2api]** 参考 Sub2API 的 APIKey 管理能力，支持自定义前缀、IP 白名单等。

#### 3.2.1 获取 API Key 列表 `GET /api/user/apikeys`

**响应**：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1,
        "name": "生产环境 Key",
        "key_prefix": "sk-th-abc123...",
        "group_name": "基础分组",
        "status": "enabled",
        "last_used_at": "2026-05-09T10:30:00Z",
        "created_at": "2026-05-01T08:00:00Z",
        "usage_24h": 12345  // 最近 24h 消耗 Token
      }
    ],
    "total": 1
  }
}
```

#### 3.2.2 创建 API Key `POST /api/user/apikeys`

```json
POST /api/user/apikeys
{
  "name": "生产环境 Key",
  "group_id": 1,           // 可选，覆盖用户默认分组
  "ip_whitelist": ["180.200.134.171/24"],  // 可选
  "daily_quota": 100000,   // 可选，每日额度上限
  "expires_at": "2026-12-31T23:59:59Z"  // 可选
}
```

**响应**（**注意**：`key` 字段仅在创建时返回一次，需妥善保存）：

```json
{
  "code": 0,
  "message": "API Key 创建成功",
  "data": {
    "id": 2,
    "name": "生产环境 Key",
    "key": "sk-th-abc123def456...",  // ⚠️ 仅创建时返回明文
    "key_prefix": "sk-th-abc123...",
    "created_at": "2026-05-09T14:00:00Z"
  }
}
```

#### 3.2.3 删除 API Key `DELETE /api/user/apikeys/:id`

#### 3.2.4 更新 API Key `PUT /api/user/apikeys/:id`

支持更新 `name`、`ip_whitelist`、`daily_quota`、`enabled` 等字段。

---

### 3.3 用户余额与用量

#### 3.3.1 查询余额 `GET /api/user/balance`

```json
{
  "code": 0,
  "data": {
    "balance": 48500,       // 剩余额度
    "currency_unit": 10000,  // 1 元 = 10000 额度
    "balance_yuan": 4.85,   // 人民币余额
    "group_name": "基础分组",
    "rate_multiplier": 1.0
  }
}
```

#### 3.3.2 查询用量统计 `GET /api/user/usage`

**请求参数**：

| 参数 | 类型 | 说明 |
|:---|:---:|:---|
| `period` | string | `today` / `7d` / `30d` / `all` |
| `model` | string | 按模型筛选（可选） |

**响应示例**：

```json
{
  "code": 0,
  "data": {
    "period": "7d",
    "total_requests": 1234,
    "total_tokens": 567890,
    "total_cost": 12.35,  // 元
    "daily_breakdown": [
      {"date": "2026-05-03", "requests": 150, "cost": 1.80},
      {"date": "2026-05-04", "requests": 200, "cost": 2.50},
      ...
    ],
    "model_breakdown": [
      {"model": "gpt-4o", "requests": 800, "cost": 10.50},
      {"model": "claude-3-5-sonnet-20241022", "requests": 434, "cost": 1.85}
    ]
  }
}
```

#### 3.3.3 导出用量明细 `GET /api/user/usage/export`

支持 `format=csv` 或 `format=json` 参数。

---

### 3.4 兑换码

#### 3.4.1 兑换 `POST /api/user/redeem`

```json
POST /api/user/redeem
{
  "code": "TH-ABC123-XYZ789"
}
```

**响应**：

```json
{
  "code": 0,
  "message": "兑换成功",
  "data": {
    "amount_added": 50000,  // 额度
    "new_balance": 98500
  }
}
```

---

### 3.5 账户设置

#### 3.5.1 修改密码 `PUT /api/user/password`

#### 3.5.2 绑定邮箱/手机号 `PUT /api/user/bind`

#### 3.5.3 设置 IP 白名单 `PUT /api/user/ip_whitelist`

---

## 四、管理员 API

> **认证方式**：JWT（需 `role=admin` 或 `role=superadmin`）  
> **[sub2api]** 参考 Sub2API 的管理后台 API 设计，功能对等或增强。

### 4.1 用户管理

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/users` | GET | 用户列表（支持搜索/筛选） |
| `/api/admin/users/:id` | GET | 用户详情 |
| `/api/admin/users/:id` | PUT | 更新用户（余额/分组/状态） |
| `/api/admin/users/:id/balance` | POST | 手动调整余额（充值/扣款） |
| `/api/admin/users/:id/ban` | POST | 禁用用户 |
| `/api/admin/users/:id/unban` | POST | 启用用户 |
| `/api/admin/users/batch-balance` | POST | 批量调整余额 |

**用户列表响应示例**：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1,
        "email": "user@example.com",
        "username": "testuser",
        "group_name": "基础分组",
        "balance": 48500,
        "status": 1,
        "total_requests": 1234,
        "created_at": "2026-05-01T08:00:00Z",
        "last_login_at": "2026-05-09T10:30:00Z"
      }
    ],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

---

### 4.2 分组管理

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/groups` | GET | 分组列表 |
| `/api/admin/groups` | POST | 创建分组 |
| `/api/admin/groups/:id` | PUT | 更新分组 |
| `/api/admin/groups/:id` | DELETE | 删除分组 |
| `/api/admin/groups/:id/model-rates` | GET | 分组模型倍率列表（含 `billable`） |
| `/api/admin/groups/:id/model-rates` | PUT | 批量更新分组模型倍率；每项可含 `billable`（默认 `true` 表示计费，`false` 表示该分组下该模型不计费） |
| `/api/admin/global-model-pricing/workspace` | GET | 管理端合并视图：模型 ID 为 **代码内置 `DefaultQuotaPer1K` 键 ∪ 全站已启用渠道声明（`models` / `model_alias`）∪ 模型市场 visible+enabled ∪ 已有 `global_model_pricings` 行** 的并集（排序）。每项含 `code_in_per_1k` / `code_out_per_1k`（`billing.LookupPricing`）、`sources`（`builtin` / `channel` / `catalog` / `global_db` 子集）、`global`（若库中有该行则嵌入完整记录，否则省略）。**查询参数**：`q`（可选，`model_name` 子串，不区分大小写）、`page`（默认 1）、`page_size`（默认 50，最大 200）。**响应**：`items`（当前页）、`total`（筛选后条数）、`page`（越界时校正后的页码）、`page_size`、`q`（回显已应用筛选词）。 |
| `/api/admin/global-model-pricing/export` | GET | 下载 UTF-8 CSV（带 BOM）。**仅数据列**：`model_name`、`enabled`、`quota_*`、`input_rate`、`output_rate`、可选 `markup_percent`（行内加成，空=全站默认）。**带 `currency`+`denom`**：与导入模板列一致，`upstream_margin_percent` 导出行内加成；反算上游价按 margin=0（成本额度）。 |
| `/api/admin/global-model-pricing/export/template` | GET | 导入模板 CSV。Query：`currency`=`cny`\|`usd`，`denom`（或 `denominator_tokens`）=`1`\|`1000`\|`1000000`。含 `upstream_*` 列且预填币种/粒度；`quota_*` 留空；若全局库已有额度则按 margin=0 反算参考上游价。 |
| `/api/admin/site-settings/reset-currency-pricing` | POST | 删除 `site.currency_quota_per_yuan`、`site.currency_quota_per_usd`、`site.pricing_cny_per_usd`、`site.default_pricing_markup_percent` 的 system_configs 行，换算与默认加成回退 YAML。`GET /api/admin/site-settings` 响应含 `default_pricing_markup_percent`（及 file/db 元数据）。 |
| `/api/admin/global-model-pricing/import/preview` | POST | 与 `import` 相同；**仅解析校验**。`items` 含可选 `markup_percent`。`upstream_*` 换算额度时 **margin 固定为 0**（成本基数）；`upstream_margin_percent` 写入行内 `markup_percent`。 |
| `/api/admin/global-model-pricing/import` | POST | `multipart/form-data` 字段 **`file`**（CSV）。表头须含 `model_name`、`enabled`、`input_rate`、`output_rate`、`quota_in_per_1k`、`quota_out_per_1k`（建议直接使用导出文件；额度列可留空表示清空覆盖，或与 `upstream_currency` 等配合由服务端换算）；`enabled` 支持 true/false、1/0、是/否 等。最多 **5000** 条数据行；文件最大 8MB。成功返回 `{ count }` 并批量 upsert（与 PUT 语义一致）。 |
| `/api/admin/global-model-pricing/derive-quota` | POST | JSON：`currency`（`cny` \| `usd`）、`denominator_tokens`（`1` \| `1000` \| `1000000`）、`price_in`（非负）、`price_out`（可选，缺省则与 `price_in` 相同）、`margin_percent`（可选，如 `5` 表示在基准额度上乘以 1.05，向上取整到整数额度）。成功返回 `quota_in_per_1k`、`quota_out_per_1k` 及 `resolved`（换算说明字符串）。美元路径：优先 `site.currency_quota_per_usd`；否则使用 `site.pricing_cny_per_usd`（1 USD = R CNY）× `site.currency_quota_per_yuan`。 |
| `/api/admin/global-model-pricing` | GET | 全站模型计价列表（独立表 `global_model_pricings`，含 `enabled=false` 行） |
| `/api/admin/global-model-pricing` | PUT | 批量 upsert；每项含可选 `markup_percent`（`null`/省略表示清除行内覆盖、用全站默认）、`billing_mode`（默认 `token`）、`quota_per_request`（`per_request` / `hybrid_floor` 时必填 >0）。**扣费**：`token` 模式走 `ModelBillRates.Cost`；`per_request` 为固定 `RequestCost()`；`hybrid_floor` 为 `max(RequestCost, Cost(tokens))`；均乘行内/全站加成与分组、用户倍率。 |
| `/api/admin/global-model-pricing` | DELETE | 查询参数 `model_name`：删除一条全站模型计价配置 |
| `/api/admin/channel-model-pricing/workspace` | GET | **渠道**合并视图：行集合 = 指定渠道的 `models` ∪ `model_alias` 键 ∪ 已有 `channel_model_pricings` 行。**Query**：`channel_id`（必填）、`q`、`page`、`page_size`（同全局 workspace）。每项含 `code_in_per_1k` / `code_out_per_1k`、`sources`（`channel_declared` / `channel_db`）、`channel`（库行）、`global`（同模型全站行，供对照）。 |
| `/api/admin/channel-model-pricing` | PUT | 批量 upsert；每项须含 `channel_id`、`model_name` 及与全局相同的计价字段。 |
| `/api/admin/channel-model-pricing` | DELETE | Query：`channel_id`、`model_name` |
| `/api/admin/channel-model-pricing/export` | GET | 单渠道 CSV 导出。**Query**：`channel_id`（必填）；可选 `currency`+`denom`（语义同全局 export）。表头与全局一致（无 `channel_id` 列）。 |
| `/api/admin/channel-model-pricing/export/template` | GET | 单渠道导入模板。**Query**：`channel_id`、`currency`、`denom`（后两者必填，同全局 template）。 |
| `/api/admin/channel-model-pricing/import/preview` | POST | 单渠道 CSV 预览。**Query**：`channel_id`；`multipart` 字段 `file`。 |
| `/api/admin/channel-model-pricing/import` | POST | 单渠道 CSV 直接落库（同 PUT 语义）。 |
| `/api/admin/channel-model-pricing/export/batch` | GET | 全部渠道合并导出；表头含 `channel_id`、`channel_name` 及计价列（或含 `upstream_*`）。**总行数上限 5000**。 |
| `/api/admin/channel-model-pricing/export/template/batch` | GET | 全部渠道导入模板；表头含 `channel_id` 与 `upstream_*`；`currency`、`denom` 必填。 |
| `/api/admin/channel-model-pricing/import/preview/batch` | POST | 多渠道 CSV 预览；表头须含 `channel_id` 列。 |
| `/api/admin/channel-model-pricing/import/batch` | POST | 多渠道 CSV 直接落库。 |

**扣费优先级**（`billing.ModelBillRates`）：分组 `GroupModelRate`（含 `billable`）> 渠道 `channel_model_pricings`（非空字段覆盖）> 全站 `global_model_pricings` > 代码内建 `DefaultQuotaPer1K` / `DefaultQuotaPerRequest`。`billing_mode` 与 `quota_per_request` 由全站行定义、渠道非空字段覆盖。网关在 `SelectChannel` 之后传入 `channel_id` 取价；`per_request` 模型预检跳过 tiktoken 估算。

全局「免计费模型」由 `site.no_bill_models` 提供：配置文件 `site.no_bill_models`（YAML 字符串数组）与/或 `PUT /api/admin/config` 写入键 `site.no_bill_models`（JSON 字符串数组）；聚合展示见 `GET /api/admin/site-settings`，管理端在「计费 → 不计费模型」中编辑。

**创建分组请求示例**：

```json
POST /api/admin/groups
{
  "name": "Pro 分组",
  "description": "Pro 用户，享受 0.8x 折扣",
  "rate_multiplier": 0.8,
  "rpm_limit": 500,
  "model_routing": {"gpt-4o": [1, 3]},  // 指定模型路由到渠道 1 和 3
  "enabled": true
}
```

---

### 4.3 上游渠道管理

> **[sub2api]** 参考 Sub2API 的 Account 管理能力，支持 OAuth/API Key/Setup Token 三类认证方式。

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/channels` | GET | 渠道列表（支持按类型/状态筛选） |
| `/api/admin/channels` | POST | 添加渠道 |
| `/api/admin/channels/:id` | PUT | 更新渠道 |
| `/api/admin/channels/:id` | DELETE | 删除渠道 |
| `/api/admin/channels/:id/test` | POST | 测试渠道连通性 |
| `/api/admin/channels/:id/health` | GET | 获取渠道健康状态 |
| `/api/admin/channels/batch-health-check` | POST | 批量健康检查 |
| `/api/admin/channels/:id/refresh-oauth` | POST | 手动刷新 OAuth Token |

**添加渠道请求示例（API Key 类型）**：

```json
POST /api/admin/channels
{
  "group_id": 1,
  "channel_type": "openai",
  "channel_name": "OpenAI 主账号",
  "auth_type": "api_key",
  "api_key": "sk-xxxxxxxx...",  // 会加密存储
  "api_base_url": "",              // 空 = 使用官方端点
  "models": ["gpt-4o", "gpt-4o-mini", "gpt-4-turbo"],
  "priority": 10,
  "weight": 100,
  "concurrency": 5,
  "enabled": true
}
```

**添加渠道请求示例（OAuth 类型 - Claude）**：

```json
POST /api/admin/channels
{
  "group_id": 1,
  "channel_type": "claude",
  "channel_name": "Claude Pro 订阅",
  "auth_type": "oauth",
  "oauth_token": "...",         // PKCE 流程获取
  "oauth_refresh_token": "...",
  "oauth_expires_at": "2026-06-01T00:00:00Z",
  "models": ["claude-3-5-sonnet-20241022", "claude-3-opus-20240229"],
  "priority": 10,
  "concurrency": 3,
  "enabled": true
}
```

---

### 4.4 调用日志查询

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/logs` | GET | 全局调用日志查询 |
| `/api/admin/logs/export` | GET | 导出日志（CSV/JSON） |
| `/api/admin/logs/stats` | GET | 日志统计（按用户/模型/渠道） |

**日志查询参数**：

| 参数 | 类型 | 说明 |
|:---|:---:|:---|
| `user_id` | integer | 按用户筛选 |
| `model` | string | 按模型筛选 |
| `channel_id` | integer | 按渠道筛选 |
| `status_code` | integer | 按状态码筛选 |
| `start_time` | string | 开始时间（ISO 8601） |
| `end_time` | string | 结束时间（ISO 8601） |
| `page` | integer | 页码 |
| `page_size` | integer | 每页数量 |

---

### 4.5 兑换码管理

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/redeem-codes` | GET | 兑换码列表 |
| `/api/admin/redeem-codes` | POST | 批量生成兑换码 |
| `/api/admin/redeem-codes/:id` | DELETE | 作废兑换码 |

**批量生成兑换码请求示例**：

```json
POST /api/admin/redeem-codes
{
  "code_type": "fixed",       // fixed / discount / full_reduction
  "amount": 50000,           // 固定额度（额度单位）
  "count": 100,              // 生成数量
  "valid_days": 90,          // 有效期（天）
  "max_uses": 1,            // 每个码最大兑换次数
  "prefix": "TH-LAUNCH-"     // 码前缀（可选）
}
```

---

### 4.6 支付配置

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/payment/config` | GET | 获取支付配置（敏感字段掩码） |
| `/api/admin/payment/config` | PUT | 更新支付配置；开启 `easypay_enabled` 时须填写 `easypay_pid`、`easypay_key`、`easypay_api_url` |
| `/api/admin/payment/orders` | GET | 订单列表 |
| `/api/admin/payment/orders/:id/sync` | POST | 易支付待支付订单查单补单 |
| `/api/admin/payment/orders/:id/refund` | POST | 退款 |

**用户端（需开通在线支付）**

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/user/payment/providers` | GET | 可用支付渠道列表 |
| `/api/user/payment/packages` | GET | 充值套餐 |
| `/api/user/payment/orders` | POST | 创建订单；`provider=easypay` 时须传 `easypay_type`：`alipay` \| `wxpay` |

**Webhook**

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/webhook/payment/easypay` | GET/POST | 易支付异步通知（MD5 验签） |

> 易支付（`easypay`）对接标准 `mapi.php` 下单、`api.php?act=order` 查单、`api.php?act=refund` 退款；配置存 DB 键 `payment.runtime`。

---

### 4.7 系统配置

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/config` | GET | 获取系统配置 |
| `/api/admin/config` | PUT | 更新系统配置 |
| `/api/admin/config/reload` | POST | 重新加载配置（不重启服务） |

---

### 4.8 仪表盘统计

| 端点 | 方法 | 描述 |
|:---|:---:|:---|
| `/api/admin/dashboard/overview` | GET | 总览数据（今日调用量/总收入/活跃用户） |
| `/api/admin/dashboard/trends` | GET | 趋势数据（按日/周/月） |
| `/api/admin/dashboard/channels` | GET | 渠道统计 |

**总览响应示例**：

```json
{
  "code": 0,
  "data": {
    "today_requests": 12345,
    "today_tokens": 5678900,
    "today_revenue": 235.50,
    "active_users_24h": 89,
    "total_users": 567,
    "channel_health": {
      "healthy": 8,
      "unhealthy": 1,
      "disabled": 1
    },
    "recent_requests_trend": [120, 145, 132, 98, ...]  // 最近 24 小时每小时请求数
  }
}
```

---

## 五、Webhook 接口

### 5.1 用量回调 `POST /webhook/usage`

> 当用户的用量发生变化时，推送到用户配置的回调地址。

**请求示例**：

```json
POST https://user-callback.example.com/webhook/usage
X-TraceHub-Signature: sha256=abcdef123456...
Content-Type: application/json

{
  "event": "usage.created",
  "created_at": "2026-05-09T14:30:00Z",
  "data": {
    "user_id": 1,
    "api_key_id": 2,
    "request_id": "req-abc-123",
    "model": "gpt-4o",
    "prompt_tokens": 100,
    "completion_tokens": 200,
    "cost": 0.007,
    "balance_after": 4.843
  }
}
```

### 5.2 余额变动回调 `POST /webhook/balance`

### 5.3 其他事件回调

| 事件类型 | 描述 |
|:---|:---|
| `usage.created` | 用量产生 |
| `balance.updated` | 余额变动（充值/消费/管理员调整） |
| `api_key.created` | API Key 创建 |
| `channel.unhealthy` | 渠道异常（需配置告警） |

---

## 六、运维端点

### 6.1 健康检查 `GET /health`

```json
{
  "status": "ok",
  "version": "1.0.0",
  "uptime": 86400
}
```

### 6.2 就绪探针 `GET /ready`

用于 Kubernetes 存活探针。

```json
{
  "status": "ready",
  "checks": {
    "database": "ok",
    "redis": "ok"
  }
}
```

### 6.3 Prometheus 指标 `GET /metrics`

暴露 Prometheus 格式指标，用于监控采集。

**核心指标列表**：

| 指标名 | 类型 | 说明 |
|:---|:---:|:---|
| `tracehub_requests_total` | Counter | 总请求数（按 model/channel/status 标签） |
| `tracehub_request_duration_seconds` | Histogram | 请求耗时（P50/P95/P99） |
| `tracehub_tokens_total` | Counter | 总 Token 消耗 |
| `tracehub_users_active` | Gauge | 活跃用户数 |
| `tracehub_channel_health` | Gauge | 渠道健康状态（1=健康 0=异常） |
| `tracehub_balance_total` | Gauge | 全平台总余额 |
| `tracehub_concurrency_in_use` | Gauge | 当前并发使用数 |

---

## 七、错误码规范

> **[sub2api]** 参考 OpenAI 错误码规范，同时兼容 Sub2API 的错误响应格式。

### 7.1 标准错误响应格式

```json
{
  "error": {
    "message": "余额不足，当前余额：0.50 元，本次预估消费：1.20 元",
    "type": "insufficient_balance_error",
    "code": "insufficient_balance",
    "param": null,
    "http_status": 402
  }
}
```

### 7.2 错误码定义

| HTTP 状态码 | 错误码（code） | 错误类型（type） | 说明 |
|:---:|:---|:---|:---|
| 400 | `invalid_request_error` | `invalid_request_error` | 请求格式错误（缺少必填参数/参数类型错误） |
| 401 | `authentication_error` | `authentication_error` | API Key 无效 / JWT Token 过期 |
| 402 | `insufficient_balance` | `insufficient_balance_error` | 余额不足 |
| 403 | `permission_denied` | `permission_error` | 无权限访问该模型 / 操作 |
| 404 | `not_found` | `not_found_error` | 资源不存在（用户/渠道/模型） |
| 429 | `rate_limit_exceeded` | `rate_limit_error` | 超出速率限制（QPS/并发/Token） |
| 500 | `internal_error` | `api_error` | 网关内部错误 |
| 502 | `upstream_error` | `upstream_error` | 上游服务不可用 / 响应异常 |
| 503 | `service_unavailable` | `upstream_error` | 无可用上游渠道 |
| 504 | `upstream_timeout` | `upstream_error` | 上游响应超时 |

### 7.3 上游错误透传

当上游返回错误时，网关应将上游错误码和错误信息透传给客户端：

```json
{
  "error": {
    "message": "上游错误：model not found",
    "type": "upstream_error",
    "code": "model_not_found",
    "upstream_status": 404,
    "upstream_error": {
      "message": "The model `gpt-5` does not exist",
      "type": "invalid_request_error",
      "code": "model_not_found"
    }
  }
}
```

---

## 八、请求/响应头规范

### 8.1 请求头

| 请求头 | 必填 | 说明 |
|:---|:---:|:---|
| `Authorization` | ✅ | `Bearer <token_or_api_key>` |
| `Content-Type` | ✅ | `application/json` |
| `X-Request-ID` | ❌ | 请求唯一 ID（推荐，用于链路追踪） |
| `X-Sticky-Session` | ❌ | `true` / `false`（启用粘性会话） |
| `X-Session-ID` | ❌ | 会话 ID（用于粘性会话路由） |

### 8.2 响应头

| 响应头 | 说明 |
|:---|:---|
| `X-Request-ID` | 回显请求 ID，便于链路追踪 |
| `X-RateLimit-Limit` | 当前限制（QPS 或 Token/min） |
| `X-RateLimit-Remaining` | 剩余配额 |
| `X-RateLimit-Reset` | 配额重置时间（Unix 时间戳） |
| `X-Channel-ID` | 实际使用的上游渠道 ID（调试用） |
| `X-Model-Upstream` | 实际上游模型名称（映射后） |

---

## 九、Go 接口定义（服务端）

### 9.1 网关 Handler 接口

```go
// internal/handler/gateway/types.go

// 标准 OpenAI 请求（精简版，完整版见 OpenAI API 文档）
type OpenAIChatRequest struct {
    Model           string          `json:"model" binding:"required"`
    Messages        []Message      `json:"messages" binding:"required"`
    Stream          bool            `json:"stream"`
    Temperature     *float64       `json:"temperature"`
    TopP            *float64       `json:"top_p"`
    MaxTokens       *int           `json:"max_tokens"`
    Tools           []Tool         `json:"tools"`
    ToolChoice      interface{}    `json:"tool_choice"`
    ResponseFormat  *ResponseFormat `json:"response_format"`
}

type Message struct {
    Role    string      `json:"role"`    // system / user / assistant / tool
    Content interface{} `json:"content"` // string 或 []ContentPart（多模态）
}

type ContentPart struct {
    Type  string `json:"type"`  // text / image_url
    Text  string `json:"text"`
    ImageURL *ImageURL `json:"image_url"`
}

type ImageURL struct {
    URL    string `json:"url"`
    Detail string `json:"detail"` // auto / low / high
}

// 标准 OpenAI 响应（非流式）
type OpenAIChatResponse struct {
    ID        string   `json:"id"`
    Object    string   `json:"object"`
    Created   int64    `json:"created"`
    Model     string   `json:"model"`
    Choices   []Choice `json:"choices"`
    Usage     Usage    `json:"usage"`
    SystemFingerprint string `json:"system_fingerprint,omitempty"`
}

type Choice struct {
    Index        int     `json:"index"`
    Message      *Message `json:"message,omitempty"`
    Delta        *Message `json:"delta,omitempty"`    // 流式使用
    FinishReason string   `json:"finish_reason"`
}

type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}
```

### 9.2 RelayContext（网关核心上下文）

```go
// internal/gateway/relay/context.go

type RelayContext struct {
    // 请求上下文
    Context    context.Context
    RequestID  string
    StartTime  time.Time
    
    // 身份认证结果
    User      *ent.User
    APIKey    *ent.APIKey
    Group     *ent.Group
    
    // 请求信息
    Request   *OpenAIChatRequest
    IsStream  bool
    
    // 路由结果
    Channel   *ent.Channel
    ModelUpstream string  // 映射后的实际上游模型
    
    // 计费
    InputTokens  int
    OutputTokens int
    PreCost      int64  // 预估费用（预扣）
    ActualCost   int64  // 实际费用（修正后）
    
    // 上游响应
    upstreamResp *http.Response
    ResponseTime time.Duration
    
    // 日志
    Log *ent.UsageLog
}
```
