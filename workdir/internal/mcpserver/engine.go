// Package mcpserver exposes the read-only results of a Ruptura engine to AI
// assistants over the Model Context Protocol. It reads the engine REST API and
// offers no tool that changes the cluster.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	engineTimeout = 15 * time.Second
	maxEngineBody = 16 << 20
)

// Failure is one failure the engine names, as /api/v2/failures returns it.
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
	Confidence        float64  `json:"confidence,omitempty"`
	RecommendedAction string   `json:"recommendedAction"`
	Evidence          []string `json:"evidence,omitempty"`
	Affects           []string `json:"affects,omitempty"`
}

// Edge is one call between two workloads, declared by the cluster or
// observed in traces.
type Edge struct {
	From           string   `json:"from"`
	To             string   `json:"to"`
	Source         string   `json:"source"`
	Via            string   `json:"via,omitempty"`
	CallsPerMinute float64  `json:"callsPerMinute,omitempty"`
	ErrorRatio     *float64 `json:"errorRatio,omitempty"`
}

// Coverage says how many watched pods Ruptura can protect.
type Coverage struct {
	Watched     int      `json:"watched"`
	Protected   int      `json:"protected"`
	UsualLevel  int      `json:"usualLevel"`
	Uncovered   int      `json:"uncovered"`
	Unprotected []string `json:"unprotected"`
}

// Engine reads a Ruptura engine API.
type Engine struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

// NewEngine returns a client for the engine at baseURL, authenticated with apiKey.
func NewEngine(baseURL, apiKey string) *Engine {
	return &Engine{BaseURL: strings.TrimRight(baseURL, "/"), APIKey: apiKey, Client: &http.Client{Timeout: engineTimeout}}
}

// Failures returns every failure the engine names now.
func (e *Engine) Failures(ctx context.Context) ([]Failure, error) {
	var out []Failure
	err := e.get(ctx, "/api/v2/failures", &out)
	return out, err
}

// Edges returns the call graph edges.
func (e *Engine) Edges(ctx context.Context) ([]Edge, error) {
	var graph struct {
		Edges []Edge `json:"edges"`
	}
	err := e.get(ctx, "/api/v2/callgraph", &graph)
	return graph.Edges, err
}

// Coverage returns how many watched pods are protected.
func (e *Engine) Coverage(ctx context.Context) (Coverage, error) {
	var out Coverage
	err := e.get(ctx, "/api/v2/coverage", &out)
	return out, err
}

// Health returns the engine health document.
func (e *Engine) Health(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	err := e.get(ctx, "/api/v2/health", &out)
	return out, err
}

func (e *Engine) get(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.BaseURL+path, nil)
	if err != nil {
		return fmt.Errorf("engine request %s: %w", path, err)
	}
	if e.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
	}
	resp, err := e.Client.Do(req)
	if err != nil {
		return fmt.Errorf("engine unreachable at %s", Redact(e.BaseURL))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("engine answered HTTP %d on %s", resp.StatusCode, path)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxEngineBody+1))
	if err != nil {
		return fmt.Errorf("engine answer on %s: %w", path, err)
	}
	if len(body) > maxEngineBody {
		return fmt.Errorf("engine answer on %s is larger than %d MiB", path, maxEngineBody>>20)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("engine answer on %s: %w", path, err)
	}
	return nil
}

// Redact removes any user and password from a URL, for logs and errors.
func Redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}
