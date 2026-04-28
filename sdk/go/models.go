package ruptura

import "time"

// HealthResponse is returned by Health().
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Host      string            `json:"host"`
	Uptime    float64           `json:"uptime_seconds"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// LoginRequest holds credentials for Login().
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is returned by Login().
type LoginResponse struct {
	Token   string `json:"token"`
	Expires int64  `json:"expires"`
	User    User   `json:"user"`
}

// User represents an OHE user.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	OrgID    string `json:"org_id,omitempty"`
}

// Metric is a single time-series data point.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Timestamp time.Time         `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
	Host      string            `json:"host"`
}

// DataPoint is a single timestamp/value pair in a range query.
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// KPISnapshot is the set of computed KPIs for one host.
type KPISnapshot struct {
	Host          string         `json:"host"`
	Timestamp     time.Time      `json:"timestamp"`
	Stress        KPI            `json:"stress"`
	Fatigue       KPI            `json:"fatigue"`
	Mood          KPI            `json:"mood"`
	Pressure      KPI            `json:"pressure"`
	Humidity      KPI            `json:"humidity"`
	Contagion     KPI            `json:"contagion"`
	Resilience    KPI            `json:"resilience"`
	Entropy       KPI            `json:"entropy"`
	Velocity      KPI            `json:"velocity"`
	HealthScore   KPI            `json:"health_score"`
	RuptureEvents []RuptureEvent `json:"rupture_events,omitempty"`
}

// KPI is one named indicator inside a KPISnapshot.
type KPI struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	State     string    `json:"state"`
	Timestamp time.Time `json:"timestamp"`
	Host      string    `json:"host"`
}

// RuptureEvent is a dual-scale CA-ILR acceleration event.
type RuptureEvent struct {
	Host         string    `json:"host"`
	Metric       string    `json:"metric"`
	RuptureIndex float64   `json:"rupture_index"`
	AlphaStable  float64   `json:"alpha_stable"`
	AlphaBurst   float64   `json:"alpha_burst"`
	Timestamp    time.Time `json:"timestamp"`
}

// Prediction is an ensemble forecast for a metric or KPI.
type Prediction struct {
	Target     string               `json:"target"`
	Current    float64              `json:"current"`
	Predicted  float64              `json:"predicted"`
	Horizon    int                  `json:"horizon_minutes"`
	Trend      string               `json:"trend"`
	Timestamp  time.Time            `json:"timestamp"`
	Lower80    float64              `json:"lower_80,omitempty"`
	Upper80    float64              `json:"upper_80,omitempty"`
	Lower95    float64              `json:"lower_95,omitempty"`
	Upper95    float64              `json:"upper_95,omitempty"`
	Confidence float64              `json:"confidence,omitempty"`
	Method     string               `json:"method,omitempty"`
	Models     []ModelContribution  `json:"models,omitempty"`
}

// ModelContribution is one model's weight inside an ensemble.
type ModelContribution struct {
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
	Mean   float64 `json:"mean"`
}

// Alert represents a triggered observability alert.
type Alert struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Severity       string     `json:"severity"`
	Status         string     `json:"status"`
	Host           string     `json:"host"`
	Metric         string     `json:"metric"`
	Value          float64    `json:"value"`
	Threshold      float64    `json:"threshold"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	CorrelationIDs []string   `json:"correlation_ids,omitempty"`
	Suppressed     bool       `json:"suppressed,omitempty"`
	GroupID        string     `json:"group_id,omitempty"`
}

// AlertRule is a persistent alert rule configuration.
type AlertRule struct {
	Name        string  `json:"name"`
	Metric      string  `json:"metric"`
	Condition   string  `json:"condition"`
	Threshold   float64 `json:"threshold"`
	Severity    string  `json:"severity"`
	Description string  `json:"description,omitempty"`
	Enabled     bool    `json:"enabled"`
}

// Dashboard is a saved observability dashboard.
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []Widget  `json:"widgets"`
	Refresh     int       `json:"refresh_seconds"`
	OrgID       string    `json:"org_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Widget is a panel inside a Dashboard.
type Widget struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Metric      string            `json:"metric,omitempty"`
	KPI         string            `json:"kpi,omitempty"`
	Aggregation string            `json:"aggregation,omitempty"`
	From        string            `json:"from,omitempty"`
	Width       int               `json:"width"`
	Height      int               `json:"height"`
	Options     map[string]string `json:"options,omitempty"`
}

