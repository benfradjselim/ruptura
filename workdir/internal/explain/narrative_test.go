package explain

import (
	"strings"
	"testing"
	"time"
)

func TestNarrativeExplain_notFound(t *testing.T) {
	e := NewEngine()
	_, err := e.NarrativeExplain("nonexistent-id")
	if err == nil {
		t.Error("expected error for unknown rupture ID")
	}
}

func TestNarrativeExplain_containsHost(t *testing.T) {
	e := NewEngine()
	rec := RuptureRecord{
		ID:        "test-001",
		Host:      "api-server-7",
		R:         4.5,
		Timestamp: time.Now(),
		MetricR:   2.5,
		LogR:      0.5,
		TraceR:    0.3,
		Metrics: []MetricContribution{
			{Metric: "cpu_percent", Weight: 0.7},
			{Metric: "latency_p99", Weight: 0.3},
		},
	}
	e.Record(rec)

	narrative, err := e.NarrativeExplain("test-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if narrative == "" {
		t.Error("expected non-empty narrative")
	}
	if !strings.Contains(narrative, "api-server-7") {
		t.Errorf("narrative should contain host name; got: %q", narrative)
	}
}

func TestNarrativeExplain_severityLabels(t *testing.T) {
	e := NewEngine()

	// Threshold table (from docs): FusedR >= 5.0 → emergency, >= 3.0 → critical,
	// >= 1.5 → warning, else → elevated.
	tests := []struct {
		id       string
		r        float64
		wantWord string
	}{
		{"r1", 6.0, "emergency"},
		{"r2", 3.5, "critical"},
		{"r3", 2.0, "warning"},
	}

	for _, tt := range tests {
		e.Record(RuptureRecord{ID: tt.id, Host: "h", R: tt.r, Timestamp: time.Now()})
		narrative, err := e.NarrativeExplain(tt.id)
		if err != nil {
			t.Fatalf("[%s] unexpected error: %v", tt.id, err)
		}
		if !strings.Contains(narrative, tt.wantWord) {
			t.Errorf("[%s] expected %q in narrative; got: %q", tt.id, tt.wantWord, narrative)
		}
	}
}

func TestNarrativeExplain_contagionNote(t *testing.T) {
	e := NewEngine()
	e.Record(RuptureRecord{
		ID:        "c-001",
		Host:      "svc-a",
		R:         5.0,
		TraceR:    2.5, // should trigger trace contagion note
		Timestamp: time.Now(),
	})

	narrative, err := e.NarrativeExplain("c-001")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(narrative, "Trace error propagation") {
		t.Errorf("expected trace contagion note; got: %q", narrative)
	}
}
