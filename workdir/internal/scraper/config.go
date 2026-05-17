// Package scraper provides active datasource scraping for Ruptura.
// It manages a set of datasource configurations (Prometheus servers, direct
// metrics endpoints) and runs per-datasource scrape loops that feed data
// into the ingest pipeline.
package scraper

import "time"

// Type constants for datasource types.
const (
	TypePrometheus = "prometheus"     // Prometheus server — queries via HTTP API
	TypeDirect     = "direct_metrics" // Direct /metrics endpoint — Prometheus text format
	TypeOTLP       = "otlp"           // OTLP — informational only, no active scraping
)

// DatasourceConfig represents a user-configured data source.
type DatasourceConfig struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"` // TypePrometheus | TypeDirect | TypeOTLP
	URL              string    `json:"url"`
	Enabled          bool      `json:"enabled"`
	ScrapeIntervalSec int      `json:"scrape_interval_seconds"` // default 30
	// For direct_metrics: workload key override (e.g. "production/Deployment/payment-api")
	WorkloadKey      string    `json:"workload_key,omitempty"`
	// For prometheus: namespace filter (empty = all)
	Namespace        string    `json:"namespace,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// DatasourceStatus is the runtime status — not persisted, computed by Manager.
type DatasourceStatus struct {
	DatasourceConfig
	Status     string    `json:"status"`               // "ok" | "error" | "pending" | "disabled"
	LastScrape time.Time `json:"last_scrape,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	ScrapedMetrics int   `json:"scraped_metrics"`      // count from last scrape
}

func (c *DatasourceConfig) scrapeInterval() time.Duration {
	if c.ScrapeIntervalSec <= 0 {
		return 30 * time.Second
	}
	return time.Duration(c.ScrapeIntervalSec) * time.Second
}
