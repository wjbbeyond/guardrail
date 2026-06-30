# GuardRail

GuardRail is a lightweight AI Agent safety gateway. It sits between agent apps and LLM providers, then adds three controls that raw provider SDKs do not give you by default: prompt/PII guardrails, spend budgets, and provider failover.

## Features

- OpenAI-compatible `POST /v1/chat/completions` gateway.
- Inbound proxy API keys and separate admin API keys.
- Tenant-aware proxy keys, budgets, rate limits, and audit records.
- OIDC/SSO bearer-token verification for proxy traffic, with optional admin group authorization.
- Provider routing for OpenAI-compatible APIs, OpenAI, Anthropic, and Google Gemini.
- API key pools with per-provider rotation.
- Prompt injection rule detection with `warn` or `block` mode.
- PII and API-key redaction before upstream forwarding.
- Approximate token counting, model pricing, and SQLite-persisted per-request and daily budget circuit breakers.
- Optional HTTP pricing feed refresh for model prices.
- SQLite audit log with admin query endpoint.
- Admin-protected Prometheus text metrics at `/metrics`.
- Versioned SQLite schema migrations.
- Docker and GitHub Actions CI.

## Quick Start

```bash
export OPENAI_API_KEY="sk-..."
export GUARDRAIL_PROXY_API_KEY="dev-proxy-key"
export GUARDRAIL_ADMIN_API_KEY="dev-admin-key"
go run ./cmd/guardrail -config configs/guardrail.yaml
```

Send a request through GuardRail:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer dev-proxy-key' \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello from GuardRail"}]
  }'
```

Health and operations:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/metrics -H 'X-GuardRail-Admin-Key: dev-admin-key'
curl 'http://localhost:8080/v1/admin/costs?tenant_id=default' -H 'X-GuardRail-Admin-Key: dev-admin-key'
curl 'http://localhost:8080/v1/admin/audit?limit=20' -H 'X-GuardRail-Admin-Key: dev-admin-key'
```

Run the local production-path demo without a real LLM provider:

```bash
./scripts/demo.sh
```

## Configuration

The default config lives at `configs/guardrail.yaml`. Provider API keys can be set in YAML or via environment variables:

- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`
- `GEMINI_API_KEY`
- `GUARDRAIL_CONFIG`
- `GUARDRAIL_LISTEN_ADDR`
- `GUARDRAIL_PROXY_API_KEY` or `GUARDRAIL_PROXY_API_KEYS`
- `GUARDRAIL_ADMIN_API_KEY` or `GUARDRAIL_ADMIN_API_KEYS`
- `GUARDRAIL_OIDC_ISSUER_URL`
- `GUARDRAIL_OIDC_CLIENT_ID`
- `GUARDRAIL_OIDC_TENANT_CLAIM`
- `GUARDRAIL_AUDIT_SQLITE_DSN`

See `docs/configuration.md` for tenant, OIDC, rate-limit, pricing, and migration details. See `docs/ha-deployment.md` for the recommended HA topology and current state boundaries.

## Development

GuardRail requires Go 1.25+.

```bash
make fmt
make test
make build
```

## MVP Scope

This repository is v0.1.0 scope from the development plan: proxy, routing, security rules, PII redaction, cost circuit breakers, audit logs, metrics, Docker, CI, and documentation. Dashboard, semantic cache, tool-call RBAC, compliance reports, and K8s operator are planned for later versions.

## License

Apache-2.0
