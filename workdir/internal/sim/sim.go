// Package sim provides synthetic failures for demoing Ruptura without a
// cluster: a failure source for the engine's demo mode, and a client that asks
// a demo engine to report a failure on a workload.
package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Pattern names exposed to the CLI.
const (
	PatternMemoryLeak     = "memory-leak"
	PatternCascadeFailure = "cascade-failure"
	PatternTrafficSurge   = "traffic-surge"
	PatternSlowBurn       = "slow-burn"
)

// AllPatterns lists every available pattern for help output.
var AllPatterns = []string{
	PatternMemoryLeak,
	PatternCascadeFailure,
	PatternTrafficSurge,
	PatternSlowBurn,
}

// Config describes one injection.
type Config struct {
	Target   string // Ruptura API base URL, e.g. "http://localhost:8080"
	Workload string // "namespace/Kind/name", e.g. "demo/Deployment/api"
	Pattern  string // one of AllPatterns
	APIKey   string // bearer token for auth-enabled servers
}

// InjectRequest is the body of POST /api/v2/sim/inject.
type InjectRequest struct {
	Workload string `json:"workload"`
	Pattern  string `json:"pattern"`
}

var simClient = &http.Client{Timeout: 10 * time.Second}

// Run asks a Ruptura engine in demo mode to report the pattern's failure on
// the workload.
func Run(cfg Config) error {
	body, err := json.Marshal(InjectRequest{Workload: cfg.Workload, Pattern: cfg.Pattern})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(cfg.Target, "/")+"/api/v2/sim/inject", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	resp, err := simClient.Do(req)
	if err != nil {
		return fmt.Errorf("inject: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("inject: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	return nil
}
