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

	// Fleet overview (all hosts aggregate)
	api.HandleFunc("/fleet", h.FleetHandler).Methods(http.MethodGet)

	// Multi-host KPI snapshot
	api.HandleFunc("/kpis/multi", h.KPIMultiHandler).Methods(http.MethodGet)

	// Alert rules CRUD
	api.HandleFunc("/alert-rules", h.AlertRuleListHandler).Methods(http.MethodGet)
	api.Handle("/alert-rules", operatorOnly(http.HandlerFunc(h.AlertRuleCreateHandler))).Methods(http.MethodPost)
	api.Handle("/alert-rules/{name}", operatorOnly(http.HandlerFunc(h.AlertRuleUpdateHandler))).Methods(http.MethodPut)
	api.Handle("/alert-rules/{name}", operatorOnly(http.HandlerFunc(h.AlertRuleDeleteHandler))).Methods(http.MethodDelete)

	// Notification channels
	api.HandleFunc("/notifications", h.NotificationChannelListHandler).Methods(http.MethodGet)
	api.Handle("/notifications", operatorOnly(http.HandlerFunc(h.NotificationChannelCreateHandler))).Methods(http.MethodPost)
	api.HandleFunc("/notifications/{id}", h.NotificationChannelGetHandler).Methods(http.MethodGet)
	api.Handle("/notifications/{id}", operatorOnly(http.HandlerFunc(h.NotificationChannelUpdateHandler))).Methods(http.MethodPut)
	api.Handle("/notifications/{id}", operatorOnly(http.HandlerFunc(h.NotificationChannelDeleteHandler))).Methods(http.MethodDelete)
	api.Handle("/notifications/{id}/test", operatorOnly(http.HandlerFunc(h.NotificationChannelTestHandler))).Methods(http.MethodPost)

	// Prometheus exposition — served at /metrics (outside /api/v1 prefix for standard compat)
	r.HandleFunc("/metrics", h.PrometheusMetricsHandler).Methods(http.MethodGet)

	// Topology (APM-style service dependency graph)
	api.HandleFunc("/topology", h.TopologyHandler).Methods(http.MethodGet)

	// Logs query + live stream
	api.HandleFunc("/logs", h.LogQueryHandler).Methods(http.MethodGet)
	api.HandleFunc("/logs/stream", h.LogStreamHandler).Methods(http.MethodGet)

	// Traces / APM (list must come before {traceID} to avoid route shadowing)
	api.HandleFunc("/traces", h.TraceSearchHandler).Methods(http.MethodGet)
	api.HandleFunc("/traces/{traceID}", h.TraceQueryHandler).Methods(http.MethodGet)

	// OTLP HTTP receiver — replaces Grafana Agent / Datadog OTEL collector
	r.HandleFunc("/otlp/v1/traces", h.OTLPTraceHandler).Methods(http.MethodPost)
	r.HandleFunc("/otlp/v1/metrics", h.OTLPMetricsHandler).Methods(http.MethodPost)
	r.HandleFunc("/otlp/v1/logs", h.OTLPLogsHandler).Methods(http.MethodPost)
	r.HandleFunc("/opentelemetry/api/v1/traces", h.OTLPTraceHandler).Methods(http.MethodPost)

	// Loki-compatible log ingestion — replaces Grafana Loki
	r.HandleFunc("/loki/api/v1/push", h.LokiPushHandler).Methods(http.MethodPost)
	r.HandleFunc("/loki/api/v1/query_range", h.LokiQueryRangeHandler).Methods(http.MethodGet)
	r.HandleFunc("/loki/api/v1/labels", h.LokiLabelsHandler).Methods(http.MethodGet)
	r.HandleFunc("/loki/api/v1/label/{name}/values", h.LokiLabelValuesHandler).Methods(http.MethodGet)

	// Elasticsearch-compatible API — replaces ELK (Filebeat/Logstash/Beats/Vector)
	r.HandleFunc("/_bulk", h.ESBulkHandler).Methods(http.MethodPost)
	r.HandleFunc("/{index}/_bulk", h.ESBulkHandler).Methods(http.MethodPost)
	r.HandleFunc("/_cat/indices", h.ESCatIndicesHandler).Methods(http.MethodGet)
	r.HandleFunc("/_search", h.ESSearchHandler).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/{index}/_search", h.ESSearchHandler).Methods(http.MethodGet, http.MethodPost)

	// Datadog-compatible metrics/logs API — replaces Datadog agent
	r.HandleFunc("/api/v1/series", h.DDMetricsHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/v2/logs", h.DDLogsHandler).Methods(http.MethodPost)

	// Embedded UI
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./web")))

	return r
}
