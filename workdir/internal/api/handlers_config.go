package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/benfradjselim/ruptura/internal/storage"
)

// handleRetentionConfig returns or updates the data retention configuration.
//
//	GET  /api/v2/config/retention
//	PUT  /api/v2/config/retention
func (h *Handlers) handleRetentionConfig(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not available")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.store.GetRetentionConfig())
	case http.MethodPut:
		var cfg storage.RetentionConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := h.store.SetRetentionConfig(cfg); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	}
}

// handlePurge deletes raw ingested data from BadgerDB.
//
//	DELETE /api/v2/ingest/purge?type=metrics|logs|traces|snapshots|all&before=<RFC3339>
//
// Without 'before': all data of the given type is dropped immediately (fast).
// With 'before': only records older than that timestamp are removed.
func (h *Handlers) handlePurge(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusServiceUnavailable, "store not available")
		return
	}
	q := r.URL.Query()
	dataType := q.Get("type")
	if dataType == "" {
		dataType = "all"
	}
	valid := map[string]bool{
		"metrics": true, "logs": true, "traces": true, "snapshots": true, "all": true,
	}
	if !valid[dataType] {
		writeError(w, http.StatusBadRequest, "type must be one of: metrics, logs, traces, snapshots, all")
		return
	}

	var before *time.Time
	if s := q.Get("before"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			writeError(w, http.StatusBadRequest, "before must be RFC3339: "+err.Error())
			return
		}
		before = &t
	}

	n, err := h.store.PurgeData(dataType, before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purge failed: "+err.Error())
		return
	}

	resp := map[string]interface{}{"type": dataType, "ok": true}
	if n == -1 {
		resp["deleted"] = "all"
	} else {
		resp["deleted"] = n
	}
	writeJSON(w, http.StatusOK, resp)
}
