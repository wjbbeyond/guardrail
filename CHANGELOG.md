# Changelog

## Unreleased

- No changes yet.

## 0.1.0 - 2026-06-30

- Initial MVP: OpenAI-compatible gateway route.
- Provider routing for OpenAI-compatible, OpenAI, Anthropic, and Google.
- Prompt injection findings and PII redaction.
- Cost tracking and budget circuit breaker.
- SQLite audit log and admin endpoints.
- Prometheus metrics, Dockerfile, Compose, CI, and documentation.
- Added tenant-aware proxy keys, budgets, rate limits, and audit records.
- Added OIDC/SSO bearer-token verification for proxy traffic and optional admin groups.
- Added versioned SQLite migrations and tenant-aware cost migration from legacy state.
- Added optional HTTP pricing-feed refresh for model prices.
- Added HA deployment guidance.
- Added inbound proxy API key auth and separate admin API key auth.
- Persisted daily budget spend to SQLite so restarts do not reset cost state.
- Added `scripts/demo.sh` to verify auth, redaction, blocking, budgets, audit, and metrics against a local mock provider.
