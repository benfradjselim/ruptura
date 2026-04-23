// Package ohe provides a Go client for the OHE observability platform API.
package ohe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultTimeout = 30 * time.Second

// Client is an authenticated OHE API client.
type Client struct {
	baseURL    string
	token      string // JWT or API key (ohe_*)
	httpClient *http.Client
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets a custom request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New creates a Client targeting baseURL (e.g. "https://ohe.example.com") and
// authenticating with token (JWT bearer or API key starting with "ohe_").
func New(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// --- low-level helpers ---

func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ohe: marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("ohe: new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ohe: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(raw)}
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("ohe: decode: %w", err)
		}
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *Client) put(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// APIError is returned when the server responds with HTTP 4xx or 5xx.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("ohe: API error %d: %s", e.StatusCode, e.Body)
}

// --- types ---

// Metric represents a single time-series observation.
type Metric struct {
	Host      string    `json:"host"`
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// MetricPoint is a (timestamp, value) pair returned by range queries.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// Alert represents a firing or resolved alert.
type Alert struct {
	ID        string    `json:"id"`
	Host      string    `json:"host"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Severity  string    `json:"severity"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AlertRule is the definition of an alerting rule.
type AlertRule struct {
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"`
}

// Dashboard is a saved OHE dashboard.
type Dashboard struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Panels    json.RawMessage `json:"panels,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// SLO defines a Service-Level Objective.
type SLO struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Metric string  `json:"metric"`
	Target float64 `json:"target"`
}

// SLOStatus is the current compliance state of an SLO.
type SLOStatus struct {
	SLOID       string  `json:"slo_id"`
	Current     float64 `json:"current"`
	Target      float64 `json:"target"`
	ErrorBudget float64 `json:"error_budget"`
	Compliant   bool    `json:"compliant"`
}

// IngestBatch is the payload for pushing metrics.
type IngestBatch struct {
	Metrics []Metric `json:"metrics"`
}

// apiResponse is the standard OHE response envelope.
type apiResponse struct {
	Data json.RawMessage `json:"data"`
}

func unwrap(raw json.RawMessage, out interface{}) error {
	return json.Unmarshal(raw, out)
}

// --- metrics ---

// Ingest pushes a batch of metric observations to OHE.
func (c *Client) Ingest(ctx context.Context, metrics []Metric) error {
	return c.post(ctx, "/api/v1/ingest", IngestBatch{Metrics: metrics}, nil)
}

// MetricRange queries a metric's historical values between from and to.
func (c *Client) MetricRange(ctx context.Context, name, host string, from, to time.Time) ([]MetricPoint, error) {
	q := url.Values{}
	q.Set("host", host)
	q.Set("from", from.UTC().Format(time.RFC3339))
	q.Set("to", to.UTC().Format(time.RFC3339))
	path := "/api/v1/metrics/" + url.PathEscape(name) + "/range?" + q.Encode()

	var env apiResponse
	if err := c.get(ctx, path, &env); err != nil {
		return nil, err
	}
	var points []MetricPoint
	return points, unwrap(env.Data, &points)
}

// --- alerts ---

// ListAlerts returns all alerts (active and resolved).
func (c *Client) ListAlerts(ctx context.Context) ([]Alert, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/alerts", &env); err != nil {
		return nil, err
	}
	var alerts []Alert
	return alerts, unwrap(env.Data, &alerts)
}

// GetAlert returns a single alert by ID.
func (c *Client) GetAlert(ctx context.Context, id string) (*Alert, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/alerts/"+url.PathEscape(id), &env); err != nil {
		return nil, err
	}
	var a Alert
	return &a, unwrap(env.Data, &a)
}

// AcknowledgeAlert marks an alert as acknowledged.
func (c *Client) AcknowledgeAlert(ctx context.Context, id string) error {
	return c.post(ctx, "/api/v1/alerts/"+url.PathEscape(id)+"/acknowledge", nil, nil)
}

// --- alert rules ---

// ListAlertRules returns all configured alert rules.
func (c *Client) ListAlertRules(ctx context.Context) ([]AlertRule, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/alert-rules", &env); err != nil {
		return nil, err
	}
	var rules []AlertRule
	return rules, unwrap(env.Data, &rules)
}

// CreateAlertRule creates a new alert rule.
func (c *Client) CreateAlertRule(ctx context.Context, rule AlertRule) (*AlertRule, error) {
	var env apiResponse
	if err := c.post(ctx, "/api/v1/alert-rules", rule, &env); err != nil {
		return nil, err
	}
	var created AlertRule
	return &created, unwrap(env.Data, &created)
}

// UpdateAlertRule replaces an existing rule by name.
func (c *Client) UpdateAlertRule(ctx context.Context, name string, rule AlertRule) error {
	return c.put(ctx, "/api/v1/alert-rules/"+url.PathEscape(name), rule, nil)
}

// DeleteAlertRule removes a rule by name.
func (c *Client) DeleteAlertRule(ctx context.Context, name string) error {
	return c.delete(ctx, "/api/v1/alert-rules/"+url.PathEscape(name))
}

// --- dashboards ---

// ListDashboards returns all dashboards visible to the caller.
func (c *Client) ListDashboards(ctx context.Context) ([]Dashboard, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/dashboards", &env); err != nil {
		return nil, err
	}
	var dashboards []Dashboard
	return dashboards, unwrap(env.Data, &dashboards)
}

// GetDashboard returns a single dashboard by ID.
func (c *Client) GetDashboard(ctx context.Context, id string) (*Dashboard, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/dashboards/"+url.PathEscape(id), &env); err != nil {
		return nil, err
	}
	var d Dashboard
	return &d, unwrap(env.Data, &d)
}

// CreateDashboard saves a new dashboard and returns it with server-assigned ID.
func (c *Client) CreateDashboard(ctx context.Context, d Dashboard) (*Dashboard, error) {
	var env apiResponse
	if err := c.post(ctx, "/api/v1/dashboards", d, &env); err != nil {
		return nil, err
	}
	var created Dashboard
	return &created, unwrap(env.Data, &created)
}

// DeleteDashboard removes a dashboard by ID.
func (c *Client) DeleteDashboard(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/dashboards/"+url.PathEscape(id))
}

// --- SLOs ---

// ListSLOs returns all SLOs for the caller's org.
func (c *Client) ListSLOs(ctx context.Context) ([]SLO, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/slos", &env); err != nil {
		return nil, err
	}
	var slos []SLO
	return slos, unwrap(env.Data, &slos)
}

// GetSLO returns a single SLO by ID.
func (c *Client) GetSLO(ctx context.Context, id string) (*SLO, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/slos/"+url.PathEscape(id), &env); err != nil {
		return nil, err
	}
	var s SLO
	return &s, unwrap(env.Data, &s)
}

// CreateSLO creates a new SLO.
func (c *Client) CreateSLO(ctx context.Context, s SLO) (*SLO, error) {
	var env apiResponse
	if err := c.post(ctx, "/api/v1/slos", s, &env); err != nil {
		return nil, err
	}
	var created SLO
	return &created, unwrap(env.Data, &created)
}

// SLOStatus returns the current compliance state of an SLO.
func (c *Client) SLOStatus(ctx context.Context, id string) (*SLOStatus, error) {
	var env apiResponse
	if err := c.get(ctx, "/api/v1/slos/"+url.PathEscape(id)+"/status", &env); err != nil {
		return nil, err
	}
	var st SLOStatus
	return &st, unwrap(env.Data, &st)
}

// DeleteSLO removes an SLO by ID.
func (c *Client) DeleteSLO(ctx context.Context, id string) error {
	return c.delete(ctx, "/api/v1/slos/"+url.PathEscape(id))
}

// --- health ---

// Health returns true if the server is healthy.
func (c *Client) Health(ctx context.Context) bool {
	return c.do(ctx, http.MethodGet, "/api/v1/health", nil, nil) == nil
}
