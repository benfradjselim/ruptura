package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// handleSimInject accepts a synthetic metric payload from ruptura-sim and feeds it
// directly into the analyzer + store pipeline. Intended for demos and local testing.
//
//	POST /api/v2/sim/inject
//	{"workload": "demo/deployment/api", "metrics": {"cpu_percent": 0.7, ...}}
func (h *Handlers) handleSimInject(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil || h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "analyzer not ready")
		return
	}

	var req struct {
		Workload string             `json:"workload"`
		Metrics  map[string]float64 `json:"metrics"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if req.Workload == "" || req.Metrics == nil {
		writeError(w, http.StatusBadRequest, "workload and metrics are required")
		return
	}

	ref := workloadRefFromSimKey(req.Workload)
	snap := h.analyzer.Update(ref, req.Metrics)
	h.store.StoreSnapshot(snap)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"workload":     ref.Key(),
		"health_score": snap.HealthScore.Value,
		"status":       snap.HealthScore.State,
	})
}

// workloadRefFromSimKey parses "namespace/kind/name" or "namespace/name" into a WorkloadRef.
func workloadRefFromSimKey(key string) models.WorkloadRef {
	parts := strings.SplitN(key, "/", 3)
	switch len(parts) {
	case 3:
		return models.WorkloadRef{Namespace: parts[0], Kind: parts[1], Name: parts[2]}
	case 2:
		return models.WorkloadRef{Namespace: parts[0], Kind: "Deployment", Name: parts[1]}
	default:
		return models.WorkloadRef{Namespace: "default", Kind: "Deployment", Name: key}
	}
}
