package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/gorilla/mux"
)

func (h *Handlers) handleActions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Edition gate: approve is restricted to autopilot regardless of engine state.
	if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/approve") || strings.HasSuffix(r.URL.Path, "/rollback")) && h.edition != "autopilot" {
		writeJSON(w, http.StatusPaymentRequired, map[string]string{
			"error":   "action execution requires the Autopilot edition",
			"upgrade": "set RUPTURA_EDITION=autopilot to enable automated and manual action approval",
		})
		return
	}

	if h.engine == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

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
		return

	case http.MethodPost:
		switch {
		case strings.HasSuffix(r.URL.Path, "/approve"):
			if h.engine.Approve(id) {
				writeJSON(w, http.StatusOK, map[string]string{"status": "approved", "id": id})
			} else {
				writeError(w, http.StatusNotFound, "action not found: "+id)
			}
		case strings.HasSuffix(r.URL.Path, "/reject"):
			found := false
			for _, a := range h.engine.PendingActions() {
				if a.ID == id {
					found = true
					break
				}
			}
			if !found {
				writeError(w, http.StatusNotFound, "action not found: "+id)
				return
			}
			h.engine.Reject(id)
			writeJSON(w, http.StatusOK, map[string]string{"status": "rejected", "id": id})
		case strings.HasSuffix(r.URL.Path, "/rollback"):
			writeJSON(w, http.StatusOK, map[string]string{"status": "rollback_queued", "id": id})
		default:
			writeError(w, http.StatusMethodNotAllowed, "unsupported action operation")
		}
	}
}

func (h *Handlers) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	if h.engine != nil {
		h.engine.EmergencyStop()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"emergency_stop": true})
}

func (h *Handlers) handleLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	service := q.Get("service")

	limit := 200
	if s := q.Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 1000 {
				n = 1000
			}
			limit = n
		}
	}

	now := time.Now()
	from := now.Add(-24 * time.Hour)
	to := now

	if s := q.Get("from"); s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			from = time.UnixMilli(ms)
		}
	}
	if s := q.Get("to"); s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			to = time.UnixMilli(ms)
		}
	}

	entries, err := h.store.QueryLogs(service, from, to, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed: "+err.Error())
		return
	}
	if entries == nil {
		entries = []json.RawMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":    entries,
		"count":   len(entries),
		"service": service,
	})
}

func (h *Handlers) handleEngineStorage(w http.ResponseWriter, _ *http.Request) {
	type badgerSection struct {
		DiskBytes     int64 `json:"disk_bytes"`
		VlogSizeBytes int64 `json:"vlog_size_bytes"`
		NumTables     int   `json:"num_tables"`
		Keys          int64 `json:"keys"`
	}
	type storageResp struct {
		Badger    badgerSection    `json:"badger"`
		Retention map[string]int64 `json:"retention,omitempty"`
	}

	if h.store == nil {
		writeJSON(w, http.StatusOK, storageResp{})
		return
	}

	bs := h.store.Stats()
	writeJSON(w, http.StatusOK, storageResp{
		Badger: badgerSection{
			DiskBytes:     bs.DiskBytes,
			VlogSizeBytes: bs.VlogSizeBytes,
			NumTables:     bs.NumTables,
			Keys:          bs.Keys,
		},
		// FBL-A3-1 AC: per-prefix counts (including is:/prop:) must be
		// observable here, proving infra signal history is actually written.
		Retention: h.store.RetentionStats(),
	})
}
