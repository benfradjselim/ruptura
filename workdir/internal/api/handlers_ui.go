package api

// handlers_ui.go — endpoints consumed by the embedded Svelte dashboard (v7.0 UI).
//
// The Svelte UI was designed against a planned API that differs from the original
// host-centric v2 endpoints. These handlers bridge the gap by mapping the new
// endpoint shapes onto the existing storage and pipeline internals.

import (
	"crypto/subtle"
	"encoding/json"
	"math"
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
	Host                string                 `json:"host"`
	State               string                 `json:"state"`
	HealthScore         float64                `json:"health_score"`
	Stress              float64                `json:"stress"`
	Fatigue             float64                `json:"fatigue"`
	Contagion           float64                `json:"contagion"`
	ActiveAlerts        int                    `json:"active_alerts"`
	LastSeen            time.Time              `json:"last_seen"`
	FusedRuptureIndex   float64                `json:"fused_rupture_index"`
	CalibrationProgress int                    `json:"calibration_progress"`
	HealthForecast      *models.HealthForecast `json:"health_forecast,omitempty"`
	RestartCount        int                    `json:"restart_count,omitempty"`
}

// crashLoopRestartThreshold is the container restart count (summed across a
// workload's pods, from the k8s API via the discovery informer) at which a
// workload is treated as demonstrably unstable regardless of what its
// resource-usage-derived composite score says. A handful of restarts can
// happen to a healthy workload (a node drain, a deploy, an OOM once under
// real load); a fleet-relevant crash loop is a sustained pattern. This
// mirrors Kubernetes' own CrashLoopBackOff heuristic, which also kicks in
// after repeated failures rather than the first one.
const crashLoopRestartThreshold = 5

type fleetResponse struct {
	TotalHosts    int         `json:"total_hosts"`
	HealthyHosts  int         `json:"healthy_hosts"`
	DegradedHosts int         `json:"degraded_hosts"`
	CriticalHosts int         `json:"critical_hosts"`
	Hosts         []fleetHost `json:"hosts"`
}

// snapshotState derives the fleet-facing state for a workload from its
// HealthScore and its pods' container restart count. HealthScore.Value is
// stored on a [0,1] scale (see analyzer.go's healthScore :=
// utils.Clamp(1-penalty, 0, 1)) — comparing it against 70/40 thresholds (a
// [0,100]-scale bug) previously meant every real workload's health score
// (typically 0.7-0.9) failed both comparisons and fell through to
// "critical" regardless of how healthy it actually was. A NaN health score
// (never computed) fails every ordered comparison in Go and must not
// silently fall through to "critical" either — it means the same thing an
// incomplete calibration does: not enough data yet.
//
// restarts overrides state to "critical" once it crosses
// crashLoopRestartThreshold, independent of the resource-usage-derived
// health score: a workload that crash-loops but uses negligible CPU/memory
// between crashes (the exact shape of the demo-crashloop/demo-oom scenarios)
// is otherwise structurally invisible to every other signal this function
// computes — none of stress/fatigue/mood/pressure/humidity/contagion/
// resilience/entropy/velocity/throughput is sourced from container restart
// counts. This check runs even during calibration, since a crash loop is
// itself real, actionable information a user shouldn't have to wait 96
// observations to see.
func snapshotState(snap models.KPISnapshot, restarts int) string {
	if restarts >= crashLoopRestartThreshold {
		return "critical"
	}
	if snap.CalibrationProgress < 100 {
		return "calibrating"
	}
	hs := snap.HealthScore.Value
	if math.IsNaN(hs) {
		return "calibrating"
	}
	if hs >= 0.70 {
		return "healthy"
	}
	if hs >= 0.40 {
		return "degraded"
	}
	return "critical"
}

// workloadRestartCount sums container restart counts across every pod the
// discovery informer currently associates with a workload. Returns 0 when
// the informer isn't wired up (bare-metal/VM/demo mode — no k8s API to ask)
// or the workload isn't found, matching every other discovery-optional path
// in this file (e.g. enrichSnapshot's nil-analyzer guard).
func (h *Handlers) workloadRestartCount(ns, kind, name string) int {
	if h.discovery == nil {
		return 0
	}
	meta, ok := h.discovery.GetWorkloadMeta(ns, kind, name)
	if !ok {
		return 0
	}
	total := 0
	for _, p := range meta.Pods {
		total += p.Restarts
	}
	return total
}

func (h *Handlers) handleFleet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, fleetResponse{Hosts: []fleetHost{}})
		return
	}

	snapshots := h.store.AllSnapshots()
	resp := fleetResponse{Hosts: make([]fleetHost, 0, len(snapshots))}

	// Track hosts already present in the store so we can add pending-only ones below.
	knownHosts := make(map[string]bool, len(snapshots))

	for i := range snapshots {
		h.enrichSnapshot(&snapshots[i])
		s := snapshots[i]

		name := s.Host
		restarts := 0
		if s.Workload.Namespace != "" {
			name = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			restarts = h.workloadRestartCount(s.Workload.Namespace, s.Workload.Kind, s.Workload.Name)
		}
		knownHosts[name] = true

		state := snapshotState(s, restarts)
		resp.TotalHosts++
		// Explicit cases only — a bare "default: critical" here previously
		// counted every "calibrating" workload (state=calibrating is grey,
		// never red — see snapshotState) as critical in the fleet-wide
		// tally, even though its own per-host State field correctly said
		// "calibrating". That's how a fleet that was mostly still warming
		// up could show 0 healthy and N critical.
		switch state {
		case "healthy":
			resp.HealthyHosts++
		case "degraded":
			resp.DegradedHosts++
		case "critical":
			resp.CriticalHosts++
		}

		resp.Hosts = append(resp.Hosts, fleetHost{
			Host:                name,
			State:               state,
			HealthScore:         s.HealthScore.Value,
			Stress:              s.Stress.Value,
			Fatigue:             s.Fatigue.Value,
			Contagion:           s.Contagion.Value,
			LastSeen:            s.Timestamp,
			FusedRuptureIndex:   s.FusedRuptureIndex,
			CalibrationProgress: s.CalibrationProgress,
			HealthForecast:      s.HealthForecast,
			RestartCount:        restarts,
		})
	}

	// Merge auto-discovered workloads that have no telemetry yet (pending_telemetry).
	if h.analyzer != nil {
		for _, s := range h.analyzer.AllAnalyzerSnapshots() {
			if s.WorkloadStatus != "pending_telemetry" {
				continue
			}
			name := s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			if knownHosts[name] {
				continue
			}
			resp.TotalHosts++
			resp.Hosts = append(resp.Hosts, fleetHost{
				Host:        name,
				State:       "pending_telemetry",
				HealthScore: 0,
				LastSeen:    s.Timestamp,
			})
		}
	}

	// Critical first, then degraded, then healthy, then still-warming-up
	// states last; within a tier sort by health_score ascending. Unlisted
	// states would otherwise default to Go's zero value (0) and sort
	// alongside "critical" — calibrating/pending_telemetry workloads are
	// explicitly listed so they never do that.
	sort.Slice(resp.Hosts, func(i, j int) bool {
		order := map[string]int{"critical": 0, "degraded": 1, "healthy": 2, "calibrating": 3, "pending_telemetry": 4}
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
	if h.apiKey != "" && subtle.ConstantTimeCompare([]byte(creds.Password), []byte(h.apiKey)) != 1 {
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
