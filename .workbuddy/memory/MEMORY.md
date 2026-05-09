# TraceHub 项目长期记忆

## 项目概况
- **项目名**：TraceHub — AI API 中转站
- **技术栈**：后端 Go 1.23+ / Gin / Ent ORM / Wire DI / PostgreSQL / Redis，前端 Vue 3 / Vite / TailwindCSS
- **产品文档**：`docs/产品文档.md`（V1.0，2026-05-09）
- **核心参考项目**：Sub2API（开源 AI API 网关平台，MIT 协议）
- **项目定位**：智能 AI API 网关与资产管理系统，聚合 OpenAI/Claude/Gemini/DeepSeek/Antigravity 等主流 AI 服务

## 开发文档体系（2026-05-09 完成）
已编写 8 份完整开发文档，存放在 `docs/development-docs/`：
1. `README.md` — 文档总览 + Sub2API/New API 参考对照
2. `technical-architecture.md` — 整体架构、技术选型、目录结构、Wire DI、请求流程
3. `database-design.md` — 完整 ER 模型、DDL、索引策略、分区方案、Redis 缓存键设计
4. `api-design.md` — 网关/用户/管理员 API 端点、错误码规范、Go 接口定义
5. `gateway-core-design.md` — 智能路由、Relay 适配器(Claude/OpenAI/Gemini/DeepSeek/Antigravity)、计费引擎、限流器、Token 统计
6. `frontend-development.md` — 前端架构、页面规划、API 封装、状态管理、embed 嵌入部署
7. `deployment-ops.md` — 三种部署方式(脚本/Docker/源码)、Nginx、备份恢复、Prometheus 监控
8. `security-compliance.md` — AES-256-GCM 加密、HTTPS、IP 白名单、内容过滤、审计日志、等保对标
9. `milestones-roadmap.md` — Phase 0~3 路线图（约 18 周）、优先级矩阵、技术债务管理

## Sub2API 关键对标功能（不能减配）
- OAuth 2.0 PKCE 认证（Claude/OpenAI/Gemini 三类）
- 粘性会话路由（跨平台，不同前缀）
- 多级倍率计费（分组/账号/管理员自定义）+ 离线兜底 + 熔断
- 三层限流（用户级+Key级+渠道级）+ Redis 并发槽
- 内置支付系统（支付宝/微信/Stripe/EasyPay）
- Antigravity 接入 + 混合调度
- 一键升级回滚 + iframe 外部系统集成
- TOTP 二次验证 + Turnstile 验证码

## 关键设计决策
- ORM 选择 Ent（类型安全，参考 Sub2API）而非 GORM
- DI 使用 Google Wire（编译期注入，参考 Sub2API）
- 前端 embed 嵌入后端二进制（单文件部署，参考 Sub2API）
- 调用日志按月分区（TimescaleDB 扩展）
- API Key SHA-256 哈希存储，上游密钥 AES-256-GCM 加密
