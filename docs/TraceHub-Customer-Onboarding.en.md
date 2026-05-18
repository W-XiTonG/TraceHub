# Multi-model AI inference, one integration

**OpenAI-compatible API · One integration for the models your operator has enabled**

Focus on your product; let the platform handle multiple service lines, protocols, and API key pools.

> **Live URLs:** Replace `{SITE_URL}`, `{API_BASE}`, `{FUTURE_SITE_URL}`, `{FUTURE_API_BASE}` using [_site-urls.yaml](_site-urls.yaml) or your operator’s current announcement.

---

## Why TraceHub

| Benefit | Summary |
|---------|---------|
| **Unified protocol** | Keep `POST /v1/chat/completions` and `GET /v1/models`; no per-vendor SDK fork |
| **Smart routing** | Health-aware selection, failover, optional Redis sticky sessions for long chats |
| **Transparent billing** | Token/usage-based quota; group and per-model multipliers |
| **Security defaults** | Service credentials encrypted (AES-GCM); user API keys hashed; admin audit log |
| **Self-host friendly** | MIT license, Docker Compose, single binary + embedded SPA |

---

## Get started in three steps

1. **Register** at `{SITE_URL}` (capabilities depend on deployment).
2. Create an **API Key** in the console (full secret shown once—save it immediately).
3. Point your OpenAI-compatible client **base URL** to `{API_BASE}` and set `Authorization: Bearer <your-key>`.

Full integration steps, routing headers, and error codes: in-app **`/docs`** on your instance.

---

## Domains (when applicable)

| | |
|---|---|
| **Current entry** | `{SITE_URL}` |
| **API base** | `{API_BASE}` |
| **After ICP / HTTPS cutover** | `{FUTURE_SITE_URL}` · `{FUTURE_API_BASE}` (announced by operator) |

---

## Elevator pitch (video / social)

Still wiring many model endpoints yourself? **TraceHub** is an open-source **AI API gateway** from Trace Future: one **OpenAI-compatible** endpoint for the models your operator enables. Sign up at `{SITE_URL}`, create an API key, point your base URL—your SDK stays the same. Smart routing, transparent token billing, encrypted credentials, audit logs, and observability hooks. Self-host with Docker, or contact us for **enterprise customization**. Official site `{FUTURE_SITE_URL}` may be pending regional filing. Business: **+86 {BUSINESS_PHONE}** (phone & WeChat).

---

## Planned vs delivered

**Delivered in the open-source tree** is summarized in the [README](../README.md) and [CHANGELOG](../CHANGELOG.md). Items such as online payments (Alipay / WeChat / Stripe), OAuth login, fine-grained RBAC, and hosted Swagger are on the [roadmap](development-docs/milestones-roadmap.md).

---

## Call to action

Open **`{SITE_URL}`** to register, read pricing, and claim trial quota—or contact your operator for onboarding.

*Capabilities, promotions, failover/sticky settings, Realtime, and CORS depend on deployment and operator configuration. See the Chinese [customer onboarding](TraceHub-客户接入宣传.md) for the full marketing narrative.*
