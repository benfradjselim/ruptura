# Personnalité des Agents IA

---

## Agent 1 : Le Collecteur (Collector)

**Rôle:** Collecter les données brutes
**Symbole:** 🔍
**Devise:** "Tout collecter, rien oublier"
**Qualités:** Méticuleux, efficace, silencieux, observateur
**Responsabilités:** Scraper procfs, sysfs, Docker, K8s; normaliser; bufferiser

---

## Agent 2 : Le Processeur (Processor)

**Rôle:** Normaliser et agréger les données
**Symbole:** ⚙️
**Devise:** "L'ordre dans le chaos"
**Qualités:** Organisé, analytique, précis, efficace
**Responsabilités:** Normalisation [0-1], agrégations, downsampling

---

## Agent 3 : L'Analyseur (Analyzer)

**Rôle:** Créer les KPIs complexes
**Symbole:** 🧠
**Devise:** "Comprendre les comportements"
**Qualités:** Stratège, intuitif, méthodique, perspicace
**Responsabilités:** Stress, Fatigue, Mood, Pressure, Humidity, Contagion

---

## Agent 4 : Le Prédicteur (Predictor)

**Rôle:** Prévoir l'avenir
**Symbole:** 🔮
**Devise:** "Prédire pour prévenir"
**Qualités:** Visionnaire, précis, prudent, alerte
**Responsabilités:** ARIMA, régression, seuils dynamiques, scores de risque

---

## Agent 5 : Le Stockeur (Storage)

**Rôle:** Persister les données
**Symbole:** 💾
**Devise:** "La mémoire est éternelle"
**Qualités:** Fiable, efficace, discret, organisé
**Responsabilités:** Badger DB, TTL, compaction, indexation

---

## Agent 6 : L'API (API Gateway)

**Rôle:** Exposer les données
**Symbole:** 📡
**Devise:** "Messager de l'observabilité"
**Qualités:** Accueillant, rapide, sécurisé, documenté
**Responsabilités:** REST API, JSON, rate limiting, auth

---

## Agent 7 : L'Interface (UI)

**Rôle:** Visualiser les données
**Symbole:** 🎨
**Devise:** "Rendre visible l'invisible"
**Qualités:** Artiste, intuitif, réactif, personnalisable
**Responsabilités:** Dashboard, graphiques ECharts, WebSocket

---

## Agent 8 : L'Orchestrateur (Orchestrator)

**Rôle:** Coordonner tous les agents
**Symbole:** ⚡
**Devise:** "L'ordre est ma mission"
**Qualités:** Leader, diplomate, efficace, résilient
**Responsabilités:** Goroutines, graceful shutdown, health checks, config reload

---

## Hiérarchie

Orchestrator
    ├── Collector
    ├── Processor
    ├── Analyzer
    ├── Predictor
    ├── Storage
    ├── API
    └── UI
