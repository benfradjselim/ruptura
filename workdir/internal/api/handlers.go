package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	oheproc "github.com/benfradjselim/ohe/internal/processor"
	"github.com/benfradjselim/ohe/internal/alerter"
	"github.com/benfradjselim/ohe/internal/analyzer"
	"github.com/benfradjselim/ohe/internal/predictor"
	"github.com/benfradjselim/ohe/internal/storage"
	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"github.com/benfradjselim/ohe/pkg/logger"
)

const version = "5.0.0"

// Handlers holds all API dependencies
// UsageRecorder is a function that records a billable usage event.
// Injected by the orchestrator via SetUsageRecorder to avoid an import cycle.
type UsageRecorder func(orgID, eventType string, value float64)

type Handlers struct {
	store          *storage.Store
	processor      *oheproc.Processor
	analyzer       *analyzer.Analyzer
	topology       *analyzer.TopologyAnalyzer
	predictor      *predictor.Predictor
	alerter        *alerter.Alerter
	hub            *Hub
	hostname       string
	jwtSecret      string
	startTime      time.Time
	authEnabled    bool
	ready          int32         // 1 = ready; 0 = not ready; accessed via atomic ops (Go 1.18 compat)
	usageRecorder  UsageRecorder // optional; no-op when nil
}

// SetUsageRecorder wires the billing meter into the handler set.
func (h *Handlers) SetUsageRecorder(fn UsageRecorder) {
	h.usageRecorder = fn
}

// recordUsage emits a billing event; safe to call when recorder is nil.
func (h *Handlers) recordUsage(r *http.Request, eventType string, value float64) {
	if h.usageRecorder == nil {
		return
	}
	h.usageRecorder(orgIDFromContext(r.Context()), eventType, value)
}

// orgStore returns a tenant-scoped store for the organisation identified in the
// request's JWT claims. All metric, KPI, alert, dashboard, datasource, SLO,
// log, and span operations must go through the returned OrgStore to ensure
// hard data isolation between tenants at the storage layer.
func (h *Handlers) orgStore(r *http.Request) *storage.OrgStore {
	return h.store.ForOrg(orgIDFromContext(r.Context()))
}

// orgQuota returns the quota for the current request's org.
// Falls back to DefaultQuota when the org record is missing.
func (h *Handlers) orgQuota(r *http.Request) models.QuotaConfig {
	var org models.Org
	if err := h.store.GetOrg(orgIDFromContext(r.Context()), &org); err != nil {
		return models.DefaultQuota()
	}
	// Zero-value quota means the org was created before quotas existed — apply defaults.
	if org.Quota == (models.QuotaConfig{}) {
		return models.DefaultQuota()
	}
	return org.Quota
}

// audit appends an immutable audit entry. Errors are logged but never fatal
// so that an audit write failure never blocks the primary operation.
func (h *Handlers) audit(r *http.Request, action, resource, resourceID, details string) {
	claims, _ := claimsFromContext(r.Context())
	username := ""
	orgID := orgIDFromContext(r.Context())
	if claims != nil {
		username = claims.Username
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	entry := storage.AuditEntry{
		Timestamp:  time.Now().UTC(),
		OrgID:      orgID,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		IPAddress:  ip,
	}
	if err := h.store.AppendAuditEntry(entry); err != nil {
		logger.Default.Error("audit write error", "err", err)
	}
}

// SetReady marks the server as ready to serve traffic (called by the orchestrator).
func (h *Handlers) SetReady(v bool) {
	if v {
		atomic.StoreInt32(&h.ready, 1)
	} else {
		atomic.StoreInt32(&h.ready, 0)
	}
}

// NewHandlers constructs the handler set
func NewHandlers(
	store *storage.Store,
	proc *oheproc.Processor,
	ana *analyzer.Analyzer,
	pred *predictor.Predictor,
	alrt *alerter.Alerter,
	hostname string,
	jwtSecret string,
	authEnabled bool,
	allowedOrigins ...[]string,
) *Handlers {
	var origins []string
	if len(allowedOrigins) > 0 {
		origins = allowedOrigins[0]
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	return &Handlers{
		store:       store,
		processor:   proc,
		analyzer:    ana,
		topology:    analyzer.NewTopologyAnalyzer(10 * time.Minute),
		predictor:   pred,
		alerter:     alrt,
		hub:         NewHub(origins),
		hostname:    hostname,
		jwtSecret:   jwtSecret,
		startTime:   time.Now(),
		authEnabled: authEnabled,
	}
}

// TopologyAnalyzer returns the topology analyzer (used by orchestrator to wire receivers)
func (h *Handlers) TopologyAnalyzer() *analyzer.TopologyAnalyzer {
	return h.topology
}

// OpenAPIHandler GET /api/v1/openapi.yaml — serves the bundled OpenAPI 3.0 spec.
// The spec file is read from docs/openapi.yaml relative to the working directory.
func (h *Handlers) OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	data, err := os.ReadFile("docs/openapi.yaml")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "SPEC_NOT_FOUND", "openapi.yaml not found")
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HealthHandler GET /api/v1/health
func (h *Handlers) HealthHandler(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{
		"storage": "ok",
	}
	if !h.store.Healthy() {
		checks["storage"] = "error"
	}

	respondSuccess(w, models.HealthResponse{
		Status:    "ok",
		Version:   version,
		Host:      h.hostname,
		Uptime:    time.Since(h.startTime).Seconds(),
		Checks:    checks,
		Timestamp: time.Now().UTC(),
	})
}

// LivenessHandler GET /api/v1/health/live
// Returns 200 while the process is running. K8s liveness probe target.
func (h *Handlers) LivenessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"alive"}`)
}

// ReadinessHandler GET /api/v1/health/ready
// Returns 200 when storage is healthy and the engine has finished initialising.
// Returns 503 otherwise. K8s readiness probe target.
func (h *Handlers) ReadinessHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if atomic.LoadInt32(&h.ready) == 0 || !h.store.Healthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = fmt.Fprintf(w, `{"status":"not_ready"}`)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ready"}`)
}

// MetricsListHandler GET /api/v1/metrics
func (h *Handlers) MetricsListHandler(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	// Return latest normalized values for common metrics
	metricNames := []string{
		"cpu_percent", "memory_percent", "disk_percent",
		"net_rx_bps", "net_tx_bps", "load_avg_1", "load_avg_5",
		"load_avg_15", "uptime_seconds", "processes",
	}

	result := make(map[string]interface{})
	for _, name := range metricNames {
		val, ok := h.processor.GetNormalized(host, name)
		if ok {
			result[name] = val
		}
	}

	respondSuccess(w, map[string]interface{}{
		"host":    host,
		"metrics": result,
	})
}

// MetricGetHandler GET /api/v1/metrics/{name}
func (h *Handlers) MetricGetHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	from, to := parseTimeRange(r)

	values, err := h.orgStore(r).GetMetricRange(host, name, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}

	respondSuccess(w, map[string]interface{}{
		"host":   host,
		"metric": name,
		"from":   from,
		"to":     to,
		"points": values,
	})
}

// MetricAggregateHandler GET /api/v1/metrics/{name}/aggregate
func (h *Handlers) MetricAggregateHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	agg, ok := h.processor.Aggregate(host, name)
	if !ok {
		respondError(w, http.StatusNotFound, "NO_DATA", "no data for metric")
		return
	}
	respondSuccess(w, agg)
}

// KPIListHandler GET /api/v1/kpis — read-only, returns last computed snapshot
func (h *Handlers) KPIListHandler(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	snap, ok := h.analyzer.Snapshot(host)
	if !ok {
		respondError(w, http.StatusNotFound, "NO_DATA", "no KPI data available yet for host")
		return
	}
	respondSuccess(w, snap)
}

