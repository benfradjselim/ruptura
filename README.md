# MLOps Anomaly Detection

Real-time anomaly detection platform with microservices architecture on Kubernetes.

## Architecture

```
Prometheus / K8s API
        |
   [COLLECTOR :8001]  -- scrapes metrics & logs every 15s
        |
   [PROCESSOR :8002]  -- normalizes + engineers features
        |          \
  [TRAINER :8003]  [DETECTOR :8004]  -- online learning / scoring
                        |
                   [EXPORTER :8005]  -- Prometheus metrics + REST API
                        |
                  [DASHBOARD :8501]  -- Streamlit real-time UI
```

All services share a SQLite database via a Kubernetes PersistentVolumeClaim.

## Services

| Service    | Port | Description |
|-----------|------|-------------|
| Collector  | 8001 | Scrapes Prometheus + K8s pod logs |
| Processor  | 8002 | Normalizes data, engineers features |
| Trainer    | 8003 | River HalfSpaceTrees online learning |
| Detector   | 8004 | Real-time anomaly scoring |
| Exporter   | 8005 | Prometheus metrics + dashboard REST API |
| Dashboard  | 8501 | Streamlit real-time visualization |

## Quick Start

### One-Click Installation

```bash
./scripts/install.sh
```

This will:
1. Build Docker images for all 6 services
2. Load images into your cluster (kind/minikube auto-detected)
3. Install the Helm chart in namespace `mlops`
4. Display service URLs

### Prerequisites

- `kubectl` configured with cluster access
- `helm` >= 3.0
- `docker`

### Custom configuration

```bash
IMAGE_REPO=my-registry/mlops IMAGE_TAG=v1.0 ./scripts/install.sh
```

Or edit `helm/values.yaml` before installing.

## Development

### Local setup

```bash
pip install -r requirements.txt
```

### Run tests

```bash
pytest tests/ -v --cov=src --cov-report=term-missing
```

### Run a service locally

```bash
DB_PATH=./data/mlops.db COLLECTOR_URL=http://localhost:8001 \
  python services/collector/main.py
```

## Data Flow

1. **Collector** scrapes Prometheus every `COLLECT_INTERVAL_SEC` (default 15s), writes to `raw_metrics` table, triggers Processor
2. **Processor** normalizes metrics with online MinMax, fans out to Trainer + Detector in parallel
3. **Trainer** calls `model.learn_one(x)` on River HalfSpaceTrees, serializes model to DB every 100 samples
4. **Detector** loads latest model, calls `model.score_one(x)`, writes anomaly scores to `anomalies` table
5. **Exporter** aggregates data from DB, exposes `/metrics` (Prometheus) and `/dashboard-data` (JSON)
6. **Dashboard** polls Exporter every 5s, renders real-time charts

## Key Design Decisions

- **SQLite + WAL mode**: Zero-ops persistence, sufficient for ~100 writes/sec
- **River HalfSpaceTrees**: Online learner, no labeled data required, sub-millisecond inference
- **Push-based communication**: Upstream services POST to downstream on new data; reconciliation loop handles failures
- **Table ownership**: Each table has one writer, avoiding SQLite write contention

## Helm Configuration

Key values in `helm/values.yaml`:

```yaml
config:
  COLLECT_INTERVAL_SEC: "15"    # Scrape interval
  ANOMALY_THRESHOLD: "0.7"      # Score threshold for anomaly flag
  HST_N_TREES: "10"             # Number of HST trees
  HST_WINDOW_SIZE: "250"        # Rolling window size

pvc:
  size: 5Gi                     # SQLite storage
```

## Uninstall

```bash
helm uninstall mlops-anomaly -n mlops
kubectl delete namespace mlops
```
