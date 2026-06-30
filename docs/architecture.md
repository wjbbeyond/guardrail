# Architecture

GuardRail is a reverse proxy for AI Agent traffic. Agents keep using the OpenAI-compatible Chat Completions shape and point `base_url` to GuardRail.

```mermaid
flowchart LR
  A["AI Agent"] --> B["GuardRail /v1/chat/completions"]
  B --> C["Inbound Auth"]
  C --> D["Security Guard"]
  D --> E["Cost Circuit Breaker"]
  E --> F["Provider Router"]
  E --> K["SQLite Cost Ledger"]
  F --> G["OpenAI-compatible"]
  F --> H["Anthropic"]
  F --> I["Google Gemini"]
  B --> J["SQLite Audit"]
  B --> L["Prometheus Metrics"]
```

## Request Flow

1. Require a valid proxy API key for chat requests when auth is enabled.
2. Decode the OpenAI-compatible chat request.
3. Inspect prompt text for prompt injection and PII findings.
4. Redact PII when configured.
5. Estimate prompt and completion tokens before sending upstream.
6. Reject requests that would exceed per-request or daily budget using persisted daily spend.
7. Route to matching providers by model, failing over on `429` and `5xx`.
8. Copy provider responses back to the caller.
9. Record cost, metrics, and audit events.

## Provider Adapters

- `openai` and `openai-compatible` forward the Chat Completions body transparently.
- `anthropic` maps Chat Completions to Messages API for non-streaming calls.
- `google` maps Chat Completions to Gemini `generateContent` for non-streaming calls.

Streaming pass-through is enabled for OpenAI-compatible providers in v0.1.
