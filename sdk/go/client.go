// Package ruptura provides a typed Go client for the Ruptura API.
//
// Quick start:
//
//	c := ruptura.New("https://ruptura.example.com", ruptura.WithAPIKey("rpt_abc123"))
//	health, err := c.Health(ctx)
//
// Authentication: use WithToken for JWT bearer auth, or WithAPIKey for
// long-lived programmatic access keys. Calling Login updates the client token
// automatically.
package ruptura

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a thread-safe Ruptura API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	apiKey     string
	orgID      string
}

// Option configures a Client.
type Option func(*Client)

// WithToken sets a JWT bearer token for all requests.
func WithToken(token string) Option {
	return func(c *Client) { c.token = token }
}

// WithAPIKey sets a long-lived API key (rpt_* format) for all requests.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithOrgID sets the org-id header for multi-tenant isolation.
func WithOrgID(orgID string) Option {
	return func(c *Client) { c.orgID = orgID }
}

// WithHTTPClient replaces the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the HTTP request timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.httpClient.Timeout = d }
}

// New creates a Ruptura client for baseURL (e.g. "https://ruptura.example.com").
func New(baseURL string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// SetToken updates the bearer token (e.g. after Login or Refresh).
func (c *Client) SetToken(token string) { c.token = token }

// Error is returned when the Ruptura API responds with a non-2xx status.
type Error struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *Error) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("ruptura %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("ruptura HTTP %d", e.StatusCode)
}

// do executes method on path, optionally sending body as JSON, and decodes the
// APIResponse.data envelope into dest (nil = no decode needed).
func (c *Client) do(ctx context.Context, method, path string, body, dest interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ruptura: marshal: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("ruptura: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if c.orgID != "" {
		req.Header.Set("X-Org-ID", c.orgID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ruptura: http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var wrapper struct {
			Error *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&wrapper)
		e := &Error{StatusCode: resp.StatusCode}
		if wrapper.Error != nil {
			e.Code = wrapper.Error.Code
			e.Message = wrapper.Error.Message
		}
		return e
	}

	if dest == nil {
		return nil
	}
	var wrapper struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return fmt.Errorf("ruptura: decode: %w", err)
	}
	if len(wrapper.Data) == 0 {
		return nil
	}
	return json.Unmarshal(wrapper.Data, dest)
}

func (c *Client) get(ctx context.Context, path string, q url.Values, dest interface{}) error {
	p := path
	if len(q) > 0 {
		p += "?" + q.Encode()
	}
	return c.do(ctx, http.MethodGet, p, nil, dest)
}

func (c *Client) post(ctx context.Context, path string, body, dest interface{}) error {
	return c.do(ctx, http.MethodPost, path, body, dest)
}

func (c *Client) put(ctx context.Context, path string, body, dest interface{}) error {
	return c.do(ctx, http.MethodPut, path, body, dest)
}

func (c *Client) del(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
