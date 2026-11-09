package api

import (
    "net/http"
    "sync/atomic"
    "time"
)

func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
    resp := h.health.Check(time.Now())
    resp.Version = h.version
    resp.Edition = h.edition
    if resp.Edition == "" {
        resp.Edition = "community"
    }
    writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) handleReady(w http.ResponseWriter, r *http.Request) {
    if atomic.LoadInt32(&h.ready) == 1 {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
}

// prometheusTextFormat is the content type of the Prometheus text exposition format.
const prometheusTextFormat = "text/plain; version=0.0.4; charset=utf-8"

func (h *Handlers) handleMetrics(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", prometheusTextFormat)
    w.Write([]byte(h.metrics.Render()))
    if h.store != nil {
        w.Write([]byte(failureMetrics(h.Failures())))
    }
}

func (h *Handlers) handleTimeline(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte("<html><body>Timeline</body></html>"))
}
