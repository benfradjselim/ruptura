package telemetry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Registry holds the engine's own Prometheus metrics.
type Registry struct {
	startTime   time.Time
	version     string
	ingestTotal sync.Map // key="source" -> *int64
}

// NewRegistry returns an empty registry for the given engine version.
func NewRegistry(version string) *Registry {
	return &Registry{startTime: time.Now(), version: version}
}

// IncIngestTotal counts one ingested item from source.
func (r *Registry) IncIngestTotal(source string) {
	actual, _ := r.ingestTotal.LoadOrStore(source, new(int64))
	atomic.AddInt64(actual.(*int64), 1)
}

// Render returns the registry in the Prometheus text format.
func (r *Registry) Render() string {
	var b strings.Builder
	b.WriteString("# HELP rpt_ingest_samples_total Ingest total\n# TYPE rpt_ingest_samples_total counter\n")
	var sources []string
	r.ingestTotal.Range(func(key, _ interface{}) bool {
		sources = append(sources, key.(string))
		return true
	})
	sort.Strings(sources)
	for _, source := range sources {
		val, _ := r.ingestTotal.Load(source)
		b.WriteString(fmt.Sprintf("rpt_ingest_samples_total{source=%q} %d\n", source, atomic.LoadInt64(val.(*int64))))
	}
	b.WriteString(fmt.Sprintf("rpt_uptime_seconds %f\n", time.Since(r.startTime).Seconds()))
	b.WriteString(fmt.Sprintf("rpt_version_info{version=%q} 1\n", r.version))
	return b.String()
}
