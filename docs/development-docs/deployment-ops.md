# TraceHub 部署运维文档

**版本**：V1.0  
**更新时间**：2026-05-09  
**参考项目**：Sub2API 部署设计

---

## 一、部署方式概览

> **[sub2api]** 参考 Sub2API 的三种部署方式，本项目提供同等部署能力。

| 方式 | 适合场景 | 难度 | 数据持久化方式 |
|:---:|:---|:---:|:---|
| **一键脚本安装** | Linux 服务器生产环境 | ⭐⭐ | 本地目录（推荐） |
| **Docker Compose** | 容器化场景 / 快速测试 | ⭐ | 命名卷或本地目录 |
| **源码编译** | 开发 / 定制化需求 | ⭐⭐⭐ | 自定义 |

---

## 二、方式一：一键脚本安装（推荐生产环境 Linux）

> **[sub2api]** 完全参考 Sub2API 的 `install.sh` 设计。

### 2.1 前置条件

| 组件 | 要求 | 说明 |
|:---|:---:|:---|
| **操作系统** | Linux（amd64 / arm64） | 推荐 Ubuntu 22.04 LTS |
| **PostgreSQL** | 16+ | 已部署并运行 |
| **Redis** | 7+ | 已部署并运行 |
| **权限** | root 或 sudo 权限 | 用于安装 systemd 服务 |

### 2.2 安装步骤

```bash
# 下载并执行一键安装脚本（自动检测架构、下载二进制、创建 systemd 服务）
curl -sSL https://raw.githubusercontent.com/你的用户名/TraceHub/main/scripts/install.sh | sudo bash

# 安装后二进制位于 /opt/tracehub/tracehub
# systemd 服务名为 tracehub
```

### 2.3 初始化配置

安装完成后访问 `http://服务器IP:8080`，会自动进入设置向导：

1. 配置数据库连接（`postgres://user:pass@localhost:5432/tracehub`）
2. 配置 Redis 连接（`localhost:6379`）
3. 创建管理员账号（邮箱 + 密码）
4. 配置站点基本信息

### 2.4 服务管理命令

```bash
# 查看服务状态
sudo systemctl status tracehub

# 查看实时日志
sudo journalctl -u tracehub -f

# 启动服务
sudo systemctl start tracehub

# 设置开机自启
sudo systemctl enable tracehub

# 重启服务
sudo systemctl restart tracehub

# 停止服务
sudo systemctl stop tracehub
```

### 2.5 卸载

```bash
curl -sSL https://raw.githubusercontent.com/你的用户名/TraceHub/main/scripts/install.sh | sudo bash -s -- uninstall -y
```

---

## 三、方式二：Docker Compose 部署（推荐容器化场景）

> **[sub2api]** 参考 Sub2API 的 `docker-compose.local.yml`，使用本地目录持久化。

### 3.1 前置条件

| 组件 | 要求 |
|:---|:---:|
| **Docker** | 20.10+ |
| **Docker Compose** | v2+ |

### 3.2 快速部署（自动化脚本）

```bash
# 使用自动化部署脚本（自动下载配置文件、生成密钥、创建数据目录）
curl -sSL https://raw.githubusercontent.com/你的用户名/TraceHub/main/scripts/docker-deploy.sh | bash

# 生成的内容在 ./tracehub/ 目录
cd tracehub

# 启动服务
docker compose -f docker-compose.local.yml up -d
```

### 3.3 手动部署步骤

```bash
# 1. 创建项目目录
mkdir -p tracehub && cd tracehub

# 2. 下载配置文件
curl -o docker-compose.local.yml https://raw.githubusercontent.com/你的用户名/TraceHub/main/deploy/docker-compose.local.yml
curl -o .env https://raw.githubusercontent.com/你的用户名/TraceHub/main/deploy/.env.example

# 3. 编辑 .env 文件（修改密码等配置）
vim .env

# 4. 创建数据目录
mkdir -p data postgres_data redis_data

# 5. 启动服务
docker compose -f docker-compose.local.yml up -d
```

### 3.4 `docker-compose.local.yml` 配置说明

