package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// NewRouter builds and returns the HTTP router with all routes registered.
// allowedOrigins is the CORS origin allowlist; empty means wildcard (dev only).
func NewRouter(h *Handlers, jwtSecret string, authEnabled bool, allowedOrigins []string) http.Handler {
	r := mux.NewRouter()

	// Apply global middleware (outermost first)
	r.Use(SecurityHeadersMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(mux.MiddlewareFunc(CORSMiddleware(allowedOrigins)))
	r.Use(mux.MiddlewareFunc(AuthMiddleware(jwtSecret, authEnabled)))
	r.Use(mux.MiddlewareFunc(RateLimitLogin))

	api := r.PathPrefix("/api/v1").Subrouter()

	// System
	api.HandleFunc("/health", h.HealthHandler).Methods(http.MethodGet)
	api.HandleFunc("/health/live", h.LivenessHandler).Methods(http.MethodGet)
	api.HandleFunc("/health/ready", h.ReadinessHandler).Methods(http.MethodGet)
	api.HandleFunc("/config", h.ConfigHandler).Methods(http.MethodGet)

	// Auth — setup endpoint only active when no users exist
	api.HandleFunc("/auth/setup", h.SetupHandler).Methods(http.MethodPost)
	api.HandleFunc("/auth/login", h.LoginHandler).Methods(http.MethodPost)
	api.HandleFunc("/auth/logout", h.LogoutHandler).Methods(http.MethodPost)
	api.HandleFunc("/auth/refresh", h.RefreshHandler).Methods(http.MethodPost)

	adminOnly := RequireRole("admin")
	operatorOnly := RequireRole("operator")

	api.Handle("/auth/users", adminOnly(http.HandlerFunc(h.UserListHandler))).Methods(http.MethodGet)
	api.Handle("/auth/users", adminOnly(http.HandlerFunc(h.UserCreateHandler))).Methods(http.MethodPost)
	api.Handle("/auth/users/{id}", adminOnly(http.HandlerFunc(h.UserGetHandler))).Methods(http.MethodGet)
	api.Handle("/auth/users/{id}", adminOnly(http.HandlerFunc(h.UserDeleteHandler))).Methods(http.MethodDelete)
	api.Handle("/reload", adminOnly(http.HandlerFunc(h.ReloadHandler))).Methods(http.MethodPost)

	// Metrics
	api.HandleFunc("/metrics", h.MetricsListHandler).Methods(http.MethodGet)
	api.HandleFunc("/metrics/{name}", h.MetricGetHandler).Methods(http.MethodGet)
	api.HandleFunc("/metrics/{name}/aggregate", h.MetricAggregateHandler).Methods(http.MethodGet)

	// Query (QQL)
	api.HandleFunc("/query", h.QueryHandler).Methods(http.MethodPost)

	// KPIs
	api.HandleFunc("/kpis", h.KPIListHandler).Methods(http.MethodGet)
	api.HandleFunc("/kpis/{name}", h.KPIGetHandler).Methods(http.MethodGet)
	api.HandleFunc("/kpis/{name}/predict", h.PredictHandler).Methods(http.MethodGet)
	api.HandleFunc("/predict", h.PredictHandler).Methods(http.MethodGet)

	// Alerts
	api.HandleFunc("/alerts", h.AlertListHandler).Methods(http.MethodGet)
	api.HandleFunc("/alerts/{id}", h.AlertGetHandler).Methods(http.MethodGet)
	api.Handle("/alerts/{id}", operatorOnly(http.HandlerFunc(h.AlertDeleteHandler))).Methods(http.MethodDelete)
	api.HandleFunc("/alerts/{id}/acknowledge", h.AlertAcknowledgeHandler).Methods(http.MethodPost)
	api.HandleFunc("/alerts/{id}/silence", h.AlertSilenceHandler).Methods(http.MethodPost)

	// Dashboards (import route must come before {id} to avoid shadowing)
	api.HandleFunc("/dashboards", h.DashboardListHandler).Methods(http.MethodGet)
	api.HandleFunc("/dashboards", h.DashboardCreateHandler).Methods(http.MethodPost)
	api.Handle("/dashboards/import", operatorOnly(http.HandlerFunc(h.DashboardImportHandler))).Methods(http.MethodPost)
	api.HandleFunc("/dashboards/{id}", h.DashboardGetHandler).Methods(http.MethodGet)
	api.Handle("/dashboards/{id}", operatorOnly(http.HandlerFunc(h.DashboardUpdateHandler))).Methods(http.MethodPut)
	api.Handle("/dashboards/{id}", operatorOnly(http.HandlerFunc(h.DashboardDeleteHandler))).Methods(http.MethodDelete)
	api.HandleFunc("/dashboards/{id}/export", h.DashboardExportHandler).Methods(http.MethodGet)

	// DataSources
	api.HandleFunc("/datasources", h.DataSourceListHandler).Methods(http.MethodGet)
	api.Handle("/datasources", operatorOnly(http.HandlerFunc(h.DataSourceCreateHandler))).Methods(http.MethodPost)
	api.HandleFunc("/datasources/{id}", h.DataSourceGetHandler).Methods(http.MethodGet)
	api.Handle("/datasources/{id}", operatorOnly(http.HandlerFunc(h.DataSourceUpdateHandler))).Methods(http.MethodPut)
	api.Handle("/datasources/{id}", operatorOnly(http.HandlerFunc(h.DataSourceDeleteHandler))).Methods(http.MethodDelete)
	api.Handle("/datasources/{id}/test", operatorOnly(http.HandlerFunc(h.DataSourceTestHandler))).Methods(http.MethodPost)

	// Templates
	api.HandleFunc("/templates", h.TemplateListHandler).Methods(http.MethodGet)
	api.HandleFunc("/templates/{id}", h.TemplateGetHandler).Methods(http.MethodGet)
	api.Handle("/templates/{id}/apply", operatorOnly(http.HandlerFunc(h.TemplateApplyHandler))).Methods(http.MethodPost)

	// Ingest (agent → central push, requires at least operator role)
	api.Handle("/ingest", operatorOnly(http.HandlerFunc(h.IngestHandler))).Methods(http.MethodPost)

	// WebSocket streaming (requires at least viewer role — enforced by AuthMiddleware)
	api.HandleFunc("/ws", h.WebSocketHandler)

	// Embedded UI
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))

	return r
}
