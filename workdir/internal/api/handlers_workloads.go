package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// handleWorkloadK8s serves GET /api/v2/workloads/{namespace}/{kind}/{name}/k8s.
// Returns the cached Kubernetes metadata (replicas, image, pods, resources, labels).
func (h *Handlers) handleWorkloadK8s(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns, kind, name := vars["namespace"], vars["kind"], vars["name"]

	if h.discovery == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "discovery not available — not running in-cluster or autodiscovery disabled",
		})
		return
	}

	meta, ok := h.discovery.GetWorkloadMeta(ns, kind, name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "workload not found in discovery cache",
		})
		return
	}

	writeJSON(w, http.StatusOK, meta)
}