// KPIGetHandler GET /api/v1/kpis/{name}
func (h *Handlers) KPIGetHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	from, to := parseTimeRange(r)

	values, err := h.orgStore(r).GetKPIRange(host, name, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, map[string]interface{}{
		"host":   host,
		"kpi":    name,
		"from":   from,
		"to":     to,
		"points": values,
	})
}

// PredictHandler GET /api/v1/kpis/{name}/predict or GET /api/v1/predict
func (h *Handlers) PredictHandler(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}
	horizonStr := r.URL.Query().Get("horizon")
	horizon := 120 // default 2 hours
	if horizonStr != "" {
		if v, err := strconv.Atoi(horizonStr); err == nil && v > 0 {
			horizon = v
		}
	}

	preds := h.predictor.PredictAll(host, horizon)
	respondSuccess(w, map[string]interface{}{
		"host":            host,
		"horizon_minutes": horizon,
		"predictions":     preds,
	})
}

// AlertListHandler GET /api/v1/alerts
func (h *Handlers) AlertListHandler(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	var alerts []*models.Alert
	if activeOnly {
		alerts = h.alerter.GetActive()
	} else {
		alerts = h.alerter.GetAll()
	}
	respondSuccess(w, alerts)
}

// AlertGetHandler GET /api/v1/alerts/{id}
func (h *Handlers) AlertGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	al, ok := h.alerter.GetByID(id)
	if !ok {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "alert not found")
		return
	}
	respondSuccess(w, al)
}

// AlertAcknowledgeHandler POST /api/v1/alerts/{id}/acknowledge
func (h *Handlers) AlertAcknowledgeHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.alerter.Acknowledge(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondSuccess(w, map[string]string{"status": "acknowledged"})
}

// AlertSilenceHandler POST /api/v1/alerts/{id}/silence
func (h *Handlers) AlertSilenceHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.alerter.Silence(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	respondSuccess(w, map[string]string{"status": "silenced"})
}

// AlertDeleteHandler DELETE /api/v1/alerts/{id}
func (h *Handlers) AlertDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.alerter.Delete(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DashboardListHandler GET /api/v1/dashboards
func (h *Handlers) DashboardListHandler(w http.ResponseWriter, r *http.Request) {
	var dashboards []*models.Dashboard
	err := h.orgStore(r).ListDashboards(func(val []byte) error {
		var d models.Dashboard
		if err := json.Unmarshal(val, &d); err != nil {
			return nil
		}
		dashboards = append(dashboards, &d)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, dashboards)
}

// DashboardCreateHandler POST /api/v1/dashboards
func (h *Handlers) DashboardCreateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.orgStore(r).CheckDashboardQuota(h.orgQuota(r).MaxDashboards); err != nil {
		respondError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", err.Error())
		return
	}
	var d models.Dashboard
	if err := decodeBody(r, &d); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	d.ID = utils.GenerateID(8)
	d.CreatedAt = time.Now()
	d.UpdatedAt = d.CreatedAt
	if err := h.orgStore(r).SaveDashboard(d.ID, d); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	h.audit(r, "create", "dashboard", d.ID, d.Name)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"data":      d,
		"timestamp": time.Now().UTC(),
	})
}

// DashboardGetHandler GET /api/v1/dashboards/{id}
func (h *Handlers) DashboardGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var d models.Dashboard
	if err := h.orgStore(r).GetDashboard(id, &d); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "dashboard not found")
		return
	}
	respondSuccess(w, d)
}

// DashboardUpdateHandler PUT /api/v1/dashboards/{id}
func (h *Handlers) DashboardUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var d models.Dashboard
	if err := h.orgStore(r).GetDashboard(id, &d); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "dashboard not found")
		return
	}
	var update models.Dashboard
	if err := decodeBody(r, &update); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	update.ID = id
	update.CreatedAt = d.CreatedAt
	update.UpdatedAt = time.Now()
	if err := h.orgStore(r).SaveDashboard(id, update); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, update)
}

// DashboardDeleteHandler DELETE /api/v1/dashboards/{id}
func (h *Handlers) DashboardDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.orgStore(r).DeleteDashboard(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "dashboard not found")
		return
	}
	h.audit(r, "delete", "dashboard", id, "")
	w.WriteHeader(http.StatusNoContent)
}

// LoginHandler POST /api/v1/auth/login
func (h *Handlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	var user models.User
	if err := h.store.GetUser(req.Username, &user); err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		respondError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid username or password")
		return
	}

	exp := time.Now().Add(24 * time.Hour)
	claims := JWTClaims{
		Username: user.Username,
		Role:     user.Role,
		OrgID:    user.OrgID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        utils.GenerateID(16), // JTI — unique per token, used for revocation
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not generate token")
		return
	}

	h.audit(r, "login", "session", user.Username, "")
	user.Password = "" // never expose hash in response
	respondSuccess(w, models.LoginResponse{
		Token:   signed,
		Expires: exp.Unix(),
		User:    user,
	})
}

// IngestHandler POST /api/v1/ingest — receives metrics from remote agents
func (h *Handlers) IngestHandler(w http.ResponseWriter, r *http.Request) {
	var batch models.MetricBatch
	if err := decodeBody(r, &batch); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	// Sanitize host to prevent Badger key namespace injection
	batch.Host = sanitizeKey(batch.Host)
	for i := range batch.Metrics {
		batch.Metrics[i].Host = sanitizeKey(batch.Metrics[i].Host)
		batch.Metrics[i].Name = sanitizeKey(batch.Metrics[i].Name)
	}

	h.processor.Ingest(batch.Metrics)

	// Store metrics in Badger
	for _, m := range batch.Metrics {
		if err := h.orgStore(r).SaveMetric(m.Host, m.Name, m.Value, m.Timestamp); err != nil {
			logger.Default.ErrorCtx(r.Context(), "SaveMetric failed", "host", m.Host, "metric", m.Name, "err", err)
		}
	}

	// Build metrics map and run KPI analysis
	metrics := h.buildMetricsMap(batch.Host)
	snapshot := h.analyzer.Update(batch.Host, metrics)

	// Store KPIs (log errors, don't abort)
	for kpiName, kpiVal := range map[string]float64{
		"stress":    snapshot.Stress.Value,
		"fatigue":   snapshot.Fatigue.Value,
		"mood":      snapshot.Mood.Value,
		"pressure":  snapshot.Pressure.Value,
		"humidity":  snapshot.Humidity.Value,
		"contagion": snapshot.Contagion.Value,
	} {
		if err := h.orgStore(r).SaveKPI(batch.Host, kpiName, kpiVal, snapshot.Timestamp); err != nil {
			logger.Default.ErrorCtx(r.Context(), "SaveKPI failed", "host", batch.Host, "kpi", kpiName, "err", err)
		}
	}

	// Feed predictor
	now := time.Now()
	for _, m := range batch.Metrics {
		h.predictor.Feed(m.Host, m.Name, m.Value, now)
	}
	h.predictor.Feed(batch.Host, "stress", snapshot.Stress.Value, now)
	h.predictor.Feed(batch.Host, "fatigue", snapshot.Fatigue.Value, now)

	// Store new ETF KPIs
	for kpiName, kpiVal := range map[string]float64{
		"resilience":   snapshot.Resilience.Value,
		"entropy":      snapshot.Entropy.Value,
		"velocity":     snapshot.Velocity.Value,
		"health_score": snapshot.HealthScore.Value,
	} {
		if err := h.orgStore(r).SaveKPI(batch.Host, kpiName, kpiVal, snapshot.Timestamp); err != nil {
			logger.Default.ErrorCtx(r.Context(), "SaveKPI failed", "host", batch.Host, "kpi", kpiName, "err", err)
		}
	}
	// Feed predictor with new KPIs
	h.predictor.Feed(batch.Host, "resilience", snapshot.Resilience.Value, now)
	h.predictor.Feed(batch.Host, "health_score", snapshot.HealthScore.Value, now)

	// Evaluate alerts
	kpiMap := map[string]float64{
		"stress":       snapshot.Stress.Value,
		"fatigue":      snapshot.Fatigue.Value,
		"mood":         snapshot.Mood.Value,
		"pressure":     snapshot.Pressure.Value,
		"humidity":     snapshot.Humidity.Value,
		"contagion":    snapshot.Contagion.Value,
		"resilience":   snapshot.Resilience.Value,
		"entropy":      snapshot.Entropy.Value,
		"velocity":     snapshot.Velocity.Value,
		"health_score": snapshot.HealthScore.Value / 100.0, // normalize to [0,1] for rules
	}
	h.alerter.Evaluate(batch.Host, kpiMap)

	// Broadcast live update to WebSocket subscribers
	if msg, err := json.Marshal(map[string]interface{}{
		"type": "kpi_update", "data": snapshot,
	}); err == nil {
		h.hub.Broadcast(msg)
	}

	// Record metered ingest event (bytes approximated as metric count × 100)
	h.recordUsage(r, "ingest_bytes", float64(len(batch.Metrics)*100))

	respondSuccess(w, map[string]interface{}{
		"accepted": len(batch.Metrics),
		"kpis":     snapshot,
	})
}

