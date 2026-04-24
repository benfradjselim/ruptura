package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/benfradjselim/kairo-core/internal/storage"
)

// AuditLogHandler GET /api/v1/audit (admin only)
// Returns audit entries in reverse-chronological order. Supports query params:
//   - org_id  : filter by organisation (admin only; defaults to caller's org)
//   - username: filter by username
//   - from    : RFC3339 start time (default: 7 days ago)
//   - to      : RFC3339 end time (default: now)
//   - limit   : max entries (default 200, max 1000)
func (h *Handlers) AuditLogHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	orgID := q.Get("org_id")
	if orgID == "" {
		orgID = orgIDFromContext(r.Context())
	}
	username := q.Get("username")

	now := time.Now().UTC()
	from := now.Add(-7 * 24 * time.Hour)
	to := now

	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}

	limit := 200
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 1000 {
				n = 1000
			}
			limit = n
		}
	}

	entries, err := h.store.QueryAuditLog(orgID, username, from, to, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORAGE_ERROR", err.Error())
		return
	}
	if entries == nil {
		entries = []storage.AuditEntry{}
	}
	respondSuccess(w, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
		"from":    from,
		"to":      to,
	})
}