```yaml
# deploy/docker-compose.local.yml

version: '3.8'

services:
  tracehub:
    build: ..
    image: tracehub:latest
    container_name: tracehub
    ports:
      - "8080:8080"
    environment:
      - DATABASE_DSN=postgres://tracehub:${POSTGRES_PASSWORD}@postgres:5432/tracehub?sslmode=disable
      - REDIS_ADDR=redis:6379
      - REDIS_PASSWORD=${REDIS_PASSWORD}
      - JWT_SECRET=${JWT_SECRET}
      - SERVER_PORT=8080
    volumes:
      - ./data:/app/data                # 本地目录持久化
    depends_on:
      - postgres
      - redis
    restart: unless-stopped

  postgres:
    image: postgres:16-alpine
    container_name: tracehub-postgres
    environment:
      - POSTGRES_USER=tracehub
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
      - POSTGRES_DB=tracehub
    volumes:
      - ./postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    container_name: tracehub-redis
    command: redis-server --requirepass ${REDIS_PASSWORD} --maxmemory 256mb --maxmemory-policy allkeys-lru
    volumes:
      - ./redis_data:/data
    ports:
      - "6379:6379"
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    container_name: tracehub-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
    depends_on:
      - tracehub
    restart: unless-stopped
```

### 3.5 Docker 常用命令

```bash
# 查看服务状态
docker compose -f docker-compose.local.yml ps

# 查看实时日志
docker compose -f docker-compose.local.yml logs -f tracehub

# 停止服务
docker compose -f docker-compose.local.yml stop

# 重启服务
docker compose -f docker-compose.local.yml restart

# 升级到最新版本（拉取最新镜像并重启）
docker compose -f docker-compose.local.yml pull && \
docker compose -f docker-compose.local.yml up -d

# 删除所有数据（⚠️ 危险操作）
docker compose -f docker-compose.local.yml down && \
rm -rf data/ postgres_data/ redis_data/
```

### 3.6 迁移说明

> **[sub2api]** 本地目录版可直接打包整个目录迁移，参考 Sub2API 迁移说明。

```bash
# 在源服务器打包
tar -czf tracehub_migration.tar.gz tracehub/

# 拷贝到新服务器
scp tracehub_migration.tar.gz user@new-server:/opt/

# 在新服务器解压并启动
cd /opt && tar -xzf tracehub_migration.tar.gz
cd tracehub && docker compose -f docker-compose.local.yml up -d
```

---

## 四、方式三：源码编译部署（开发 / 定制化场景）

> **[sub2api]** 参考 Sub2API 的源码编译流程。

### 4.1 前置条件

| 组件 | 要求 |
|:---|:---:|
| **Go** | 1.23+ |
| **Node.js** | 18+（推荐 20+） |
| **pnpm** | 8+ |
| **PostgreSQL** | 15+ |
| **Redis** | 7+ |

### 4.2 编译步骤

```bash
# 1. 克隆仓库
git clone https://github.com/你的用户名/TraceHub.git && cd TraceHub

# 2. 编译前端（产物输出到 internal/web/dist/）
cd web/frontend
pnpm install
pnpm run build
cd ../../

# 3. 编译后端（使用 -tags embed 将前端嵌入二进制）
cd internal
go build -tags embed -o tracehub ./cmd/server

# 4. 复制配置文件
cp deploy/config.example.yaml config.yaml
vim config.yaml  # 修改数据库、Redis 等配置

# 5. 启动服务
./tracehub
```

### 4.3 开发模式

```bash
# 终端 1：启动后端（热重载）
cd internal
go run -tags embed ./cmd/server

# 终端 2：启动前端（热重载，Vite 开发服务器）
cd web/frontend
pnpm run dev
# 前端开发服务器运行在 http://localhost:5173
# API 请求通过 Vite proxy 转发到后端（http://localhost:8080）
```

### 4.4 代码修改后处理

> **[sub2api]** 参考 Sub2API，若修改了 `internal/ent/schema`，需重新执行 Ent 代码生成。

