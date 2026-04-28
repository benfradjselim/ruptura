package api

import (
    "net/http"
    "sync/atomic"
    "time"
)

func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
    writeJSON(w, http.StatusOK, h.health.Check(time.Now()))
}

func (h *Handlers) handleReady(w http.ResponseWriter, r *http.Request) {
    if atomic.LoadInt32(&h.ready) == 1 {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

func (h *Handlers) handleMetrics(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte(h.metrics.Render()))
}

func (h *Handlers) handleTimeline(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte("<html><body>Timeline</body></html>"))
}
