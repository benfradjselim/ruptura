# ── Stage 1: build the UI ────────────────────────────────────────────────────
FROM node:20-alpine AS ui-builder
WORKDIR /app/ui
COPY ui/package*.json ./
RUN npm ci --silent
COPY ui/ ./
RUN npm run build          # outputs to ../web (vite.config.js: outDir: '../web')

# ── Stage 2: build the Go binary ─────────────────────────────────────────────
FROM golang:1.21-alpine AS go-builder
WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy sources
COPY cmd/    cmd/
COPY internal/ internal/
COPY pkg/    pkg/

# Copy compiled UI artefacts from stage 1
COPY --from=ui-builder /app/web ./web

# Embed web dir at build time (served at runtime via http.FileServer)
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ohe ./cmd/ohe

# ── Stage 3: minimal runtime image ───────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=go-builder /ohe /ohe

# Embed the compiled UI so the binary can serve it
COPY --from=go-builder /app/web /web

WORKDIR /
EXPOSE 8080

# Liveness  probe: GET /api/v1/health/live  → 200
# Readiness probe: GET /api/v1/health/ready → 200
ENTRYPOINT ["/ohe", "central"]
