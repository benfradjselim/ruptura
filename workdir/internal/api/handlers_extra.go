package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
	"github.com/benfradjselim/ruptura/internal/alerter"
	apicontext "github.com/benfradjselim/ruptura/internal/context"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/gorilla/mux"
)

// enrichSnapshot attaches calibration status, HealthScore forecast, fingerprint match,
// and business signals to a snapshot. Safe to call when h.analyzer is nil (no-op).
func (h *Handlers) enrichSnapshot(snap *models.KPISnapshot) {
	if h.analyzer == nil {
		snap.WorkloadStatus = "active"
		snap.CalibrationProgress = 100
		return
	}
	status, progress, eta := h.analyzer.CalibrationInfo(snap.Workload)
	snap.WorkloadStatus = status
	snap.CalibrationProgress = progress
	snap.CalibrationETA = eta
	if status == "active" {
		snap.HealthForecast = h.analyzer.ForecastHealthScore(snap.Workload)
		// Pattern match: warn if current signal vector resembles a past rupture.
		if pm := h.analyzer.MatchFingerprint(*snap, snap.FusedRuptureIndex); pm != nil {
			snap.PatternMatch = pm
		}
	}
	// Business signals are always computed (blast_radius and recovery_debt are useful
	// even during calibration; slo_burn_velocity requires no baseline).
	biz := h.analyzer.ComputeBusinessSignals(snap.Workload, snap.FusedRuptureIndex)
	snap.Business = &biz
}

// handleAnomalies returns recent anomaly events, optionally filtered by host.
//
//	GET /api/v2/anomalies                  — all hosts, last 15 min
//	GET /api/v2/anomalies?since=<RFC3339>  — custom time window
//	GET /api/v2/anomalies/{host}           — single host
func (h *Handlers) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if h.pipeline == nil {
		writeJSON(w, http.StatusOK, []models.AnomalyEvent{})
		return
	}

	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-15 * time.Minute)
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	vars := mux.Vars(r)
	host := vars["host"]

	var events []models.AnomalyEvent
	if host != "" {
		events = h.pipeline.RecentAnomalies(host, since)
	} else {
		for _, hostKey := range h.pipeline.AllHosts() {
			events = append(events, h.pipeline.RecentAnomalies(hostKey, since)...)
		}
	}
	if events == nil {
		events = []models.AnomalyEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

// handleRupture returns the latest KPISnapshot for the given host.
func (h *Handlers) handleRupture(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	vars := mux.Vars(r)
	host := vars["host"]
	snap, ok := h.store.LatestSnapshot(host)
	if !ok {
		writeError(w, http.StatusNotFound, "no data for host: "+host)
		return
	}
	h.enrichSnapshot(&snap)
	sanitizeSnapshot(&snap)
	writeJSON(w, http.StatusOK, snap)
}

// handleDataflow returns cumulative ingest counts (metrics, logs, traces).
func (h *Handlers) handleDataflow(w http.ResponseWriter, r *http.Request) {
	var mCount, lCount, tCount int64
	if h.ingest != nil {
		mCount, lCount, tCount = h.ingest.IngestCounts()
	}
	writeJSON(w, http.StatusOK, map[string]int64{
		"metrics": mCount,
		"logs":    lCount,
		"traces":  tCount,
	})
}

// sanitizeSnapshot replaces NaN/Inf float64 values with 0 so json.Marshal never
// returns UnsupportedValueError when returning full KPISnapshot payloads.
func sanitizeSnapshot(s *models.KPISnapshot) {
	for _, k := range []*models.KPI{
		&s.Stress, &s.Fatigue, &s.Mood, &s.Pressure,
		&s.Humidity, &s.Contagion, &s.Resilience, &s.Entropy,
		&s.Velocity, &s.HealthScore, &s.Throughput,
	} {
		k.Value = safeF64(k.Value)
	}
	s.FusedRuptureIndex = safeF64(s.FusedRuptureIndex)
	if s.HealthForecast != nil {
		s.HealthForecast.In15Min = safeF64(s.HealthForecast.In15Min)
		s.HealthForecast.In30Min = safeF64(s.HealthForecast.In30Min)
	}
	if s.PatternMatch != nil {
		s.PatternMatch.Similarity = safeF64(s.PatternMatch.Similarity)
	}
	if s.Business != nil {
		s.Business.SLOBurnVelocity = safeF64(s.Business.SLOBurnVelocity)
	}
}

// handleRuptures returns all KPISnapshots for all known hosts.
func (h *Handlers) handleRuptures(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []models.KPISnapshot{})
		return
	}
	snapshots := h.store.AllSnapshots()
	if snapshots == nil {
		snapshots = []models.KPISnapshot{}
	}
	now := time.Now()
	for i := range snapshots {
		h.enrichSnapshot(&snapshots[i])
		sanitizeSnapshot(&snapshots[i])
		key := snapshots[i].Host
		if snapshots[i].Workload.Namespace != "" {
			key = snapshots[i].Workload.Namespace + "/" + snapshots[i].Workload.Kind + "/" + snapshots[i].Workload.Name
		}
		if h.historyMgr != nil {
			h.historyMgr.MaybePush(key, snapshots[i], now, 30*time.Second)
		}
		if h.eventBus != nil {
			h.eventBus.ObserveFusedR(key, snapshots[i].FusedRuptureIndex)
		}
	}
	writeJSON(w, http.StatusOK, snapshots)
}

