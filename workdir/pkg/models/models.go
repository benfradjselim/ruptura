package models

import (
	"fmt"
	"time"
)

// Validatable defines an interface for request payload validation.
type Validatable interface {
	Validate() error
}

// Metric represents a single time-series data point
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
	Host      string            `json:"host"`
	Workload  WorkloadRef       `json:"workload,omitempty"` // K8s workload identity
}

// MetricBatch is a collection of metrics sent by an agent
type MetricBatch struct {
	AgentID   string    `json:"agent_id"`
	Host      string    `json:"host"`
	Metrics   []Metric  `json:"metrics"`
	Timestamp time.Time `json:"timestamp"`
}

// KPI represents a computed composite KPI
type KPI struct {
	Name      string      `json:"name"`
	Value     float64     `json:"value"`
	State     string      `json:"state"`
	Timestamp time.Time   `json:"timestamp"`
	Host      string      `json:"host"`
	Workload  WorkloadRef `json:"workload,omitempty"` // K8s workload identity
}

// KPISnapshot holds all current KPIs for a host
type KPISnapshot struct {
	Host        string      `json:"host"`
	Workload    WorkloadRef `json:"workload,omitempty"` // K8s workload identity
	Timestamp   time.Time   `json:"timestamp"`
	Stress      KPI         `json:"stress"`
	Fatigue     KPI         `json:"fatigue"`
	Mood        KPI         `json:"mood"`
	Pressure    KPI         `json:"pressure"`
	Humidity    KPI         `json:"humidity"`
	Contagion   KPI         `json:"contagion"`
	// ETF-style composed KPIs
	Resilience  KPI `json:"resilience"`   // ability to absorb disruption
	Entropy     KPI `json:"entropy"`      // system disorder level
	Velocity    KPI `json:"velocity"`     // rate of change (momentum)
	HealthScore KPI `json:"health_score"` // single composite executive KPI [0-100]
	// v6.1: throughput collapse signal
	Throughput KPI `json:"throughput"`
	// v6.2: fused rupture index (metric R + log R + trace R combined)
	FusedRuptureIndex float64 `json:"fused_rupture_index,omitempty"`
	// v5.0: dual-scale CA-ILR rupture events (omitted when none detected)
	RuptureEvents []RuptureEvent `json:"rupture_events,omitempty"`
	// v6.3: calibration warm-up state
	WorkloadStatus      string `json:"status"`                                 // "calibrating" | "active"
	CalibrationProgress int    `json:"calibration_progress"`                    // 0–100
	CalibrationETA      int    `json:"calibration_eta_minutes,omitempty"`        // minutes until active; 0 when active
	// v6.3: HealthScore trend forecast (nil when calibrating or insufficient data)
	HealthForecast *HealthForecast `json:"health_forecast,omitempty"`
	// v6.4: rupture fingerprint pattern match (nil when no historical match found)
	PatternMatch *PatternMatch `json:"pattern_match,omitempty"`
	// v6.4: business-layer signals
	Business *BusinessSignals `json:"business,omitempty"`
}

// RuptureFingerprint is the 11-dimensional KPI signal vector captured at a confirmed
// rupture (FusedR > 3.0). Used by the fingerprint engine for cosine similarity matching.
type RuptureFingerprint struct {
	ID          string      `json:"id"`
	WorkloadKey string      `json:"workload_key"`
	CapturedAt  time.Time   `json:"captured_at"`
	Vector      [11]float64 `json:"vector"` // [stress, fatigue, 1-mood, pressure, humidity, contagion, 1-resilience, entropy, velocity, throughput, fusedR/10]
	FusedR      float64     `json:"fused_r"`
	Resolution  string      `json:"resolution,omitempty"` // how the rupture was resolved
}

// PatternMatch indicates the current snapshot resembles a past rupture fingerprint.
type PatternMatch struct {
	Similarity       float64   `json:"similarity"`
	MatchedRuptureID string    `json:"matched_rupture_id"`
	MatchedAt        time.Time `json:"matched_at"`
	Resolution       string    `json:"resolution,omitempty"`
}

