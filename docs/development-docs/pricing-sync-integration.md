# 模型价格同步对接文档

TraceHub 与外部系统通过 **`tracehub_pricing_v1`** JSON 格式交换定价配置。同步的是 **加成前的 DB 结构化字段**（`quota_*`、`markup_percent`、倍率等），不是运行时合并后的扣费价。

**仅支持 Pull（拉取）**：服务端提供导出接口与密钥；客户端配置远端后主动拉取。不支持 Push 写入。

## 1. 角色说明

| 角色 | 本站点要做的事 |
|------|----------------|
| **服务端** | 开启功能 → 生成入站同步密钥 → 对外提供 Export URL |
| **客户端** | 开启功能 → 配置远端 `base_url` + 密钥 + `pull_path` → 预览 → 确认拉取 |

TraceHub ↔ TraceHub：A 为服务端生成密钥，B 为客户端在「出站 API Key」填 A 的密钥。

## 2. 服务端 Export 接口

```http
GET {site_root}/api/pricing/sync/export
Authorization: Bearer {inbound_token}
Accept: application/json
```

### 响应体（tracehub_pricing_v1）

```json
{
  "version": "tracehub_pricing_v1",
  "generated_at": "2026-05-23T08:00:00Z",
  "global": [{ "model_name": "gpt-4o", "enabled": true, "quota_in_per_1k": 120, "quota_out_per_1k": 480 }],
  "channels": [{ "ref": "openai-main", "items": [] }],
  "groups": [{ "ref": "vip", "items": [] }]
}
```

管理端生成密钥：

| 方法 | 路径 |
|------|------|
| GET | `/api/admin/pricing-sync/inbound-info` |
| POST | `/api/admin/pricing-sync/inbound-token/regenerate` |

## 3. 客户端拉取流程

1. 创建同步目标（`direction` 固定为 `pull`）
2. `base_url` = 服务端站点根地址
3. `pull_path` 默认 `/api/pricing/sync/export`
4. `api_key` = 服务端入站密钥
5. `POST .../test-connection` → `POST .../sync/preview` → `POST .../sync`（须 `preview_id`）

## 4. 定时 Worker

- 站点开关 + `site.pricing_sync.worker_enabled` 开启后，Worker **每分钟**扫描一次
- 仅处理 `enabled=true` 且 `schedule_enabled=true` 的客户端目标
- 每个目标按 **`schedule_interval_min`**（分钟，60–10080）距 **上次拉取** 是否到期决定是否执行
- 定时拉取**无预览**，直接 Pull 落库

## 5. 管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/admin/pricing-sync/targets` | 列表 |
| POST | `/api/admin/pricing-sync/targets` | 创建（superadmin，direction=pull） |
| PUT | `/api/admin/pricing-sync/targets/:id` | 更新 |
| DELETE | `/api/admin/pricing-sync/targets/:id` | 删除 |
| POST | `/api/admin/pricing-sync/targets/:id/test-connection` | 测试拉取连通性 |
| POST | `/api/admin/pricing-sync/targets/:id/sync/preview` | 预览（direction 须 pull） |
| POST | `/api/admin/pricing-sync/targets/:id/sync` | 确认执行 |

### 站点开关（`system_configs`）

| Key | 默认 |
|-----|------|
| `site.pricing_sync.enabled` | `false` |
| `site.pricing_sync.worker_enabled` | `false` |
| `site.pricing_sync.inbound_token_sha256` | 未配置 |

## 6. 安全说明

- 管理 API 需 admin JWT；执行同步默认仅 **superadmin**
- 出站拉取须 **preview → confirm**（`preview_id` 5 分钟有效）
- 出站 URL 做 SSRF 防护；生产环境默认仅 HTTPS
- **不提供** `POST /api/pricing/sync/import`（不接受外部 Push 写入）

## 7. 字段语义

| 字段 | 说明 |
|------|------|
| `quota_in/out_per_1k` | 成本额度基数（加成前） |
| `markup_percent` | 行内加成 % |
| `input_rate` / `output_rate` | 倍率（加成前） |

不同步：分组/用户 `rate_multiplier`、站点默认加成、运行时有效扣费价。
