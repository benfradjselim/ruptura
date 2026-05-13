# syntax=docker/dockerfile:1

# ── Stage 0: Frontend build ────────────────────────────────────────────────────
# Build the Svelte UI from source — guarantees the correct bundle is embedded
# regardless of which static files are pre-committed in git.
FROM node:20-alpine AS frontend
WORKDIR /app
COPY ui/package.json ui/package-lock.json* ./
RUN npm ci
COPY ui/ ./
# vite.config.js: outDir: '../web' → output lands at /web
RUN npm run build

# ── Stage 1: Build Go binary ──────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependency layer separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      cmd/
COPY internal/ internal/
COPY pkg/      pkg/

# Overwrite pre-committed static files with the fresh Vite build above.
# This is the authoritative source — always built from current source.
COPY --from=frontend /web internal/ui/static

RUN echo "=== UI static assets (fresh Vite build) ===" && ls internal/ui/static/assets/

RUN CGO_ENABLED=0 GOOS=linux \
    go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /ruptura \
      ./cmd/ruptura

RUN echo "=== Binary embed check ===" && \
    grep -c "BOiiv4FG" /ruptura && echo "CORRECT: BOiiv4FG in binary" || echo "WRONG: BOiiv4FG not in binary"; \
    grep -c "Bob2sadS" /ruptura && echo "WRONG: Bob2sadS in binary" || echo "correct: Bob2sadS not in binary"

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
# ubi-micro: minimal Red Hat UBI — required for Red Hat OperatorHub preflight certification.
FROM registry.access.redhat.com/ubi9/ubi-micro

ARG VERSION=6.7.0

LABEL name="ruptura" \
      vendor="Ruptura" \
      version="${VERSION}" \
      release="1" \
      summary="Predictive workload health platform for Kubernetes" \
      description="Ruptura detects workload ruptures before they happen by modeling composite health signals from OTLP metrics, logs, and traces."

COPY --from=builder /ruptura /ruptura

# REST API + Prometheus remote_write
EXPOSE 8080
# OTLP ingest (metrics, logs, traces)
EXPOSE 4317
# gRPC ingest
EXPOSE 9090

# Mount a PVC here for BadgerDB persistence.
VOLUME ["/var/lib/ruptura/data"]

USER 65532:65532

# Liveness:  GET /api/v2/health → 200
# Readiness: GET /api/v2/ready  → 200
ENTRYPOINT ["/ruptura"]
CMD ["--port=8080", "--otlp-port=4317", "--storage=/var/lib/ruptura/data"]
