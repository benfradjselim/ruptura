package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/benfradjselim/kairo-core/pkg/utils"
)

// kpiExplanation is the canonical response shape for GET /api/v1/explain/:kpi.
// Defined here (not in handlers.go) to keep the explain logic self-contained.
type kpiExplanation struct {
	KPI               string             `json:"kpi"`
	Host              string             `json:"host"`
	Value             float64            `json:"value"`
	State             string             `json:"state"`
	Formula           string             `json:"formula"`
	Contributions     map[string]float64 `json:"contributions"`
	DominantDriver    string             `json:"dominant_driver"`
	ThresholdBreached string             `json:"threshold_breached"`
	Recommendation    string             `json:"recommendation"`
	RuptureIndex      float64            `json:"rupture_index,omitempty"`
}

func dominantInputKey(m map[string]float64) string {
	var best string
	var max float64
	for k, v := range m {
		if v > max {
			max = v
			best = k
		}
	}
	return best
}

// ExplainHandler GET /api/v1/explain/{kpi} — v5.0 XAI endpoint.
// Returns the formula, weights, per-input contributions, dominant driver,
// and a plain-language recommendation for the requested KPI.
func (h *Handlers) ExplainHandler(w http.ResponseWriter, r *http.Request) {
	kpiName := mux.Vars(r)["kpi"]
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	snap, ok := h.analyzer.Snapshot(host)
	if !ok {
		respondError(w, http.StatusNotFound, "NO_DATA",
			"no KPI data for host — collection has not completed a cycle yet")
		return
	}

	rawMetrics, _ := h.analyzer.LastMetrics(host)
	if rawMetrics == nil {
		rawMetrics = map[string]float64{}
	}

	get := func(name string) float64 { return utils.Clamp(rawMetrics[name], 0, 1) }

	var expl kpiExplanation
	expl.Host = host
	expl.KPI = kpiName

	switch kpiName {
	case "stress":
		cpu := get("cpu_percent")
		ram := get("memory_percent")
		lat := get("load_avg_1")
		err := get("error_rate")
		tout := get("timeout_rate")

		contribs := map[string]float64{
			"CPU":      utils.RoundTo(0.30*cpu, 4),
			"RAM":      utils.RoundTo(0.20*ram, 4),
			"Latency":  utils.RoundTo(0.20*lat, 4),
			"Errors":   utils.RoundTo(0.20*err, 4),
			"Timeouts": utils.RoundTo(0.10*tout, 4),
		}
		dominant := dominantInputKey(contribs)
		expl.Value = snap.Stress.Value
		expl.State = snap.Stress.State
		expl.Formula = "S = 0.3·CPU + 0.2·RAM + 0.2·Latency + 0.2·Errors + 0.1·Timeouts"
		expl.Contributions = contribs
		expl.DominantDriver = dominant
		expl.ThresholdBreached = stressThresholdLabel(snap.Stress.Value)
		expl.Recommendation = fmt.Sprintf(
			"Monitor %s — top contributor at %.0f%% of total stress",
			dominant, contribs[dominant]/snap.Stress.Value*100,
		)
		expl.RuptureIndex = h.predictor.RuptureIndex(host, "cpu_percent")

	case "fatigue":
		expl.Value = snap.Fatigue.Value
		expl.State = snap.Fatigue.State
		expl.Formula = "F_t = max(0, F_{t−1} + (S_t − R_threshold) − λ)"
		expl.Contributions = map[string]float64{
			"Stress":      utils.RoundTo(snap.Stress.Value, 4),
			"R_threshold": 0.3,
			"Lambda":      0.05,
		}
		expl.DominantDriver = "Stress"
		expl.ThresholdBreached = fatigueThresholdLabel(snap.Fatigue.Value)
		expl.Recommendation = fatigueRecommendation(snap.Fatigue.Value)
		expl.RuptureIndex = h.predictor.RuptureIndex(host, "fatigue")

	case "mood":
		expl.Value = snap.Mood.Value
		expl.State = snap.Mood.State
		expl.Formula = "M = (Uptime × Req) / (Err × Tout × Restart + ε)"
		expl.Contributions = map[string]float64{
			"Uptime":   get("uptime_seconds"),
			"Requests": get("request_rate"),
			"Errors":   get("error_rate"),
			"Timeouts": get("timeout_rate"),
		}
		expl.DominantDriver = "Errors"
		expl.ThresholdBreached = moodThresholdLabel(snap.Mood.Value)
		expl.Recommendation = "Improve error rate and uptime to elevate mood score."

	case "pressure":
		expl.Value = snap.Pressure.Value
		expl.State = snap.Pressure.State
		expl.Formula = "P(t) = dS̄/dt + ∫₀ᵗ Ē(τ) dτ"
		expl.Contributions = map[string]float64{
			"StressDerivative": utils.RoundTo(snap.Pressure.Value, 4),
			"ErrorIntegral":    get("error_rate"),
		}
		expl.DominantDriver = "StressDerivative"
		expl.ThresholdBreached = pressureThresholdLabel(snap.Pressure.Value)
		expl.Recommendation = pressureRecommendation(snap.Pressure.Value)

	case "humidity":
		expl.Value = snap.Humidity.Value
		expl.State = snap.Humidity.State
		expl.Formula = "H(t) = (Ē(t) × T̄(t)) / Q̄(t)"
		expl.Contributions = map[string]float64{
			"Errors":     get("error_rate"),
			"Timeouts":   get("timeout_rate"),
			"Throughput": get("request_rate"),
		}
		expl.DominantDriver = dominantInputKey(map[string]float64{
			"Errors":   get("error_rate"),
			"Timeouts": get("timeout_rate"),
		})
		expl.ThresholdBreached = humidityThresholdLabel(snap.Humidity.Value)
		expl.Recommendation = "Reduce error and timeout rates to lower humidity."

	case "contagion":
		expl.Value = snap.Contagion.Value
		expl.State = snap.Contagion.State
		expl.Formula = "C(t) = Σ_{i,j} E_{ij}(t) × D_{ij}"
		expl.Contributions = map[string]float64{
			"Errors": get("error_rate"),
			"CPU":    get("cpu_percent"),
		}
		expl.DominantDriver = dominantInputKey(expl.Contributions)
		expl.ThresholdBreached = contagionThresholdLabel(snap.Contagion.Value)
		expl.Recommendation = "Isolate high-error services to contain contagion spread."

	case "health_score":
		s := snap.Stress.Value
		f := snap.Fatigue.Value
		m := snap.Mood.Value
		p := snap.Pressure.Value
		hum := snap.Humidity.Value
		c := snap.Contagion.Value
		contribs := map[string]float64{
			"Stress(inverted)":   utils.RoundTo(0.25*(1-s), 4),
			"Mood":               utils.RoundTo(0.20*m, 4),
			"Fatigue(inverted)":  utils.RoundTo(0.20*(1-f), 4),
			"Pressure(inverted)": utils.RoundTo(0.15*(1-p), 4),
			"Humidity(inverted)": utils.RoundTo(0.10*(1-hum), 4),
			"Contagion(inverted)": utils.RoundTo(0.10*(1-c), 4),
		}
		expl.Value = snap.HealthScore.Value
		expl.State = snap.HealthScore.State
		expl.Formula = "H = 0.25(1−S) + 0.20·M + 0.20(1−F) + 0.15(1−P) + 0.10(1−H) + 0.10(1−C)"
		expl.Contributions = contribs
		expl.DominantDriver = dominantInputKey(contribs)
		expl.ThresholdBreached = healthScoreThresholdLabel(snap.HealthScore.Value)
		expl.Recommendation = fmt.Sprintf("Dominant factor: %s. Focus improvements there for the highest HealthScore gain.", dominantInputKey(contribs))

	default:
		respondError(w, http.StatusBadRequest, "UNKNOWN_KPI",
			"supported KPIs: stress, fatigue, mood, pressure, humidity, contagion, health_score")
		return
	}

	respondSuccess(w, expl)
}

