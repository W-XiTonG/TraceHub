# Contributing to TraceHub

Thank you for your interest in TraceHub. Issues and pull requests are welcome.

## Before you start

- Read the [README](README.md) and [docs index](docs/README.md) for scope.
- **Implemented behavior** is defined by code + [README](README.md) + [CHANGELOG](CHANGELOG.md).
- **Planned behavior** is in [docs/产品文档.md](docs/产品文档.md) (implementation status table) and [milestones-roadmap.md](docs/development-docs/milestones-roadmap.md).

## Development setup

1. Copy `configs/config.example.yaml` to `configs/config.yaml` and set `database.dsn`, `security.jwt_secret`.
2. Start PostgreSQL (and Redis if you need sticky sessions / captcha).
3. Backend: `go mod download` && `go run ./cmd/server -config configs/config.yaml`
4. Frontend: `cd web/frontend && npm install && npm run dev`

See [deployment-ops.md](docs/development-docs/deployment-ops.md) for Docker Compose and production notes.

## Common commands

| Command | Purpose |
|---------|---------|
| `make run` | Run backend with local config |
| `make fe` | Frontend dev server |
| `make build` | Build frontend + `bin/tracehub` |
| `make ent` | Regenerate Ent from `internal/ent/schema` |

Requires **Go 1.25+** (see [go.mod](go.mod)).

## Pull requests

Use [.github/pull_request_template.md](.github/pull_request_template.md). Before opening a PR:

- [ ] `go build ./...` passes
- [ ] If you changed frontend: `npm run build` in `web/frontend`
- [ ] If you changed gateway routes or request/response shapes: update [api-design.md](docs/development-docs/api-design.md) and **all three** locale files under `web/frontend/src/locales/part-*-docs.ts` (zh-CN, zh-TW, en) when user-facing docs are affected
- [ ] If you changed `configs/config.example.yaml`: update comments and relevant docs
- [ ] For **marketing copy** on the public site (home/console/docs): follow [公网展示与运维文案分离说明](docs/公网展示与运维文案分离说明.md)—avoid exposing ops paths and **upstream vendor/channel names** in user-facing strings; keep `/metrics` and channel-type details in admin or development-docs only

## Documentation guidelines

| Change type | Update |
|-------------|--------|
| New `/v1` endpoint | `api-design.md`, README API matrix, `part-*-docs.ts` |
| New config key | `config.example.yaml`, deployment-ops or product doc if user-visible |
| Marketing URLs | `docs/_site-urls.yaml` and placeholder-based `.md` only |

## Security

Report vulnerabilities privately—see [SECURITY.md](SECURITY.md). Do not open public issues for sensitive reports.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
