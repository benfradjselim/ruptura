# Contributing

Contributions are welcome — bug reports, features, documentation improvements, and SDK extensions.

## Branch strategy

| Branch | Purpose |
|--------|---------|
| `main` | Stable release — no direct pushes |
| `v6.1` | Current development branch — target PRs here |
| `v6.1_*` | Feature branches (e.g. `v6.1_india`, `v6.1_juliet`) |

## Development setup

```bash
git clone https://github.com/benfradjselim/kairo-core.git
cd kairo-core/workdir

# Build
go build ./...

# Run all tests
go test -race -timeout=120s ./...

# Coverage
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total

# Lint
golangci-lint run --timeout=5m
```

Requires: **Go 1.18+**

## Pull request checklist

- [ ] Targets `v6.1` branch
- [ ] `go build ./...` passes
- [ ] `go test -race ./...` passes with ≥ 80% coverage on changed packages
- [ ] No hardcoded credentials or secrets
- [ ] Commit messages follow `feat:` / `fix:` / `docs:` / `test:` / `ci:` convention
- [ ] Description explains *why*, not just *what*

## Test coverage requirement

All packages must maintain **≥ 80% test coverage**. The CI gate fails below this threshold.

```bash
# Check coverage for a specific package
go test -coverprofile=coverage.out ./internal/pipeline/metrics/...
go tool cover -func=coverage.out
```

## Reporting bugs

Open an issue on GitHub with:

1. Kairo version (`kairo-core --version`)
2. `kairo.yaml` (sanitised — remove secrets)
3. Relevant logs
4. Steps to reproduce

## Proposing features

Open a GitHub Discussion under the *Ideas* category. Include:

- Problem statement
- Proposed solution
- Alternatives considered

Large features (new ingest protocol, new composite signal, new action provider) benefit from a short design doc before implementation.

## Code style

- **Go 1.18** — no generics (`any`, `~`, type parameters), no `log/slog`, no `errors.Join`
- Immutable data patterns — never mutate a struct in-place, return new copies
- No comments explaining *what* the code does — only *why* (non-obvious constraints)
- Files ≤ 800 lines, functions ≤ 50 lines
- Module path: `github.com/benfradjselim/kairo-core`
