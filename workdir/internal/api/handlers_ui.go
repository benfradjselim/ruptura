package api

// handlers_ui.go — endpoints consumed by the embedded Svelte dashboard (v7.0 UI).
//
// The Svelte UI was designed against a planned API that differs from the original
// host-centric v2 endpoints. These handlers bridge the gap by mapping the new
// endpoint shapes onto the existing storage and pipeline internals.

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/gorilla/mux"
)

// ── /api/v2/kpis ──────────────────────────────────────────────────────────────

// handleKPIs returns a flat KPI map for the Dashboard.svelte landing page.
// Optional ?host= query param selects a specific workload; omitting it picks the first snapshot.
func (h *Handlers) handleKPIs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, map[string]models.KPI{})
		return
	}

	host := r.URL.Query().Get("host")
	var snap models.KPISnapshot
	var ok bool

	if host != "" {
		snap, ok = h.store.LatestSnapshot(host)
	} else {
		all := h.store.AllSnapshots()
		if len(all) > 0 {
			snap = all[0]
			ok = true
		}
	}

	if !ok {
		writeJSON(w, http.StatusOK, map[string]models.KPI{})
		return
	}

	h.enrichSnapshot(&snap)
	writeJSON(w, http.StatusOK, kpiMap(snap))
}

func kpiMap(s models.KPISnapshot) map[string]models.KPI {
	return map[string]models.KPI{
		"stress":       s.Stress,
		"fatigue":      s.Fatigue,
		"mood":         s.Mood,
		"pressure":     s.Pressure,
		"humidity":     s.Humidity,
		"contagion":    s.Contagion,
		"resilience":   s.Resilience,
		"entropy":      s.Entropy,
		"velocity":     s.Velocity,
		"health_score": s.HealthScore,
	}
}

// ── /api/v2/fleet ─────────────────────────────────────────────────────────────

type fleetHost struct {
	Host         string    `json:"host"`
	State        string    `json:"state"`
	HealthScore  float64   `json:"health_score"`
	Stress       float64   `json:"stress"`
	Fatigue      float64   `json:"fatigue"`
	Contagion    float64   `json:"contagion"`
	ActiveAlerts int       `json:"active_alerts"`
	LastSeen     time.Time `json:"last_seen"`
}

type fleetResponse struct {
	TotalHosts    int         `json:"total_hosts"`
	HealthyHosts  int         `json:"healthy_hosts"`
	DegradedHosts int         `json:"degraded_hosts"`
	CriticalHosts int         `json:"critical_hosts"`
	Hosts         []fleetHost `json:"hosts"`
}

func snapshotState(snap models.KPISnapshot) string {
	hs := snap.HealthScore.Value
	if hs >= 70 {
		return "healthy"
	}
	if hs >= 40 {
		return "degraded"
	}
	return "critical"
}

func (h *Handlers) handleFleet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, fleetResponse{Hosts: []fleetHost{}})
		return
	}

	snapshots := h.store.AllSnapshots()
	resp := fleetResponse{Hosts: make([]fleetHost, 0, len(snapshots))}

	for i := range snapshots {
		h.enrichSnapshot(&snapshots[i])
		s := snapshots[i]

		name := s.Host
		if s.Workload.Namespace != "" {
			name = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
		}

		state := snapshotState(s)
		resp.TotalHosts++
		switch state {
		case "healthy":
			resp.HealthyHosts++
		case "degraded":
			resp.DegradedHosts++
		default:
			resp.CriticalHosts++
		}

		resp.Hosts = append(resp.Hosts, fleetHost{
			Host:        name,
			State:       state,
			HealthScore: s.HealthScore.Value,
			Stress:      s.Stress.Value,
			Fatigue:     s.Fatigue.Value,
			Contagion:   s.Contagion.Value,
			LastSeen:    s.Timestamp,
		})
	}

	// Critical first, then degraded, then healthy; within a tier sort by health_score ascending.
	sort.Slice(resp.Hosts, func(i, j int) bool {
		order := map[string]int{"critical": 0, "degraded": 1, "healthy": 2}
		if order[resp.Hosts[i].State] != order[resp.Hosts[j].State] {
			return order[resp.Hosts[i].State] < order[resp.Hosts[j].State]
		}
		return resp.Hosts[i].HealthScore < resp.Hosts[j].HealthScore
	})

	writeJSON(w, http.StatusOK, resp)
}

