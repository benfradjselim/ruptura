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

	// All other /api/v2 routes require authentication
	api := r.PathPrefix("/api/v2").Subrouter()
	api.Use(h.authMiddleware)

	api.HandleFunc("/timeline", h.handleTimeline).Methods("GET")
	api.HandleFunc("/metrics", h.handleMetrics).Methods("GET")

	api.HandleFunc("/write", h.handleWrite).Methods("POST")
	api.HandleFunc("/ingest/purge", h.handleIngestPurge).Methods("DELETE")

	// Host-based rupture routes (backward-compat)
	api.HandleFunc("/rupture/{host}", h.handleRupture).Methods("GET")
	api.HandleFunc("/rupture/{host}/history", h.handleRupture).Methods("GET")
	api.HandleFunc("/rupture/{host}/profile", h.handleRupture).Methods("GET")

	api.HandleFunc("/dataflow", h.handleDataflow).Methods("GET")
	api.HandleFunc("/ruptures", h.handleRuptures).Methods("GET")

	api.HandleFunc("/rupture/{namespace}/{kind}/{workload}", h.handleRuptureByWorkload3).Methods("GET")
	api.HandleFunc("/rupture/{namespace}/{workload}", h.handleRuptureByWorkload).Methods("GET")
	api.HandleFunc("/kpi/{name}/{namespace}/{workload}", h.handleKPIByWorkload).Methods("GET")

	api.HandleFunc("/forecast", h.handleForecast).Methods("POST")
	api.HandleFunc("/forecast/{metric}/{host}", h.handleForecast).Methods("GET")
	api.HandleFunc("/forecast/{metric}/{namespace}/{workload}", h.handleForecast).Methods("GET")

	api.HandleFunc("/kpi/{name}/{host}", h.handleKPI).Methods("GET")
	api.HandleFunc("/kpi/{name}/{host}/history", h.handleKPI).Methods("GET")

	api.HandleFunc("/actions/emergency-stop", h.handleEmergencyStop).Methods("POST")
	api.HandleFunc("/actions", h.handleActions).Methods("GET")
	api.HandleFunc("/actions/{id}", h.handleActions).Methods("GET")
	api.HandleFunc("/actions/{id}/approve", h.handleActions).Methods("POST")
	api.HandleFunc("/actions/{id}/reject", h.handleActions).Methods("POST")
	api.HandleFunc("/actions/{id}/rollback", h.handleActions).Methods("POST")

	api.HandleFunc("/context", h.handleContext).Methods("POST", "GET")
	api.HandleFunc("/context/{id}", h.handleDeleteContext).Methods("DELETE")

	api.HandleFunc("/suppressions", h.handleSuppressions).Methods("POST", "GET")
	api.HandleFunc("/suppressions/{id}", h.handleSuppressions).Methods("DELETE")

	api.HandleFunc("/anomalies", h.handleAnomalies).Methods("GET")
	api.HandleFunc("/anomalies/{host}", h.handleAnomalies).Methods("GET")

	api.HandleFunc("/logs", h.handleLogs).Methods("GET")

	api.HandleFunc("/sim/inject", h.handleSimInject).Methods("POST")

	api.HandleFunc("/config/weights", h.handleConfigWeights).Methods("GET", "POST")

	api.HandleFunc("/history", h.handleHistory).Methods("GET")
	api.HandleFunc("/history/{workload:.+}", h.handleHistory).Methods("GET")
	api.HandleFunc("/events", h.handleEvents).Methods("GET")

	api.HandleFunc("/explain/{rupture_id:.+}/narrative", h.handleExplain).Methods("GET")
	api.HandleFunc("/explain/{rupture_id:.+}/formula", h.handleExplain).Methods("GET")
	api.HandleFunc("/explain/{rupture_id:.+}/pipeline", h.handleExplain).Methods("GET")
	api.HandleFunc("/explain/{rupture_id:.+}", h.handleExplain).Methods("GET")

	api.HandleFunc("/v1/{signal:metrics|logs|traces}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMisdirectedRequest, map[string]string{
			"error": "OTLP ingestion runs on a separate port. Send to :4318/otlp/v1/{metrics,logs,traces}",
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
	api.HandleFunc("/kpis", h.handleKPIs).Methods("GET")
	api.HandleFunc("/kpis/multi", h.handleKPIs).Methods("GET")

	// Fleet summary (Fleet.svelte)
	api.HandleFunc("/fleet", h.handleFleet).Methods("GET")

	// Alerts (Alerts.svelte — backed by anomaly events)
	api.HandleFunc("/alerts", h.handleAlertList).Methods("GET")
	api.HandleFunc("/alerts/{id}", h.handleAlertGet).Methods("GET")
	api.HandleFunc("/alerts/{id}", h.handleAlertOp).Methods("DELETE")
	api.HandleFunc("/alerts/{id}/acknowledge", h.handleAlertOp).Methods("POST")
	api.HandleFunc("/alerts/{id}/silence", h.handleAlertOp).Methods("POST")

	// Predict (Dashboard.svelte predictions panel)
	api.HandleFunc("/predict", h.handlePredict).Methods("GET")

	// Traces (Traces.svelte — stub until trace query store is added)
	api.HandleFunc("/traces", h.handleTraceSearch).Methods("GET")
	api.HandleFunc("/traces/{id}", h.handleTraceGet).Methods("GET")

	// Logs stream (Logs.svelte SSE — stub; polling via /logs is used instead)
	api.HandleFunc("/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}).Methods("GET")

	// Alert rules (AlertRules.svelte — stub)
	api.HandleFunc("/alert-rules", stubList).Methods("GET")
	api.HandleFunc("/alert-rules", stubCreate).Methods("POST")
	api.HandleFunc("/alert-rules/{name:.+}", stubWithID).Methods("PUT", "DELETE")

	// Dashboards (Dashboards.svelte — stub)
	api.HandleFunc("/dashboards", stubList).Methods("GET")
	api.HandleFunc("/dashboards", stubCreate).Methods("POST")
	api.HandleFunc("/dashboards/import", stubCreate).Methods("POST")
	api.HandleFunc("/dashboards/{id}", stubWithID).Methods("GET", "PUT", "DELETE")
	api.HandleFunc("/dashboards/{id}/export", stubWithID).Methods("GET")

	// Dashboard templates (Dashboards.svelte — stub)
	api.HandleFunc("/templates", stubList).Methods("GET")
	api.HandleFunc("/templates/{id}", stubWithID).Methods("GET")
	api.HandleFunc("/templates/{id}/apply", stubWithID).Methods("POST")

	// SLOs (SLOs.svelte — stub)
	api.HandleFunc("/slos", stubList).Methods("GET")
	api.HandleFunc("/slos", stubCreate).Methods("POST")
	api.HandleFunc("/slos/{id}", stubWithID).Methods("GET", "PUT", "DELETE")
	api.HandleFunc("/slos/status", stubList).Methods("GET")
	api.HandleFunc("/slos/{id}/status", stubWithID).Methods("GET")

	// Notifications (Notifications.svelte — stub)
	api.HandleFunc("/notifications", stubList).Methods("GET")
	api.HandleFunc("/notifications", stubCreate).Methods("POST")
	api.HandleFunc("/notifications/{id}", stubWithID).Methods("GET", "PUT", "DELETE")
	api.HandleFunc("/notifications/{id}/test", stubWithID).Methods("POST")

	// Datasources (Datasources.svelte — stub)
	api.HandleFunc("/datasources", stubList).Methods("GET")
	api.HandleFunc("/datasources", stubCreate).Methods("POST")
	api.HandleFunc("/datasources/{id}", stubWithID).Methods("GET", "PUT", "DELETE")
	api.HandleFunc("/datasources/{id}/test", stubWithID).Methods("POST")
	api.HandleFunc("/datasources/{id}/proxy", stubWithID).Methods("POST")

	// Orgs (Orgs.svelte — stub)
	api.HandleFunc("/orgs", stubList).Methods("GET")
	api.HandleFunc("/orgs", stubCreate).Methods("POST")
	api.HandleFunc("/orgs/{id}", stubWithID).Methods("GET", "PUT", "DELETE")

	// Topology
	api.HandleFunc("/topology", h.handleTopology).Methods("GET")

	// Node health
	api.HandleFunc("/nodes", h.handleNodes).Methods("GET")
	api.HandleFunc("/nodes/{node:.+}", h.handleNode).Methods("GET")

	// Workload k8s metadata
	api.HandleFunc("/workloads/{namespace}/{kind}/{name}/k8s", h.handleWorkloadK8s).Methods("GET")

	// Engine internals
	api.HandleFunc("/engine/status", h.handleEngineStatus).Methods("GET")
	api.HandleFunc("/engine/storage", h.handleEngineStorage).Methods("GET")
	api.HandleFunc("/engine/fusion/{namespace}/{kind}/{name}", h.handleFusionState).Methods("GET")

	return r
}