// ConfigHandler GET /api/v1/config
func (h *Handlers) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	respondSuccess(w, map[string]interface{}{
		"version":      version,
		"auth_enabled": h.authEnabled,
	})
}

// ReloadHandler POST /api/v1/reload — signal config reload (no-op for now, returns ok)
func (h *Handlers) ReloadHandler(w http.ResponseWriter, r *http.Request) {
	respondSuccess(w, map[string]string{"status": "reloaded"})
}

// --- DataSource handlers ---

// DataSourceListHandler GET /api/v1/datasources
func (h *Handlers) DataSourceListHandler(w http.ResponseWriter, r *http.Request) {
	var sources []*models.DataSource
	err := h.orgStore(r).ListDataSources(func(val []byte) error {
		var ds models.DataSource
		if err := json.Unmarshal(val, &ds); err != nil {
			return nil
		}
		sources = append(sources, &ds)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, sources)
}

// DataSourceCreateHandler POST /api/v1/datasources
func (h *Handlers) DataSourceCreateHandler(w http.ResponseWriter, r *http.Request) {
	if err := h.orgStore(r).CheckDataSourceQuota(h.orgQuota(r).MaxDataSources); err != nil {
		respondError(w, http.StatusPaymentRequired, "QUOTA_EXCEEDED", err.Error())
		return
	}
	var ds models.DataSource
	if err := decodeBody(r, &ds); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	// Validate URL at creation time to prevent stored SSRF
	if ds.URL != "" {
		if err := validateDataSourceURL(ds.URL); err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_URL", "datasource URL is not allowed")
			return
		}
	}
	ds.ID = utils.GenerateID(8)
	ds.Enabled = true
	if err := h.orgStore(r).SaveDataSource(ds.ID, ds); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	h.audit(r, "create", "datasource", ds.ID, ds.Name)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": ds, "timestamp": time.Now().UTC(),
	})
}

// DataSourceGetHandler GET /api/v1/datasources/{id}
func (h *Handlers) DataSourceGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ds models.DataSource
	if err := h.orgStore(r).GetDataSource(id, &ds); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "datasource not found")
		return
	}
	respondSuccess(w, ds)
}

// DataSourceUpdateHandler PUT /api/v1/datasources/{id}
func (h *Handlers) DataSourceUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ds models.DataSource
	if err := decodeBody(r, &ds); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	// Validate URL at update time to prevent stored SSRF
	if ds.URL != "" {
		if err := validateDataSourceURL(ds.URL); err != nil {
			respondError(w, http.StatusBadRequest, "INVALID_URL", "datasource URL is not allowed")
			return
		}
	}
	ds.ID = id
	if err := h.orgStore(r).SaveDataSource(id, ds); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, ds)
}

// DataSourceDeleteHandler DELETE /api/v1/datasources/{id}
func (h *Handlers) DataSourceDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.orgStore(r).DeleteDataSource(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "datasource not found")
		return
	}
	h.audit(r, "delete", "datasource", id, "")
	w.WriteHeader(http.StatusNoContent)
}

// DataSourceTestHandler POST /api/v1/datasources/{id}/test
func (h *Handlers) DataSourceTestHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var ds models.DataSource
	if err := h.orgStore(r).GetDataSource(id, &ds); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "datasource not found")
		return
	}
	if err := validateDataSourceURL(ds.URL); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_URL", "datasource URL is not allowed")
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ds.URL)
	if err != nil {
		// Do not leak raw error (may contain internal hostnames/IPs)
		respondSuccess(w, map[string]interface{}{"status": "error", "message": "connection failed"})
		return
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	respondSuccess(w, map[string]interface{}{"status": "ok", "http_status": resp.StatusCode})
}

// --- User management handlers ---

// UserListHandler GET /api/v1/auth/users
func (h *Handlers) UserListHandler(w http.ResponseWriter, r *http.Request) {
	var users []models.User
	err := h.store.ListUsers(func(val []byte) error {
		var u models.User
		if err := json.Unmarshal(val, &u); err != nil {
			return nil
		}
		u.Password = "" // never expose hash
		users = append(users, u)
		return nil
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondSuccess(w, users)
}

// UserCreateHandler POST /api/v1/auth/users
func (h *Handlers) UserCreateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "password must be at least 8 characters")
		return
	}
	if len(req.Password) > 72 {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "password must not exceed 72 characters (bcrypt limit)")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12) // OWASP: cost≥12
	if err != nil {
		respondError(w, http.StatusInternalServerError, "HASH_ERROR", "could not hash password")
		return
	}
	role := req.Role
	if role == "" {
		role = "viewer"
	}
	user := models.User{
		ID:       utils.GenerateID(8),
		Username: req.Username,
		Password: string(hash),
		Role:     role,
	}
	if err := h.store.SaveUser(req.Username, user); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	user.Password = ""
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": user, "timestamp": time.Now().UTC(),
	})
}

// UserGetHandler GET /api/v1/auth/users/{id}
func (h *Handlers) UserGetHandler(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["id"]
	var user models.User
	if err := h.store.GetUser(username, &user); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	user.Password = ""
	respondSuccess(w, user)
}

// UserDeleteHandler DELETE /api/v1/auth/users/{id}
func (h *Handlers) UserDeleteHandler(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["id"]
	if err := h.store.DeleteUser(username); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// LogoutHandler POST /api/v1/auth/logout — adds the token's JTI to the revocation blocklist.
// The Badger entry TTL is set to the token's remaining lifetime so no cleanup is needed.
func (h *Handlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if ok && claims.ID != "" && claims.ExpiresAt != nil {
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			_ = h.store.RevokeToken(claims.ID, ttl)
		}
	}
	h.audit(r, "logout", "session", "", "")
	respondSuccess(w, map[string]string{"status": "logged out"})
}

// RefreshHandler POST /api/v1/auth/refresh — issue a new token from valid existing one
func (h *Handlers) RefreshHandler(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", "no claims in context")
		return
	}
	exp := time.Now().Add(24 * time.Hour)
	newClaims := JWTClaims{
		Username: claims.Username,
		Role:     claims.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)
	signed, err := token.SignedString([]byte(h.jwtSecret))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "TOKEN_ERROR", "could not generate token")
		return
	}
	respondSuccess(w, map[string]interface{}{
		"token":   signed,
		"expires": exp.Unix(),
	})
}

// DashboardExportHandler GET /api/v1/dashboards/{id}/export
func (h *Handlers) DashboardExportHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var d models.Dashboard
	if err := h.orgStore(r).GetDashboard(id, &d); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "dashboard not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="dashboard-%s.json"`, id))
	_ = json.NewEncoder(w).Encode(d)
}

