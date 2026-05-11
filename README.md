# TraceHub

**English** · [简体中文](README.zh-CN.md)

**TraceHub** is an open-source **AI API gateway / relay**: it exposes multiple upstreams (OpenAI, Claude, Gemini, DeepSeek, etc.) behind a single **OpenAI-compatible API**, with user accounts, API keys, groups & model rate multipliers, usage metering & billing, an admin console, and a marketing site UI.

For full product requirements and the roadmap, see the docs in this repository:

- [Product requirements (Chinese)](docs/产品文档.md)
- [Development documentation index](docs/development-docs/README.md)

---

## Stack

| Layer | Technologies |
|--------|----------------|
| Backend | Go 1.25+, Gin, [Ent](https://entgo.io/) ORM, PostgreSQL, Redis |
| Frontend | Vue 3, Vite, Tailwind CSS v4, Pinia, Vue Router, Axios |
| Observability | Prometheus `/metrics`, `/health`, `/ready` |

---

## Features (high level)

- **Gateway**: `POST /v1/chat/completions` (OpenAI-compatible upstreams with SSE; native **Claude** / **Gemini** channels emit OpenAI-style `chat.completion.chunk` streams), `GET /v1/models`; optional **passthrough** for additional `/v1/*` paths via `site.openai_passthrough.allow_prefixes` (disabled by default); account/admin-style OpenAI paths under `/v1` are blocked by default (`site.block_openai_account_admin_paths`)
- **Routing**: channels filtered by group & model, weighted random selection; optional Redis sticky sessions
- **Billing**: token estimation & quota deduction; group / user / per-model multipliers
- **Users**: registration & login (JWT), API keys (hashed), balance & usage, redeem codes, password change
- **Admin**: users & channels, groups & model rates, usage logs, batch redeem codes, audit log writes; channel secrets stored with AES-GCM
- **Web**: landing (hero / models / pricing / FAQ), user console, admin UI; production build can be served as SPA from the Go binary

Planned items (payments, OAuth, layered rate limits, finer RBAC, etc.) are described in [milestones-roadmap.md](docs/development-docs/milestones-roadmap.md).

---

## Prerequisites

- Go 1.25+
- Node.js 22+ (local dev / frontend build only)
- PostgreSQL 16+
- Redis 7+ (optional; sticky session features degrade without it)

---

## Quick start

### 1. Configuration

Copy the sample config and edit DSN & secrets:

**Windows (cmd/PowerShell)**

```powershell
copy configs\config.example.yaml configs\config.yaml
```

**Linux / macOS**

```bash
cp configs/config.example.yaml configs/config.yaml
```

Edit `configs/config.yaml`: `database.dsn`, `security.jwt_secret`, and optionally `bootstrap` admin seeding.

### 2. PostgreSQL & Redis

Run services locally or use Docker Compose (below).

### 3. Backend

```bash
go mod download
go run ./cmd/server -config configs/config.yaml
```

Listens on `server.addr` from config (example `:8080`). First run applies Ent migrations and seed data (default group & admin when `bootstrap` is set).

### 4. Frontend (development)

```bash
cd web/frontend
npm install
npm run dev
```

Vite proxies `/api`, `/v1`, etc. to `http://127.0.0.1:8080`.

### 5. Production build (single binary + static)

```bash
make build
```

Or: `npm run build` under `web/frontend`, then `go build -o bin/tracehub ./cmd/server`. If `web/frontend/dist` exists, the server serves static assets and SPA fallback routes.

---

## Docker Compose (all-in-one local stack)

```bash
docker compose -f deploy/docker-compose.local.yml up --build
```

Mounts `configs/config.docker.yaml` as `/app/config.yaml` in the app container. **Change default passwords and `jwt_secret` before production.**

---

## Makefile shortcuts

| Target | Description |
|--------|-------------|
| `make run` | Run backend with `configs/config.yaml` |
| `make fe` | Dev server for the frontend |
| `make build` | Build frontend + compile backend to `bin/tracehub` |
| `make ent` | Regenerate Ent code from `internal/ent/schema` |
| `make docker-up` | Build & start the Compose stack |

---

## Repository layout (partial)

```text
cmd/server/          # Application entrypoint
internal/ent/      # Ent schemas & generated code
internal/gateway/    # Router selection & relay adapters
internal/handler/    # HTTP handlers (user / admin / gateway)
internal/server/     # Gin router & optional SPA hosting
web/frontend/        # Vue 3 SPA
configs/             # Sample & Docker-oriented configs
docs/                # Product & engineering docs
deploy/              # Compose & deployment descriptors
```

---

## License

[MIT License](LICENSE).

---

## Contributing

Issues and pull requests are welcome. PRs can follow [.github/pull_request_template.md](.github/pull_request_template.md).
