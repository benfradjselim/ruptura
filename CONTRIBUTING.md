# Contributing to Ruptura

Welcome, and thank you for your interest in contributing to Ruptura! Ruptura is a predictive AIOps engine for Kubernetes that uses a 5-model ML ensemble, the Fused Rupture Index (FRI), and 10 composite KPI signals to detect workload anomalies before they cause outages — and automatically remediates them via an action engine with Kubernetes-native actuators. Every contribution, from bug reports to new features to documentation improvements, helps make cloud-native infrastructure more reliable for everyone.

---

## Ways to Contribute

- **Bug reports and feature requests** — Open a [GitHub Issue](https://github.com/benfradjselim/ruptura/issues). Search for existing issues first to avoid duplicates.
- **Pull requests** — Fix bugs, implement features, or improve performance. See the PR process below.
- **Documentation** — Improve the README, workdir docs, inline code comments, or API spec.
- **Community** — Answer questions in [GitHub Discussions](https://github.com/benfradjselim/ruptura/discussions), share use cases, or write blog posts about Ruptura.
- **Testing** — Add unit tests, integration tests, or report edge cases you encounter in production.

---

## Development Setup

### Prerequisites

- **Go 1.22+** — <https://go.dev/dl/>
- **Node.js 20+** and **npm** — for the Svelte UI
- **kubectl** and a Kubernetes cluster (or [kind](https://kind.sigs.k8s.io/)) for integration testing
- **Helm 3** — for chart development

### Building

```bash
# Clone the repository
git clone https://github.com/benfradjselim/ruptura.git
cd ruptura

# Build the Go engine and CLI
make build

# Run all Go tests
make test

# Start the Svelte dashboard in development mode
cd ui && npm install && npm run dev
```

The engine binary lands at `workdir/bin/ruptura` and the CLI at `workdir/bin/ruptura-ctl`.

### Running Tests

```bash
# Run the full Go test suite
cd workdir && go test ./...

# Run tests for a specific package
cd workdir && go test ./internal/analyzer/...

# Run with race detector
cd workdir && go test -race ./...
```

---

## PR Process

### Branch Naming

Use the format `<type>/<short-description>`:

```
feat/slo-config-ui
fix/json-crash-on-empty-payload
docs/update-helm-quickstart
chore/bump-go-1.23
```

### Opening a Pull Request

1. Fork the repository and create a branch from `main`.
2. Make your changes with appropriate tests.
3. Run `make test` and ensure all tests pass.
4. Run `gofmt` and `golangci-lint run` on any Go code you changed.
5. Sign off every commit (see DCO section below).
6. Open a PR against `main`. Fill in the PR template — describe the problem, the solution, and how to test.
7. A maintainer will review within a reasonable timeframe. Please be patient; this is a small project.
8. Address review feedback. Once approved, a maintainer will merge.

### PR Template Checklist

- [ ] Tests added or updated
- [ ] `make test` passes
- [ ] `gofmt` and `golangci-lint` clean
- [ ] DCO sign-off on all commits (`git commit -s`)
- [ ] PR description explains the change and how to verify it

---

## Code Standards

### Go

- Format with `gofmt` before committing. CI will reject unformatted code.
- Lint with `golangci-lint run` (config in `.golangci.yml`).
- Keep functions focused and testable. Prefer table-driven tests.
- Add comments on exported symbols following Go doc conventions.

### Svelte / TypeScript (UI)

- Use TypeScript for all new UI code.
- Follow the existing component structure under `ui/src/`.
- Run `npm run lint` before pushing.
- Keep components small; extract logic into `lib/` utilities where possible.

### General

- Prefer clarity over cleverness.
- Avoid breaking changes to public API surfaces without a discussion issue first.
- Keep commits atomic — one logical change per commit.

---

## Commit Message Format

Ruptura uses [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

[optional body]

[optional footer — DCO sign-off, closes #N]
```

**Types:** `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `ci`

**Examples:**

```
feat(analyzer): add velocity signal to 10-KPI set
fix(api): return 400 on malformed OTLP payload
docs(helm): add values reference table to chart README
chore(deps): upgrade Go toolchain to 1.23.1
```

The summary line must be 72 characters or fewer, written in the imperative mood ("add", not "added" or "adds").

---

## Developer Certificate of Origin (DCO)

Ruptura uses the [Developer Certificate of Origin](https://developercertificate.org/) (version 1.1) as its contributor agreement. By signing off a commit, you certify that:

1. You wrote the contribution yourself, OR
2. You have the right to submit it under the Apache 2.0 license, OR
3. A third party certified (1) or (2) and you have not modified it.

**Every commit must carry a `Signed-off-by` line** matching your real name and email:

```bash
git commit -s -m "feat(pipeline): improve ensemble re-weighting"
# Adds: Signed-off-by: Your Name <you@example.com>
```

PRs with unsigned commits will not be merged. If you forgot to sign off, use:

```bash
git rebase --signoff HEAD~N   # sign last N commits
git push --force-with-lease
```

---

## Community Channels

- **GitHub Discussions** — <https://github.com/benfradjselim/ruptura/discussions> — Questions, ideas, and general discussion.
- **GitHub Issues** — <https://github.com/benfradjselim/ruptura/issues> — Bug reports and feature requests.

---

## Recognition

All contributors are acknowledged in release notes. Sustained contributors may be invited to take on Reviewer or Maintainer roles as described in [GOVERNANCE.md](GOVERNANCE.md).

---

## License

By contributing to Ruptura, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).
