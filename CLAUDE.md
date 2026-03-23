# MLOPS Anomaly Detection Project

## Objectif
Développer une solution MLOPS complète pour la détection d'anomalies en temps réel sur Kubernetes.

## Composants à développer
1. **Collecteur de métriques** (`src/collector/metrics.py`) - Prometheus
2. **Collecteur de logs** (`src/collector/logs.py`) - Kubernetes API
3. **Modèle online learning** (`src/model/online_detector.py`) - River HalfSpaceTrees
4. **API FastAPI** (`src/api/main.py`) - endpoints /ingest, /predict, /health
5. **Dashboard Streamlit** (`dashboard/app.py`) - visualisation temps réel
6. **Déploiement Helm** (`deploy/helm/`) - Kubernetes

## Technologies
- Python 3.9+, River, FastAPI, Streamlit, Prometheus, Kubernetes/Helm

## Règles
- TDD obligatoire
- 80% coverage minimum
- Type hints et docstrings
- Gestion des erreurs robuste
