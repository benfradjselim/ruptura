package metrics

import "time"

// MetricPipeline is the contract consumed by fusion, composites, and api packages.
// AGENTS.md ALPHA section.
type MetricPipeline interface {
	Ingest(host, metric string, value float64, ts time.Time)
	RuptureIndex(host, metric string) (float64, error)
	TTF(host, metric string) (time.Duration, error)
	Confidence(host, metric string) (float64, error)
	SurgeProfile(host, metric string) (string, error)
}
