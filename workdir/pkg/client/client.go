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

// Client is a typed HTTP client for the Kairo Core v6 REST API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Config holds client configuration.
type Config struct {
	BaseURL string        // e.g. "http://localhost:8080"
	APIKey  string        // Bearer token; empty disables auth header
	Timeout time.Duration // default 10s
}

// New creates a new Client from Config.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL: cfg.BaseURL,
		apiKey:  cfg.APIKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// --- Response types ---

type HealthResponse struct {
	Status           string `json:"status"`
	RuptureDetection string `json:"rupture_detection"`
	Message          string `json:"message"`
}

type RuptureEvent struct {
	Host         string    `json:"host"`
	Metric       string    `json:"metric"`
	RuptureIndex float64   `json:"rupture_index"`
	Timestamp    time.Time `json:"timestamp"`
}

type KPIValue struct {
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

type ContextEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Service   string    `json:"service"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type EmergencyStopResponse struct {
	EmergencyStop bool `json:"emergency_stop"`
}

// --- API Methods ---

func (c *Client) do(ctx context.Context, method, path string, body interface{}, response interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewBuffer(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("kairo: %s %s: status %d", method, path, resp.StatusCode)
	}

	if response != nil {
		return json.NewDecoder(resp.Body).Decode(response)
	}
	return nil
}

// Health returns the server health status.
func (c *Client) Health(ctx context.Context) (HealthResponse, error) {
	var res HealthResponse
	err := c.do(ctx, "GET", "/api/v2/health", nil, &res)
	return res, err
}

// Ruptures returns all recent rupture events.
func (c *Client) Ruptures(ctx context.Context) ([]RuptureEvent, error) {
	var res []RuptureEvent
	err := c.do(ctx, "GET", "/api/v2/ruptures", nil, &res)
	return res, err
}

// RuptureForHost returns the latest rupture event for a specific host.
func (c *Client) RuptureForHost(ctx context.Context, host string) (RuptureEvent, error) {
	var res RuptureEvent
	err := c.do(ctx, "GET", fmt.Sprintf("/api/v2/ruptures/%s", host), nil, &res)
	return res, err
}

// KPI returns the latest KPI value for a given name and host.
func (c *Client) KPI(ctx context.Context, name, host string) (KPIValue, error) {
	var res KPIValue
	err := c.do(ctx, "GET", fmt.Sprintf("/api/v2/kpi/%s/%s", name, host), nil, &res)
	return res, err
}

// AddContext registers a manual context entry.
func (c *Client) AddContext(ctx context.Context, entry ContextEntry) (ContextEntry, error) {
	var res ContextEntry
	err := c.do(ctx, "POST", "/api/v2/context", entry, &res)
	return res, err
}

// DeleteContext removes a manual context entry by ID.
func (c *Client) DeleteContext(ctx context.Context, id string) error {
	return c.do(ctx, "DELETE", fmt.Sprintf("/api/v2/context/%s", id), nil, nil)
}

// ListContexts returns all active context entries.
func (c *Client) ListContexts(ctx context.Context) ([]ContextEntry, error) {
	var res []ContextEntry
	err := c.do(ctx, "GET", "/api/v2/context", nil, &res)
	return res, err
}

// EmergencyStop triggers the emergency stop on the server.
func (c *Client) EmergencyStop(ctx context.Context) (EmergencyStopResponse, error) {
	var res EmergencyStopResponse
	err := c.do(ctx, "POST", "/api/v2/emergency_stop", nil, &res)
	return res, err
}

// Metrics returns the raw Prometheus text from /api/v2/metrics.
func (c *Client) Metrics(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/v2/metrics", nil)
	if err != nil {
		return "", err
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("kairo: GET /api/v2/metrics: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
