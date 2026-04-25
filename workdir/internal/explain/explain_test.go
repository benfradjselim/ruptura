package explain

import (
	"testing"
)

func TestExplain_notFound(t *testing.T) {
	e := NewEngine()
	_, err := e.Explain("unknown")
	if err == nil {
		t.Error("Expected error for not found")
	}
}

func TestExplain_logic(t *testing.T) {
	e := NewEngine()
	rec := RuptureRecord{
		ID:        "r1",
		MetricR:   1.0,
		LogR:      2.0,
		TraceR:    0.5,
		Metrics:   []MetricContribution{{Weight: 1.0}, {Weight: 1.0}},
	}
	e.Record(rec)

	resp, err := e.Explain("r1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.FirstPipeline != "log" {
		t.Errorf("Expected log, got %s", resp.FirstPipeline)
	}
	if len(resp.Contributions) != 2 || resp.Contributions[0].Weight != 0.5 {
		t.Errorf("Normalization failed: %v", resp.Contributions)
	}
}

func TestFormulaAudit_values(t *testing.T) {
	e := NewEngine()
	rec := RuptureRecord{
		ID:           "r1",
		AlphaBurst:   0.1,
		AlphaStable:  0.2,
		R:            0.3,
		TTFSeconds:   10,
		Confidence:   0.9,
		FusedR:       0.4,
		MetricR:      0.5,
		LogR:         0.6,
		TraceR:       0.7,
	}
	e.Record(rec)
	audit, err := e.FormulaAudit("r1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if audit.AlphaBurst != 0.1 || audit.FusedR != 0.4 {
		t.Errorf("Wrong values: %v", audit)
	}
}

func TestPipelineDebug_values(t *testing.T) {
	e := NewEngine()
	rec := RuptureRecord{ID: "r1", MetricR: 0.5, LogR: 0.6, TraceR: 0.7, FusedR: 0.8}
	e.Record(rec)
	dbg, err := e.PipelineDebug("r1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if dbg.FusedR != 0.8 {
		t.Errorf("Wrong values: %v", dbg)
	}
}

func TestRecord_overwrite(t *testing.T) {
	e := NewEngine()
	e.Record(RuptureRecord{ID: "r1", R: 1.0})
	e.Record(RuptureRecord{ID: "r1", R: 2.0})
	audit, _ := e.FormulaAudit("r1")
	if audit.RuptureIndex != 2.0 {
		t.Errorf("Expected 2.0, got %f", audit.RuptureIndex)
	}
}
