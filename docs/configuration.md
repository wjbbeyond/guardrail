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
  oidc:
    enabled: false
    issuer_url: https://accounts.example.com
    client_id: guardrail
    tenant_claim: tenant
    admin_group_claim: groups
    admin_groups:
      - guardrail-admins
```

When auth is enabled, `POST /v1/chat/completions` requires a proxy key via `Authorization: Bearer <key>` or `X-GuardRail-API-Key`. Admin endpoints and `/metrics` require an admin key via `Authorization: Bearer <key>` or `X-GuardRail-Admin-Key`.

When OIDC is enabled, GuardRail discovers the provider from `issuer_url`, verifies bearer ID tokens for the configured `client_id`, and maps the tenant from `tenant_claim`. Admin OIDC access is granted only when `admin_groups` contains a value from `admin_group_claim`.

Environment variables can provide one key or a comma-separated list:

- `GUARDRAIL_PROXY_API_KEY`
- `GUARDRAIL_PROXY_API_KEYS`
- `GUARDRAIL_ADMIN_API_KEY`
- `GUARDRAIL_ADMIN_API_KEYS`
- `GUARDRAIL_OIDC_ISSUER_URL`
- `GUARDRAIL_OIDC_CLIENT_ID`
- `GUARDRAIL_OIDC_TENANT_CLAIM`

## Tenants

```yaml
tenants:
  - id: acme
    proxy_api_keys:
      - gr_proxy_acme_...
    daily_budget_usd: 50
    per_request_budget_usd: 0.5
    rate_limit:
      enabled: true
      requests_per_minute: 120
      burst: 20
```

Top-level `auth.proxy_api_keys` map to the `default` tenant. Tenant `proxy_api_keys` map directly to that tenant. Tenant budgets and rate limits override the global defaults when set.

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
Daily spend is persisted per tenant to the configured SQLite database, so restarting GuardRail does not reset the active daily budget.

## Rate Limit

```yaml
rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10
```

The built-in limiter is tenant-aware and instance-local. In multi-replica deployments, use an edge or shared external limiter if the business requires strict global limits.

## Pricing

```yaml
pricing:
  url: https://example.com/guardrail-pricing.json
  refresh_interval: 24h
```

The optional pricing feed is JSON:

```json
{
  "models": {
    "gpt-4o-mini": {
      "input_per_mtok": 0.15,
      "output_per_mtok": 0.60
    }
  }
}
```

GuardRail keeps built-in prices when no feed is configured. If refresh fails, the last valid table remains active.
