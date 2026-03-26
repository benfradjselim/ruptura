#!/bin/bash
set -e

PROJECT_ROOT="/workspaces/Mlops_crew_automation"
cd "$PROJECT_ROOT"

echo "============================================"
echo "  MLOps Anomaly Detection - Création v2.0"
echo "============================================"
echo ""

git stash push -m "backup before v2.0" 2>/dev/null || true
git checkout main 2>/dev/null || true

echo ""
echo "🌿 Création de la branche v2.0..."
git checkout -b v2.0

echo ""
echo "🐳 Création de Dockerfile.v2..."
cat > Dockerfile.v2 << 'DOCKER_EOF'
ARG SERVICE=collector

FROM python:3.11-slim AS builder
ARG SERVICE
WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc g++ curl && rm -rf /var/lib/apt/lists/*

COPY requirements/common.txt /build/common.txt
RUN pip install --no-cache-dir --prefix=/install -r /build/common.txt

COPY requirements/${SERVICE}.txt /build/service.txt
RUN pip install --no-cache-dir --prefix=/install -r /build/service.txt

FROM python:3.11-slim AS runtime
ARG SERVICE
ENV SERVICE=${SERVICE}
ENV DB_PATH=/data/mlops.db

RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /install /usr/local

WORKDIR /app
COPY src/shared/ ./shared/
COPY services/${SERVICE}/ ./service/

RUN if [ "$SERVICE" = "exporter" ] && [ -f /app/service/main_v2.py ]; then \
        mv /app/service/main_v2.py /app/service/main.py; \
    fi

RUN mkdir -p /data && chmod 777 /data

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:${PORT:-8001}/health || exit 1

CMD ["sh", "-c", "python /app/service/main.py"]
DOCKER_EOF
echo "   ✅ Dockerfile.v2"

echo ""
echo "📝 Création de services/exporter/main_v2.py..."
mkdir -p services/exporter
cp services/exporter/main.py services/exporter/main.py.bak 2>/dev/null || true

cat > services/exporter/main_v2.py << 'EXPORTER_EOF'
"""MLOps Exporter - Version corrigée v2.0"""
import logging
import sys
import os
from datetime import datetime, timezone, timedelta
import uvicorn
from fastapi import FastAPI, Response
import sqlite3

sys.path.insert(0, "/app")
from shared import config

logging.basicConfig(level=config.get_str("LOG_LEVEL", "INFO"))
logger = logging.getLogger("exporter")

app = FastAPI(title="MLOps Exporter v2.0")
DB_PATH = config.get_str("DB_PATH", "/data/mlops.db")

@app.get("/health")
async def health():
    return {"status": "ok", "service": "exporter", "version": "2.0"}

@app.get("/summary")
async def summary():
    try:
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        result = conn.execute("SELECT COUNT(*) as count FROM anomalies").fetchone()
        total = result["count"] if result else 0
        conn.close()
        return {
            "total_anomalies_24h": total,
            "anomaly_rate": 1.0 if total > 0 else 0.0,
            "total_predictions": total
        }
    except Exception as e:
        logger.error(f"Error in summary: {e}")
        return {"total_anomalies_24h": 0, "anomaly_rate": 0.0, "total_predictions": 0}

@app.get("/dashboard-data")
async def dashboard_data(window: str = "24h"):
    try:
        hours = 24
        if window.endswith("h"):
            try:
                hours = max(1, min(int(window[:-1]), 168))
            except:
                pass
        
        since = (datetime.now(timezone.utc) - timedelta(hours=hours)).isoformat()
        conn = sqlite3.connect(DB_PATH)
        conn.row_factory = sqlite3.Row
        
        rows = conn.execute(
            "SELECT timestamp, anomaly_score, is_anomaly FROM anomalies "
            "WHERE timestamp >= ? ORDER BY timestamp LIMIT 500",
            (since,)
        ).fetchall()
        anomaly_series = [
            {"timestamp": r["timestamp"], "value": float(r["anomaly_score"]), 
             "is_anomaly": bool(r["is_anomaly"])} 
            for r in rows
        ]
        
        rows = conn.execute(
            "SELECT timestamp, value FROM raw_metrics "
            "WHERE timestamp >= ? ORDER BY timestamp LIMIT 500",
            (since,)
        ).fetchall()
        metric_series = [
            {"timestamp": r["timestamp"], "value": float(r["value"])} 
            for r in rows
        ]
        
        rows = conn.execute(
            "SELECT id, timestamp, anomaly_score, is_anomaly, pod_name, namespace "
            "FROM anomalies ORDER BY timestamp DESC LIMIT 20"
        ).fetchall()
        recent = [
            {
                "id": r["id"],
                "timestamp": r["timestamp"],
                "anomaly_score": float(r["anomaly_score"]),
                "is_anomaly": bool(r["is_anomaly"]),
                "pod_name": r["pod_name"] or "unknown",
                "namespace": r["namespace"] or "mlops"
            }
            for r in rows
        ]
        
        total = conn.execute("SELECT COUNT(*) as count FROM anomalies").fetchone()["count"]
        conn.close()
        
        return {
            "anomaly_series": anomaly_series,
            "metric_series": metric_series,
            "summary": {
                "total_anomalies_24h": total,
                "anomaly_rate": 1.0 if total > 0 else 0.0,
                "total_predictions": total
            },
            "recent_anomalies": recent,
            "window": window
        }
    except Exception as e:
        logger.error(f"Error in dashboard-data: {e}")
        return {"error": str(e)}

@app.get("/metrics")
async def metrics():
    from prometheus_client import generate_latest, CONTENT_TYPE_LATEST
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)

if __name__ == "__main__":
    port = int(os.environ.get("EXPORTER_PORT", 8005))
    uvicorn.run(app, host="0.0.0.0", port=port)
EXPORTER_EOF
echo "   ✅ services/exporter/main_v2.py"

echo ""
echo "📁 Création de manifests/mlops-v2.0.yaml..."
mkdir -p manifests
cat > manifests/mlops-v2.0.yaml << 'MANIFEST_EOF'
---
apiVersion: v1
kind: Namespace
metadata:
  name: mlops
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: mlops-data
  namespace: mlops
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: collector
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: collector
  template:
    metadata:
      labels:
        app: collector
    spec:
      containers:
      - name: collector
        image: selimbf/mlops-collector:v2.0
        ports:
        - containerPort: 8001
        env:
        - name: COLLECTOR_PORT
          value: "8001"
        - name: DB_PATH
          value: "/data/mlops.db"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: collector
  namespace: mlops
spec:
  selector:
    app: collector
  ports:
  - port: 8001
    targetPort: 8001
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: processor
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: processor
  template:
    metadata:
      labels:
        app: processor
    spec:
      containers:
      - name: processor
        image: selimbf/mlops-processor:v2.0
        ports:
        - containerPort: 8002
        env:
        - name: PROCESSOR_PORT
          value: "8002"
        - name: DB_PATH
          value: "/data/mlops.db"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: processor
  namespace: mlops
spec:
  selector:
    app: processor
  ports:
  - port: 8002
    targetPort: 8002
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: trainer
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: trainer
  template:
    metadata:
      labels:
        app: trainer
    spec:
      containers:
      - name: trainer
        image: selimbf/mlops-trainer:v2.0
        ports:
        - containerPort: 8003
        env:
        - name: TRAINER_PORT
          value: "8003"
        - name: DB_PATH
          value: "/data/mlops.db"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: trainer
  namespace: mlops
spec:
  selector:
    app: trainer
  ports:
  - port: 8003
    targetPort: 8003
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: detector
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: detector
  template:
    metadata:
      labels:
        app: detector
    spec:
      containers:
      - name: detector
        image: selimbf/mlops-detector:v2.0
        ports:
        - containerPort: 8004
        env:
        - name: DETECTOR_PORT
          value: "8004"
        - name: DB_PATH
          value: "/data/mlops.db"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: detector
  namespace: mlops
spec:
  selector:
    app: detector
  ports:
  - port: 8004
    targetPort: 8004
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: exporter
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: exporter
  template:
    metadata:
      labels:
        app: exporter
    spec:
      containers:
      - name: exporter
        image: selimbf/mlops-exporter:v2.0
        ports:
        - containerPort: 8005
        env:
        - name: EXPORTER_PORT
          value: "8005"
        - name: DB_PATH
          value: "/data/mlops.db"
        - name: LOG_LEVEL
          value: "INFO"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: exporter
  namespace: mlops
spec:
  selector:
    app: exporter
  ports:
  - port: 8005
    targetPort: 8005
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dashboard
  namespace: mlops
spec:
  replicas: 1
  selector:
    matchLabels:
      app: dashboard
  template:
    metadata:
      labels:
        app: dashboard
    spec:
      containers:
      - name: dashboard
        image: selimbf/mlops-dashboard:v2.0
        ports:
        - containerPort: 8501
        env:
        - name: DASHBOARD_PORT
          value: "8501"
        - name: EXPORTER_URL
          value: "http://exporter:8005"
        - name: DASHBOARD_REFRESH_SEC
          value: "5"
        - name: ANOMALY_THRESHOLD
          value: "0.7"
        - name: STREAMLIT_SERVER_PORT
          value: "8501"
        - name: STREAMLIT_SERVER_ADDRESS
          value: "0.0.0.0"
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: mlops-data
---
apiVersion: v1
kind: Service
metadata:
  name: dashboard
  namespace: mlops
spec:
  selector:
    app: dashboard
  ports:
  - port: 8501
    targetPort: 8501
    nodePort: 30851
  type: NodePort
MANIFEST_EOF
echo "   ✅ manifests/mlops-v2.0.yaml"echo ""
echo "📖 Création de README-v2.0.md..."
cat > README-v2.0.md << 'README_EOF'
# MLOps Anomaly Detection - Version 2.0

## Corrections

| Problème | Solution |
|----------|----------|
| Dashboard affichait 0 anomalies | Exporter corrigé pour lire la table anomalies |
| Base de données non partagée | PVC mlops-data monté sur tous les services |
| Variable EXPORTER_PORT incorrecte | Forcée à 8005 |

## Installation

```bash
git checkout v2.0
./scripts/deploy-v2.0.shREADME_EOF
echo "   ✅ README-v2.0.md"

echo ""
echo "🔧 Création de scripts/deploy-v2.0.sh..."
mkdir -p scripts
cat > scripts/deploy-v2.0.sh << 'DEPLOY_EOF'
#!/bin/bash
set -euo pipefail

NAMESPACE="mlops"
IMAGE_REPO="${IMAGE_REPO:-selimbf}"
IMAGE_TAG="${IMAGE_TAG:-v2.0}"

echo "Deploying MLOps V2.0..."

for cmd in kubectl docker; do
command -v "$cmd" &>/dev/null || { echo "ERROR: $cmd not found"; exit 1; }
done

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "Building Docker images..."
for service in collector processor trainer detector exporter dashboard; do
echo "  Building $service..."
docker build 
        --build-arg SERVICE="$service" 
        -t "${IMAGE_REPO}/mlops-${service}:${IMAGE_TAG}" 
        -f "${PROJECT_ROOT}/Dockerfile.v2" 
        "${PROJECT_ROOT}" > /dev/null 2>&1
done

echo "Deploying to Kubernetes..."
kubectl apply -f "${PROJECT_ROOT}/manifests/mlops-v2.0.yaml"

echo "Waiting for pods..."
sleep 10
kubectl wait --for=condition=ready pod -l app -n "$NAMESPACE" --timeout=300s 2>/dev/null || true

kubectl get pods -n "$NAMESPACE"

DASHBOARD_PORT=$(kubectl get svc dashboard -n "$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}' 2>/dev/null || echo "30851")
NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "localhost")

echo ""
echo "Deployment complete!"
echo "Dashboard: http://${NODE_IP}:${DASHBOARD_PORT}"
DEPLOY_EOF
chmod +x scripts/deploy-v2.0.sh
echo "   ✅ scripts/deploy-v2.0.sh"

echo ""
echo "⚙️ Création de .github/workflows/docker-image-v2.0.yaml..."
mkdir -p .github/workflows
cat > .github/workflows/docker-image-v2.0.yaml << 'WORKFLOW_EOF'
name: Build MLOps Images v2.0

on:
push:
branches: [ "v2.0" ]
tags: [ "v2.*" ]

jobs:
build:
runs-on: ubuntu-latest
strategy:
matrix:
service: [collector, processor, trainer, detector, exporter, dashboard]
steps:
- uses: actions/checkout@v4
- name: Login to Docker Hub
uses: docker/login-action@v3
with:
username: ${{ secrets.DOCKER_USERNAME }}
password: ${{ secrets.DOCKER_TOKEN }}
- name: Build and push
uses: docker/build-push-action@v5
with:
context: .
file: Dockerfile.v2
build-args: SERVICE=${{ matrix.service }}
push: true
tags: |
${{ secrets.DOCKER_USERNAME }}/mlops-${{ matrix.service }}:v2.0
${{ secrets.DOCKER_USERNAME }}/mlops-${{ matrix.service }}:latest-v2
WORKFLOW_EOF
echo "   ✅ .github/workflows/docker-image-v2.0.yaml"

echo ""
echo "📤 Commit et push..."
git add Dockerfile.v2 services/exporter/main_v2.py manifests/ scripts/deploy-v2.0.sh README-v2.0.md .github/workflows/docker-image-v2.0.yaml
git commit -m "v2.0: Corrections majeures"
git push origin v2.0 --force
git tag -a v2.0 -m "Release v2.0"
git push origin v2.0

echo ""
echo "✅ Branche v2.0 créée et poussée"
echo "   Pour déployer: git checkout v2.0 && ./scripts/deploy-v2.0.sh"
