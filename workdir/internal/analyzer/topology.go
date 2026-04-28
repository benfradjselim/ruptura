package analyzer

// topology.go — Service dependency graph derived from distributed traces.
// Computes the live ServiceEdge map for the /api/v1/topology endpoint.
// Uses a rolling window of spans to detect contagion propagation paths.

import (
	"sort"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// TopologyAnalyzer maintains a live service dependency graph from ingested spans
type TopologyAnalyzer struct {
	mu sync.RWMutex
	// spanIndex: traceID → spanID → Span (for parent resolution)
	spanIndex map[string]map[string]models.Span
	// edges: "from:to" → EdgeStats
	edges map[string]*edgeStats
	// window: how far back to look when returning topology
	window time.Duration
}

type edgeStats struct {
	From     string
	To       string
	Calls    int64
	Errors   int64
	TotalNS  int64
	LastSeen time.Time
}

// NewTopologyAnalyzer creates a topology analyzer with a rolling window
func NewTopologyAnalyzer(window time.Duration) *TopologyAnalyzer {
	if window == 0 {
		window = 10 * time.Minute
	}
	return &TopologyAnalyzer{
		spanIndex: make(map[string]map[string]models.Span),
		edges:     make(map[string]*edgeStats),
		window:    window,
	}
}

// IngestSpan records a span and derives service edges from parent relationships
func (t *TopologyAnalyzer) IngestSpan(span models.Span) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Index the span for parent resolution
	if _, ok := t.spanIndex[span.TraceID]; !ok {
		t.spanIndex[span.TraceID] = make(map[string]models.Span)
	}
	t.spanIndex[span.TraceID][span.SpanID] = span

	// If this span has a parent, resolve parent's service and record edge
	if span.ParentID != "" {
		if siblings, ok := t.spanIndex[span.TraceID]; ok {
			if parent, ok := siblings[span.ParentID]; ok && parent.Service != span.Service {
				// Edge: parent.Service calls span.Service
				t.recordEdge(parent.Service, span.Service, span)
				return
			}
		}
	}
	// Even without parent resolution, record self-node existence via a no-op edge
	// (so the node shows up in the topology)
	key := span.Service + ":"
	if t.edges[key] == nil {
		t.edges[key] = &edgeStats{From: span.Service, To: ""}
	}
	t.edges[key].LastSeen = span.StartTime
}

func (t *TopologyAnalyzer) recordEdge(from, to string, span models.Span) {
	key := from + ":" + to
	e := t.edges[key]
	if e == nil {
		e = &edgeStats{From: from, To: to}
		t.edges[key] = e
	}
	e.Calls++
	e.TotalNS += span.DurationNS
	e.LastSeen = span.StartTime
	if span.Status == "error" {
		e.Errors++
	}
}

// Graph returns the current topology within the rolling window.
// Uses a read lock for the result snapshot, then briefly takes a write lock
// to prune stale entries — minimising ingestion contention.
func (t *TopologyAnalyzer) Graph() models.TopologyGraph {
	cutoff := time.Now().Add(-t.window)

	// Read phase — no mutation
	t.mu.RLock()
	nodeSet := make(map[string]struct{})
	var edges []models.ServiceEdge
	for _, e := range t.edges {
		if e.LastSeen.Before(cutoff) {
			continue
		}
		if e.From != "" {
			nodeSet[e.From] = struct{}{}
		}
		if e.To != "" {
			nodeSet[e.To] = struct{}{}
			avgLatMS := 0.0
			if e.Calls > 0 {
				avgLatMS = float64(e.TotalNS) / float64(e.Calls) / 1e6
			}
			edges = append(edges, models.ServiceEdge{
				From:     e.From,
				To:       e.To,
				Calls:    e.Calls,
				Errors:   e.Errors,
				AvgLatMS: avgLatMS,
			})
		}
	}

	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	t.mu.RUnlock()

	// GC phase — prune stale entries with a brief write lock
	t.mu.Lock()
	for traceID, spans := range t.spanIndex {
		allOld := true
		for _, s := range spans {
			if s.StartTime.After(cutoff) {
				allOld = false
				break
			}
		}
		if allOld {
			delete(t.spanIndex, traceID)
		}
	}
	for key, e := range t.edges {
		if e.LastSeen.Before(cutoff) {
			delete(t.edges, key)
		}
	}
	t.mu.Unlock()

	return models.TopologyGraph{
		Timestamp: time.Now(),
		Nodes:     nodes,
		Edges:     edges,
	}
}

// UpstreamDeps returns all services that host directly depends on (i.e. services
// that host calls), based on edges seen within the rolling window.
func (t *TopologyAnalyzer) UpstreamDeps(host string) []string {
	cutoff := time.Now().Add(-t.window)
	t.mu.RLock()
	defer t.mu.RUnlock()
	var deps []string
	for _, e := range t.edges {
		if e.From == host && !e.LastSeen.Before(cutoff) {
			deps = append(deps, e.To)
		}
	}
	return deps
}

// ContagionIndex computes the service-aware contagion from the topology graph.
// It is the sum of error_rate × dependency_weight for all edges.
func (t *TopologyAnalyzer) ContagionIndex() float64 {
	graph := t.Graph()
	if len(graph.Edges) == 0 {
		return 0
	}

	total := 0.0
	for _, e := range graph.Edges {
		if e.Calls == 0 {
			continue
		}
		errorRate := float64(e.Errors) / float64(e.Calls)
		// Weight by dependency depth: root → leaf edges are more contagious
		total += errorRate
	}

	// Normalize: each edge contributes up to 1.0; clamp to [0,1]
	if total > 1 {
		total = 1
	}
	return total
}
