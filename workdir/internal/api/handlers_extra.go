package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/alerter"
	apicontext "github.com/benfradjselim/ruptura/internal/context"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/gorilla/mux"
)

// handleRupture returns the latest KPISnapshot for the given host.
func (h *Handlers) handleRupture(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	vars := mux.Vars(r)
	host := vars["host"]
	snap, ok := h.store.LatestSnapshot(host)
	if !ok {
		writeError(w, http.StatusNotFound, "no data for host: "+host)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handleRuptures returns all KPISnapshots for all known hosts.
func (h *Handlers) handleRuptures(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []models.KPISnapshot{})
		return
	}
	snapshots := h.store.AllSnapshots()
	if snapshots == nil {
		snapshots = []models.KPISnapshot{}
	}
	writeJSON(w, http.StatusOK, snapshots)
}

// handleRuptureByWorkload returns the latest KPISnapshot for a namespace/workload key.
func (h *Handlers) handleRuptureByWorkload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "store not available")
		return
	}
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	workload := vars["workload"]

	// Try Deployment first (most common), then other kinds, then bare namespace/workload.
	var snap models.KPISnapshot
	var ok bool
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "host"} {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + kind + "/" + workload)
		if ok {
			break
		}
	}
	if !ok {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + workload)
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no data for workload: "+namespace+"/"+workload)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// handleKPI returns a specific KPI from the latest snapshot for a host.
func (h *Handlers) handleKPI(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, models.KPI{})
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	host := vars["host"]

	snap, ok := h.store.LatestSnapshot(host)
	if !ok {
		writeError(w, http.StatusNotFound, "no data for host: "+host)
		return
	}

	var kpi models.KPI
	switch name {
	case "stress":
		kpi = snap.Stress
	case "fatigue":
		kpi = snap.Fatigue
	case "mood":
		kpi = snap.Mood
	case "pressure":
		kpi = snap.Pressure
	case "humidity":
		kpi = snap.Humidity
	case "contagion":
		kpi = snap.Contagion
	case "resilience":
		kpi = snap.Resilience
	case "entropy":
		kpi = snap.Entropy
	case "velocity":
		kpi = snap.Velocity
	case "health_score":
		kpi = snap.HealthScore
	case "throughput":
		kpi = snap.Throughput
	default:
		writeError(w, http.StatusBadRequest, "unknown KPI: "+name)
		return
	}
	writeJSON(w, http.StatusOK, kpi)
}

// handleKPIByWorkload returns a specific KPI for a namespace/workload key.
func (h *Handlers) handleKPIByWorkload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "store not available")
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	workload := vars["workload"]

	var snap models.KPISnapshot
	var ok bool
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "host"} {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + kind + "/" + workload)
		if ok {
			break
		}
	}
	if !ok {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + workload)
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no data for workload: "+namespace+"/"+workload)
		return
	}

	var kpi models.KPI
	switch name {
	case "stress":
		kpi = snap.Stress
	case "fatigue":
		kpi = snap.Fatigue
	case "mood":
		kpi = snap.Mood
	case "pressure":
		kpi = snap.Pressure
	case "humidity":
		kpi = snap.Humidity
	case "contagion":
		kpi = snap.Contagion
	case "resilience":
		kpi = snap.Resilience
	case "entropy":
		kpi = snap.Entropy
	case "velocity":
		kpi = snap.Velocity
	case "health_score":
		kpi = snap.HealthScore
	case "throughput":
		kpi = snap.Throughput
	default:
		writeError(w, http.StatusBadRequest, "unknown KPI: "+name)
		return
	}
	writeJSON(w, http.StatusOK, kpi)
}

// handleForecast returns a forecast stub (predictor not injected into Handlers).
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	host := vars["host"]
	horizon := 60 // default 60 minutes
	if hStr := r.URL.Query().Get("horizon"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 {
			horizon = v
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"host":    host,
		"metric":  metric,
		"horizon": horizon,
		"note":    "predictor not yet injected into Handlers — wire in cmd/ruptura/main.go",
	})
}

