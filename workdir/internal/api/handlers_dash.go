package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// handleHistory returns time-series history for all workloads or a specific one.
func (h *Handlers) handleHistory(w http.ResponseWriter, r *http.Request) {
	if h.historyMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	vars := mux.Vars(r)
	if key := vars["workload"]; key != "" {
		writeJSON(w, http.StatusOK, h.historyMgr.Get(key))
		return
	}
	writeJSON(w, http.StatusOK, h.historyMgr.All())
}

// handleEvents serves either:
//   - text/event-stream (SSE) when the client sends Accept: text/event-stream
//   - application/json  (REST) otherwise — returns the N most recent events
//
// SSE query params: ?namespace=&min_fused_r=&min_health_score=
// REST query param:  ?limit= (default 50, max 1000)
func (h *Handlers) handleEvents(w http.ResponseWriter, r *http.Request) {
	if h.eventBus == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	wantsSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	if !wantsSSE {
		// REST fallback — return recent events as JSON.
		n := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				if parsed > 1000 {
					parsed = 1000
				}
				n = parsed
			}
		}
		writeJSON(w, http.StatusOK, h.eventBus.Recent(n))
		return
	}

	// SSE stream.
	q := r.URL.Query()
	nsFilter := q.Get("namespace")
	minFusedR := 0.0
	if v := q.Get("min_fused_r"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			minFusedR = f
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering

	ch := h.eventBus.Subscribe()
	defer h.eventBus.Unsubscribe(ch)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	writeSSEEvent := func(payload interface{}) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	writeSSEEvent(map[string]string{"type": "connected", "ts": time.Now().Format(time.RFC3339)})

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			writeSSEEvent(map[string]string{"type": "heartbeat", "ts": time.Now().Format(time.RFC3339)})
		case ev, open := <-ch:
			if !open {
				return
			}
			if nsFilter != "" && !strings.HasPrefix(ev.Workload, nsFilter+"/") {
				continue
			}
			if ev.FusedR < minFusedR {
				continue
			}
			writeSSEEvent(ev)
		}
	}
}
