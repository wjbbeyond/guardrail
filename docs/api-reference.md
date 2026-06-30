# API Reference

## `GET /healthz`

Returns service health.

## `GET /metrics`

Returns Prometheus text metrics.

Requires an admin API key when auth is enabled.

## `POST /v1/chat/completions`

Accepts OpenAI-compatible Chat Completions payloads.

```json
{
  "model": "gpt-4o-mini",
  "messages": [
    {"role": "user", "content": "Hello"}
  ]
}
```

The response is OpenAI-compatible. For OpenAI-compatible upstreams, GuardRail forwards the provider response unchanged.

Requires a proxy API key when auth is enabled.

## `GET /v1/admin/costs`

Returns current daily spend and configured budgets for a tenant. Use `?tenant_id=acme`; omitted `tenant_id` means `default`.

Requires an admin API key when auth is enabled.

## `GET /v1/admin/audit?limit=100`

Returns recent audit events from SQLite. Events include `tenant_id`.

Requires an admin API key when auth is enabled.
