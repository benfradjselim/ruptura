package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/benfradjselim/ruptura/internal/sim"
)

// handleSimInject reports a synthetic failure on a workload. Only an engine in
// demo mode accepts it, so a real cluster never sends fake alerts.
func (h *Handlers) handleSimInject(w http.ResponseWriter, r *http.Request) {
	if h.injector == nil {
		writeError(w, http.StatusConflict, "synthetic failures are only accepted in demo mode")
		return
	}
	var req sim.InjectRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
		return
	}
	if err := h.injector.Inject(req.Workload, req.Pattern, time.Now()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}
