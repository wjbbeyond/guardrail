# Changelog

## Unreleased

- Added inbound proxy API key auth and separate admin API key auth.
- Persisted daily budget spend to SQLite so restarts do not reset cost state.
- Added `scripts/demo.sh` to verify auth, redaction, blocking, budgets, audit, and metrics against a local mock provider.

## 0.1.0

- Initial MVP: OpenAI-compatible gateway route.
- Provider routing for OpenAI-compatible, OpenAI, Anthropic, and Google.
- Prompt injection findings and PII redaction.
- Cost tracking and budget circuit breaker.
- SQLite audit log and admin endpoints.
- Prometheus metrics, Dockerfile, Compose, CI, and documentation.