// SLOConfig defines the error budget contract for a workload.
type SLOConfig struct {
	TargetPercent      float64 `json:"target_percent"`       // e.g. 99.9
	WindowDays         int     `json:"window_days"`          // e.g. 30
	ErrorBudgetMinutes float64 `json:"error_budget_minutes"` // e.g. 43.2
}

// BusinessSignals holds P1 business-layer KPIs (v6.4).
type BusinessSignals struct {
	// SLOBurnVelocity is the ratio of current error rate to the allowed error rate.
	// 1.0 = exactly on budget, > 1.0 = burning too fast. 0 = no SLO configured.
	SLOBurnVelocity float64 `json:"slo_burn_velocity,omitempty"`
	// BlastRadius is the count of unique downstream workloads that depend on this one.
	BlastRadius int `json:"blast_radius"`
	// RecoveryDebt is the count of near-misses (FusedR 2–3, recovered without rupture) in the last 7 days.
	RecoveryDebt int `json:"recovery_debt"`
}

// HealthForecast is a lightweight linear projection of HealthScore trend.
type HealthForecast struct {
	Trend              string  `json:"trend"`                          // "stable" | "improving" | "degrading"
	In15Min            float64 `json:"in_15min"`                       // projected HealthScore (0–100) in 15 minutes
	In30Min            float64 `json:"in_30min"`                       // projected HealthScore (0–100) in 30 minutes
	CriticalETAMinutes int     `json:"critical_eta_minutes,omitempty"` // minutes until HealthScore < 40; 0 if not degrading to critical
}

// NotificationChannel is a configured alert delivery target
type NotificationChannel struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     string            `json:"type"`    // "webhook", "slack", "pagerduty"
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	Enabled  bool              `json:"enabled"`
	// Routing: only fire for severities in this list (empty = all)
	Severities []string        `json:"severities,omitempty"`
}

// QuotaConfig defines resource limits for an org. Zero values mean unlimited.
type QuotaConfig struct {
	MaxDashboards  int `json:"max_dashboards,omitempty"`   // 0 = unlimited
	MaxDataSources int `json:"max_datasources,omitempty"`  // 0 = unlimited
	MaxAPIKeys     int `json:"max_api_keys,omitempty"`     // 0 = unlimited
	MaxAlertRules  int `json:"max_alert_rules,omitempty"`  // 0 = unlimited
	MaxSLOs        int `json:"max_slos,omitempty"`         // 0 = unlimited
	IngestRateRPM  int `json:"ingest_rate_rpm,omitempty"`  // requests/min; 0 = unlimited
}

// DefaultQuota returns sensible limits for a free-tier org.
func DefaultQuota() QuotaConfig {
	return QuotaConfig{
		MaxDashboards:  10,
		MaxDataSources: 5,
		MaxAPIKeys:     5,
		MaxAlertRules:  20,
		MaxSLOs:        5,
		IngestRateRPM:  300,
	}
}

// Org is a tenant workspace. Resources (dashboards, datasources, etc.) can be
// scoped to an org. The built-in "default" org always exists and holds legacy
// resources that predate multi-tenancy.
type Org struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`            // URL-safe identifier
	Description string      `json:"description,omitempty"`
	Quota       QuotaConfig `json:"quota"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// OrgMember binds a user to an org with a role.
type OrgMember struct {
	OrgID    string `json:"org_id"`
	Username string `json:"username"`
	Role     string `json:"role"` // "admin" | "editor" | "viewer"
}

// FleetStatus is the aggregated health summary for all known hosts
type FleetStatus struct {
	Timestamp    time.Time              `json:"timestamp"`
	TotalHosts   int                    `json:"total_hosts"`
	HealthyHosts int                    `json:"healthy_hosts"`
	DegradedHosts int                   `json:"degraded_hosts"`
	CriticalHosts int                   `json:"critical_hosts"`
	Hosts        []HostSummary          `json:"hosts"`
}

