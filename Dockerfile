# Multi-stage Dockerfile for all MLOps microservices
# Usage: docker build --build-arg SERVICE=collector -t mlops-anomaly-collector:latest .
# Valid SERVICE values: collector, processor, trainer, detector, exporter, dashboard

ARG SERVICE=collector

# ============================================================
# Stage 1: Builder - install dependencies
# ============================================================
FROM python:3.11-slim AS builder

ARG SERVICE
WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements/common.txt /build/common.txt
RUN pip install --no-cache-dir --prefix=/install -r /build/common.txt

COPY requirements/${SERVICE}.txt /build/service.txt
RUN pip install --no-cache-dir --prefix=/install -r /build/service.txt

# ============================================================
# Stage 2: Runtime - slim image
# ============================================================
FROM python:3.11-slim AS runtime

ARG SERVICE
ENV SERVICE=${SERVICE}

RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /install /usr/local

WORKDIR /app

COPY src/shared/ ./shared/
COPY services/${SERVICE}/ ./service/

RUN mkdir -p /data

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT:-8001}/health || exit 1

CMD ["sh", "-c", "python /app/service/main.py"]