// handleActions returns action recommendations or handles action operations.
func (h *Handlers) handleActions(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	vars := mux.Vars(r)
	id := vars["id"]

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			actions := h.engine.PendingActions()
			if actions == nil {
				actions = []engine.ActionRecommendation{}
			}
			writeJSON(w, http.StatusOK, actions)
			return
		}
		for _, a := range h.engine.PendingActions() {
			if a.ID == id {
				writeJSON(w, http.StatusOK, a)
				return
			}
		}
		writeError(w, http.StatusNotFound, "action not found: "+id)

	case http.MethodPost:
		switch {
		case strings.HasSuffix(r.URL.Path, "/approve"):
			if h.engine.Approve(id) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
			} else {
				writeError(w, http.StatusNotFound, "action not found: "+id)
			}
		case strings.HasSuffix(r.URL.Path, "/reject"):
			h.engine.Reject(id)
			writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "id": id})
		case strings.HasSuffix(r.URL.Path, "/rollback"):
			writeJSON(w, http.StatusOK, map[string]string{"status": "rollback_queued", "id": id})
		default:
			writeError(w, http.StatusMethodNotAllowed, "unsupported action operation")
		}
	}
}

// handleSuppressions handles POST (create window) and GET (list windows).
func (h *Handlers) handleSuppressions(w http.ResponseWriter, r *http.Request) {
	if h.alerter == nil {
		writeJSON(w, http.StatusOK, []alerter.MaintenanceWindow{})
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req struct {
			WorkloadKey string `json:"workload_key"`
			From        string `json:"from"`  // RFC3339 or empty = now
			Until       string `json:"until"` // RFC3339, required
			Reason      string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		until, err := time.Parse(time.RFC3339, req.Until)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid until: "+err.Error())
			return
		}
		from := time.Now()
		if req.From != "" {
			if f, err := time.Parse(time.RFC3339, req.From); err == nil {
				from = f
			}
		}
		id := h.alerter.AddMaintenanceWindow(alerter.MaintenanceWindow{
			WorkloadKey: req.WorkloadKey,
			From:        from,
			Until:       until,
			Reason:      req.Reason,
		})
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})

	case http.MethodGet:
		windows := h.alerter.ListMaintenanceWindows()
		if windows == nil {
			windows = []alerter.MaintenanceWindow{}
		}
		writeJSON(w, http.StatusOK, windows)

	case http.MethodDelete:
		vars := mux.Vars(r)
		id := vars["id"]
		h.alerter.RemoveMaintenanceWindow(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleEmergencyStop triggers an emergency stop on the action engine.
func (h *Handlers) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"emergency_stop": true})
}

// handleContext manages manual context entries.
func (h *Handlers) handleContext(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		writeJSON(w, http.StatusCreated, apicontext.ContextEntry{ID: "c1"})
	} else {
		writeJSON(w, http.StatusOK, []apicontext.ContextEntry{})
	}
}

// handleDeleteContext removes a manual context entry.
func (h *Handlers) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// handleExplain returns structured explanation for a rupture event.
func (h *Handlers) handleExplain(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	vars := mux.Vars(r)
	id := vars["rupture_id"]

	switch {
	case strings.HasSuffix(r.URL.Path, "/formula"):
		audit, err := h.explainer.FormulaAudit(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, audit)

	case strings.HasSuffix(r.URL.Path, "/pipeline"):
		dbg, err := h.explainer.PipelineDebug(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, dbg)

	case strings.HasSuffix(r.URL.Path, "/narrative"):
		narrative, err := h.explainer.NarrativeExplain(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"narrative": narrative})

	default:
		exp, err := h.explainer.Explain(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// Also include narrative
		narrative, _ := h.explainer.NarrativeExplain(id)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"explain":   exp,
			"narrative": narrative,
		})
	}
}