// HostSummary is a lightweight per-host health view for fleet overview
type HostSummary struct {
	Host        string    `json:"host"`
	HealthScore float64   `json:"health_score"`
	State       string    `json:"state"`   // "healthy", "degraded", "critical"
	Stress      float64   `json:"stress"`
	Fatigue     float64   `json:"fatigue"`
	Contagion   float64   `json:"contagion"`
	ActiveAlerts int      `json:"active_alerts"`
	LastSeen    time.Time `json:"last_seen"`
}

// ModelContribution is a single model's contribution inside an ensemble forecast
type ModelContribution struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Mean   float64 `json:"mean"`
}

// ForecastPoint is a single step in a multi-step forecast
type ForecastPoint struct {
	OffsetMinutes int     `json:"offset_minutes"`
	Mean          float64 `json:"mean"`
	Lower80       float64 `json:"lower_80"`
	Upper80       float64 `json:"upper_80"`
	Lower95       float64 `json:"lower_95"`
	Upper95       float64 `json:"upper_95"`
}

// ForecastResult is the full ensemble forecast response
type ForecastResult struct {
	Host      string              `json:"host"`
	Metric    string              `json:"metric"`
	Current   float64             `json:"current"`
	Trend     string              `json:"trend"`
	Confidence float64            `json:"confidence"`
	Models    []ModelContribution `json:"models"`
	Points    []ForecastPoint     `json:"points"`
	Timestamp time.Time           `json:"timestamp"`
	WarmingUp bool                `json:"warming_up,omitempty"`
}

// Prediction is a forecasted value for a metric/KPI
type Prediction struct {
	Target    string    `json:"target"`
	Current   float64   `json:"current"`
	Predicted float64   `json:"predicted"`
	Horizon   int       `json:"horizon_minutes"`
	Trend     string    `json:"trend"` // "rising", "stable", "falling"
	Timestamp time.Time `json:"timestamp"`
	// v4.5.0: confidence intervals and ensemble metadata (additive, backwards-compatible)
	Lower80    float64            `json:"lower_80,omitempty"`
	Upper80    float64            `json:"upper_80,omitempty"`
	Lower95    float64            `json:"lower_95,omitempty"`
	Upper95    float64            `json:"upper_95,omitempty"`
	Confidence float64            `json:"confidence,omitempty"`
	Method     string             `json:"method,omitempty"`
	Models     []ModelContribution `json:"models,omitempty"`
}