// ── /api/v2/alerts ────────────────────────────────────────────────────────────

func (h *Handlers) handleAlertList(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		writeJSON(w, http.StatusOK, []models.AnomalyEvent{})
		return
	}
	since := time.Now().Add(-24 * time.Hour)
	var events []models.AnomalyEvent
	for _, hk := range h.pipeline.AllHosts() {
		events = append(events, h.pipeline.RecentAnomalies(hk, since)...)
	}
	if events == nil {
		events = []models.AnomalyEvent{}
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})
	writeJSON(w, http.StatusOK, events)
}

func (h *Handlers) handleAlertGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func (h *Handlers) handleAlertOp(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ── /api/v2/predict ───────────────────────────────────────────────────────────
// Returns { predictions: [...] } matching the Dashboard.svelte fmtPred shape.

type predEntry struct {
	Target         string  `json:"target"`
	Current        float64 `json:"current"`
	Predicted      float64 `json:"predicted"`
	Trend          string  `json:"trend"`
	HorizonMinutes int     `json:"horizon_minutes"`
}

func (h *Handlers) handlePredict(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	horizon := uiParseInt(r.URL.Query().Get("horizon"), 120)

	if h.predictor == nil || h.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"predictions": []predEntry{}})
		return
	}

	// Resolve host from first snapshot when not provided.
	if host == "" {
		if all := h.store.AllSnapshots(); len(all) > 0 {
			s := all[0]
			if s.Workload.Namespace != "" {
				host = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			} else {
				host = s.Host
			}
		}
	}
	if host == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"predictions": []predEntry{}})
		return
	}

	signals := []string{
		"stress", "fatigue", "mood", "pressure", "humidity", "contagion",
		"resilience", "entropy", "velocity", "health_score",
	}

	snap, snapOK := h.store.LatestSnapshot(host)
	if snapOK {
		h.enrichSnapshot(&snap)
	}

	preds := make([]predEntry, 0, len(signals))
	for _, sig := range signals {
		cur := 0.0
		if snapOK {
			cur = signalValue(snap, sig)
		}

		pred := cur
		trend := "stable"
		if res, ok := h.predictor.Forecast(host, sig, horizon); ok {
			if len(res.Points) > 0 {
				pred = res.Points[len(res.Points)-1].Mean
			}
			trend = res.Trend
		}

		preds = append(preds, predEntry{
			Target:         sig,
			Current:        cur,
			Predicted:      pred,
			Trend:          trend,
			HorizonMinutes: horizon,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"predictions": preds})
}

// ── /api/v2/traces ────────────────────────────────────────────────────────────

func (h *Handlers) handleTraceSearch(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"traces": []interface{}{}})
}

func (h *Handlers) handleTraceGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "trace not found"})
}

// ── /api/v2/auth/* ────────────────────────────────────────────────────────────

func (h *Handlers) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	// Always report "already configured" so the Svelte login page shows the login form.
	writeJSON(w, http.StatusConflict, map[string]string{"error": "already configured"})
}

func (h *Handlers) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if h.apiKey != "" && creds.Password != h.apiKey {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": h.apiKey,
		"user":  map[string]string{"username": creds.Username, "role": "admin"},
	})
}

func (h *Handlers) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handlers) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"token": h.apiKey})
}

func (h *Handlers) handleAuthUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []interface{}{
		map[string]string{"username": "admin", "role": "admin"},
	})
}

// ── Stub CRUD handlers ────────────────────────────────────────────────────────
// Dashboards, SLOs, alert rules, notifications, datasources, orgs.
// These return valid empty responses so Svelte pages render a clean empty state.
// Persistent storage for these resources will be added in a later release.

func stubList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []interface{}{})
}

func stubCreate(w http.ResponseWriter, r *http.Request) {
	var body interface{}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body == nil {
		body = map[string]interface{}{}
	}
	writeJSON(w, http.StatusCreated, body)
}

func stubGetOrDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodDelete {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

func stubOp(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// stubWithID is a helper that swallows the {id} var requirement from mux.
func stubWithID(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)
	stubGetOrDelete(w, r)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func uiParseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return def
	}
	return n
}
