package api

import (
	"net/http"

	"github.com/gorilla/mux"
)

// handleFusionState serves GET /api/v2/engine/fusion/{namespace}/{kind}/{name}.
// Returns the per-signal breakdown and dominant pipeline for the requested workload.
func (h *Handlers) handleFusionState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns, kind, name := vars["namespace"], vars["kind"], vars["name"]
	key := ns + "/" + kind + "/" + name

	if h.fusionEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "fusion engine not available",
		})
		return
	}

	state, err := h.fusionEng.StateByWorkload(key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, state)
}
