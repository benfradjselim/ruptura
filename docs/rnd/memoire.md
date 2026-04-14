# Mémoire Technique - Observability Holistic Engine v4.0.0

## Règles et Contraintes pour l'IA de Développement

---

## 1. Contraintes Techniques Fondamentales

| Contrainte | Valeur | Raison |
|------------|--------|--------|
| Mémoire Agent | < 100MB | Portable sur edge, IoT, conteneurs |
| Mémoire Central | < 500MB | Adapté à K8s small nodes |
| CPU Agent | < 1 core | Ne pas impacter les workloads |
| CPU Central | < 2 cores | Scalable horizontalement |
| Stockage | < 10GB pour 30j | Rotation automatique |
| Langage | Go | Binaire unique, performance, concurrency |
| Installation | One-liner | curl -sSL https://ohe.io/install | bash |
| Port unique | 8080 | Configurable |
| Authentification | Bearer token | Optionnel |

---

## 2. Structure du Code

workdir/
├── cmd/agent/
│   └── main.go
├── internal/
│   ├── collector/
│   │   ├── system.go
│   │   ├── container.go
│   │   └── logs.go
│   ├── processor/
│   │   ├── normalize.go
│   │   ├── aggregate.go
│   │   └── downsample.go
│   ├── analyzer/
│   │   ├── stress.go
│   │   ├── fatigue.go
│   │   ├── mood.go
│   │   ├── pressure.go
│   │   ├── humidity.go
│   │   └── contagion.go
│   ├── predictor/
│   │   ├── arima.go
│   │   ├── threshold.go
│   │   └── anomaly.go
│   ├── storage/
│   │   └── badger.go
│   ├── api/
│   │   └── handlers.go
│   └── web/
│       └── embed.go
├── pkg/
│   ├── models/
│   │   ├── metric.go
│   │   ├── kpi.go
│   │   └── alert.go
│   └── utils/
│       ├── math.go
│       └── time.go
└── configs/
    └── agent.yaml

---

## 3. APIs

| Endpoint | Méthode | Description |
|----------|---------|-------------|
| /api/v1/health | GET | Health check |
| /api/v1/metrics | GET | Raw metrics |
| /api/v1/kpis | GET | Complex KPIs |
| /api/v1/predict | GET | Predictions |
| /api/v1/alerts | GET | Active alerts |

---

## 4. Modèles IA

| Modèle | Usage | Paramètres |
|--------|-------|------------|
| ARIMA | Prédictions séries temporelles | Ordre [1,1,1] par défaut |
| Régression linéaire | Tendances | Simple, léger |
| FFT | Détection cycles | 24h, 168h, 720h |
| Seuils dynamiques | Anomalies | Moyenne + 3σ |

---

## 5. Règles de Performance

- Buffer circulaire: 10000 points
- Batch écriture: 1000 points
- Compression: Snappy
- TTL: 7d metrics, 30d logs, 30d predictions
- Downsampling: 1m → 5m → 1h → 1d

---

## 6. Règles de Sécurité

- Authentification: Bearer token optionnel
- HTTPS optionnel
- RBAC pour K8s
- Isolation: agent par node

---

## 7. Règles d'Évolutivité

- Agents: DaemonSet (1 par node)
- Central: 3 réplicas minimum
- Storage: Raft (3 nodes)
- Horizontal scaling par réplicas

---

---

## 8. Règles de Sécurité (mis à jour)

| Règle | Détail |
|-------|--------|
| JWT Secret | Ne jamais utiliser la valeur par défaut. `auth_enabled=true` requiert un secret non vide |
| SSRF | DataSource URLs validées: schéma http/https uniquement, IPs privées bloquées |
| RBAC | Rôles: viewer < operator < admin. Routes admin protégées par `RequireRole("admin")` |
| Rate limiting | 5 tentatives/min par IP sur `/auth/login` |
| Bcrypt | Coût minimum 12 (OWASP 2024). Longueur mot de passe: 8-72 caractères |
| Injection clés | Caractères dangereux (`:`, `\`, `/`) filtrés des noms host/metric/username |
| CORS | Wildcard `*` uniquement en mode dev; configurer `allowed_origins` en production |

## 9. Règles de Performance (mis à jour)

| Règle | Détail |
|-------|--------|
| Clés Badger | Format zero-padded `{20 chiffres}` pour scan par plage O(log N) via Seek |
| KPI GET | `Analyzer.Snapshot()` pour accès lecture seule; `Update()` uniquement depuis le pipeline |
| Réseau | Deltas compteurs NIC en int64 signé pour gérer le wrap-around |
| Collecteur | Mutex sur `SystemCollector` pour accès concurrent |
| Fatigue dt | Plafonné à 30s pour éviter le spike au redémarrage |
| WebSocket | Canal par client avec fermeture sécurisée (sync.Once), sans panic |

## 10. Collecteurs Disponibles

| Collecteur | Fichier | Description |
|------------|---------|-------------|
| SystemCollector | collector/system.go | /proc: CPU, RAM, disk, réseau, load avg |
| ContainerCollector | collector/container.go | Docker socket ou cgroup fallback |
| LogCollector | collector/logs.go | Tail de fichiers log, classification niveaux |

**Document destiné aux agents IA. Respecter strictement ces contraintes.**
