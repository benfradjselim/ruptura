package api

import (
	"net/http"

	"github.com/benfradjselim/ruptura/internal/ui"
	"github.com/gorilla/mux"
)

func (h *Handlers) NewRouter() http.Handler {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	// Dashboard served without auth — browser loads HTML first, then uses the API key field
	r.PathPrefix("/ui").Handler(ui.Handler(h.apiKey))
	r.HandleFunc("/", func(w http.ResponseWriter, rq *http.Request) {
		http.Redirect(w, rq, "/ui/", http.StatusFound)
	}).Methods("GET")

	r.HandleFunc("/timeline", h.handleTimeline).Methods("GET")

	// Probe endpoints are always public — k8s liveness/readiness probes carry no auth
	r.HandleFunc("/api/v2/health", h.handleHealth).Methods("GET")
	r.HandleFunc("/api/v2/ready", h.handleReady).Methods("GET")
	r.HandleFunc("/api/v2/version", h.handleHealth).Methods("GET")

	// All other /api/v2 routes require authentication
	api := r.PathPrefix("/api/v2").Subrouter()
	api.Use(h.authMiddleware)

	api.HandleFunc("/metrics", h.handleMetrics).Methods("GET")

	api.HandleFunc("/write", h.handleWrite).Methods("POST")

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

	return r
}
