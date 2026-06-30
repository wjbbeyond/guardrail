# GuardRail

GuardRail is a lightweight AI Agent safety gateway. It sits between agent apps and LLM providers, then adds three controls that raw provider SDKs do not give you by default: prompt/PII guardrails, spend budgets, and provider failover.

## Features

- OpenAI-compatible `POST /v1/chat/completions` gateway.
- Provider routing for OpenAI-compatible APIs, OpenAI, Anthropic, and Google Gemini.
- API key pools with per-provider rotation.
- Prompt injection rule detection with `warn` or `block` mode.
- PII and API-key redaction before upstream forwarding.
- Approximate token counting, model pricing, per-request and daily budget circuit breakers.
- SQLite audit log with admin query endpoint.
- Prometheus text metrics at `/metrics`.
- Docker and GitHub Actions CI.

## Quick Start

```bash
export OPENAI_API_KEY="sk-..."
go run ./cmd/guardrail -config configs/guardrail.yaml
```

Send a request through GuardRail:

```bash
curl http://localhost:8080/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello from GuardRail"}]
  }'
```

Health and operations:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/metrics
curl http://localhost:8080/v1/admin/costs
curl http://localhost:8080/v1/admin/audit?limit=20
```

## Configuration

The default config lives at `configs/guardrail.yaml`. Provider API keys can be set in YAML or via environment variables:

- `OPENAI_API_KEY`
- `ANTHROPIC_API_KEY`
- `GEMINI_API_KEY`
- `GUARDRAIL_CONFIG`
- `GUARDRAIL_LISTEN_ADDR`
- `GUARDRAIL_AUDIT_SQLITE_DSN`

## Development

GuardRail requires Go 1.24+.

```bash
make fmt
make test
make build
```

## MVP Scope

This repository is v0.1.0 scope from the development plan: proxy, routing, security rules, PII redaction, cost circuit breakers, audit logs, metrics, Docker, CI, and documentation. Dashboard, semantic cache, tool-call RBAC, compliance reports, and K8s operator are planned for later versions.

## License

Apache-2.0
