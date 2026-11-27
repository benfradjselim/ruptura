package api

// handlers_ui.go — endpoints consumed by the embedded Svelte dashboard (v7.0 UI).
//
// The Svelte UI was designed against a planned API that differs from the original
// host-centric v2 endpoints. These handlers bridge the gap by mapping the new
// endpoint shapes onto the existing storage and pipeline internals.

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/gorilla/mux"
)

// ── /api/v2/kpis ──────────────────────────────────────────────────────────────

// handleKPIs returns a flat KPI map for the Dashboard.svelte landing page.
// Optional ?host= query param selects a specific workload; omitting it picks the first snapshot.
// ── /api/v2/fleet ─────────────────────────────────────────────────────────────

type fleetHost struct {
	Host         string         `json:"host"`
	State        string         `json:"state"`
	Risk         float64        `json:"risk"`
	RestartCount int            `json:"restart_count,omitempty"`
	ETFSeconds   int            `json:"etf_seconds,omitempty"`
	FailureMode  string         `json:"failure_mode,omitempty"`
	Failure      *FailureResult `json:"failure,omitempty"`
}

// crashLoopRestartThreshold is the restart count at which a workload counts
// as crash looping, as Kubernetes' own CrashLoopBackOff does after repeated
// failures rather than the first one.
const crashLoopRestartThreshold = 5

type fleetResponse struct {
	TotalHosts    int         `json:"total_hosts"`
	HealthyHosts  int         `json:"healthy_hosts"`
	DegradedHosts int         `json:"degraded_hosts"`
	CriticalHosts int         `json:"critical_hosts"`
	Hosts         []fleetHost `json:"hosts"`
}

// WorkloadLister lists the workloads a failure source knows.
type WorkloadLister interface {
	Workloads(now time.Time) []string
}

// workloadRestartCount sums container restarts across the pods the informer
// associates with a workload; 0 without an informer.
func (h *Handlers) workloadRestartCount(workload string) int {
	if h.discovery == nil {
		return 0
	}
	parts := strings.SplitN(workload, "/", 3)
	if len(parts) != 3 {
		return 0
	}
	meta, ok := h.discovery.GetWorkloadMeta(parts[0], parts[1], parts[2])
	if !ok {
		return 0
	}
	total := 0
	for _, p := range meta.Pods {
		total += p.Restarts
	}
	return total
}

// fleetWorkloads lists the workloads Kubernetes reports and those the failure
// source knows, as namespace/Kind/name.
func (h *Handlers) fleetWorkloads(now time.Time) []string {
	seen := map[string]bool{}
	if h.discovery != nil {
		for _, ref := range h.discovery.KnownRefs() {
			seen[ref.Namespace+"/"+ref.Kind+"/"+ref.Name] = true
		}
	}
	if lister, ok := h.failureModes.(WorkloadLister); ok {
		for _, w := range lister.Workloads(now) {
			seen[w] = true
		}
	}
	out := make([]string, 0, len(seen))
	for w := range seen {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

func (h *Handlers) handleFleet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.buildFleet())
}

// buildFleet gives every workload a state from its failures: healthy,
// degraded or critical, or unknown when no failure source is wired.
func (h *Handlers) buildFleet() fleetResponse {
	now := time.Now()
	workloads := h.fleetWorkloads(now)
	resp := fleetResponse{Hosts: make([]fleetHost, 0, len(workloads))}
	for _, name := range workloads {
		restarts := h.workloadRestartCount(name)
		state := "healthy"
		if h.failureModes == nil {
			state = "unknown"
		}
		stateRestarts := h.stateRestarts(name, restarts)
		risk := 0.0
		if stateRestarts >= crashLoopRestartThreshold {
			state = "critical"
		}
		state, risk = applyCrashLoop(state, risk, stateRestarts)
		etfSeconds, failureMode := 0, ""
		var estimate *failuremodes.Estimate
		if est, ok := h.workloadEstimate(name); ok {
			state, risk = applyEstimate(state, risk, est, h.warningHorizon())
			etfSeconds, failureMode = int(est.ETF.Seconds()), string(est.Mode)
			estimate = reportedEstimate(est, h.warningHorizon())
			h.noteForecast(name, est)
		}
		resp.TotalHosts++
		switch state {
		case "healthy":
			resp.HealthyHosts++
		case "degraded":
			resp.DegradedHosts++
		case "critical":
			resp.CriticalHosts++
		}
		resp.Hosts = append(resp.Hosts, fleetHost{
			Host: name, State: state, Risk: risk, RestartCount: restarts,
			ETFSeconds: etfSeconds, FailureMode: failureMode,
			Failure: failureResult(name, state, estimate, stateRestarts >= crashLoopRestartThreshold),
		})
	}
	order := map[string]int{"critical": 0, "degraded": 1, "healthy": 2, "unknown": 3}
	sort.SliceStable(resp.Hosts, func(i, j int) bool {
		return order[resp.Hosts[i].State] < order[resp.Hosts[j].State]
	})
	return resp
}

// handleTraceSearch lists stored traces, newest first, optionally for one service.
func (h *Handlers) handleTraceSearch(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"traces": []interface{}{}})
		return
	}
	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 500 {
		limit = v
	}
	traces, err := h.store.QueryTraceList(r.URL.Query().Get("service"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"traces": traces})
}

// handleTraceGet returns the spans of one trace.
func (h *Handlers) handleTraceGet(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}
	spans, err := h.store.QuerySpansByTrace(mux.Vars(r)["id"])
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(spans) == 0 {
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"spans": spans})
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

// stubWithID is a helper that swallows the {id} var requirement from mux.
func stubWithID(w http.ResponseWriter, r *http.Request) {
	_ = mux.Vars(r)
	stubGetOrDelete(w, r)
}

// ── helpers ───────────────────────────────────────────────────────────────────