// handleRuptureByWorkload3 returns the latest KPISnapshot for a namespace/kind/workload key.
func (h *Handlers) handleRuptureByWorkload3(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "store not available")
		return
	}
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	kind := vars["kind"]
	workload := vars["workload"]

	snap, ok := h.store.LatestSnapshot(namespace + "/" + kind + "/" + workload)
	if !ok {
		writeError(w, http.StatusNotFound, "no data for workload: "+namespace+"/"+kind+"/"+workload)
		return
	}
	h.enrichSnapshot(&snap)
	sanitizeSnapshot(&snap)
	writeJSON(w, http.StatusOK, snap)
}

// handleRuptureByWorkload returns the latest KPISnapshot for a namespace/workload key.
func (h *Handlers) handleRuptureByWorkload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "store not available")
		return
	}
	vars := mux.Vars(r)
	namespace := vars["namespace"]
	workload := vars["workload"]

	// Try Deployment first (most common), then other kinds, then bare namespace/workload.
	var snap models.KPISnapshot
	var ok bool
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "host"} {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + kind + "/" + workload)
		if ok {
			break
		}
	}
	if !ok {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + workload)
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no data for workload: "+namespace+"/"+workload)
		return
	}
	h.enrichSnapshot(&snap)
	sanitizeSnapshot(&snap)
	writeJSON(w, http.StatusOK, snap)
}

// handleKPI returns a specific KPI from the latest snapshot for a host.
func (h *Handlers) handleKPI(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, models.KPI{})
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	host := vars["host"]

	snap, ok := h.store.LatestSnapshot(host)
	if !ok {
		writeError(w, http.StatusNotFound, "no data for host: "+host)
		return
	}

	var kpi models.KPI
	switch name {
	case "stress":
		kpi = snap.Stress
	case "fatigue":
		kpi = snap.Fatigue
	case "mood":
		kpi = snap.Mood
	case "pressure":
		kpi = snap.Pressure
	case "humidity":
		kpi = snap.Humidity
	case "contagion":
		kpi = snap.Contagion
	case "resilience":
		kpi = snap.Resilience
	case "entropy":
		kpi = snap.Entropy
	case "velocity":
		kpi = snap.Velocity
	case "health_score":
		kpi = snap.HealthScore
	case "throughput":
		kpi = snap.Throughput
	default:
		writeError(w, http.StatusBadRequest, "unknown KPI: "+name)
		return
	}
	writeJSON(w, http.StatusOK, kpi)
}

// handleKPIByWorkload returns a specific KPI for a namespace/workload key.
func (h *Handlers) handleKPIByWorkload(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeError(w, http.StatusNotFound, "store not available")
		return
	}
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	workload := vars["workload"]

	var snap models.KPISnapshot
	var ok bool
	for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet", "Job", "host"} {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + kind + "/" + workload)
		if ok {
			break
		}
	}
	if !ok {
		snap, ok = h.store.LatestSnapshot(namespace + "/" + workload)
	}
	if !ok {
		writeError(w, http.StatusNotFound, "no data for workload: "+namespace+"/"+workload)
		return
	}

	var kpi models.KPI
	switch name {
	case "stress":
		kpi = snap.Stress
	case "fatigue":
		kpi = snap.Fatigue
	case "mood":
		kpi = snap.Mood
	case "pressure":
		kpi = snap.Pressure
	case "humidity":
		kpi = snap.Humidity
	case "contagion":
		kpi = snap.Contagion
	case "resilience":
		kpi = snap.Resilience
	case "entropy":
		kpi = snap.Entropy
	case "velocity":
		kpi = snap.Velocity
	case "health_score":
		kpi = snap.HealthScore
	case "throughput":
		kpi = snap.Throughput
	default:
		writeError(w, http.StatusBadRequest, "unknown KPI: "+name)
		return
	}
	writeJSON(w, http.StatusOK, kpi)
}

