package api

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
	"github.com/gorilla/mux"
)

// ---------------------------------------------------------------------------
// SLO CRUD
// ---------------------------------------------------------------------------

// SLOListHandler GET /api/v1/slos
func (h *Handlers) SLOListHandler(w http.ResponseWriter, r *http.Request) {
	var slos []models.SLO
	_ = h.store.ListSLOs(func(val []byte) error {
		var s models.SLO
		if err := json.Unmarshal(val, &s); err != nil {
			return nil
		}
		slos = append(slos, s)
		return nil
	})
	if slos == nil {
		slos = []models.SLO{}
	}
	respondSuccess(w, slos)
}

// SLOCreateHandler POST /api/v1/slos
func (h *Handlers) SLOCreateHandler(w http.ResponseWriter, r *http.Request) {
	var s models.SLO
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if s.Name == "" || s.Metric == "" {
		respondError(w, http.StatusBadRequest, "MISSING_FIELDS", "name and metric are required")
		return
	}
	if s.Target <= 0 {
		s.Target = 99.9
	}
	if s.Window == "" {
		s.Window = "30d"
	}
	if s.Comparator == "" {
		s.Comparator = "lte"
	}
	s.ID = utils.GenerateID(8)
	now := time.Now()
	s.CreatedAt, s.UpdatedAt = now, now

	if err := h.store.SaveSLO(s.ID, s); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success": true, "data": s, "timestamp": now,
	})
}

// SLOGetHandler GET /api/v1/slos/{id}
func (h *Handlers) SLOGetHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var s models.SLO
	if err := h.store.GetSLO(id, &s); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "SLO not found")
		return
	}
	respondSuccess(w, s)
}

// SLOUpdateHandler PUT /api/v1/slos/{id}
func (h *Handlers) SLOUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var existing models.SLO
	if err := h.store.GetSLO(id, &existing); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "SLO not found")
		return
	}
	var patch models.SLO
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if patch.Name != "" {
		existing.Name = patch.Name
	}
	if patch.Description != "" {
		existing.Description = patch.Description
	}
	if patch.Metric != "" {
		existing.Metric = patch.Metric
	}
	if patch.Target > 0 {
		existing.Target = patch.Target
	}
	if patch.Window != "" {
		existing.Window = patch.Window
	}
	if patch.Comparator != "" {
		existing.Comparator = patch.Comparator
	}
	if patch.Threshold != 0 {
		existing.Threshold = patch.Threshold
	}
	existing.UpdatedAt = time.Now()

	if err := h.store.SaveSLO(id, existing); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	respondSuccess(w, existing)
}

// SLODeleteHandler DELETE /api/v1/slos/{id}
func (h *Handlers) SLODeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.store.DeleteSLO(id); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "SLO not found")
		return
	}
	respondSuccess(w, map[string]string{"status": "deleted"})
}

// SLOStatusHandler GET /api/v1/slos/{id}/status
// Computes live error budget, burn rate, compliance for a single SLO.
func (h *Handlers) SLOStatusHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var s models.SLO
	if err := h.store.GetSLO(id, &s); err != nil {
		respondError(w, http.StatusNotFound, "NOT_FOUND", "SLO not found")
		return
	}
	status := computeSLOStatus(h, s)
	respondSuccess(w, status)
}

// SLOAllStatusHandler GET /api/v1/slos/status
// Returns status for all SLOs.
func (h *Handlers) SLOAllStatusHandler(w http.ResponseWriter, r *http.Request) {
	var statuses []models.SLOStatus
	_ = h.store.ListSLOs(func(val []byte) error {
		var s models.SLO
		if err := json.Unmarshal(val, &s); err != nil {
			return nil
		}
		statuses = append(statuses, computeSLOStatus(h, s))
		return nil
	})
	if statuses == nil {
		statuses = []models.SLOStatus{}
	}
	respondSuccess(w, statuses)
}

// computeSLOStatus calculates the live SLO status from stored time-series data.
func computeSLOStatus(h *Handlers, s models.SLO) models.SLOStatus {
	windowDur := parseWindow(s.Window)
	now := time.Now()
	from := now.Add(-windowDur)

	// Fetch time-series for the metric
	var points []struct{ v float64 }
	tvs, err := h.store.GetMetricRangeTiered(h.hostname, s.Metric, from, now)
	if err == nil {
		for _, p := range tvs {
			points = append(points, struct{ v float64 }{p.Value})
		}
	}
	// Also try KPI range
	if len(points) == 0 {
		tvs, err = h.store.GetKPIRangeTiered(h.hostname, s.Metric, from, now)
		if err == nil {
			for _, p := range tvs {
				points = append(points, struct{ v float64 }{p.Value})
			}
		}
	}

	total := len(points)
	if total == 0 {
		return models.SLOStatus{
			SLO:   s,
			State: "no_data",
		}
	}

	// Count "good" samples
	good := 0
	for _, p := range points {
		if isGood(p.v, s.Comparator, s.Threshold) {
			good++
		}
	}

	compliance := float64(good) / float64(total) * 100.0
	targetFraction := s.Target / 100.0
	allowedBadFraction := 1.0 - targetFraction
	actualBadFraction := 1.0 - (float64(good) / float64(total))

	var burnRate float64
	if allowedBadFraction > 0 {
		burnRate = actualBadFraction / allowedBadFraction
	} else {
		burnRate = math.Inf(1)
	}
	if math.IsInf(burnRate, 1) || math.IsNaN(burnRate) {
		burnRate = 0
	}

	// Error budget = how much of the allowed bad minutes remain
	windowMinutes := windowDur.Minutes()
	allowedBadMinutes := windowMinutes * allowedBadFraction
	usedBadMinutes := windowMinutes * actualBadFraction
	remainingMinutes := allowedBadMinutes - usedBadMinutes
	errorBudgetPct := 0.0
	if allowedBadMinutes > 0 {
		errorBudgetPct = (remainingMinutes / allowedBadMinutes) * 100.0
	}
	errorBudgetPct = math.Max(0, math.Min(100, errorBudgetPct))

	var state string
	switch {
	case compliance >= s.Target:
		state = "healthy"
	case compliance >= s.Target*0.98:
		state = "at_risk"
	default:
		state = "breached"
	}

	return models.SLOStatus{
		SLO:              s,
		ErrorBudget:      math.Round(errorBudgetPct*100) / 100,
		BurnRate:         math.Round(burnRate*100) / 100,
		Compliance:       math.Round(compliance*100) / 100,
		RemainingMinutes: math.Round(remainingMinutes*10) / 10,
		State:            state,
	}
}

func isGood(v float64, comparator string, threshold float64) bool {
	switch comparator {
	case "gte":
		return v >= threshold
	default: // "lte"
		return v <= threshold
	}
}

func parseWindow(w string) time.Duration {
	switch w {
	case "7d":
		return 7 * 24 * time.Hour
	case "90d":
		return 90 * 24 * time.Hour
	default: // "30d"
		return 30 * 24 * time.Hour
	}
}
