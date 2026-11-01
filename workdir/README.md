# Ruptura engine

Source of the Ruptura engine, command line tool, operator and web UI. For an overview of the project, see the [main README](../README.md).

## Layout

| Path | Content |
|---|---|
| `cmd/ruptura` | Engine entry point |
| `cmd/ruptura-ctl` | Command line tool |
| `internal/` | Engine packages |
| `pkg/` | Public Go packages |
| `operator/` | Kubernetes operator, a separate Go module |
| `ui/` | Svelte web UI |

## Build and test

Requires Go 1.18 or later.

```bash
go build ./...
go test ./...
```

Run the engine locally with synthetic data:

```bash
go run ./cmd/ruptura --demo
```

Web UI:

```bash
cd ui
npm install
npm run dev
```

Operator:

```bash
cd operator
go test ./...
```

## Contributing

See the [contributing guide](../CONTRIBUTING.md).