// handleForecast returns a real ensemble forecast (ILR + Holt-Winters + ARIMA).
func (h *Handlers) handleForecast(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	metric := vars["metric"]
	host := vars["host"]

	horizon := 60 // default 60 minutes
	if hStr := r.URL.Query().Get("horizon"); hStr != "" {
		if v, err := strconv.Atoi(hStr); err == nil && v > 0 {
			if v > 10080 {
				v = 10080 // cap at 1 week
			}
			horizon = v
		}
	}

	if r.Method == http.MethodPost {
		// POST /api/v2/forecast — batch forecast request body
		var req struct {
			Host    string `json:"host"`
			Metric  string `json:"metric"`
			Horizon int    `json:"horizon"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.Host != "" {
				host = req.Host
			}
			if req.Metric != "" {
				metric = req.Metric
			}
			if req.Horizon > 0 {
				horizon = req.Horizon
			}
		}
	}

	if h.predictor == nil || host == "" || metric == "" {
		writeError(w, http.StatusUnprocessableEntity, "predictor not available or missing host/metric params")
		return
	}

	result, ok := h.predictor.Forecast(host, metric, horizon)
	if !ok {
		// Predictor is warming up — return current stub with warming_up flag
		var current float64
		if h.store != nil {
			if snap, found := h.store.LatestSnapshot(host); found {
				current = signalValue(snap, metric)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"host":       host,
			"metric":     metric,
			"horizon":    horizon,
			"warming_up": true,
			"current":    current,
			"note":       "accumulating observations — forecast available after ~15 minutes of data",
		})
		return
	}

	// Bounded KPI signals are always in [0,1]. Clamp forecast output to
	// prevent corrupted ILR state from producing absurd extrapolations.
	if boundedKPIs[metric] {
		clampForecastResult(&result)
	}

	writeJSON(w, http.StatusOK, result)
}

// actionResp is the wire format for action recommendations returned to the UI.
// It extends ActionRecommendation with computed state/description fields.
type actionResp struct {
	engine.ActionRecommendation
	State       string `json:"state"`
	Description string `json:"description"`
}

func toActionResp(a engine.ActionRecommendation) actionResp {
	state := "pending"
	if a.Executed {
		state = "executed"
	} else if a.Approved {
		state = "approved"
	}
	desc := fmt.Sprintf("%s on %s", a.ActionType, a.Host)
	return actionResp{ActionRecommendation: a, State: state, Description: desc}
}

// handleActions returns action recommendations or handles action operations.
func (h *Handlers) handleActions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if h.engine == nil {
		writeJSON(w, http.StatusOK, []actionResp{})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if id == "" {
			raw := h.engine.PendingActions()
			resp := make([]actionResp, len(raw))
			for i, a := range raw {
				resp[i] = toActionResp(a)
			}
			writeJSON(w, http.StatusOK, resp)
			return
		}
		for _, a := range h.engine.PendingActions() {
			if a.ID == id {
				writeJSON(w, http.StatusOK, toActionResp(a))
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

// suppressionResp is the wire format for suppression objects returned to clients.
type suppressionResp struct {
	ID       string    `json:"id"`
	Workload string    `json:"workload"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Reason   string    `json:"reason,omitempty"`
}

func windowToResp(w alerter.MaintenanceWindow) suppressionResp {
	return suppressionResp{
		ID:       w.ID,
		Workload: w.WorkloadKey,
		Start:    w.From,
		End:      w.Until,
		Reason:   w.Reason,
	}
}

// handleSuppressions handles POST (create window) and GET (list windows).
func (h *Handlers) handleSuppressions(w http.ResponseWriter, r *http.Request) {
	if h.alerter == nil {
		writeJSON(w, http.StatusOK, []suppressionResp{})
		return
	}
	switch r.Method {
	case http.MethodPost:
		var req struct {
			Workload string    `json:"workload"`
			Start    time.Time `json:"start"`
			End      time.Time `json:"end"`
			Reason   string    `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if req.End.IsZero() {
			writeError(w, http.StatusBadRequest, "end time is required")
			return
		}
		if req.Start.IsZero() {
			req.Start = time.Now()
		}
		win := alerter.MaintenanceWindow{
			WorkloadKey: req.Workload,
			From:        req.Start,
			Until:       req.End,
			Reason:      req.Reason,
		}
		id := h.alerter.AddMaintenanceWindow(win)
		win.ID = id
		writeJSON(w, http.StatusCreated, windowToResp(win))

	case http.MethodGet:
		windows := h.alerter.ListMaintenanceWindows()
		resp := make([]suppressionResp, 0, len(windows))
		for _, win := range windows {
			resp = append(resp, windowToResp(win))
		}
		writeJSON(w, http.StatusOK, resp)

	case http.MethodDelete:
		vars := mux.Vars(r)
		id := vars["id"]
		h.alerter.RemoveMaintenanceWindow(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleEmergencyStop triggers an emergency stop on the action engine.
func (h *Handlers) handleEmergencyStop(w http.ResponseWriter, r *http.Request) {
	if h.engine != nil {
		h.engine.EmergencyStop()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"emergency_stop": true})
}

// signalValue extracts the named KPI signal value from a snapshot.
func signalValue(snap models.KPISnapshot, metric string) float64 {
	switch metric {
	case "stress":
		return snap.Stress.Value
	case "fatigue":
		return snap.Fatigue.Value
	case "mood":
		return snap.Mood.Value
	case "pressure":
		return snap.Pressure.Value
	case "humidity":
		return snap.Humidity.Value
	case "contagion":
		return snap.Contagion.Value
	case "resilience":
		return snap.Resilience.Value
	case "entropy":
		return snap.Entropy.Value
	case "velocity":
		return snap.Velocity.Value
	case "throughput":
		return snap.Throughput.Value
	default:
		return snap.HealthScore.Value
	}
}

// handleContext manages manual context entries.
func (h *Handlers) handleContext(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		writeJSON(w, http.StatusCreated, apicontext.ContextEntry{ID: "c1"})
	} else {
		writeJSON(w, http.StatusOK, []apicontext.ContextEntry{})
	}
}

// handleDeleteContext removes a manual context entry.
func (h *Handlers) handleDeleteContext(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// handleExplain returns structured explanation for a rupture event.
func (h *Handlers) handleExplain(w http.ResponseWriter, r *http.Request) {
	if h.explainer == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	vars := mux.Vars(r)
	id := vars["rupture_id"]
	// The UI encodes slashes as %2F via encodeURIComponent. Gorilla/mux may leave
	// them encoded in the variable; decode so lookups by workload key work correctly.
	if decoded, err := url.PathUnescape(id); err == nil {
		id = decoded
	}

	switch {
	case strings.HasSuffix(r.URL.Path, "/formula"):
		audit, err := h.explainer.FormulaAudit(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, audit)

	case strings.HasSuffix(r.URL.Path, "/pipeline"):
		dbg, err := h.explainer.PipelineDebug(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, dbg)

	case strings.HasSuffix(r.URL.Path, "/narrative"):
		narrative, err := h.explainer.NarrativeExplain(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"narrative": narrative})

	default:
		exp, err := h.explainer.Explain(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		// Also include narrative
		narrative, _ := h.explainer.NarrativeExplain(id)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"explain":   exp,
			"narrative": narrative,
		})
	}
}

// handleLogs returns stored OTLP log entries.
//
//	GET /api/v2/logs?service=<svc>&from=<unix-ms>&to=<unix-ms>&limit=<n>
//
// All query params are optional. Default limit is 200, max is 1000.
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

// handleConfigWeights manages per-workload HealthScore signal weight overrides.
//
//	GET  /api/v2/config/weights — list current weight configs
//	POST /api/v2/config/weights — replace the full list
func (h *Handlers) handleConfigWeights(w http.ResponseWriter, r *http.Request) {
	if h.analyzer == nil {
		writeError(w, http.StatusServiceUnavailable, "analyzer not available")
		return
	}
	switch r.Method {
	case http.MethodGet:
		cfgs := h.analyzer.WeightConfigs()
		if cfgs == nil {
			cfgs = []models.SignalWeights{}
		}
		writeJSON(w, http.StatusOK, cfgs)

	case http.MethodPost:
		var cfgs []models.SignalWeights
		if err := json.NewDecoder(r.Body).Decode(&cfgs); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		h.analyzer.SetWeightConfigs(cfgs)
		writeJSON(w, http.StatusOK, map[string]interface{}{"applied": len(cfgs)})
	}
}

// boundedKPIs lists KPI signals whose values are always in [0, 1].
// Used to clamp forecast output so that a corrupted ILR slope cannot
// produce absurd extrapolations visible in the UI.
var boundedKPIs = map[string]bool{
	"stress": true, "fatigue": true, "mood": true, "pressure": true,
	"humidity": true, "contagion": true, "entropy": true,
	"resilience": true, "velocity": true, "health_score": true,
	"error_rate": true,
}

func clampForecastResult(r *models.ForecastResult) {
	clamp01 := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	r.Current = clamp01(r.Current)
	for i := range r.Points {
		r.Points[i].Mean = clamp01(r.Points[i].Mean)
		r.Points[i].Lower80 = clamp01(r.Points[i].Lower80)
		r.Points[i].Upper80 = clamp01(r.Points[i].Upper80)
		r.Points[i].Lower95 = clamp01(r.Points[i].Lower95)
		r.Points[i].Upper95 = clamp01(r.Points[i].Upper95)
	}
	for i := range r.Models {
		r.Models[i].Mean = clamp01(r.Models[i].Mean)
	}
}
