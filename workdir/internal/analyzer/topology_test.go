package analyzer

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestTopologyAnalyzer_IngestSpan(t *testing.T) {
	topo := NewTopologyAnalyzer(5 * time.Minute)

	// Parent span from service A
	parent := models.Span{
		TraceID:   "trace-001",
		SpanID:    "span-A",
		Service:   "service-a",
		Operation: "GET /users",
		StartTime: time.Now(),
		Status:    "ok",
		Host:      "host-1",
	}
	topo.IngestSpan(parent)

	// Child span from service B (calls A)
	child := models.Span{
		TraceID:   "trace-001",
		SpanID:    "span-B",
		ParentID:  "span-A",
		Service:   "service-b",
		Operation: "db.query",
		StartTime: time.Now(),
		DurationNS: 5_000_000, // 5ms
		Status:    "ok",
		Host:      "host-2",
	}
	topo.IngestSpan(child)

	graph := topo.Graph()

	if len(graph.Nodes) == 0 {
		t.Error("expected at least one node in topology graph")
	}

	// Should have service-a and service-b
	nodeSet := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeSet[n] = true
	}
	if !nodeSet["service-a"] {
		t.Error("expected service-a in nodes")
	}
	if !nodeSet["service-b"] {
		t.Error("expected service-b in nodes")
	}

	// Should have an edge A→B
	var foundEdge bool
	for _, e := range graph.Edges {
		if e.From == "service-a" && e.To == "service-b" {
			foundEdge = true
			if e.Calls != 1 {
				t.Errorf("calls: got %d, want 1", e.Calls)
			}
			if e.Errors != 0 {
				t.Errorf("errors: got %d, want 0", e.Errors)
			}
		}
	}
	if !foundEdge {
		t.Error("expected edge service-a → service-b")
	}
}

func TestTopologyAnalyzer_ErrorContagion(t *testing.T) {
	topo := NewTopologyAnalyzer(5 * time.Minute)

	parent := models.Span{
		TraceID: "trace-err", SpanID: "p1",
		Service: "frontend", StartTime: time.Now(), Status: "ok",
	}
	topo.IngestSpan(parent)

	// Error span from backend called by frontend
	errSpan := models.Span{
		TraceID: "trace-err", SpanID: "c1", ParentID: "p1",
		Service: "backend", StartTime: time.Now(), Status: "error",
	}
	topo.IngestSpan(errSpan)

	// Another error
	errSpan2 := models.Span{
		TraceID: "trace-err2", SpanID: "p2",
		Service: "frontend", StartTime: time.Now(), Status: "ok",
	}
	topo.IngestSpan(errSpan2)
	child2 := models.Span{
		TraceID: "trace-err2", SpanID: "c2", ParentID: "p2",
		Service: "backend", StartTime: time.Now(), Status: "error",
	}
	topo.IngestSpan(child2)

	graph := topo.Graph()

	var backendEdge *models.ServiceEdge
	for i, e := range graph.Edges {
		if e.From == "frontend" && e.To == "backend" {
			backendEdge = &graph.Edges[i]
			break
		}
	}
	if backendEdge == nil {
		t.Fatal("expected frontend→backend edge")
	}
	if backendEdge.Errors == 0 {
		t.Error("expected non-zero errors on edge")
	}

	ci := topo.ContagionIndex()
	if ci <= 0 {
		t.Errorf("contagion index: got %g, want > 0", ci)
	}
}

func TestTopologyAnalyzer_SameServiceNoEdge(t *testing.T) {
	topo := NewTopologyAnalyzer(5 * time.Minute)

	parent := models.Span{
		TraceID: "t1", SpanID: "s1",
		Service: "myservice", StartTime: time.Now(), Status: "ok",
	}
	topo.IngestSpan(parent)

	child := models.Span{
		TraceID: "t1", SpanID: "s2", ParentID: "s1",
		Service: "myservice", StartTime: time.Now(), Status: "ok",
	}
	topo.IngestSpan(child)

	graph := topo.Graph()

	// Intra-service spans should not create cross-service edges
	for _, e := range graph.Edges {
		if e.From == e.To && e.From != "" {
			t.Errorf("unexpected self-loop edge: %s→%s", e.From, e.To)
		}
	}
}