// --- threshold label helpers ---

func stressThresholdLabel(v float64) string {
	switch {
	case v >= 0.8:
		return "≥0.8 (Panic)"
	case v >= 0.6:
		return "≥0.6 (Stressed)"
	case v >= 0.3:
		return "≥0.3 (Nervous)"
	default:
		return "<0.3 (Calm)"
	}
}

func fatigueThresholdLabel(v float64) string {
	switch {
	case v >= 0.8:
		return "≥0.8 (Burnout imminent)"
	case v >= 0.6:
		return "≥0.6 (Exhausted)"
	case v >= 0.3:
		return "≥0.3 (Tired)"
	default:
		return "<0.3 (Rested)"
	}
}

func fatigueRecommendation(v float64) string {
	switch {
	case v >= 0.8:
		return "Burnout imminent — schedule a preventive restart now."
	case v >= 0.6:
		return "System exhausted — plan maintenance window within 4 hours."
	case v >= 0.3:
		return "Monitor fatigue trajectory; reduce load during off-peak periods."
	default:
		return "System well-rested. Dissipative λ recovery is working normally."
	}
}

func moodThresholdLabel(v float64) string {
	switch {
	case v > 0.75:
		return ">0.75 (Happy)"
	case v > 0.50:
		return ">0.5 (Content)"
	case v > 0.25:
		return ">0.25 (Neutral)"
	default:
		return "≤0.25 (Sad/Depressed)"
	}
}

func pressureThresholdLabel(v float64) string {
	switch {
	case v > 0.7:
		return ">0.7 (Storm approaching)"
	case v > 0.55:
		return ">0.55 (Rising)"
	default:
		return "≤0.55 (Stable/Improving)"
	}
}

func pressureRecommendation(v float64) string {
	if v > 0.7 {
		return "Storm in ~2 hours — scale up capacity or activate circuit breakers now."
	}
	if v > 0.55 {
		return "Pressure rising — monitor closely and prepare scaling playbook."
	}
	return "Conditions stable — no immediate action required."
}

func humidityThresholdLabel(v float64) string {
	switch {
	case v >= 0.5:
		return "≥0.5 (Storm)"
	case v >= 0.3:
		return "≥0.3 (Very humid)"
	case v >= 0.1:
		return "≥0.1 (Humid)"
	default:
		return "<0.1 (Dry)"
	}
}

func contagionThresholdLabel(v float64) string {
	switch {
	case v >= 0.8:
		return "≥0.8 (Pandemic)"
	case v >= 0.6:
		return "≥0.6 (Epidemic)"
	case v >= 0.3:
		return "≥0.3 (Moderate)"
	default:
		return "<0.3 (Low)"
	}
}

func healthScoreThresholdLabel(v float64) string {
	// v is on 0–100 scale
	switch {
	case v > 80:
		return ">80 (Excellent)"
	case v > 60:
		return ">60 (Good)"
	case v > 40:
		return ">40 (Fair)"
	case v > 20:
		return ">20 (Poor)"
	default:
		return "≤20 (Critical)"
	}
}