// AnomalyEvent is a detected anomaly from the multi-method engine
type AnomalyEvent struct {
	Host      string    `json:"host"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Expected  float64   `json:"expected"`
	Score     float64   `json:"score"`
	Method    string    `json:"method"` // "zscore" | "mad" | "seasonal"
	Severity  string    `json:"severity"`
	Timestamp time.Time `json:"timestamp"`
}

// BurstEvent records a detected log error burst
type BurstEvent struct {
	ID           string    `json:"id"`
	Service      string    `json:"service"`
	StartTS      time.Time `json:"start_ts"`
	EndTS        time.Time `json:"end_ts"`
	Count        int64     `json:"count"`
	BaselineRate float64   `json:"baseline_rate"`
	Level        string    `json:"level"` // "error" | "warn"
}

// CorrelationEvent links a log burst to a KPI degradation
type CorrelationEvent struct {
	ID         string    `json:"id"`
	Host       string    `json:"host"`
	BurstID    string    `json:"burst_id"`
	AlertID    string    `json:"alert_id,omitempty"`
	KPIName    string    `json:"kpi_name"`
	KPIDelta   float64   `json:"kpi_delta"`
	Confidence float64   `json:"confidence"`
	CreatedAt  time.Time `json:"created_at"`
}

// AlertGroup groups related alerts by topology dependency
type AlertGroup struct {
	ID             string    `json:"id"`
	Representative string    `json:"representative_id"`
	AlertIDs       []string  `json:"alert_ids"`
	SuppressedIDs  []string  `json:"suppressed_ids"`
	CreatedAt      time.Time `json:"created_at"`
}

// CostSnapshot is a point-in-time cost attribution record
type CostSnapshot struct {
	OrgID          string    `json:"org_id"`
	Team           string    `json:"team"`
	Service        string    `json:"service"`
	Env            string    `json:"env"`
	PointsIngested int64     `json:"points_ingested"`
	BytesStored    int64     `json:"bytes_stored"`
	QueriesServed  int64     `json:"queries_served"`
	CostUSD        float64   `json:"cost_usd"`
	Period         time.Time `json:"period"` // start of hour
}

// FatigueConfig holds the parameters for dissipative fatigue (v5.0).
// Defaults applied when zero-value: RThreshold=0.3, Lambda=0.05.
type FatigueConfig struct {
	RThreshold float64 `yaml:"r_threshold" json:"r_threshold"` // rest threshold (default 0.3)
	Lambda     float64 `yaml:"lambda"      json:"lambda"`      // recovery coefficient per 15s interval (default 0.05)
}

// DefaultFatigueConfig returns the canonical v5.0 defaults.
func DefaultFatigueConfig() FatigueConfig {
	return FatigueConfig{RThreshold: 0.3, Lambda: 0.05}
}

// RuptureEvent records a detected acceleration event from the dual-scale CA-ILR.
type RuptureEvent struct {
	Host         string    `json:"host"`
	Metric       string    `json:"metric"`
	RuptureIndex float64   `json:"rupture_index"` // α_burst / α_stable
	AlphaStable  float64   `json:"alpha_stable"`
	AlphaBurst   float64   `json:"alpha_burst"`
	Timestamp    time.Time `json:"timestamp"`
}

// Alert severity levels
const (
	SeverityInfo      = "info"
	SeverityWarning   = "warning"
	SeverityCritical  = "critical"
	SeverityEmergency = "emergency"
)

// Alert status
const (
	StatusActive       = "active"
	StatusAcknowledged = "acknowledged"
	StatusSilenced     = "silenced"
	StatusResolved     = "resolved"
)

// Alert represents a triggered observability alert
type Alert struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Severity       string    `json:"severity"`
	Status         string    `json:"status"`
	Host           string    `json:"host"`
	Metric         string    `json:"metric"`
	Value          float64   `json:"value"`
	Threshold      float64   `json:"threshold"`
	Prediction     string    `json:"prediction,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	// v4.5.0: correlation and grouping metadata
	CorrelationIDs []string  `json:"correlation_ids,omitempty"`
	Suppressed     bool      `json:"suppressed,omitempty"`
	SuppressedBy   string    `json:"suppressed_by,omitempty"`
	GroupID        string    `json:"group_id,omitempty"`
}

// User for auth
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // bcrypt hash — omitempty ensures it is never serialised when cleared
	Role     string `json:"role"`               // admin, viewer, operator
	OrgID    string `json:"org_id,omitempty"`   // "" = default org (global)
}

// Dashboard configuration
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []Widget  `json:"widgets"`
	Refresh     int       `json:"refresh_seconds"`
	OrgID       string    `json:"org_id,omitempty"` // "" = default org (global)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SLO defines a Service Level Objective
type SLO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Metric      string    `json:"metric"`            // metric or KPI name to track
	Target      float64   `json:"target"`            // e.g. 99.9 (percent)
	Window      string    `json:"window"`            // "7d", "30d", "90d"
	Comparator  string    `json:"comparator"`        // "gte" (value >= threshold = good) | "lte"
	Threshold   float64   `json:"threshold"`         // value that defines "good" state
	OrgID       string    `json:"org_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SLOStatus is the live computed state of an SLO
type SLOStatus struct {
	SLO            SLO     `json:"slo"`
	ErrorBudget    float64 `json:"error_budget_pct"`    // remaining % of allowed bad minutes
	BurnRate       float64 `json:"burn_rate"`            // current burn rate (1 = steady-state)
	Compliance     float64 `json:"compliance_pct"`       // actual uptime % over window
	RemainingMinutes float64 `json:"remaining_minutes"`  // minutes of budget left
	State          string  `json:"state"`                // "healthy" | "at_risk" | "breached"
}

// RetentionStats exposes storage tier counts
type RetentionStats struct {
	RawMetrics    int64 `json:"raw_metrics"`
	RawKPIs       int64 `json:"raw_kpis"`
	Rollup5mMetrics int64 `json:"rollup_5m_metrics"`
	Rollup5mKPIs  int64 `json:"rollup_5m_kpis"`
	Rollup1hMetrics int64 `json:"rollup_1h_metrics"`
	Rollup1hKPIs  int64 `json:"rollup_1h_kpis"`
}

// Widget types
const (
	WidgetTypeTimeseries = "timeseries"
	WidgetTypeGauge      = "gauge"
	WidgetTypeKPI        = "kpi"
	WidgetTypeStat       = "stat"
	WidgetTypeAlerts     = "alerts"
)

// Widget is a dashboard panel
type Widget struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Metric      string            `json:"metric,omitempty"`
	KPI         string            `json:"kpi,omitempty"`
	Aggregation string            `json:"aggregation,omitempty"` // avg, min, max, p95, p99
	From        string            `json:"from,omitempty"`        // relative: -1h, -24h
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Options     map[string]string `json:"options,omitempty"`
}

// DataSource represents an external data source
type DataSource struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"` // prometheus, loki, custom
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Enabled bool              `json:"enabled"`
	OrgID   string            `json:"org_id,omitempty"` // "" = default org (global)
}

