# Configuration

GuardRail loads `configs/guardrail.yaml` by default. Pass another file with:

```bash
guardrail -config path/to/config.yaml
```

## Server

```yaml
server:
  listen_addr: ":8080"
  read_timeout: 15s
  write_timeout: 0s
  shutdown_timeout: 20s
  log_level: info
```

`write_timeout: 0s` allows long LLM streams.

## Auth

```yaml
auth:
  enabled: true
  proxy_api_keys:
    - gr_proxy_...
  admin_api_keys:
    - gr_admin_...
```

When auth is enabled, `POST /v1/chat/completions` requires a proxy key via `Authorization: Bearer <key>` or `X-GuardRail-API-Key`. Admin endpoints and `/metrics` require an admin key via `Authorization: Bearer <key>` or `X-GuardRail-Admin-Key`.

Environment variables can provide one key or a comma-separated list:

- `GUARDRAIL_PROXY_API_KEY`
- `GUARDRAIL_PROXY_API_KEYS`
- `GUARDRAIL_ADMIN_API_KEY`
- `GUARDRAIL_ADMIN_API_KEYS`

## Providers

```yaml
providers:
  - name: openai
    type: openai
    base_url: https://api.openai.com/v1
    api_keys:
      - sk-...
    models:
      - gpt-4o-mini
```

Supported `type` values:

- `openai`
- `openai-compatible`
- `anthropic`
- `google`

## Security

```yaml
security:
  prompt_injection_mode: warn
  pii_mode: redact
```

Modes:

- `off`
- `warn`
- `redact` for PII
- `block`

## Cost

```yaml
cost:
  daily_budget_usd: 10
  per_request_budget_usd: 1
```

Budgets use model price estimates and approximate token counting before the request. Provider `usage` fields improve completion-token accounting after the response.
Daily spend is persisted to the configured SQLite database, so restarting GuardRail does not reset the active daily budget.
