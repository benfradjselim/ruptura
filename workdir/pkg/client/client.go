package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a typed HTTP client for the Ruptura v2 REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config holds client configuration.
type Config struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (c *Client) doRaw(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// --- Response types ---

// HealthResponse is the answer of GET /api/v2/health.
type HealthResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Message       string `json:"message"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	Edition       string `json:"edition"`
	DemoMode      bool   `json:"demo_mode"`
}

// Failure is one failure the engine names, predicted or happening now.
type Failure struct {
	Level             string   `json:"level"`
	Target            string   `json:"target"`
	Workload          string   `json:"workload,omitempty"`
	Pod               string   `json:"pod,omitempty"`
	FailureMode       string   `json:"failureMode"`
	Kind              string   `json:"kind"`
	State             string   `json:"state"`
	ETASeconds        int      `json:"etaSeconds,omitempty"`
	ETAEarlySeconds   int      `json:"etaEarlySeconds,omitempty"`
	ETALateSeconds    int      `json:"etaLateSeconds,omitempty"`
	RecommendedAction string   `json:"recommendedAction"`
	Evidence          []string `json:"evidence,omitempty"`
	Affects           []string `json:"affects,omitempty"`
}

// FleetWorkload is one workload of the fleet with its state.
type FleetWorkload struct {
	Host         string   `json:"host"`
	State        string   `json:"state"`
	Risk         float64  `json:"risk"`
	RestartCount int      `json:"restart_count,omitempty"`
	ETFSeconds   int      `json:"etf_seconds,omitempty"`
	FailureMode  string   `json:"failure_mode,omitempty"`
	Failure      *Failure `json:"failure,omitempty"`
}

// Fleet is the answer of GET /api/v2/fleet.
type Fleet struct {
	TotalHosts    int             `json:"total_hosts"`
	HealthyHosts  int             `json:"healthy_hosts"`
	DegradedHosts int             `json:"degraded_hosts"`
	CriticalHosts int             `json:"critical_hosts"`
	Hosts         []FleetWorkload `json:"hosts"`
}

// ActionItem is an action recommendation of the action engine.
type ActionItem struct {
	ID         string    `json:"id"`
	WorkloadID string    `json:"workload_id"`
	Type       string    `json:"type"`
	Tier       int       `json:"tier"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// Health returns the engine health.
func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var out HealthResponse
	return out, c.do(ctx, http.MethodGet, "/api/v2/health", nil, &out)
}

// Metrics returns the engine metrics in the Prometheus text format.
func (c *Client) Metrics(ctx context.Context) (string, error) {
	body, err := c.doRaw(ctx, "/api/v2/metrics")
	return string(body), err
}

// Failures returns every failure the engine names now, most urgent first.
func (c *Client) Failures(ctx context.Context) ([]Failure, error) {
	var out []Failure
	return out, c.do(ctx, http.MethodGet, "/api/v2/failures", nil, &out)
}

// Fleet returns every workload with its state.
func (c *Client) Fleet(ctx context.Context) (Fleet, error) {
	var out Fleet
	return out, c.do(ctx, http.MethodGet, "/api/v2/fleet", nil, &out)
}

// Actions returns the action recommendations.
func (c *Client) Actions(ctx context.Context) ([]ActionItem, error) {
	var out []ActionItem
	return out, c.do(ctx, http.MethodGet, "/api/v2/actions", nil, &out)
}

// ApproveAction approves an action; community engines answer 402.
func (c *Client) ApproveAction(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/api/v2/actions/"+id+"/approve", nil, nil)
}

// RejectAction rejects an action.
func (c *Client) RejectAction(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodPost, "/api/v2/actions/"+id+"/reject", nil, nil)
}

// EmergencyStop stops every pending action.
func (c *Client) EmergencyStop(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/api/v2/actions/emergency-stop", nil, nil)
}

// SimInject asks an engine in demo mode to report a synthetic failure.
func (c *Client) SimInject(ctx context.Context, workload, pattern string) error {
	body := map[string]string{"workload": workload, "pattern": pattern}
	return c.do(ctx, http.MethodPost, "/api/v2/sim/inject", body, nil)
}
