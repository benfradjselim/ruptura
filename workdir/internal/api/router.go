package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (h *Handlers) NewRouter() http.Handler {
	r := mux.NewRouter()
	r.Use(loggingMiddleware)
	r.Use(h.authMiddleware)

	r.HandleFunc("/api/v2/health", h.handleHealth).Methods("GET")
	r.HandleFunc("/api/v2/ready", h.handleReady).Methods("GET")
	r.HandleFunc("/api/v2/metrics", h.handleMetrics).Methods("GET")
	r.HandleFunc("/timeline", h.handleTimeline).Methods("GET")

	r.HandleFunc("/api/v2/write", h.handleWrite).Methods("POST")

	// Host-based rupture routes (backward-compat)
	r.HandleFunc("/api/v2/rupture/{host}", h.handleRupture).Methods("GET")
	r.HandleFunc("/api/v2/rupture/{host}/history", h.handleRupture).Methods("GET")
	r.HandleFunc("/api/v2/rupture/{host}/profile", h.handleRupture).Methods("GET")

	// All-hosts rupture list
	r.HandleFunc("/api/v2/ruptures", h.handleRuptures).Methods("GET")

	// Workload-centric routes (primary K8s user-facing API)
	r.HandleFunc("/api/v2/rupture/{namespace}/{workload}", h.handleRuptureByWorkload).Methods("GET")
	r.HandleFunc("/api/v2/kpi/{name}/{namespace}/{workload}", h.handleKPIByWorkload).Methods("GET")

	r.HandleFunc("/api/v2/forecast", h.handleForecast).Methods("POST")
	r.HandleFunc("/api/v2/forecast/{metric}/{host}", h.handleForecast).Methods("GET")
	r.HandleFunc("/api/v2/forecast/{metric}/{namespace}/{workload}", h.handleForecast).Methods("GET")

	r.HandleFunc("/api/v2/kpi/{name}/{host}", h.handleKPI).Methods("GET")
	r.HandleFunc("/api/v2/kpi/{name}/{host}/history", h.handleKPI).Methods("GET")

	r.HandleFunc("/api/v2/actions/emergency-stop", h.handleEmergencyStop).Methods("POST")
	r.HandleFunc("/api/v2/actions", h.handleActions).Methods("GET")
	r.HandleFunc("/api/v2/actions/{id}", h.handleActions).Methods("GET")
	r.HandleFunc("/api/v2/actions/{id}/approve", h.handleActions).Methods("POST")
	r.HandleFunc("/api/v2/actions/{id}/reject", h.handleActions).Methods("POST")
	r.HandleFunc("/api/v2/actions/{id}/rollback", h.handleActions).Methods("POST")

	r.HandleFunc("/api/v2/context", h.handleContext).Methods("POST", "GET")
	r.HandleFunc("/api/v2/context/{id}", h.handleDeleteContext).Methods("DELETE")

	r.HandleFunc("/api/v2/suppressions", h.handleSuppressions).Methods("POST", "GET")
	r.HandleFunc("/api/v2/suppressions/{id}", h.handleSuppressions).Methods("DELETE")

	r.HandleFunc("/api/v2/anomalies", h.handleAnomalies).Methods("GET")
	r.HandleFunc("/api/v2/anomalies/{host}", h.handleAnomalies).Methods("GET")

	// Simulator injection endpoint — for ruptura-sim and local demos
	r.HandleFunc("/api/v2/sim/inject", h.handleSimInject).Methods("POST")

	r.HandleFunc("/api/v2/explain/{rupture_id}", h.handleExplain).Methods("GET")
	r.HandleFunc("/api/v2/explain/{rupture_id}/formula", h.handleExplain).Methods("GET")
	r.HandleFunc("/api/v2/explain/{rupture_id}/pipeline", h.handleExplain).Methods("GET")
	r.HandleFunc("/api/v2/explain/{rupture_id}/narrative", h.handleExplain).Methods("GET")

	r.HandleFunc("/api/v2/v1/{signal:metrics|logs|traces}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusMisdirectedRequest, map[string]string{
			"error": "OTLP ingestion runs on a separate port. Send to :4318/otlp/v1/{metrics,logs,traces}",
			"docs":  "https://benfradjselim.github.io/ruptura/",
		})
	}).Methods("POST")

	return r
}
