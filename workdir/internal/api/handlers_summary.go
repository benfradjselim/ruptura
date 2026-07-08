package api

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/pkg/models"
)

// WorkloadSummary is the composed, SRE-readable answer to "what is this
// workload doing and what should I do about it" — FBL-A2-1's "forecast as
// the hero" panel. It is the first thing shown on workload detail, before
// any chart or raw signal.
type WorkloadSummary struct {
	Headline          string  `json:"headline"`
	TTFSeconds        int64   `json:"ttf_seconds,omitempty"`
	Confidence        float64 `json:"confidence"`
	RecommendedAction string  `json:"recommended_action"`
	WarmingUp         bool    `json:"warming_up"`
}

// handleWorkloadSummary serves GET /api/v2/workloads/{namespace}/{kind}/{name}/summary.
// It composes the existing forecast + calibration + pending-action data into
// a single sentence an SRE can act on at 3am, instead of raw JSON.
func (h *Handlers) handleWorkloadSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns, kind, name := vars["namespace"], vars["kind"], vars["name"]
	ref := models.WorkloadRef{Namespace: ns, Kind: kind, Name: name}

	if h.store == nil {
		writeJSON(w, http.StatusOK, noDataSummary(ref))
		return
	}
	if _, ok := h.store.LatestSnapshot(ref.Key()); !ok {
		writeJSON(w, http.StatusOK, noDataSummary(ref))
		return
	}

	var forecast *models.HealthForecast
	etaMinutes := 24 * 60
	if h.analyzer != nil {
		forecast = h.analyzer.ForecastHealthScore(ref)
		_, _, etaMinutes = h.analyzer.CalibrationInfo(ref)
	}

	var pending *engine.ActionRecommendation
	if h.engine != nil {
		for _, rec := range h.engine.PendingActions() {
			// event.Host (and therefore rec.Host) is always the workload key —
			// rec.Namespace/rec.Kind are only populated on the RuptureEvent path,
			// not the anomaly-consensus path, so Host is the one reliable match key.
			if rec.Host == ref.Key() {
				r := rec
				pending = &r
				break
			}
		}
	}

	writeJSON(w, http.StatusOK, buildWorkloadSummary(ref, forecast, etaMinutes, pending))
}

func noDataSummary(ref models.WorkloadRef) WorkloadSummary {
	return WorkloadSummary{
		Headline:          fmt.Sprintf("%s — no data received yet.", displayName(ref)),
		Confidence:        0,
		RecommendedAction: "Waiting for first telemetry",
		WarmingUp:         false,
	}
}

// buildWorkloadSummary is the pure, table-tested core of handleWorkloadSummary.
// forecast is nil when the workload has data but hasn't cleared calibration yet
// (warming-up case); etaMinutes is the calibration ETA used for that headline.
func buildWorkloadSummary(ref models.WorkloadRef, forecast *models.HealthForecast, etaMinutes int, pending *engine.ActionRecommendation) WorkloadSummary {
	name := displayName(ref)

	if forecast == nil {
		hours := etaMinutes / 60
		if hours < 1 {
			hours = 1
		}
		return WorkloadSummary{
			Headline:          fmt.Sprintf("Learning %s's baseline — first forecast in ~%dh.", name, hours),
			Confidence:        0,
			RecommendedAction: "None — still calibrating",
			WarmingUp:         true,
		}
	}

	confidence := clamp01(float64(forecast.ConfidenceWindow) / 60.0)
	action := recommendedActionText(pending)

	if forecast.CriticalETAMinutes > 0 {
		return WorkloadSummary{
			Headline: fmt.Sprintf("%s is predicted to breach its reliability threshold in %s (%d%% confidence). Recommended action: %s.",
				name, formatETA(forecast.CriticalETAMinutes), int(confidence*100), action),
			TTFSeconds:        int64(forecast.CriticalETAMinutes) * 60,
			Confidence:        confidence,
			RecommendedAction: action,
			WarmingUp:         false,
		}
	}

	trendWord := "stable"
	if forecast.Trend == "improving" {
		trendWord = "improving"
	} else if forecast.Trend == "degrading" {
		trendWord = "trending down, but not predicted to breach yet"
	}
	return WorkloadSummary{
		Headline:          fmt.Sprintf("%s is healthy and %s.", name, trendWord),
		Confidence:        confidence,
		RecommendedAction: action,
		WarmingUp:         false,
	}
}

func displayName(ref models.WorkloadRef) string {
	if ref.Name == "" {
		return "this workload"
	}
	return ref.Name
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// formatETA renders a minute count the way an SRE reads it: minutes under an
// hour, hours under a day, days beyond that.
func formatETA(minutes int) string {
	switch {
	case minutes < 60:
		return fmt.Sprintf("%d minutes", minutes)
	case minutes < 24*60:
		return fmt.Sprintf("%d hours", minutes/60)
	default:
		return fmt.Sprintf("%d days", minutes/(24*60))
	}
}

// recommendedActionText turns a queued ActionRecommendation into the same
// kind of plain sentence a human on-call would write, or a sane default when
// nothing is queued yet.
func recommendedActionText(pending *engine.ActionRecommendation) string {
	if pending == nil {
		return "Monitor — no automated action queued yet"
	}
	switch pending.ActionType {
	case "scale":
		delta := pending.ScaleDelta
		if delta == 0 {
			delta = 1
		}
		verb, n := "out", delta
		if delta < 0 {
			verb, n = "in", -delta
		}
		return fmt.Sprintf("Scale %s by %d replica(s)", verb, n)
	case "restart":
		return "Restart the workload"
	case "cordon":
		return "Cordon the node"
	case "page":
		return "Page on-call immediately"
	case "alert", "notify":
		return "Alert on-call"
	default:
		return "Review the recommended action in the Actions tab"
	}
}
