# Contributing

Thanks for helping improve GuardRail. This project is intentionally small and operational: changes should make the gateway safer, easier to deploy, or easier to trust in production.

## Local Setup

```bash
go version
go mod download
make test
make build
```

Use Go 1.25 or newer. Run the demo before opening a larger PR:

```bash
./scripts/demo.sh
```

## Good First Contributions

- Documentation fixes that make setup clearer.
- Provider compatibility fixes with tests.
- Security rule improvements with false-positive examples.
- Deployment examples that keep secrets out of source control.
- Tests for migration, budget, rate-limit, or auth edge cases.

## Pull Requests

1. Create a focused branch.
2. Add or update tests for changed behavior.
3. Run `make test`, `make build`, and `./scripts/demo.sh` when the change touches runtime behavior.
4. Keep changes scoped and explain user impact in the PR body.
5. Update docs when config, APIs, deployment behavior, or security posture changes.

Security-sensitive changes should describe the threat model, false-positive risk, and regression tests.

## Compatibility

GuardRail exposes an OpenAI-compatible `POST /v1/chat/completions` surface. Avoid breaking that request/response contract unless the change is explicitly versioned and documented.

Config changes should be backward compatible when practical. If a migration is required, add a test that opens a legacy database or config shape and proves the upgrade path.

## Security and Cost Changes

For auth, OIDC, tenant isolation, prompt inspection, budget, or rate-limit changes, include:

- the failure mode being prevented
- how false positives or false negatives are handled
- a regression test for the risky path
- any operational tradeoff in `docs/`

Do not include real provider API keys, customer prompts, production tenant IDs, or private logs in issues or pull requests.

## Release Notes

User-visible changes belong in `CHANGELOG.md`. Keep entries short and written from the user's point of view.