// SLO is a Service Level Objective definition.
type SLO struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Metric      string    `json:"metric"`
	Target      float64   `json:"target"`
	Window      string    `json:"window"`
	Comparator  string    `json:"comparator"`
	Threshold   float64   `json:"threshold"`
	OrgID       string    `json:"org_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SLOStatus is the live compliance state of an SLO.
type SLOStatus struct {
	SLO              SLO     `json:"slo"`
	ErrorBudget      float64 `json:"error_budget_pct"`
	BurnRate         float64 `json:"burn_rate"`
	Compliance       float64 `json:"compliance_pct"`
	RemainingMinutes float64 `json:"remaining_minutes"`
	State            string  `json:"state"`
}

// Org is a tenant workspace.
type Org struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Description string      `json:"description,omitempty"`
	Quota       QuotaConfig `json:"quota"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// QuotaConfig defines resource limits for an Org.
type QuotaConfig struct {
	MaxDashboards  int `json:"max_dashboards,omitempty"`
	MaxDataSources int `json:"max_datasources,omitempty"`
	MaxAPIKeys     int `json:"max_api_keys,omitempty"`
	MaxAlertRules  int `json:"max_alert_rules,omitempty"`
	MaxSLOs        int `json:"max_slos,omitempty"`
	IngestRateRPM  int `json:"ingest_rate_rpm,omitempty"`
}

// OrgMember binds a user to an org with a role.
type OrgMember struct {
	OrgID    string `json:"org_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// APIKey is a long-lived programmatic access token.
type APIKey struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Prefix    string    `json:"prefix"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastUsed  time.Time `json:"last_used"`
	Active    bool      `json:"active"`
}

// APIKeyCreateRequest is the body for CreateAPIKey().
type APIKeyCreateRequest struct {
	Name      string `json:"name"`
	Role      string `json:"role"`
	ExpiresIn string `json:"expires_in,omitempty"`
}

// APIKeyCreateResponse is returned once; contains the plaintext key.
type APIKeyCreateResponse struct {
	APIKey
	PlaintextKey string `json:"key"`
}

// IngestRequest pushes raw metrics to OHE.
type IngestRequest struct {
	AgentID   string    `json:"agent_id"`
	Host      string    `json:"host"`
	Metrics   []Metric  `json:"metrics"`
	Timestamp time.Time `json:"timestamp"`
}

// QueryRequest is the body for QQL queries.
type QueryRequest struct {
	Query string    `json:"query"`
	From  time.Time `json:"from"`
	To    time.Time `json:"to"`
	Step  int       `json:"step_seconds"`
}

// QueryResult holds QQL query results.
type QueryResult struct {
	Metric string      `json:"metric"`
	Points []DataPoint `json:"points"`
}

// FleetStatus is the aggregated health summary for all hosts.
type FleetStatus struct {
	Timestamp     time.Time     `json:"timestamp"`
	TotalHosts    int           `json:"total_hosts"`
	HealthyHosts  int           `json:"healthy_hosts"`
	DegradedHosts int           `json:"degraded_hosts"`
	CriticalHosts int           `json:"critical_hosts"`
	Hosts         []HostSummary `json:"hosts"`
}

// HostSummary is a per-host health snapshot inside FleetStatus.
type HostSummary struct {
	Host         string    `json:"host"`
	HealthScore  float64   `json:"health_score"`
	State        string    `json:"state"`
	Stress       float64   `json:"stress"`
	Fatigue      float64   `json:"fatigue"`
	Contagion    float64   `json:"contagion"`
	ActiveAlerts int       `json:"active_alerts"`
	LastSeen     time.Time `json:"last_seen"`
}

// ExplainResult is the XAI explainability response for a KPI.
type ExplainResult struct {
	KPI            string             `json:"kpi"`
	Host           string             `json:"host"`
	TopDrivers     []FeatureDriver    `json:"top_drivers"`
	Recommendation string             `json:"recommendation"`
	Timestamp      time.Time          `json:"timestamp"`
}

// FeatureDriver is a contributing factor in an XAI explanation.
type FeatureDriver struct {
	Feature    string  `json:"feature"`
	Importance float64 `json:"importance"`
	Direction  string  `json:"direction"`
}

// LogEntry is a single log line returned by QueryLogs.
type LogEntry struct {
	Host      string                 `json:"host"`
	Service   string                 `json:"service"`
	Level     string                 `json:"level"`
	Body      string                 `json:"body"`
	Timestamp time.Time              `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Span is an APM trace span.
type Span struct {
	TraceID   string                 `json:"trace_id"`
	SpanID    string                 `json:"span_id"`
	Name      string                 `json:"name"`
	Service   string                 `json:"service"`
	StartTime time.Time              `json:"start_time"`
	Duration  float64                `json:"duration_ms"`
	Status    string                 `json:"status"`
	Tags      map[string]interface{} `json:"tags,omitempty"`
}
