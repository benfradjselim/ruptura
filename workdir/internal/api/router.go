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
    r.HandleFunc("/api/v2/rupture/{host}", h.handleRupture).Methods("GET")
    r.HandleFunc("/api/v2/rupture/{host}/history", h.handleRupture).Methods("GET")
    r.HandleFunc("/api/v2/rupture/{host}/profile", h.handleRupture).Methods("GET")
    r.HandleFunc("/api/v2/ruptures", h.handleRupture).Methods("GET")
    
    r.HandleFunc("/api/v2/forecast", h.handleForecast).Methods("POST")
    r.HandleFunc("/api/v2/forecast/{metric}/{host}", h.handleForecast).Methods("GET")
    
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
    
    r.HandleFunc("/api/v2/explain/{rupture_id}", h.handleExplain).Methods("GET")
    r.HandleFunc("/api/v2/explain/{rupture_id}/formula", h.handleExplain).Methods("GET")
    r.HandleFunc("/api/v2/explain/{rupture_id}/pipeline", h.handleExplain).Methods("GET")
    
    r.HandleFunc("/api/v2/v1/metrics", h.handleOTLP).Methods("POST")
    r.HandleFunc("/api/v2/v1/logs", h.handleOTLP).Methods("POST")
    r.HandleFunc("/api/v2/v1/traces", h.handleOTLP).Methods("POST")

    return r
}
