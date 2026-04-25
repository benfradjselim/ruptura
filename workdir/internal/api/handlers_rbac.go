package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/utils"
	"github.com/gorilla/mux"
)

// ---------------------------------------------------------------------------
// RBAC: user → org assignment
// ---------------------------------------------------------------------------

// UserAssignOrgHandler PUT /api/v1/auth/users/{id}/org
// Assigns or moves a user to an org. Admin only.
func (h *Handlers) UserAssignOrgHandler(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var body struct {
		OrgID string `json:"org_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	var user models.User
	if err := h.store.GetUser(id, &user); err != nil {
		respondError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}

	// Validate org exists (skip for "default" which is always implicit)
	if body.OrgID != "" && body.OrgID != "default" {
		var org models.Org
		if err := h.store.GetOrg(body.OrgID, &org); err != nil {
			respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
			return
		}
	}

	user.OrgID = body.OrgID
	if err := h.store.SaveUser(id, user); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	user.Password = "" // never expose hash
	respondSuccess(w, user)
}

// ---------------------------------------------------------------------------
// Retention: stats + tiered range query
// ---------------------------------------------------------------------------

// RetentionStatsHandler GET /api/v1/retention/stats
// Returns the count of data points in each storage tier.
func (h *Handlers) RetentionStatsHandler(w http.ResponseWriter, r *http.Request) {
	raw := h.store.RetentionStats()
	stats := models.RetentionStats{
		RawMetrics:      raw["m:"],
		RawKPIs:         raw["k:"],
		Rollup5mMetrics: raw["r5:"],
		Rollup5mKPIs:    raw["kr5:"],
		Rollup1hMetrics: raw["r1h:"],
		Rollup1hKPIs:    raw["kr1h:"],
	}
	respondSuccess(w, stats)
}

// RetentionCompactHandler POST /api/v1/retention/compact
// Triggers an on-demand compaction pass (operator+).
func (h *Handlers) RetentionCompactHandler(w http.ResponseWriter, r *http.Request) {
	go h.store.Compact() // non-blocking
	respondSuccess(w, map[string]string{"status": "compaction started"})
}

// MetricRangeTieredHandler GET /api/v1/metrics/{name}/range
// Returns a time-series using the best-fit retention tier.
// Query params: host, from (RFC3339 or unix), to (RFC3339 or unix)
func (h *Handlers) MetricRangeTieredHandler(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	host := r.URL.Query().Get("host")
	if host == "" {
		host = h.hostname
	}

	from, to := parseTimeRange(r)

	points, err := h.store.GetMetricRangeTiered(host, name, from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(points))
	for _, p := range points {
		result = append(result, map[string]interface{}{
			"timestamp": p.Timestamp,
			"value":     p.Value,
		})
	}
	respondSuccess(w, map[string]interface{}{
		"metric": name,
		"host":   host,
		"from":   from,
		"to":     to,
		"points": result,
	})
}

// OrgMemberListHandler GET /api/v1/orgs/{id}/members
// Returns all users that belong to an org.
func (h *Handlers) OrgMemberListHandler(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["id"]

	// Validate org exists
	var org models.Org
	if err := h.store.GetOrg(orgID, &org); err != nil {
		respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
		return
	}

	var members []models.User
	_ = h.store.ListUsers(func(val []byte) error {
		var u models.User
		if err := json.Unmarshal(val, &u); err != nil {
			return nil
		}
		if u.OrgID == orgID {
			u.Password = ""
			members = append(members, u)
		}
		return nil
	})
	if members == nil {
		members = []models.User{}
	}
	respondSuccess(w, members)
}

// OrgInviteHandler POST /api/v1/orgs/{id}/members
// Creates a new user scoped to an org.
func (h *Handlers) OrgInviteHandler(w http.ResponseWriter, r *http.Request) {
	orgID := mux.Vars(r)["id"]

	var org models.Org
	if err := h.store.GetOrg(orgID, &org); err != nil {
		respondError(w, http.StatusNotFound, "ORG_NOT_FOUND", "org not found")
		return
	}

	var body struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if body.Username == "" {
		respondError(w, http.StatusBadRequest, "MISSING_USERNAME", "username required")
		return
	}
	role := body.Role
	if role == "" {
		role = "viewer"
	}

	user := models.User{
		ID:       utils.GenerateID(8),
		Username: body.Username,
		Role:     role,
		OrgID:    orgID,
	}
	if err := h.store.SaveUser(body.Username, user); err != nil {
		respondError(w, http.StatusInternalServerError, "STORE_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"success":   true,
		"data":      user,
		"timestamp": time.Now(),
	})
}
