# Security Policy

## Supported versions

Security fixes are applied to the latest release on the default branch. Self-hosted deployments should track `main` or tagged releases and keep dependencies updated.

## Reporting a vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report privately to the repository maintainers (e.g. via the contact on your deployment’s site or the email listed in your organization’s TraceHub operations channel). Include:

- Description and impact
- Steps to reproduce
- Affected version / commit
- Suggested fix (if any)

We aim to acknowledge reports within a reasonable timeframe and coordinate disclosure after a fix is available.

## Deployment hardening (self-hosted)

Before production:

1. Replace `security.jwt_secret` and `security.aes_key_base64` in [configs/config.yaml](configs/config.yaml)—never use example placeholders.
2. Use HTTPS in front of the gateway (reverse proxy / load balancer).
3. Restrict `/metrics` with `security.metrics_bearer_token` or network policy.
4. Rotate API keys and upstream channel secrets on compromise; user keys are stored hashed—revoke in the console.
5. Keep PostgreSQL and Redis credentials out of version control.

See [security-compliance.md](docs/development-docs/security-compliance.md) and [deployment-ops.md](docs/development-docs/deployment-ops.md) for checklists.

## Out of scope

- Misconfiguration of a public instance by the operator (exposed admin passwords, open Redis, etc.)
- Abuse of API keys obtained through phishing or client-side leakage
