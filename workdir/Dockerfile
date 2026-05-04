# syntax=docker/dockerfile:1
# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependency layer separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      cmd/
COPY internal/ internal/
COPY pkg/      pkg/

RUN CGO_ENABLED=0 GOOS=linux \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /ruptura \
      ./cmd/ruptura

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# distroless/static: no shell, minimal attack surface, non-root by default.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /ruptura /ruptura

# REST API + Prometheus remote_write
EXPOSE 8080
# OTLP ingest (metrics, logs, traces)
EXPOSE 4317
# gRPC ingest
EXPOSE 9090

# Mount a PVC here for BadgerDB persistence.
VOLUME ["/var/lib/ruptura/data"]

# Liveness:  GET /api/v2/health → 200
# Readiness: GET /api/v2/ready  → 200
ENTRYPOINT ["/ruptura"]
CMD ["--port=8080", "--otlp-port=4317", "--storage=/var/lib/ruptura/data"]
