package api

import (
	"fmt"
	"net/http"
	"time"
)

func (h *Handlers) handleWrite(w http.ResponseWriter, r *http.Request) {
	h.metrics.IncIngestTotal("prometheus")
	w.WriteHeader(http.StatusNoContent)
}

// handleIngestPurge handles DELETE /api/v2/ingest/purge
// Query params: type=signals|kpis|all, from=RFC3339, to=RFC3339
func (h *Handlers) handleIngestPurge(w http.ResponseWriter, r *http.Request) {
	purgeType := r.URL.Query().Get("type")
	if purgeType == "" {
		purgeType = "all"
	}
	switch purgeType {
	case "signals", "kpis", "all":
	default:
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("invalid type %q — must be signals, kpis, or all", purgeType))
		return
	}

	var from, to time.Time
	var err error
	if fs := r.URL.Query().Get("from"); fs != "" {
		if from, err = time.Parse(time.RFC3339, fs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid from: must be RFC3339")
			return
		}
	}
	if ts := r.URL.Query().Get("to"); ts != "" {
		if to, err = time.Parse(time.RFC3339, ts); err != nil {
			writeError(w, http.StatusBadRequest, "invalid to: must be RFC3339")
			return
		}
	}

	if from.IsZero() {
		from = time.Time{}
	}
	if to.IsZero() {
		to = time.Now()
	}

	var purgedMetrics, purgedKPIs int64

	if purgeType == "signals" || purgeType == "all" {
		purgedMetrics, err = h.store.PurgeMetrics(from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "purge metrics failed: "+err.Error())
			return
		}
	}
	if purgeType == "kpis" || purgeType == "all" {
		purgedKPIs, err = h.store.PurgeKPIs(from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "purge KPIs failed: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"purged_metrics": purgedMetrics,
		"purged_kpis":    purgedKPIs,
		"type":           purgeType,
		"from":           from.Format(time.RFC3339),
		"to":             to.Format(time.RFC3339),
	})
}
