package explain

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
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

// RuptureRecord holds all data needed to explain a rupture event.
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

	// KPI signals at time of rupture — populated by main wiring loop.
	KPISnapshot models.KPISnapshot

	// Upstream services whose error propagation contributed to contagion.
	// Populated from topology edges when contagion signal > threshold.
	ContagionSources []string
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

	first := dominantPipeline(rec)

	var sum float64
	for _, m := range rec.Metrics {
		sum += m.Weight
	}
	normalized := make([]MetricContribution, len(rec.Metrics))
	for i, m := range rec.Metrics {
		normalized[i] = m
		if sum > 0 {
			normalized[i].Weight = m.Weight / sum
		}
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

// NarrativeExplain returns a causal-chain explanation of a rupture event.
// The narrative is built from signal values, topology edges, and model contributions
// — no LLM required.
func (e *Engine) NarrativeExplain(id string) (string, error) {
	val, ok := e.records.Load(id)
	if !ok {
		return "", fmt.Errorf("explain: rupture %s not found", id)
	}
	rec := val.(RuptureRecord)
	return buildNarrative(rec), nil
}

// buildNarrative constructs the human-readable causal chain from a RuptureRecord.
func buildNarrative(rec RuptureRecord) string {
	snap := rec.KPISnapshot
	var b strings.Builder

	workloadName := rec.Host
	if !snap.Workload.IsEmpty() {
		workloadName = snap.Workload.Name
	}

	// --- Opening: workload + severity + FusedR ---
	// Use the higher of FusedR and R so existing records that only set R still classify correctly.
	effectiveR := rec.FusedR
	if rec.R > effectiveR {
		effectiveR = rec.R
	}
	severity, stateLabel := classifySeverity(effectiveR)
	fmt.Fprintf(&b, "%s is in %s state (FusedR=%.2f", workloadName, stateLabel, effectiveR)
	if rec.Confidence > 0 {
		fmt.Fprintf(&b, ", confidence=%.0f%%", rec.Confidence*100)
	}
	b.WriteString(").\n\n")

	// --- Signal analysis: describe elevated signals ---
	signals := buildSignalSentences(snap, rec)
	if len(signals) > 0 {
		b.WriteString("Signal analysis:\n")
		for _, s := range signals {
			fmt.Fprintf(&b, "  • %s\n", s)
		}
		b.WriteString("\n")
	}

	// --- Cascade / contagion ---
	cascadeDesc := buildCascadeDescription(rec, snap)
	if cascadeDesc != "" {
		b.WriteString(cascadeDesc)
		b.WriteString("\n\n")
	}

	// --- Primary pipeline ---
	primary := dominantPipeline(rec)
	switch primary {
	case "log":
		fmt.Fprintf(&b, "Log burst is the dominant signal (logR=%.2f — error/warn rate %.1fx above baseline).\n\n",
			rec.LogR, rec.LogR+1)
	case "trace":
		fmt.Fprintf(&b, "Trace error propagation is the dominant signal (traceR=%.2f — span errors and P99 latency both elevated).\n\n",
			rec.TraceR)
	default:
		if rec.AlphaBurst != 0 && rec.AlphaStable != 0 {
			fmt.Fprintf(&b, "Metric acceleration is the dominant signal (metricR=%.2f — "+
				"5-min slope %.2f vs 60-min baseline slope %.2f).\n\n",
				rec.MetricR, rec.AlphaBurst, rec.AlphaStable)
		} else {
			fmt.Fprintf(&b, "Metric acceleration is the dominant signal (metricR=%.2f).\n\n", rec.MetricR)
		}
	}

	// --- Recommended action ---
	action := recommendAction(rec, snap, severity, effectiveR)
	fmt.Fprintf(&b, "Recommended action: %s\n\n", action)

	// --- TTF ---
	if rec.TTFSeconds > 0 {
		mins := int(math.Round(rec.TTFSeconds / 60))
		if mins < 1 {
			fmt.Fprintf(&b, "Estimated time to failure: %ds.\n", int(rec.TTFSeconds))
		} else {
			fmt.Fprintf(&b, "Estimated time to failure: %d minute(s).\n", mins)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// buildSignalSentences returns one descriptive sentence per elevated KPI signal.
func buildSignalSentences(snap models.KPISnapshot, rec RuptureRecord) []string {
	var out []string

	if snap.Stress.Value >= 0.6 {
		out = append(out, fmt.Sprintf("stress=%.2f (%s) — workload is under heavy load", snap.Stress.Value, snap.Stress.State))
	}
	if snap.Fatigue.Value >= 0.6 {
		// Estimate how long fatigue has been building based on value and lambda=0.05
		// fatigue accumulates at ~0.05 per interval; rough duration estimate
		hoursAccumulating := snap.Fatigue.Value / 0.05 * (15.0 / 3600.0)
		out = append(out, fmt.Sprintf("fatigue=%.2f (%s) — stress accumulated over ~%.0fh",
			snap.Fatigue.Value, snap.Fatigue.State, hoursAccumulating))
	} else if snap.Fatigue.Value >= 0.3 {
		out = append(out, fmt.Sprintf("fatigue=%.2f (%s) — fatigue building", snap.Fatigue.Value, snap.Fatigue.State))
	}
	if snap.Pressure.Value >= 0.5 {
		out = append(out, fmt.Sprintf("pressure=%.2f (%s) — rate of change is escalating", snap.Pressure.Value, snap.Pressure.State))
	}
	if snap.Humidity.Value >= 0.5 {
		out = append(out, fmt.Sprintf("humidity=%.2f — error/timeout density is high relative to throughput", snap.Humidity.Value))
	}
	if snap.Contagion.Value >= 0.4 {
		out = append(out, fmt.Sprintf("contagion=%.2f (%s) — errors are propagating to downstream services",
			snap.Contagion.Value, snap.Contagion.State))
	}
	if snap.Entropy.Value >= 0.5 {
		out = append(out, fmt.Sprintf("entropy=%.2f — behavior is unpredictable (high HealthScore variance)", snap.Entropy.Value))
	}
	if snap.Velocity.Value >= 0.5 {
		out = append(out, fmt.Sprintf("velocity=%.2f — HealthScore is degrading rapidly", snap.Velocity.Value))
	}
	if snap.Mood.Value > 0 && snap.Mood.Value < 0.4 {
		out = append(out, fmt.Sprintf("mood=%.2f (%s) — user-facing experience is degraded", snap.Mood.Value, snap.Mood.State))
	}
	if snap.HealthScore.Value > 0 {
		out = append(out, fmt.Sprintf("health_score=%.0f/100 (%s)", snap.HealthScore.Value, snap.HealthScore.State))
	}

	return out
}

// buildCascadeDescription returns a paragraph describing the contagion cascade,
// or an empty string if no cascade is detected.
func buildCascadeDescription(rec RuptureRecord, snap models.KPISnapshot) string {
	if len(rec.ContagionSources) > 0 {
		sources := strings.Join(rec.ContagionSources, ", ")
		if snap.Contagion.Value >= 0.6 {
			return fmt.Sprintf("Cascade rupture detected: error propagation from %s (via real trace service edges) "+
				"pushed contagion to %.2f and contributed to the FusedR spike. "+
				"This is a cascade rupture, not an isolated event.", sources, snap.Contagion.Value)
		}
		return fmt.Sprintf("Contagion detected from upstream: %s. "+
			"Monitor these dependencies closely.", sources)
	}
	if rec.TraceR > 1.5 {
		return fmt.Sprintf("Trace error propagation is elevated (traceR=%.2f). "+
			"Span errors and P99 latency deviation suggest a dependency is contributing to this rupture. "+
			"Check the service dependency graph at GET /api/v2/topology.", rec.TraceR)
	}
	if rec.LogR > 1.5 {
		return fmt.Sprintf("Log burst is significant (logR=%.2f — %.1fx above baseline error rate). "+
			"A dependency may be flooding this service with errors.", rec.LogR, rec.LogR+1)
	}
	return ""
}

// recommendAction returns a concrete remediation recommendation based on the top signal.
func recommendAction(rec RuptureRecord, snap models.KPISnapshot, severity string, effectiveR float64) string {
	name := rec.Host
	if !snap.Workload.IsEmpty() {
		name = snap.Workload.Name
	}

	// Cascade with known upstream → circuit-break first
	if len(rec.ContagionSources) > 0 && snap.Contagion.Value >= 0.5 {
		upstream := rec.ContagionSources[0]
		if snap.Fatigue.Value >= 0.7 {
			return fmt.Sprintf("circuit-break the %s dependency AND perform a rolling restart of %s to clear accumulated fatigue", upstream, name)
		}
		return fmt.Sprintf("circuit-break the %s dependency to stop error propagation into %s", upstream, name)
	}

	// Emergency / critical → scale + restart
	if severity == "emergency" {
		if snap.Fatigue.Value >= 0.7 {
			return fmt.Sprintf("immediate rolling restart of %s (Tier-1 auto-action eligible) — fatigue is near burnout threshold", name)
		}
		return fmt.Sprintf("scale %s by +2 replicas immediately (Tier-1 auto-action eligible)", name)
	}

	// Fatigue-dominated → restart
	if snap.Fatigue.Value >= 0.75 {
		return fmt.Sprintf("rolling restart of %s to reset accumulated fatigue (current fatigue=%.2f, burnout threshold=0.80)", name, snap.Fatigue.Value)
	}

	// Stress-dominated → scale
	if snap.Stress.Value >= 0.7 {
		return fmt.Sprintf("scale %s by +1 replica to distribute load (current stress=%.2f)", name, snap.Stress.Value)
	}

	// High pressure → investigate before acting
	if snap.Pressure.Value >= 0.6 {
		return fmt.Sprintf("investigate rate of change on %s — pressure=%.2f suggests a storm is approaching. "+
			"Check deploy history and upstream error rates before scaling.", name, snap.Pressure.Value)
	}

	// High entropy → no action yet, observe
	if snap.Entropy.Value >= 0.6 {
		return fmt.Sprintf("monitor %s closely — entropy=%.2f indicates unpredictable behavior. "+
			"Enable Tier-2 suggested actions and review at next 15s tick.", name, snap.Entropy.Value)
	}

	return fmt.Sprintf("monitor %s — FusedR=%.2f is in warning range. No automated action recommended yet.", name, effectiveR)
}

// dominantPipeline returns which of the three signal sources has the highest R value.
func dominantPipeline(rec RuptureRecord) string {
	primary := "metric"
	max := rec.MetricR
	if rec.LogR > max {
		primary = "log"
		max = rec.LogR
	}
	if rec.TraceR > max {
		primary = "trace"
	}
	return primary
}

// classifySeverity returns a severity label and state description for a FusedR value.
func classifySeverity(fusedR float64) (severity, label string) {
	switch {
	case fusedR >= 5.0:
		return "emergency", "emergency"
	case fusedR >= 3.0:
		return "critical", "critical"
	case fusedR >= 1.5:
		return "warning", "warning"
	default:
		return "elevated", "elevated"
	}
}
