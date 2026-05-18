# Changelog

All notable changes to this project are documented here. Version numbers align with `site.app_version` in [configs/config.example.yaml](configs/config.example.yaml) when applicable.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Documentation

- Added public docs index ([docs/README.md](docs/README.md)), implementation status in product PRD, API capability matrix in README, marketing URL placeholders ([docs/_site-urls.yaml](docs/_site-urls.yaml)), and standard OSS pages (CONTRIBUTING, SECURITY).

## [1.0.0] - 2026-05-18

### Added

- OpenAI-compatible gateway: `POST /v1/chat/completions`, `GET /v1/models`, optional `/v1/*` passthrough whitelist.
- Native Claude / Gemini channels with OpenAI-style streaming chunks.
- Extended APIs: embeddings, images, audio, videos (including Volcengine Ark), Realtime calls (deployment-dependent).
- Routing: group + model channel selection, priority/weight, failover, optional Redis sticky sessions.
- Billing: token estimation, quota deduction, group/user/model multipliers, no-bill model lists.
- Users: JWT auth, API keys (hashed), balance & usage, redeem codes, email/captcha login (configurable).
- Admin: users, channels, groups, model rates, usage logs, redeem batch, audit log, site branding.
- Web: marketing site, user console, admin UI, in-app `/docs` (zh-CN / zh-TW / en).
- Ops: `/health`, `/ready`, `/metrics`, Docker Compose, single-binary SPA hosting.

### Planned (see [milestones-roadmap.md](docs/development-docs/milestones-roadmap.md))

- Online payments (Alipay / WeChat / Stripe), OAuth login, layered rate limits, fine-grained RBAC, content moderation, webhooks, semantic cache, OpenAPI/Swagger hosting.

[Unreleased]: #
[1.0.0]: #