```bash
cd internal

# 重新生成 Ent 代码（当 schema 变更时）
go generate ./ent

# 重新生成 Wire 依赖注入代码（当依赖关系变更时）
go generate ./cmd/server
```

---

## 五、Nginx 反向代理配置

> **[sub2api]** 参考 Sub2API 的 Nginx 配置要求，特别是 `underscores_in_headers on;` 配置。

### 5.1 标准 Nginx 配置

```nginx
# deploy/nginx/tracehub.conf

server {
    listen 80;
    server_name api.tracehub.com;

    # 强制 HTTPS 跳转
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.tracehub.com;

    # SSL 证书配置
    ssl_certificate     /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # ⚠️ 重要：开启下划线请求头支持（粘性会话依赖 X-Sticky-Session 等请求头）
    underscores_in_headers on;

    # 请求大小限制（支持多模态图片上传）
    client_max_body_size 20M;

    # 转发到 TraceHub 后端
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;  # 应用内「强制 HTTPS」依赖此头
        
        # WebSocket 支持（SSE 流式响应）
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 关闭代理缓冲（保证 SSE 实时性）
        proxy_buffering off;
        proxy_cache off;
    }
}
```

### 5.2 TraceHub 应用内 HTTPS（可选）

除 Nginx 终结 TLS 外，TraceHub 支持两种互补能力：

| 能力 | 配置 | 管理后台 | 生效 |
|:---|:---|:---|:---|
| 强制 HTTP→HTTPS、HSTS | `site.https.*` 或 `system_configs` | 系统配置 → 访问安全 | 保存后约 10s（内存缓存），**无需重启** |
| 内置 TLS 监听 | `server.tls.cert_file` / `key_file`（仅 YAML） | 访问安全 →「启用内置 TLS」 | **须重启**进程 |

**推荐生产（高 QPS）**：Nginx/Caddy 监听 443 并设置 `proxy_set_header X-Forwarded-Proto $scheme;`，TraceHub 保持 `server.tls.enabled: false`，在管理后台开启「强制 HTTP 跳转 HTTPS」即可（或由 Nginx `return 301` 承担跳转）。

**单机无反代**：在 `config.yaml` 配置证书路径并 `server.tls.enabled: true`，重启后进程监听 `:443`；若 `server.addr` 为 `:80` 将并行提供 HTTP→HTTPS 重定向。

```yaml
server:
  addr: ":80"
  tls:
    enabled: true
    addr: ":443"
    cert_file: /etc/tracehub/ssl/fullchain.pem
    key_file: /etc/tracehub/ssl/privkey.pem
site:
  https:
    force_redirect: true
    hsts_enabled: true
```

开启 HTTPS 后请将 `site.cors_origins` 改为 `https://` 源地址。

### 5.3 Cloudflare / CDN 前置场景

若使用 Cloudflare 等 CDN 作为第一层防护：

1. **关闭 Cloudflare 的 HTTP/2 to Origin**（避免协议不兼容）
2. **配置 Page Rule**：对 `/v1/chat/completions` 关闭缓存
3. **增加超时时间**：Cloudflare 默认 100s 超时，需调整为 300s+

---

## 六、环境变量配置

> **[sub2api]** 参考 Sub2API 的 `.env` 配置设计。

### 6.1 核心环境变量

| 变量名 | 说明 | 默认值 |
|:---|:---|:---:|
| `DATABASE_DRIVER` | 数据库驱动 | `postgres` |
| `DATABASE_DSN` | 数据库连接串 | — |
| `REDIS_ADDR` | Redis 地址 | `localhost:6379` |
| `REDIS_PASSWORD` | Redis 密码 | — |
| `REDIS_DB` | Redis 数据库编号 | `0` |
| `JWT_SECRET` | JWT 签名密钥（⚠️ 生产环境必须修改） | — |
| `JWT_EXPIRE_HOURS` | JWT Token 有效期（小时） | `72` |
| `SERVER_PORT` | 服务监听端口 | `8080` |
| `SERVER_MODE` | 服务运行模式 | `release` |
| `RATE_MULTIPLIER` | 全局费率倍率 | `1.0` |
| `BILLING_CIRCUIT_BREAKER` | 计费异常熔断开关 | `true` |
| `DEFAULT_USER_BALANCE` | 新用户默认余额（额度） | `0` |
| `API_KEY_PREFIX` | API Key 默认前缀 | `sk-th-` |
| `ALLOW_INSECURE_HTTP` | 允许 HTTP 上游（⚠️ 仅开发环境） | `false` |
| `RUN_MODE` | 运行模式（`simple`=简易模式，隐藏 SaaS 功能） | `normal` |

