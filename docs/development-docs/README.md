# TraceHub 开发文档总览

**项目名称**：TraceHub — AI API 中转站  
**文档版本**：V1.0  
**更新时间**：2026-05-09  
**技术栈**：后端 Go 1.23+ / Gin / Ent ORM / PostgreSQL / Redis，前端 Vue 3 / Vite / TailwindCSS

---

## 文档索引

本目录包含 TraceHub 项目开发所需的全部技术文档，每份文档独立成章，覆盖从架构设计到部署运维的完整开发生命周期。

### 架构与设计

| # | 文档名称 | 文件 | 说明 |
|:-:|:---------|:-----|:-----|
| 1 | **技术架构设计文档** | [technical-architecture.md](./technical-architecture.md) | 整体分层架构、技术选型、模块划分、依赖注入设计、核心目录结构 |
| 2 | **网关核心设计文档** | [gateway-core-design.md](./gateway-core-design.md) | Relay 适配器体系、智能路由调度、计费引擎、限流器、Token 统计、协议转换 |
| 3 | **数据库设计文档** | [database-design.md](./database-design.md) | 完整 ER 模型、全部数据表结构（DDL）、索引策略、分区方案、迁移管理 |

### 接口与通信

| # | 文档名称 | 文件 | 说明 |
|:-:|:---------|:-----|:-----|
| 4 | **API 接口设计文档** | [api-design.md](./api-design.md) | 全部端点定义（网关 API / 用户 API / 管理员 API）、请求/响应结构、错误码规范、认证机制 |

### 前端开发

| # | 文档名称 | 文件 | 说明 |
|:-:|:---------|:-----|:-----|
| 5 | **前端开发文档** | [frontend-development.md](./frontend-development.md) | 前端架构设计、页面规划、组件规范、状态管理、样式约定、构建部署 |

### 安全、部署与运营

| # | 文档名称 | 文件 | 说明 |
|:-:|:---------|:-----|:-----|
| 6 | **安全与合规文档** | [security-compliance.md](./security-compliance.md) | 密钥加密、传输安全、访问控制、内容安全、审计日志、合规认证、安全配置清单 |
| 7 | **部署运维文档** | [deployment-ops.md](./deployment-ops.md) | Docker Compose 编排、Nginx 配置、一键部署脚本、环境变量说明、备份恢复、监控告警、升级回滚 |

### 规划与管理

| # | 文档名称 | 文件 | 说明 |
|:-:|:---------|:-----|:-----|
| 8 | **项目里程碑与开发路线图** | [milestones-roadmap.md](./milestones-roadmap.md) | MVP/Phase1/Phase2/Phase3 功能拆分、里程碑验收标准、优先级矩阵、技术债务管理 |

---

## 参考项目

本项目的核心设计参考了以下开源项目的成熟实践：

### Sub2API（主要参考）

| 维度 | 参考内容 |
|:-----|:---------|
| **项目定位** | AI API 网关平台，分发和管理 AI 产品订阅的 API 配额 |
| **上游平台** | Anthropic Claude、OpenAI、Google Gemini、Antigravity |
| **认证方式** | OAuth 2.0（PKCE 流程）、API Key、Setup Token |
| **智能调度** | 粘性会话路由、多账号负载均衡、优先级调度、混合调度 |
| **计费系统** | Token 级精准计费、多级倍率、全局/分组「模型不计费」策略（见 [gateway-core-design.md](./gateway-core-design.md) §4.2.1）、离线兜底计费 |
| **限流体系** | 用户级 + API Key 级 + 账号级三层并发控制与速率限制 |
| **协议适配** | 统一兼容 Claude Messages API 格式，内部自动转换至各平台原生协议 |
| **数据层** | PostgreSQL（Ent ORM）+ Redis（缓存/协调）双持久化策略 |
| **依赖注入** | Google Wire 编译期依赖注入 |
| **支付集成** | 内置支付系统（EasyPay / 支付宝 / 微信 / Stripe） |
| **运维能力** | 一键升级回滚、iframe 外部系统集成、简易模式、数据管理 |

> **重要原则**：参考 Sub2API 的功能设计时，**不能减配**，在满足本产品文档需求的基础上可以适当增强功能。

### New API

| 维度 | 参考内容 |
|:-----|:---------|
| **多渠道管理** | 50+ 通道类型接入的渠道抽象设计 |
| **模型映射** | 请求模型名称到实际模型的灵活映射机制 |
| **管理后台** | 完善的管理后台 UI/UX 设计 |

---

## 文档使用约定

1. **版本绑定**：所有技术文档与产品文档 `docs/产品文档.md` V1.0 对齐
2. **功能对齐**：每份文档必须覆盖产品文档中对应章节的全部功能点
3. **Sub2API 参考**：标注 `[sub2api]` 的部分表示直接参考 sub2api 的成熟设计，必要时可内嵌其代码
4. **扩展原则**：在产品文档功能要求之上可以增加功能，但不得裁剪
5. **代码优先**：架构设计文档中的关键模块会直接给出 Go 接口定义和数据结构

---

## 快速开始

```bash
# 查看技术选型与整体架构
cat docs/development-docs/technical-architecture.md

# 查看数据库表结构
cat docs/development-docs/database-design.md

# 查看全部 API 端点
cat docs/development-docs/api-design.md

# 查看开发路线图
cat docs/development-docs/milestones-roadmap.md
```
