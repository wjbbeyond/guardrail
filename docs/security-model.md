# Security Model

GuardRail v0.1 focuses on low-latency deterministic controls.

## Inbound Auth

GuardRail separates inbound client access from upstream provider credentials. Clients call the proxy with a GuardRail proxy API key, while provider API keys stay in server-side config. Admin endpoints and `/metrics` use a separate admin key set.

## OIDC / SSO

When enabled, OIDC bearer tokens are verified with provider discovery and JWKS validation through `go-oidc`. The token `tenant_claim` selects the tenant for budgets, rate limits, and audit. Admin SSO requires a configured group claim and group allow-list.

## Tenant Isolation

Tenant identity is resolved before request body parsing. Security decisions, budget checks, rate limits, and audit records use that tenant ID. Unknown OIDC tenants are rejected when explicit tenants are configured.

## Prompt Injection

The gateway scans prompt text for common prompt-injection patterns:

- instruction override attempts
- system prompt exfiltration attempts
- developer mode jailbreak wording
- XML-style fake role tags

`prompt_injection_mode` decides whether findings only annotate the request or block it.

## PII Redaction

The gateway detects and redacts:

- email addresses
- credit-card-like numbers
- OpenAI-style API keys
- generic `api_key`, `token`, and `secret` assignments
- custom regex patterns from config

Redaction runs before forwarding the request upstream.

## Audit

Each request records:

- request ID
- route
- provider
- model
- status code
- token counts
- estimated cost
- security action
- latency

Audit data is stored in SQLite by default.

## Cost State

Daily spend is also persisted per tenant in SQLite. This prevents a process restart from resetting active daily budget enforcement.
