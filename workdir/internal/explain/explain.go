package explain

import (
	"fmt"
	"sync"
	"time"
)

type MetricContribution struct {
	Metric   string
	Pipeline string // "metric"|"log"|"trace"
	Weight   float64
	RValue   float64
}

type ExplainResponse struct {
	RuptureID     string
	Host          string
	R             float64
	Confidence    float64
	Timestamp     time.Time
	Contributions []MetricContribution
	FirstPipeline string // pipeline that fired first ("metric"|"log"|"trace")
}

type FormulaAuditResponse struct {
	RuptureID    string
	AlphaBurst   float64
	AlphaStable  float64
	RuptureIndex float64
	TTFSeconds   float64
	Confidence   float64
	FusedR       float64
	MetricR      float64
	LogR         float64
	TraceR       float64
}

type PipelineDebugResponse struct {
	RuptureID string
	MetricR   float64
	LogR      float64
	TraceR    float64
	FusedR    float64
	Timestamp time.Time
}

type Explainer interface {
	Explain(ruptureID string) (*ExplainResponse, error)
	FormulaAudit(ruptureID string) (*FormulaAuditResponse, error)
	PipelineDebug(ruptureID string) (*PipelineDebugResponse, error)
}

type RuptureRecord struct {
	ID          string
	Host        string
	R           float64
	Confidence  float64
	Timestamp   time.Time
	AlphaBurst  float64
	AlphaStable float64
	TTFSeconds  float64
	MetricR     float64
	LogR        float64
	TraceR      float64
	FusedR      float64
	Metrics     []MetricContribution
}

type Engine struct {
	records sync.Map
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Record(rec RuptureRecord) {
	e.records.Store(rec.ID, rec)
}

func (e *Engine) Explain(id string) (*ExplainResponse, error) {
	val, ok := e.records.Load(id)
	if !ok {
		return nil, fmt.Errorf("explain: rupture %s not found", id)
	}
	rec := val.(RuptureRecord)

	// Determine FirstPipeline
	first := "metric"
	max := rec.MetricR
	if rec.LogR > max {
		first = "log"
		max = rec.LogR
	}
	if rec.TraceR > max {
		first = "trace"
	}

	// Normalize contributions
	var sum float64
	for _, m := range rec.Metrics {
		sum += m.Weight
	}
	var normalized []MetricContribution
	for _, m := range rec.Metrics {
		c := m
		if sum > 0 {
			c.Weight = c.Weight / sum
		}
		normalized = append(normalized, c)
	}

	return &ExplainResponse{
		RuptureID:     rec.ID,
		Host:          rec.Host,
		R:             rec.R,
		Confidence:    rec.Confidence,
		Timestamp:     rec.Timestamp,
		Contributions: normalized,
		FirstPipeline: first,
	}, nil
}

func (e *Engine) FormulaAudit(id string) (*FormulaAuditResponse, error) {
	val, ok := e.records.Load(id)
	if !ok {
		return nil, fmt.Errorf("explain: rupture %s not found", id)
	}
	rec := val.(RuptureRecord)
	return &FormulaAuditResponse{
		RuptureID:    rec.ID,
		AlphaBurst:   rec.AlphaBurst,
		AlphaStable:  rec.AlphaStable,
		RuptureIndex: rec.R,
		TTFSeconds:   rec.TTFSeconds,
		Confidence:   rec.Confidence,
		FusedR:       rec.FusedR,
		MetricR:      rec.MetricR,
		LogR:         rec.LogR,
		TraceR:       rec.TraceR,
	}, nil
}

func (e *Engine) PipelineDebug(id string) (*PipelineDebugResponse, error) {
	val, ok := e.records.Load(id)
	if !ok {
		return nil, fmt.Errorf("explain: rupture %s not found", id)
	}
	rec := val.(RuptureRecord)
	return &PipelineDebugResponse{
		RuptureID: rec.ID,
		MetricR:   rec.MetricR,
		LogR:      rec.LogR,
		TraceR:    rec.TraceR,
		FusedR:    rec.FusedR,
		Timestamp: rec.Timestamp,
	}, nil
}
