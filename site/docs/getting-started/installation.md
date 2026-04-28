# Installation

## Kubernetes

The recommended production deployment uses the bundled manifests or Helm chart.

### Using `kubectl`

```bash
git clone https://github.com/benfradjselim/kairo-core.git
cd kairo-core

# Build the image (or use a pre-built tag)
docker build -t kairo-core:6.1.0 .

# Create namespace + deploy
kubectl apply -f deploy/

# Verify pods are running
kubectl get pods -n kairo-system

# Port-forward to test locally
kubectl port-forward svc/kairo-core 8080:8080 -n kairo-system
curl http://localhost:8080/api/v2/health
```

### Using Helm

```bash
helm install kairo-core ./helm \
  --namespace kairo-system \
  --create-namespace \
  --set auth.jwtSecret=$(openssl rand -hex 32) \
  --set storage.size=20Gi
```

To upgrade:

```bash
helm upgrade kairo-core ./helm --namespace kairo-system
```

### Using the KairoInstance CRD (Operator)

If you have the Kairo Operator installed, deploy a full instance declaratively:

```yaml
apiVersion: kairo.io/v1alpha1
kind: KairoInstance
metadata:
  name: production
  namespace: kairo-system
spec:
  image: kairo-core:6.1.0
  port: 8080
  storageSize: 20Gi
  apiKey:
    secretRef: kairo-api-key
  replicas: 1
```

```bash
kubectl apply -f kairo-instance.yaml
```

See [Operator →](../architecture/operator.md) for full CRD reference.

---

## Docker

```bash
docker run -d \
  --name kairo \
  -p 8080:8080 \
  -p 9090:9090 \
  -v kairo-data:/var/lib/kairo \
  -e KAIRO_JWT_SECRET=$(openssl rand -hex 32) \
  kairo-core:6.1.0
```

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | REST API v2, Prometheus metrics |
| 9090 | gRPC | gRPC ingest (v6.1) |

Verify:

```bash
curl http://localhost:8080/api/v2/health
# {"status":"ok","rupture_detection":"active"}
```

---

## Binary

Download the latest release binary and run it directly:

```bash
# Linux amd64
curl -fsSL https://github.com/benfradjselim/kairo-core/releases/latest/download/kairo-linux-amd64 \
  -o /usr/local/bin/kairo-core
chmod +x /usr/local/bin/kairo-core

kairo-core --config=/etc/kairo/kairo.yaml
```

Kairo ships as a **single static binary** — no runtime dependencies, no external database.

---

## Build from Source

Requires Go 1.18+:

```bash
git clone https://github.com/benfradjselim/kairo-core.git
cd kairo-core/workdir
go build -o kairo-core ./cmd/kairo-core
./kairo-core --config=configs/kairo.yaml
```

Run tests:

```bash
go test -race -timeout=120s ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | grep total
```