// DashboardImportHandler POST /api/v1/dashboards/import
func (h *Handlers) DashboardImportHandler(w http.ResponseWriter, r *http.Request) {
	var d models.Dashboard
	if err := decodeBody(r, &d); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	d.ID = utils.GenerateID(8)
	d.CreatedAt = time.Now()
	d.UpdatedAt = d.CreatedAt
	if err := h.orgStore(r).SaveDashboard(d.ID, d); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": d, "timestamp": time.Now().UTC(),
	})
}

// QueryHandler POST /api/v1/query — simple metric query by name and time range
func (h *Handlers) QueryHandler(w http.ResponseWriter, r *http.Request) {
	var req models.QueryRequest
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if req.To.IsZero() {
		req.To = time.Now()
	}
	if req.From.IsZero() {
		req.From = req.To.Add(-time.Hour)
	}

	host := r.URL.Query().Get("host")
	if host == "" {
		host = "localhost"
	}

	// req.Query is a metric name for now
	tvs, err := h.orgStore(r).GetMetricRange(host, req.Query, req.From, req.To)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "QUERY_ERROR", err.Error())
		return
	}

	points := make([]models.DataPoint, 0, len(tvs))
	for _, tv := range tvs {
		points = append(points, models.DataPoint{Timestamp: tv.Timestamp, Value: tv.Value})
	}

	// Downsample if step > 0
	if req.Step > 0 {
		points = oheproc.Downsample(points, time.Duration(req.Step)*time.Second)
	}

	respondSuccess(w, models.QueryResult{Metric: req.Query, Points: points})
}

// SetupHandler POST /api/v1/auth/setup — creates the first admin account.
// Only works when the user store is empty; returns 409 if any user already exists.
func (h *Handlers) SetupHandler(w http.ResponseWriter, r *http.Request) {
	// Check if any users already exist
	count := 0
	_ = h.store.ListUsers(func([]byte) error {
		count++
		return nil
	})
	if count > 0 {
		respondError(w, http.StatusConflict, "ALREADY_SETUP", "system has already been configured")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeBody(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if len(req.Password) < 8 || len(req.Password) > 72 {
		respondError(w, http.StatusBadRequest, "BAD_REQUEST", "password must be 8–72 characters")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "HASH_ERROR", "could not hash password")
		return
	}
	user := models.User{
		ID:       utils.GenerateID(8),
		Username: req.Username,
		Password: string(hash),
		Role:     "admin",
	}
	if err := h.store.SaveUser(req.Username, user); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	user.Password = ""
	logger.Default.InfoCtx(r.Context(), "first admin user created via setup endpoint", "username", req.Username)
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": user, "timestamp": time.Now().UTC(),
	})
}

// --- Built-in dashboard templates ---

type dashboardTemplate struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Tags        []string         `json:"tags"`
	Category    string           `json:"category"`
	Icon        string           `json:"icon"`
	Dashboard   models.Dashboard `json:"dashboard"`
}

// widgetsToPrediction converts timeseries/gauge/stat widgets to prediction widgets
// so templates can be applied in "predicted" mode.
func widgetsToPrediction(widgets []models.Widget) []models.Widget {
	out := make([]models.Widget, len(widgets))
	for i, w := range widgets {
		out[i] = w
		if w.Type == "timeseries" || w.Type == "gauge" || w.Type == "stat" {
			out[i].Type = "prediction"
			if out[i].Options == nil {
				out[i].Options = map[string]string{}
			}
			out[i].Options["horizon"] = "60"
		}
	}
	return out
}

