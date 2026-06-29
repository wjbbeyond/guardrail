# AGENTS.md

Go 1.23+ HTTP reverse proxy for OpenAI-compatible AI Agent traffic.

## Commands
- `make fmt` — format Go files.
- `make test` — run race-enabled tests.
- `make build` — build `bin/guardrail`.
- `make run` — run the gateway with `configs/guardrail.yaml`.

## Architecture
- `cmd/guardrail/main.go` — entrypoint only.
- `internal/gateway/` — HTTP routes, proxy flow, admin API.
- `internal/provider/` — provider routing and OpenAI/Anthropic/Google adapters.
- `internal/security/` — prompt injection findings and PII redaction.
- `internal/cost/` — token estimates, pricing, budget decisions.
- `internal/audit/` — SQLite audit event store.

## Conventions
- Use `slog` for logs.
- `context.Context` is first for public I/O functions.
- Errors are wrapped with `%w`.
- Keep source files below 250 pure LOC.