// SystemMetrics holds all raw collected system metrics
type SystemMetrics struct {
	Host         string    `json:"host"`
	Timestamp    time.Time `json:"timestamp"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemoryPercent float64  `json:"memory_percent"`
	MemoryUsedMB float64   `json:"memory_used_mb"`
	MemoryTotalMB float64  `json:"memory_total_mb"`
	DiskPercent  float64   `json:"disk_percent"`
	DiskUsedGB   float64   `json:"disk_used_gb"`
	DiskTotalGB  float64   `json:"disk_total_gb"`
	NetRxBps     float64   `json:"net_rx_bps"`
	NetTxBps     float64   `json:"net_tx_bps"`
	LoadAvg1     float64   `json:"load_avg_1"`
	LoadAvg5     float64   `json:"load_avg_5"`
	LoadAvg15    float64   `json:"load_avg_15"`
	Processes    int       `json:"processes"`
	Uptime       float64   `json:"uptime_seconds"`
}

// APIResponse wraps all API responses
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     *APIError   `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// APIError contains error details
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// QueryRequest for QQL queries
type QueryRequest struct {
	Query string    `json:"query"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Step  int       `json:"step_seconds"`
}

// QueryResult holds query results
type QueryResult struct {
	Metric string      `json:"metric"`
	Points []DataPoint `json:"points"`
}

// DataPoint is a single time-value pair
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// LoginRequest for auth endpoint
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (r *LoginRequest) Validate() error {
	if r.Username == "" {
		return fmt.Errorf("username is required")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

// LoginResponse contains JWT token
type LoginResponse struct {
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
	User    User   `json:"user"`
}

// HealthResponse for health check
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Host      string            `json:"host"`
	Uptime    float64           `json:"uptime_seconds"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// APIKey represents a long-lived API key for programmatic access.
// The full key (prefix + secret) is only returned once at creation time;
// only the bcrypt hash is persisted so the server cannot reconstruct it.
type APIKey struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`        // human-readable label
	Role      string    `json:"role"`        // role granted (viewer/operator/admin)
	KeyHash   string    `json:"key_hash"`    // bcrypt hash of the full key — never returned to clients
	Prefix    string    `json:"prefix"`      // first 8 chars of key for display (e.g. "ohe_a1b2")
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`  // zero = no expiry
	LastUsed  time.Time `json:"last_used"`
	Active    bool      `json:"active"`
}

// APIKeyCreateRequest is the request body for POST /api/v1/api-keys.
type APIKeyCreateRequest struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	ExpiresIn string `json:"expires_in"` // duration string e.g. "30d", "90d", "" = never
}

func (r *APIKeyCreateRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Role == "" {
		return fmt.Errorf("role is required")
	}
	return nil
}

// APIKeyCreateResponse is returned once at creation — includes the full plaintext key.
type APIKeyCreateResponse struct {
	APIKey
	PlaintextKey string `json:"key"` // only present in create response; store it — it won't be shown again
}