var builtinTemplates = []dashboardTemplate{
	{
		ID:          "system-overview",
		Name:        "System Overview",
		Description: "CPU, memory, disk and network metrics for a single host",
		Tags:        []string{"system", "infrastructure"},
		Category:    "Infrastructure",
		Icon:        "server",
		Dashboard: models.Dashboard{
			Name:    "System Overview",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "CPU Usage", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory Usage", Type: "timeseries", Metric: "memory_percent"},
				{Title: "Disk Usage", Type: "gauge", Metric: "disk_percent"},
				{Title: "Load Average 1m", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "Network RX", Type: "timeseries", Metric: "net_rx_bps"},
				{Title: "Network TX", Type: "timeseries", Metric: "net_tx_bps"},
			},
		},
	},
	{
		ID:          "kpi-holistic",
		Name:        "Holistic KPI Dashboard",
		Description: "The six OHE vital signs: Stress, Fatigue, Mood, Pressure, Humidity, Contagion",
		Tags:        []string{"kpi", "holistic", "ohe"},
		Category:    "OHE KPIs",
		Icon:        "activity",
		Dashboard: models.Dashboard{
			Name:    "Holistic KPI Dashboard",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Stress Index", Type: "gauge", KPI: "stress"},
				{Title: "Fatigue Level", Type: "timeseries", KPI: "fatigue"},
				{Title: "System Mood", Type: "timeseries", KPI: "mood"},
				{Title: "Atmospheric Pressure", Type: "timeseries", KPI: "pressure"},
				{Title: "Error Humidity", Type: "gauge", KPI: "humidity"},
				{Title: "Contagion Index", Type: "gauge", KPI: "contagion"},
			},
		},
	},
	{
		ID:          "kpi-etf",
		Name:        "ETF Composite KPIs",
		Description: "4 composed ETF-style KPIs: HealthScore, Resilience, Entropy, Velocity",
		Tags:        []string{"kpi", "etf", "composite", "ohe"},
		Category:    "OHE KPIs",
		Icon:        "trending-up",
		Dashboard: models.Dashboard{
			Name:    "ETF Composite KPIs",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Health Score (0-100)", Type: "gauge", KPI: "health_score"},
				{Title: "Resilience", Type: "gauge", KPI: "resilience"},
				{Title: "System Entropy", Type: "timeseries", KPI: "entropy"},
				{Title: "Change Velocity", Type: "timeseries", KPI: "velocity"},
			},
		},
	},
	{
		ID:          "container-overview",
		Name:        "Container Overview",
		Description: "Per-container CPU and memory usage",
		Tags:        []string{"containers", "docker"},
		Category:    "Containers",
		Icon:        "box",
		Dashboard: models.Dashboard{
			Name:    "Container Overview",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "Container CPU", Type: "timeseries", Metric: "container_cpu_percent"},
				{Title: "Container Memory %", Type: "gauge", Metric: "container_mem_percent"},
				{Title: "Container Memory MB", Type: "timeseries", Metric: "container_mem_used_mb"},
				{Title: "Net RX Bytes", Type: "timeseries", Metric: "container_net_rx_bytes"},
				{Title: "Net TX Bytes", Type: "timeseries", Metric: "container_net_tx_bytes"},
			},
		},
	},
	{
		ID:          "k8s-node",
		Name:        "Kubernetes Node Health",
		Description: "Per-node holistic health for K8s deployments — works with the OHE DaemonSet agent",
		Tags:        []string{"kubernetes", "k8s", "node", "ohe"},
		Category:    "Kubernetes",
		Icon:        "layers",
		Dashboard: models.Dashboard{
			Name:    "Kubernetes Node Health",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Node Health Score", Type: "gauge", KPI: "health_score"},
				{Title: "Node Stress", Type: "timeseries", KPI: "stress"},
				{Title: "Node Fatigue", Type: "timeseries", KPI: "fatigue"},
				{Title: "Contagion (pod errors spreading)", Type: "gauge", KPI: "contagion"},
				{Title: "CPU %", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory %", Type: "timeseries", Metric: "memory_percent"},
				{Title: "Load Avg 1m", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "k8s-cluster-fleet",
		Name:        "Kubernetes Cluster Fleet",
		Description: "Multi-node fleet overview — aggregate health across all K8s nodes",
		Tags:        []string{"kubernetes", "k8s", "fleet", "ohe"},
		Category:    "Kubernetes",
		Icon:        "globe",
		Dashboard: models.Dashboard{
			Name:    "Kubernetes Cluster Fleet",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "Fleet Health Matrix", Type: "stat", KPI: "health_score"},
				{Title: "Cluster Stress", Type: "timeseries", KPI: "stress"},
				{Title: "Cluster Resilience", Type: "gauge", KPI: "resilience"},
				{Title: "Cluster Entropy", Type: "timeseries", KPI: "entropy"},
				{Title: "Contagion Spread", Type: "gauge", KPI: "contagion"},
				{Title: "Active Cluster Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "sre-golden-signals",
		Name:        "SRE Golden Signals",
		Description: "Latency, Traffic, Errors, Saturation — mapped to OHE KPIs",
		Tags:        []string{"sre", "golden-signals", "reliability"},
		Category:    "SRE",
		Icon:        "shield",
		Dashboard: models.Dashboard{
			Name:    "SRE Golden Signals",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Latency (Load Avg)", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "Traffic (Request Rate)", Type: "timeseries", Metric: "request_rate"},
				{Title: "Errors (Error Rate)", Type: "timeseries", Metric: "error_rate"},
				{Title: "Saturation (CPU + RAM)", Type: "gauge", KPI: "stress"},
				{Title: "Error Humidity (compound error score)", Type: "gauge", KPI: "humidity"},
				{Title: "System Mood (reliability index)", Type: "timeseries", KPI: "mood"},
			},
		},
	},
	{
		ID:          "incident-response",
		Name:        "Incident Response",
		Description: "Storm detection, contagion spread, pressure spikes — for on-call engineers",
		Tags:        []string{"incident", "oncall", "sre"},
		Category:    "SRE",
		Icon:        "alert-triangle",
		Dashboard: models.Dashboard{
			Name:    "Incident Response",
			Refresh: 10,
			Widgets: []models.Widget{
				{Title: "Pressure (storm indicator)", Type: "gauge", KPI: "pressure"},
				{Title: "Contagion (blast radius)", Type: "gauge", KPI: "contagion"},
				{Title: "Humidity (error storm)", Type: "gauge", KPI: "humidity"},
				{Title: "Fatigue (burnout risk)", Type: "timeseries", KPI: "fatigue"},
				{Title: "Velocity (rate of change)", Type: "timeseries", KPI: "velocity"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	// ── New Templates ────────────────────────────────────────────────────────
	{
		ID:          "capacity-planning",
		Name:        "Capacity Planning",
		Description: "Trend-based forecasts for CPU, memory, disk growth — plan before you hit limits",
		Tags:        []string{"capacity", "planning", "forecast"},
		Category:    "Infrastructure",
		Icon:        "bar-chart-2",
		Dashboard: models.Dashboard{
			Name:    "Capacity Planning",
			Refresh: 60,
			Widgets: []models.Widget{
				{Title: "CPU Forecast", Type: "prediction", Metric: "cpu_percent", Options: map[string]string{"horizon": "120"}},
				{Title: "Memory Forecast", Type: "prediction", Metric: "memory_percent", Options: map[string]string{"horizon": "120"}},
				{Title: "Disk Growth Forecast", Type: "prediction", Metric: "disk_percent", Options: map[string]string{"horizon": "120"}},
				{Title: "Load Average Trend", Type: "prediction", Metric: "load_avg_15", Options: map[string]string{"horizon": "60"}},
				{Title: "Health Score Trend", Type: "prediction", KPI: "health_score", Options: map[string]string{"horizon": "60"}},
				{Title: "Stress Forecast", Type: "prediction", KPI: "stress", Options: map[string]string{"horizon": "60"}},
			},
		},
	},
	{
		ID:          "security-anomaly",
		Name:        "Security & Anomaly Detection",
		Description: "Entropy spikes, contagion bursts, and sudden metric deviations indicating attacks or anomalies",
		Tags:        []string{"security", "anomaly", "entropy"},
		Category:    "Security",
		Icon:        "eye",
		Dashboard: models.Dashboard{
			Name:    "Security & Anomaly Detection",
			Refresh: 10,
			Widgets: []models.Widget{
				{Title: "System Entropy (disorder)", Type: "timeseries", KPI: "entropy"},
				{Title: "Contagion Index", Type: "gauge", KPI: "contagion"},
				{Title: "Error Humidity", Type: "timeseries", KPI: "humidity"},
				{Title: "Pressure Spikes", Type: "timeseries", KPI: "pressure"},
				{Title: "CPU Anomaly", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "executive-health",
		Name:        "Executive Health Summary",
		Description: "Single-pane C-level view: Health Score, Resilience, SLA proxies and alert summary",
		Tags:        []string{"executive", "summary", "health"},
		Category:    "OHE KPIs",
		Icon:        "award",
		Dashboard: models.Dashboard{
			Name:    "Executive Health Summary",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "Overall Health Score", Type: "kpi", KPI: "health_score"},
				{Title: "Resilience Index", Type: "kpi", KPI: "resilience"},
				{Title: "System Mood (SLA proxy)", Type: "gauge", KPI: "mood"},
				{Title: "Change Velocity", Type: "stat", KPI: "velocity"},
				{Title: "Health Score Trend", Type: "timeseries", KPI: "health_score"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "performance-deep-dive",
		Name:        "Performance Deep Dive",
		Description: "Detailed CPU, memory, I/O and load breakdown for deep performance analysis",
		Tags:        []string{"performance", "cpu", "memory", "io"},
		Category:    "Infrastructure",
		Icon:        "zap",
		Dashboard: models.Dashboard{
			Name:    "Performance Deep Dive",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "CPU %", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory %", Type: "timeseries", Metric: "memory_percent"},
				{Title: "Disk I/O Read", Type: "timeseries", Metric: "disk_read_bps"},
				{Title: "Disk I/O Write", Type: "timeseries", Metric: "disk_write_bps"},
				{Title: "Load Avg 1m", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "Load Avg 5m", Type: "timeseries", Metric: "load_avg_5"},
				{Title: "Load Avg 15m", Type: "timeseries", Metric: "load_avg_15"},
				{Title: "Process Count", Type: "stat", Metric: "processes"},
			},
		},
	},
	{
		ID:          "network-analysis",
		Name:        "Network Analysis",
		Description: "Bandwidth, packet rates, and network health for infrastructure monitoring",
		Tags:        []string{"network", "bandwidth", "traffic"},
		Category:    "Infrastructure",
		Icon:        "wifi",
		Dashboard: models.Dashboard{
			Name:    "Network Analysis",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "RX Bandwidth", Type: "timeseries", Metric: "net_rx_bps"},
				{Title: "TX Bandwidth", Type: "timeseries", Metric: "net_tx_bps"},
				{Title: "RX Packets", Type: "timeseries", Metric: "net_rx_packets"},
				{Title: "TX Packets", Type: "timeseries", Metric: "net_tx_packets"},
				{Title: "Contagion (network errors propagating)", Type: "gauge", KPI: "contagion"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "database-health",
		Name:        "Database Health",
		Description: "Memory pressure, connection pool, query latency and error rate for database services",
		Tags:        []string{"database", "db", "postgres", "mysql"},
		Category:    "Applications",
		Icon:        "database",
		Dashboard: models.Dashboard{
			Name:    "Database Health",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "DB Memory %", Type: "timeseries", Metric: "memory_percent"},
				{Title: "CPU (query load)", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Disk Usage", Type: "gauge", Metric: "disk_percent"},
				{Title: "Error Rate", Type: "timeseries", Metric: "error_rate"},
				{Title: "Stress (overall load)", Type: "gauge", KPI: "stress"},
				{Title: "Fatigue (sustained load)", Type: "timeseries", KPI: "fatigue"},
			},
		},
	},
	{
		ID:          "full-stack-prediction",
		Name:        "Full-Stack Prediction",
		Description: "Forecast for all major KPIs — stress, fatigue, resilience, entropy and health score",
		Tags:        []string{"prediction", "forecast", "ohe", "ml"},
		Category:    "Prediction",
		Icon:        "cpu",
		Dashboard: models.Dashboard{
			Name:    "Full-Stack Prediction",
			Refresh: 60,
			Widgets: []models.Widget{
				{Title: "Stress Forecast", Type: "prediction", KPI: "stress", Options: map[string]string{"horizon": "60"}},
				{Title: "Fatigue Forecast", Type: "prediction", KPI: "fatigue", Options: map[string]string{"horizon": "60"}},
				{Title: "Mood Forecast", Type: "prediction", KPI: "mood", Options: map[string]string{"horizon": "60"}},
				{Title: "Resilience Forecast", Type: "prediction", KPI: "resilience", Options: map[string]string{"horizon": "60"}},
				{Title: "Entropy Forecast", Type: "prediction", KPI: "entropy", Options: map[string]string{"horizon": "60"}},
				{Title: "Health Score Forecast", Type: "prediction", KPI: "health_score", Options: map[string]string{"horizon": "60"}},
			},
		},
	},
	{
		ID:          "sre-error-budget",
		Name:        "SRE Error Budget",
		Description: "Error budget consumption: error rate, timeout rate, mood degradation and humidity burn",
		Tags:        []string{"sre", "error-budget", "reliability"},
		Category:    "SRE",
		Icon:        "percent",
		Dashboard: models.Dashboard{
			Name:    "SRE Error Budget",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Error Rate", Type: "timeseries", Metric: "error_rate"},
				{Title: "Timeout Rate", Type: "timeseries", Metric: "timeout_rate"},
				{Title: "Error Humidity (compound)", Type: "gauge", KPI: "humidity"},
				{Title: "System Mood (SLA index)", Type: "timeseries", KPI: "mood"},
				{Title: "Contagion (error blast radius)", Type: "gauge", KPI: "contagion"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "devops-pipeline",
		Name:        "DevOps Pipeline Health",
		Description: "CI/CD pipeline proxy: velocity, entropy, deploy-triggered stress spikes",
		Tags:        []string{"devops", "cicd", "pipeline"},
		Category:    "Applications",
		Icon:        "git-branch",
		Dashboard: models.Dashboard{
			Name:    "DevOps Pipeline Health",
			Refresh: 20,
			Widgets: []models.Widget{
				{Title: "Change Velocity", Type: "timeseries", KPI: "velocity"},
				{Title: "System Entropy", Type: "timeseries", KPI: "entropy"},
				{Title: "Deploy Stress Spike", Type: "gauge", KPI: "stress"},
				{Title: "CPU (build load)", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory (build load)", Type: "timeseries", Metric: "memory_percent"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	// ── Extended Templates ────────────────────────────────────────────────────
	{
		ID:          "mlops-pipeline",
		Name:        "MLOps Pipeline",
		Description: "ML training/inference health — CPU saturation, memory pressure, model stress and prediction drift",
		Tags:        []string{"mlops", "ai", "ml", "inference"},
		Category:    "MLOps",
		Icon:        "brain",
		Dashboard: models.Dashboard{
			Name:    "MLOps Pipeline",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "CPU Saturation (training)", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory Pressure (model)", Type: "gauge", Metric: "memory_percent"},
				{Title: "GPU / Process Load", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "Model Stress Index", Type: "gauge", KPI: "stress"},
				{Title: "Inference Fatigue", Type: "timeseries", KPI: "fatigue"},
				{Title: "Prediction Drift Forecast", Type: "prediction", KPI: "entropy", Options: map[string]string{"horizon": "120"}},
				{Title: "Active Pipeline Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "api-gateway",
		Name:        "API Gateway",
		Description: "Request rate, latency proxy (load avg), error rate, saturation and SLA health for API-facing services",
		Tags:        []string{"api", "gateway", "latency", "sre"},
		Category:    "Applications",
		Icon:        "crosshair",
		Dashboard: models.Dashboard{
			Name:    "API Gateway",
			Refresh: 10,
			Widgets: []models.Widget{
				{Title: "Request Rate", Type: "timeseries", Metric: "request_rate"},
				{Title: "Error Rate", Type: "timeseries", Metric: "error_rate"},
				{Title: "Latency Proxy (Load Avg 1m)", Type: "timeseries", Metric: "load_avg_1"},
				{Title: "CPU Saturation", Type: "gauge", Metric: "cpu_percent"},
				{Title: "Error Humidity (compound)", Type: "gauge", KPI: "humidity"},
				{Title: "System Mood (SLA index)", Type: "kpi", KPI: "mood"},
				{Title: "Error Rate Forecast", Type: "prediction", Metric: "error_rate", Options: map[string]string{"horizon": "60"}},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "cache-redis",
		Name:        "Cache / Redis Health",
		Description: "Memory usage, eviction pressure, connection load and cache-stress KPIs for in-memory stores",
		Tags:        []string{"redis", "cache", "memory", "eviction"},
		Category:    "Applications",
		Icon:        "package",
		Dashboard: models.Dashboard{
			Name:    "Cache / Redis Health",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Memory Usage %", Type: "timeseries", Metric: "memory_percent"},
				{Title: "Memory Gauge", Type: "gauge", Metric: "memory_percent"},
				{Title: "Connection Load (CPU proxy)", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Eviction Stress", Type: "gauge", KPI: "stress"},
				{Title: "Pressure (eviction bursts)", Type: "timeseries", KPI: "pressure"},
				{Title: "Memory Forecast", Type: "prediction", Metric: "memory_percent", Options: map[string]string{"horizon": "60"}},
			},
		},
	},
	{
		ID:          "iot-edge",
		Name:        "IoT / Edge Nodes",
		Description: "Lightweight health telemetry for edge and IoT devices — CPU, memory, network and holistic KPIs",
		Tags:        []string{"iot", "edge", "embedded", "nodes"},
		Category:    "IoT",
		Icon:        "radio",
		Dashboard: models.Dashboard{
			Name:    "IoT / Edge Nodes",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "Node Health Score", Type: "kpi", KPI: "health_score"},
				{Title: "CPU %", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Memory %", Type: "gauge", Metric: "memory_percent"},
				{Title: "Network RX", Type: "timeseries", Metric: "net_rx_bps"},
				{Title: "Network TX", Type: "timeseries", Metric: "net_tx_bps"},
				{Title: "Node Stress", Type: "timeseries", KPI: "stress"},
				{Title: "Contagion (error spread)", Type: "gauge", KPI: "contagion"},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "microservices-mesh",
		Name:        "Microservices Mesh",
		Description: "Per-service health across the mesh — contagion spread, entropy, fatigue and request error rates",
		Tags:        []string{"microservices", "mesh", "service-health"},
		Category:    "Applications",
		Icon:        "terminal",
		Dashboard: models.Dashboard{
			Name:    "Microservices Mesh",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Fleet Health Score", Type: "gauge", KPI: "health_score"},
				{Title: "Contagion (blast radius)", Type: "timeseries", KPI: "contagion"},
				{Title: "System Entropy (churn)", Type: "timeseries", KPI: "entropy"},
				{Title: "Service Fatigue", Type: "gauge", KPI: "fatigue"},
				{Title: "Request Error Rate", Type: "timeseries", Metric: "error_rate"},
				{Title: "CPU Load", Type: "timeseries", Metric: "cpu_percent"},
				{Title: "Contagion Forecast", Type: "prediction", KPI: "contagion", Options: map[string]string{"horizon": "60"}},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	{
		ID:          "slo-compliance",
		Name:        "SLO Compliance",
		Description: "Error budget burn, SLA mood index, reliability resilience and forecasted SLO breach risk",
		Tags:        []string{"slo", "sla", "error-budget", "reliability"},
		Category:    "SRE",
		Icon:        "crosshair",
		Dashboard: models.Dashboard{
			Name:    "SLO Compliance",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "SLA Mood Index", Type: "kpi", KPI: "mood"},
				{Title: "Resilience Score", Type: "kpi", KPI: "resilience"},
				{Title: "Error Budget (Error Rate)", Type: "timeseries", Metric: "error_rate"},
				{Title: "Humidity (compound error)", Type: "gauge", KPI: "humidity"},
				{Title: "Contagion (SLO blast)", Type: "gauge", KPI: "contagion"},
				{Title: "Mood Forecast (SLA trend)", Type: "prediction", KPI: "mood", Options: map[string]string{"horizon": "120"}},
				{Title: "Resilience Forecast", Type: "prediction", KPI: "resilience", Options: map[string]string{"horizon": "120"}},
				{Title: "Active Alerts", Type: "alerts"},
			},
		},
	},
	// ── v4.4.0 Rich Templates ────────────────────────────────────────────────
	{
		ID:          "executive-health",
		Name:        "Executive Health Board",
		Description: "C-suite view: single health score, SRE vitals, forecasts, alert summary",
		Tags:        []string{"executive", "health", "summary"},
		Category:    "Executive",
		Icon:        "bar-chart-2",
		Dashboard: models.Dashboard{
			Name:    "Executive Health Board",
			Refresh: 60,
			Widgets: []models.Widget{
				{Title: "Health Score", Type: "gauge", KPI: "health_score", Width: 2, Height: 2},
				{Title: "Resilience", Type: "gauge", KPI: "resilience", Width: 1, Height: 2},
				{Title: "Entropy", Type: "gauge", KPI: "entropy", Width: 1, Height: 2},
				{Title: "Health Score Trend", Type: "timeseries", KPI: "health_score", Width: 2, Height: 1},
				{Title: "Stress vs Fatigue", Type: "timeseries", KPI: "stress", Width: 2, Height: 1},
				{Title: "Health Forecast 2h", Type: "prediction", KPI: "health_score", Width: 2, Height: 1, Options: map[string]string{"horizon": "120"}},
				{Title: "Resilience Forecast 2h", Type: "prediction", KPI: "resilience", Width: 2, Height: 1, Options: map[string]string{"horizon": "120"}},
				{Title: "Active Alerts", Type: "alerts", Width: 4, Height: 1},
			},
		},
	},
	{
		ID:          "capacity-planning",
		Name:        "Capacity Planning",
		Description: "CPU, memory, disk trend lines with 6h and 24h forecasts for capacity decisions",
		Tags:        []string{"capacity", "planning", "prediction"},
		Category:    "Infrastructure",
		Icon:        "database",
		Dashboard: models.Dashboard{
			Name:    "Capacity Planning",
			Refresh: 60,
			Widgets: []models.Widget{
				{Title: "CPU Now", Type: "stat", Metric: "cpu_percent", Width: 1, Height: 1},
				{Title: "Memory Now", Type: "stat", Metric: "memory_percent", Width: 1, Height: 1},
				{Title: "Disk Now", Type: "stat", Metric: "disk_percent", Width: 1, Height: 1},
				{Title: "Load Avg 1m", Type: "stat", Metric: "load_avg_1", Width: 1, Height: 1},
				{Title: "CPU Trend (7d)", Type: "timeseries", Metric: "cpu_percent", Width: 2, Height: 2},
				{Title: "Memory Trend (7d)", Type: "timeseries", Metric: "memory_percent", Width: 2, Height: 2},
				{Title: "CPU Forecast +24h", Type: "prediction", Metric: "cpu_percent", Width: 2, Height: 2, Options: map[string]string{"horizon": "1440"}},
				{Title: "Memory Forecast +24h", Type: "prediction", Metric: "memory_percent", Width: 2, Height: 2, Options: map[string]string{"horizon": "1440"}},
				{Title: "Disk Usage Forecast", Type: "prediction", Metric: "disk_percent", Width: 2, Height: 1, Options: map[string]string{"horizon": "360"}},
				{Title: "Velocity (rate of change)", Type: "timeseries", KPI: "velocity", Width: 2, Height: 1},
			},
		},
	},
	{
		ID:          "full-stack-app",
		Name:        "Full-Stack Application",
		Description: "Application health: error rate, latency, request rate, KPIs, predictions",
		Tags:        []string{"application", "apm", "error-rate", "latency"},
		Category:    "Application",
		Icon:        "layers",
		Dashboard: models.Dashboard{
			Name:    "Full-Stack Application",
			Refresh: 15,
			Widgets: []models.Widget{
				{Title: "Error Rate", Type: "gauge", Metric: "error_rate", Width: 1, Height: 2},
				{Title: "Stress Index", Type: "gauge", KPI: "stress", Width: 1, Height: 2},
				{Title: "Contagion", Type: "gauge", KPI: "contagion", Width: 1, Height: 2},
				{Title: "Health Score", Type: "gauge", KPI: "health_score", Width: 1, Height: 2},
				{Title: "Error Rate over Time", Type: "timeseries", Metric: "error_rate", Width: 2, Height: 2},
				{Title: "CPU Usage", Type: "timeseries", Metric: "cpu_percent", Width: 2, Height: 2},
				{Title: "Memory Usage", Type: "timeseries", Metric: "memory_percent", Width: 2, Height: 2},
				{Title: "Stress Forecast", Type: "prediction", KPI: "stress", Width: 2, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "Active Alerts", Type: "alerts", Width: 4, Height: 1},
			},
		},
	},
	{
		ID:          "network-monitor",
		Name:        "Network Monitor",
		Description: "Bandwidth, packet rates, RX/TX trends, contagion spread detection",
		Tags:        []string{"network", "bandwidth", "throughput"},
		Category:    "Infrastructure",
		Icon:        "wifi",
		Dashboard: models.Dashboard{
			Name:    "Network Monitor",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "RX Throughput", Type: "stat", Metric: "net_rx_bps", Width: 2, Height: 1},
				{Title: "TX Throughput", Type: "stat", Metric: "net_tx_bps", Width: 2, Height: 1},
				{Title: "RX Bytes/s Trend", Type: "timeseries", Metric: "net_rx_bps", Width: 2, Height: 2},
				{Title: "TX Bytes/s Trend", Type: "timeseries", Metric: "net_tx_bps", Width: 2, Height: 2},
				{Title: "Contagion Index", Type: "timeseries", KPI: "contagion", Width: 2, Height: 2},
				{Title: "Entropy Level", Type: "timeseries", KPI: "entropy", Width: 2, Height: 2},
				{Title: "RX Forecast +1h", Type: "prediction", Metric: "net_rx_bps", Width: 2, Height: 1, Options: map[string]string{"horizon": "60"}},
				{Title: "TX Forecast +1h", Type: "prediction", Metric: "net_tx_bps", Width: 2, Height: 1, Options: map[string]string{"horizon": "60"}},
			},
		},
	},
	{
		ID:          "sre-golden-signals",
		Name:        "SRE Golden Signals",
		Description: "Latency, traffic, errors, saturation — the four SRE golden signals mapped to OHE KPIs",
		Tags:        []string{"sre", "golden-signals", "reliability"},
		Category:    "SRE",
		Icon:        "activity",
		Dashboard: models.Dashboard{
			Name:    "SRE Golden Signals",
			Refresh: 15,
			Widgets: []models.Widget{
				// Saturation → pressure/fatigue
				{Title: "Saturation (Pressure)", Type: "gauge", KPI: "pressure", Width: 1, Height: 2},
				// Errors → error_rate/contagion
				{Title: "Error Signal (Contagion)", Type: "gauge", KPI: "contagion", Width: 1, Height: 2},
				// Latency proxy → stress
				{Title: "Latency Proxy (Stress)", Type: "gauge", KPI: "stress", Width: 1, Height: 2},
				// Availability → health_score
				{Title: "Availability (Health)", Type: "gauge", KPI: "health_score", Width: 1, Height: 2},
				{Title: "Error Rate", Type: "timeseries", Metric: "error_rate", Width: 2, Height: 2},
				{Title: "CPU Saturation", Type: "timeseries", Metric: "cpu_percent", Width: 2, Height: 2},
				{Title: "Stress Trend", Type: "timeseries", KPI: "stress", Width: 2, Height: 2},
				{Title: "Pressure Trend", Type: "timeseries", KPI: "pressure", Width: 2, Height: 2},
				{Title: "Active Alerts", Type: "alerts", Width: 4, Height: 1},
			},
		},
	},
	{
		ID:          "ml-predictions-panorama",
		Name:        "Predictions Panorama",
		Description: "Full forecasting view: all major KPIs predicted 1h and 6h ahead",
		Tags:        []string{"prediction", "forecast", "ml"},
		Category:    "ML / Forecast",
		Icon:        "trending-up",
		Dashboard: models.Dashboard{
			Name:    "Predictions Panorama",
			Refresh: 30,
			Widgets: []models.Widget{
				{Title: "Health Score +1h", Type: "prediction", KPI: "health_score", Width: 2, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "Health Score +6h", Type: "prediction", KPI: "health_score", Width: 2, Height: 2, Options: map[string]string{"horizon": "360"}},
				{Title: "Stress +1h", Type: "prediction", KPI: "stress", Width: 1, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "Fatigue +1h", Type: "prediction", KPI: "fatigue", Width: 1, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "CPU +1h", Type: "prediction", Metric: "cpu_percent", Width: 1, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "Memory +1h", Type: "prediction", Metric: "memory_percent", Width: 1, Height: 2, Options: map[string]string{"horizon": "60"}},
				{Title: "Resilience +6h", Type: "prediction", KPI: "resilience", Width: 2, Height: 2, Options: map[string]string{"horizon": "360"}},
				{Title: "Entropy +6h", Type: "prediction", KPI: "entropy", Width: 2, Height: 2, Options: map[string]string{"horizon": "360"}},
			},
		},
	},
}

// TemplateListHandler GET /api/v1/templates
func (h *Handlers) TemplateListHandler(w http.ResponseWriter, r *http.Request) {
	type entry struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Category    string   `json:"category"`
		Icon        string   `json:"icon"`
		WidgetCount int      `json:"widget_count"`
	}
	list := make([]entry, 0, len(builtinTemplates))
	for _, t := range builtinTemplates {
		list = append(list, entry{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Tags:        t.Tags,
			Category:    t.Category,
			Icon:        t.Icon,
			WidgetCount: len(t.Dashboard.Widgets),
		})
	}
	respondSuccess(w, list)
}

// TemplateGetHandler GET /api/v1/templates/{id}
func (h *Handlers) TemplateGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	for _, t := range builtinTemplates {
		if t.ID == id {
			respondSuccess(w, t)
			return
		}
	}
	respondError(w, http.StatusNotFound, "NOT_FOUND", "template not found")
}

// TemplateApplyHandler POST /api/v1/templates/{id}/apply — instantiates a template as a new dashboard
// Body: {"name": "optional override", "mode": "current"|"predicted"}
func (h *Handlers) TemplateApplyHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var tmpl *dashboardTemplate
	for i := range builtinTemplates {
		if builtinTemplates[i].ID == id {
			tmpl = &builtinTemplates[i]
			break
		}
	}
	if tmpl == nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "template not found")
		return
	}

	var opts struct {
		Name string `json:"name"`
		Mode string `json:"mode"` // "current" (default) or "predicted"
	}
	_ = decodeBody(r, &opts)

	d := tmpl.Dashboard
	d.ID = utils.GenerateID(8)
	d.CreatedAt = time.Now()
	d.UpdatedAt = d.CreatedAt
	if opts.Name != "" {
		d.Name = opts.Name
	}
	// In "predicted" mode, swap timeseries/gauge/stat widgets for prediction widgets
	if opts.Mode == "predicted" {
		d.Widgets = widgetsToPrediction(d.Widgets)
		if opts.Name == "" {
			d.Name = d.Name + " (Predicted)"
		}
	}

	if err := h.orgStore(r).SaveDashboard(d.ID, d); err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": d, "timestamp": time.Now().UTC(),
	})
}

// --- Helpers ---

func (h *Handlers) buildMetricsMap(host string) map[string]float64 {
	names := []string{
		"cpu_percent", "memory_percent", "disk_percent",
		"load_avg_1", "error_rate", "timeout_rate",
		"request_rate", "uptime_seconds",
	}
	m := make(map[string]float64, len(names))
	for _, name := range names {
		if v, ok := h.processor.GetNormalized(host, name); ok {
			m[name] = v
		}
	}
	return m
}

func parseTimeRange(r *http.Request) (from, to time.Time) {
	now := time.Now()
	to = now

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		} else {
			// Handle relative offsets: -5m, -1h, -6h, -24h, -7d, -30d
			from = parseRelativeOffset(fromStr, now)
		}
	}
	if from.IsZero() {
		from = now.Add(-time.Hour)
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}
	return from, to
}

// parseRelativeOffset parses strings like "-5m", "-1h", "-6h", "-24h", "-7d", "-30d"
func parseRelativeOffset(s string, now time.Time) time.Time {
	if len(s) < 2 || s[0] != '-' {
		return time.Time{}
	}
	rest := s[1:]
	if len(rest) < 2 {
		return time.Time{}
	}
	unit := rest[len(rest)-1]
	numStr := rest[:len(rest)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	switch unit {
	case 'm':
		return now.Add(-time.Duration(n) * time.Minute)
	case 'h':
		return now.Add(-time.Duration(n) * time.Hour)
	case 'd':
		return now.Add(-time.Duration(n) * 24 * time.Hour)
	}
	return time.Time{}
}

func decodeBody(r *http.Request, dest interface{}) error {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, 10<<20)) // 10MB limit
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// trustedDatasourceHosts returns hosts/IPs explicitly trusted via
// OHE_TRUSTED_DATASOURCE_HOSTS (comma-separated). Intended for in-cluster
// ClusterIPs when cluster DNS is unavailable.
func trustedDatasourceHosts() map[string]struct{} {
	m := make(map[string]struct{})
	for _, h := range strings.Split(os.Getenv("OHE_TRUSTED_DATASOURCE_HOSTS"), ",") {
		if h = strings.TrimSpace(h); h != "" {
			m[h] = struct{}{}
		}
	}
	return m
}

// isClusterInternalHost returns true for Kubernetes in-cluster service DNS names
// (e.g. prometheus.monitoring.svc.cluster.local, prometheus.monitoring.svc).
// These are trusted because they can only resolve inside the cluster network.
func isClusterInternalHost(host string) bool {
	for _, suffix := range []string{".svc.cluster.local", ".svc"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// validateDataSourceURL enforces scheme allowlist and blocks SSRF targets.
// Kubernetes cluster-internal DNS names and hosts in OHE_TRUSTED_DATASOURCE_HOSTS
// are explicitly allowed.
func validateDataSourceURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme not allowed: %s", u.Scheme)
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		host = u.Host
	}
	if isClusterInternalHost(host) {
		return nil
	}
	if trusted := trustedDatasourceHosts(); len(trusted) > 0 {
		if _, ok := trusted[host]; ok {
			return nil
		}
	}
	addrs, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("hostname resolution failed")
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("target IP is in a private/reserved range")
		}
		for _, blocked := range []string{"169.254.169.254", "metadata.google.internal"} {
			if addr == blocked || host == blocked {
				return fmt.Errorf("metadata endpoint not allowed")
			}
		}
	}
	return nil
}

// sanitizeKey replaces characters that corrupt Badger key namespacing
var keyUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._\-]`)

func sanitizeKey(s string) string {
	return keyUnsafe.ReplaceAllString(s, "_")
}

// validateUsername enforces safe username characters
func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("username is required")
	}
	if len(username) > 64 {
		return fmt.Errorf("username too long")
	}
	if strings.ContainsAny(username, ":/\\") {
		return fmt.Errorf("username contains invalid characters")
	}
	return nil
}

