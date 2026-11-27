package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (h *Handlers) NewRouter() http.Handler {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Root redirect removed — the v7 ruptura-ui pod serves the dashboard on its own NodePort.
	// Probe endpoints are always public — k8s liveness/readiness probes carry no auth

	// Probe endpoints are always public — k8s liveness/readiness probes carry no auth
	r.HandleFunc("/api/v2/health", h.handleHealth).Methods("GET")
	r.HandleFunc("/api/v2/ready", h.handleReady).Methods("GET")
	r.HandleFunc("/api/v2/version", h.handleHealth).Methods("GET")
	// Prometheus metrics scrape endpoint — must be unauthenticated so scrapers
	// (Prometheus, Datadog, etc.) don't need an API key configured.
	r.HandleFunc("/api/v2/metrics", h.handleMetrics).Methods("GET")

	// All other /api/v2 routes require authentication
	api := r.PathPrefix("/api/v2").Subrouter()
	api.Use(h.authMiddleware)

	api.HandleFunc("/write", h.handleWrite).Methods("POST")
	api.HandleFunc("/ingest/purge", h.handleIngestPurge).Methods("DELETE")

	// Host-based rupture routes (backward-compat)

	api.HandleFunc("/failures", h.handleFailures).Methods("GET")
	api.HandleFunc("/failures/series", h.handleFailureSeries).Methods("GET")
	api.HandleFunc("/failures/closest", h.handleClosest).Methods("GET")
	api.HandleFunc("/callgraph", h.handleCallGraph).Methods("GET")
	api.HandleFunc("/inputs", h.handleInputs).Methods("GET")
	api.HandleFunc("/track-record", h.handleTrackRecord).Methods("GET")
	api.HandleFunc("/coverage", h.handleCoverage).Methods("GET")
	api.HandleFunc("/event-history", h.handleEventHistory).Methods("GET")

	api.HandleFunc("/actions/emergency-stop", h.handleEmergencyStop).Methods("POST")
	api.HandleFunc("/actions", h.handleActions).Methods("GET")
	api.HandleFunc("/actions/{id}", h.handleActions).Methods("GET")
	api.HandleFunc("/actions/{id}/approve", h.handleActions).Methods("POST")
	api.HandleFunc("/actions/{id}/reject", h.handleActions).Methods("POST")
	api.HandleFunc("/actions/{id}/rollback", h.handleActions).Methods("POST")

	api.HandleFunc("/logs", h.handleLogs).Methods("GET")

	api.HandleFunc("/sim/inject", h.handleSimInject).Methods("POST")

	api.HandleFunc("/v1/{signal:metrics|logs|traces}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMisdirectedRequest, map[string]string{
			"error": "OTLP ingestion runs on a separate port. Send OTLP gRPC to :4317, or OTLP HTTP to :4318/v1/{metrics,logs,traces}",
			"docs":  "https://benfradjselim.github.io/ruptura/",
		})
	}).Methods("POST")

	// ── Svelte dashboard v7.0 API bridge ─────────────────────────────────────
	// Auth stubs (real auth = Bearer API key via window.__RUPTURA_KEY__)
	r.HandleFunc("/api/v2/auth/setup", h.handleAuthSetup).Methods("POST")
	r.HandleFunc("/api/v2/auth/login", h.handleAuthLogin).Methods("POST")
	r.HandleFunc("/api/v2/auth/logout", h.handleAuthLogout).Methods("POST")
	r.HandleFunc("/api/v2/auth/refresh", h.handleAuthRefresh).Methods("POST")
	api.HandleFunc("/auth/users", h.handleAuthUsers).Methods("GET")
	api.HandleFunc("/auth/users/{username}", stubWithID).Methods("POST", "DELETE")

	// KPIs flat map (Dashboard.svelte)

	// Fleet summary (Fleet.svelte)
	api.HandleFunc("/fleet", h.handleFleet).Methods("GET")

	// Alerts (Alerts.svelte — backed by anomaly events)

	// Predict (Dashboard.svelte predictions panel)

	// Traces (Traces.svelte — stub until trace query store is added)
	api.HandleFunc("/traces", h.handleTraceSearch).Methods("GET")
	api.HandleFunc("/traces/{id}", h.handleTraceGet).Methods("GET")

	// Logs stream (Logs.svelte SSE — stub; polling via /logs is used instead)
	api.HandleFunc("/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}).Methods("GET")

	// Orgs (Orgs.svelte — stub)
	api.HandleFunc("/orgs", stubList).Methods("GET")
	api.HandleFunc("/orgs", stubCreate).Methods("POST")
	api.HandleFunc("/orgs/{id}", stubWithID).Methods("GET", "PUT", "DELETE")

	// Topology

	// Node health
	api.HandleFunc("/nodes", h.handleNodes).Methods("GET")
	api.HandleFunc("/nodes/{node:.+}", h.handleNode).Methods("GET")

	// Infra collector — GroupSnapshot aggregates and CGPM propagation
	api.HandleFunc("/infra/groups", h.handleInfraGroups).Methods("GET")
	api.HandleFunc("/infra/groups/{group}/history", h.handleInfraGroupHistory).Methods("GET")
	api.HandleFunc("/infra/nodes", h.handleInfraNodes).Methods("GET")
	api.HandleFunc("/infra/mcp", h.handleInfraMCP).Methods("GET")
	api.HandleFunc("/infra/operators", h.handleInfraOperators).Methods("GET")
	api.HandleFunc("/infra/network", h.handleInfraNetwork).Methods("GET")
	api.HandleFunc("/infra/storage", h.handleInfraStorage).Methods("GET")
	api.HandleFunc("/infra/admission", h.handleInfraAdmission).Methods("GET")
	api.HandleFunc("/infra/tenancy", h.handleInfraTenancy).Methods("GET")
	api.HandleFunc("/propagation", h.handlePropagation).Methods("GET")

	// Workload k8s metadata
	api.HandleFunc("/workloads/{namespace}/{kind}/{name}/k8s", h.handleWorkloadK8s).Methods("GET")
	// FBL-A2-1: forecast-as-hero summary sentence for workload detail

	// Engine internals
	api.HandleFunc("/engine/storage", h.handleEngineStorage).Methods("GET")

	return r
}