### 6.2 `.env.example`（Docker 部署用）

```bash
# deploy/.env.example

# 数据库配置
POSTGRES_PASSWORD=changeme_please!
POSTGRES_USER=tracehub
POSTGRES_DB=tracehub

# Redis 配置
REDIS_PASSWORD=changeme_please!

# JWT 密钥（⚠️ 生产环境必须使用强随机密钥）
JWT_SECRET=replace_with_strong_random_secret!

# 全局配置
RATE_MULTIPLIER=1.0
BILLING_CIRCUIT_BREAKER=true
DEFAULT_USER_BALANCE=0
API_KEY_PREFIX=sk-th-
```

---

## 七、数据库备份与恢复

### 7.1 备份脚本

> **[sub2api]** 参考 Sub2API 的备份脚本设计。

```bash
#!/bin/bash
# scripts/backup.sh

BACKUP_DIR="/opt/tracehub/backups"
DATE=$(date +%Y%m%d_%H%M%S)
FILENAME="tracehub_backup_$DATE.sql.gz"

mkdir -p "$BACKUP_DIR"

# 使用 pg_dump 备份（压缩）
pg_dump -h localhost -U tracehub -d tracehub | gzip > "$BACKUP_DIR/$FILENAME"

# 保留最近 7 天的备份，自动删除旧备份
find "$BACKUP_DIR" -name "tracehub_backup_*.sql.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR/$FILENAME"
```

### 7.2 恢复脚本

```bash
#!/bin/bash
# scripts/restore.sh

if [ -z "$1" ]; then
    echo "Usage: $0 <backup_file.sql.gz>"
    exit 1
fi

BACKUP_FILE="$1"

echo "⚠️  警告：此操作将覆盖当前数据库！"
read -p "确认恢复？(y/N) " confirm
if [ "$confirm" != "y" ]; then
    echo "已取消"
    exit 0
fi

# 解压并恢复
gunzip -c "$BACKUP_FILE" | psql -h localhost -U tracehub -d tracehub

echo "恢复完成"
```

### 7.3 自动化备份（Cron）

```bash
# 编辑 crontab
crontab -e

# 每天凌晨 2 点执行备份
0 2 * * * /opt/tracehub/scripts/backup.sh >> /var/log/tracehub-backup.log 2>&1
```

---

## 八、升级与回滚

> **[sub2api]** 参考 Sub2API 的后台一键升级回滚设计。

### 8.1 脚本安装版本的升级

```bash
# 重新执行安装脚本（自动下载最新版本）
curl -sSL https://raw.githubusercontent.com/你的用户名/TraceHub/main/scripts/install.sh | sudo bash

# 重启服务
sudo systemctl restart tracehub
```

### 8.2 Docker Compose 版本的升级

```bash
# 拉取最新镜像
docker compose -f docker-compose.local.yml pull

# 重启服务（零停机）
docker compose -f docker-compose.local.yml up -d

# 清理旧镜像
docker image prune -f
```

### 8.3 回滚到上一版本

```bash
# 脚本安装版本：查看历史版本
ls /opt/tracehub/backups/

# 停止当前版本
sudo systemctl stop tracehub

# 恢复上一版本二进制
cp /opt/tracehub/backups/tracehub_v0.9.0 /opt/tracehub/tracehub

# 重启
sudo systemctl start tracehub
```

### 8.4 数据库 schema 与二进制升级

