# TraceHub 文档索引

按读者角色选择入口。实现边界以仓库 [README](../README.md) 与 [CHANGELOG](../CHANGELOG.md) 为准；完整 API 字段见 [development-docs/api-design.md](development-docs/api-design.md)。

| 我想… | 推荐阅读 |
|--------|----------|
| **接入 API、改 Base URL** | 线上实例的 **`/docs` 文档中心**（见 [_site-urls.yaml](_site-urls.yaml) 中 `docs_url`）+ [TraceHub-客户接入宣传.md](TraceHub-客户接入宣传.md) |
| **英文客户简介** | [TraceHub-Customer-Onboarding.en.md](TraceHub-Customer-Onboarding.en.md) |
| **自托管 / Docker 部署** | [README 快速开始](../README.md) + [deployment-ops.md](development-docs/deployment-ops.md) |
| **售前 / 投标 / 商务** | [TraceHub-销售介绍.md](TraceHub-销售介绍.md) |
| **宣传视频 / 物料** | [TraceHub-宣传视频制作包.md](TraceHub-宣传视频制作包.md) |
| **二次开发 / 运维** | [development-docs 索引](development-docs/README.md) |
| **产品全景（含规划）** | [产品文档.md](产品文档.md)（文首含**实现状态**表） |
| **官网文案与运维分离约定** | [公网展示与运维文案分离说明.md](公网展示与运维文案分离说明.md) |

## 运营 URL 占位符

营销类 Markdown 使用 `{SITE_URL}`、`{API_BASE}` 等占位符；示例值见 [_site-urls.yaml](_site-urls.yaml)。发布前请按当期运营公告替换或脚本渲染。

## 文档分层（维护约定）

| 层级 | 写什么 | 不写什么 |
|------|--------|----------|
| README | 定位、快速开始、能力矩阵、文档索引 | 长 API 字段表 |
| 营销 / 销售 MD | 价值主张、场景、CTA | curl 全量参数、**具体上游厂商/渠道类型** |
| 站内 `/docs` | 集成步骤、错误码、控制台说明 | Prometheus 路径、**上游供应商名称与内部转发路径**（见[分离说明](公网展示与运维文案分离说明.md) §4） |
| development-docs | 端点定义、DDL、部署、安全清单（含渠道类型，供自建） | 新用户活动文案 |

贡献文档时请同步 [CONTRIBUTING.md](../CONTRIBUTING.md) 中的检查项。
