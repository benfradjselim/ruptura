package api

import "net/http"

func (h *Handlers) handleWrite(w http.ResponseWriter, r *http.Request) {
    h.metrics.IncIngestTotal("prometheus")
    w.WriteHeader(http.StatusNoContent)
}