TraceHub 默认在**每次进程启动**时执行 Ent 的 `Schema.Create`（PostgreSQL），将库表结构对齐到当前二进制内嵌的 Ent 模型（例如新增列、新表、新索引）。因此：

- **仅替换二进制并重启服务**，在能连上同一数据库且迁移成功的前提下，一般会**自动完成**与版本匹配的 schema 变更；无需单独执行一条 `migrate` 子命令。
- 若 DDL 与线上数据冲突（极少见），启动会失败并打 `database migration failed`，服务不会进入监听状态。
- **升级前建议备份数据库**；大版本或敏感环境可在预发库先起一次新版本验证迁移日志。

Docker 场景同理：拉取新镜像并 `up -d` 后，容器内进程启动时执行同一套迁移逻辑。

---

## 九、Prometheus + Grafana 监控

### 9.1 Prometheus 配置

```yaml
# prometheus/prometheus.yml

scrape_configs:
  - job_name: 'tracehub'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
```

### 9.2 关键 Grafana 面板

| 面板名称 | 数据源 | 说明 |
|:---|:---:|:---|
| **请求 QPS** | Prometheus | 按模型 / 渠道分组的请求速率 |
| **请求延迟（P50/P95/P99）** | Prometheus | 网关响应延迟分布 |
| **错误率** | Prometheus | 4xx / 5xx 错误占比 |
| **Token 消耗速率** | Prometheus | 按用户 / 模型分组的 Token 消耗 |
| **渠道健康状态** | Prometheus | 各上游渠道健康状态（绿色=健康） |
| **并发使用量** | Prometheus | 当前用户 / 渠道并发使用量 |
| **余额总量** | PostgreSQL | 全平台总余额趋势 |

### 9.3 AlertManager 告警规则示例

```yaml
# prometheus/alerts.yml

groups:
  - name: tracehub
    rules:
      - alert: HighErrorRate
        expr: rate(tracehub_requests_total{status=~"5.."}) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "TraceHub 高错误率"
          description: "错误率超过 5% 已持续 2 分钟"
      
      - alert: ChannelUnhealthy
        expr: tracehub_channel_health == 0
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "渠道不健康"
          description: "渠道 {{ $labels.channel_name }} 已被标记为不健康"
      
      - alert: LowBalance
        expr: tracehub_balance_total < 100000
        labels:
          severity: warning
        annotations:
          summary: "平台总余额过低"
          description: "全平台总余额已低于 10 元（10000 额度）"
```

---

## 十、故障排查

### 10.1 常见问题

| 问题 | 排查步骤 |
|:---|:---|
| **服务无法启动** | 检查日志 `journalctl -u tracehub -n 50`；检查端口占用 `ss -tlnp \| grep 8080` |
| **数据库连接失败** | 检查 `config.yaml` 中 DSN 是否正确；手动连接测试 `psql -h ... -U tracehub` |
| **Redis 连接失败** | 检查 Redis 是否运行 `redis-cli ping`；检查密码配置 |
| **上游请求失败** | 检查渠道配置中的 `api_key` 是否正确；检查 `api_base_url` 是否可达 |
| **粘性会话失效** | 检查 Nginx 是否配置了 `underscores_in_headers on;` |
| **计费异常** | 检查 `billing.circuit_breaker` 是否开启；查看 `usage_logs` 表异常记录 |

### 10.2 日志级别调整

```yaml
# config.yaml

logging:
  level: "debug"  # debug / info / warn / error
  output: "stdout"  # stdout / file
  file_path: "/var/log/tracehub/app.log"
```

---

## 十一、总结

部署运维文档覆盖三种部署方式，完全参考 Sub2API 的成熟实践：

| 部署方式 | 参考 Sub2API | 增强内容 |
|:---|:---:|:---|
| 一键脚本安装 | ✅ 完全参考 | 无 |
| Docker Compose | ✅ 完全参考 | 增加 Nginx 容器 |
| 源码编译 | ✅ 完全参考 | 无 |
| 数据库备份恢复 | ✅ 参考 | 自动化 Cron 备份 |
| Prometheus 监控 | 新增 | 完整的告警规则示例 |
