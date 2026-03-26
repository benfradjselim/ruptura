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
./scripts/deploy-v2.0.shls
