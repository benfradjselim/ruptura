# ── Stage 1: build the Go binary ─────────────────────────────────────────────
FROM golang:1.22-alpine AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/      cmd/
COPY internal/ internal/
COPY pkg/      pkg/
COPY sdk/      sdk/

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /kairo-core \
    ./cmd/kairo-core

# ── Stage 2: minimal runtime image ───────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=builder /kairo-core /kairo-core

WORKDIR /
EXPOSE 8080 9090

# Liveness  probe: GET /api/v2/health -> 200
# Readiness probe: GET /api/v2/ready  -> 200
ENTRYPOINT ["/kairo-core"]
CMD ["--port", "8080", "--storage", "/var/lib/kairo/data"]
