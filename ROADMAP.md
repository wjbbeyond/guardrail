# Roadmap

GuardRail's goal is to be the small, practical safety gateway teams can put in front of AI agents before they are ready for a full internal platform.

## Product Goals

- Zero-rewrite adoption for OpenAI-compatible clients.
- Server-side protection for provider API keys.
- Deterministic guardrails for common prompt injection, PII, and secret leakage risks.
- Tenant-level cost and rate controls that are visible to operators.
- Simple deployment first: one binary, Docker, SQLite, and clear HA boundaries.

## Non-Goals For Now

- Replacing a full API gateway or service mesh.
- Becoming a distributed billing database.
- Hosting a managed SaaS control plane in this repository.
- Adding heavyweight policy languages before the core gateway is stable.

## Shipped In v0.1.0

- OpenAI-compatible chat completions proxy.
- Provider routing and failover for OpenAI-compatible APIs, OpenAI, Anthropic, and Google Gemini.
- Prompt injection detection and PII/API-key redaction.
- Tenant proxy keys, budgets, rate limits, and audit records.
- OIDC/SSO token verification with admin group support.
- SQLite audit and cost persistence with versioned migrations.
- Prometheus metrics, Docker, CI, and production-path demo script.
- HA deployment guidance with explicit SQLite boundaries.

## Next Releases

### v0.2

- Expand provider compatibility tests and documented model mappings.
- Add richer audit filtering and export paths.
- Add stricter config validation and startup diagnostics.
- Improve prompt-injection and PII rule packs with measured false-positive examples.

### v0.3

- Optional dashboard for audit, spend, and blocked-request review.
- Shared external stores for globally strict budgets and rate limits.
- Tool-call policy controls for agent workflows.
- Compliance-oriented reports for security and cost events.

### Later

- Semantic cache integration.
- Kubernetes manifests or operator support.
- Policy bundle distribution and signed rule updates.
- Multi-region deployment reference architecture.

## Contribution Priorities

The best contributions are small, tested, and operationally clear. Provider adapters, migration tests, security-rule examples, and deployment docs are especially useful.
