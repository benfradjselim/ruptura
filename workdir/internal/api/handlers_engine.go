package api

import (
	"math"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// handleFusionState serves GET /api/v2/engine/fusion/{namespace}/{kind}/{name}.
// Returns the per-signal breakdown and dominant pipeline for the requested workload.
func (h *Handlers) handleFusionState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	ns, kind, name := vars["namespace"], vars["kind"], vars["name"]
	key := ns + "/" + kind + "/" + name

	if h.fusionEng == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "fusion engine not available",
		})
		return
	}

	state, err := h.fusionEng.StateByWorkload(key)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, state)
}

type topologyNode struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Namespace   string  `json:"namespace"`
	Kind        string  `json:"kind"`
	HealthScore float64 `json:"health_score"`
	FusedR      float64 `json:"fused_r"`
	State       string  `json:"state"`
	// Live KPI signals — used by the UI for pulse intensity and detail panel
	Stress    float64 `json:"stress"`
	Fatigue   float64 `json:"fatigue"`
	Contagion float64 `json:"contagion"`
	Mood      float64 `json:"mood"`
	Velocity  float64 `json:"velocity"`
	Entropy   float64 `json:"entropy"`
}

type topologyEdge struct {
	Source       string  `json:"source"`
	Target       string  `json:"target"`
	CallRate     int64   `json:"call_rate"`
	ErrorRate    float64 `json:"error_rate"`
	P99LatencyMS float64 `json:"p99_latency_ms"`
	// EdgeType: "trace" = derived from OTLP span parent-child, "inferred" = from KPI correlation
	EdgeType string  `json:"edge_type"`
	// Strength: 0–1 confidence/weight of the edge (1.0 = trace-confirmed)
	Strength float64 `json:"strength"`
}

type topologyResponse struct {
	Nodes []topologyNode `json:"nodes"`
	Edges []topologyEdge `json:"edges"`
}

// handleTopology serves GET /api/v2/topology.
//
// Nodes: all known workloads with full KPI signals.
// Edges: two sources:
//   - "trace" — confirmed from OTLP span parent-child relationships
//   - "inferred" — derived from KPI signal correlation across live snapshots
//     (contagion spread + co-degradation patterns). Active when no trace data is available.
func (h *Handlers) handleTopology(w http.ResponseWriter, r *http.Request) {
	resp := topologyResponse{
		Nodes: []topologyNode{},
		Edges: []topologyEdge{},
	}

	// Build node set from live snapshots with full KPI signals.
	type snapEntry struct {
		id   string
		node topologyNode
	}
	var snapList []snapEntry

	if h.store != nil {
		snapshots := h.store.AllSnapshots()
		for i := range snapshots {
			h.enrichSnapshot(&snapshots[i])
			s := snapshots[i]
			id := s.Host
			label := s.Host
			ns := ""
			kind := ""
			if s.Workload.Namespace != "" {
				ns = s.Workload.Namespace
				kind = s.Workload.Kind
				id = ns + "/" + kind + "/" + s.Workload.Name
				label = s.Workload.Name
			}
			n := topologyNode{
				ID:          id,
				Label:       label,
				Namespace:   ns,
				Kind:        kind,
				HealthScore: safeF64(s.HealthScore.Value),
				FusedR:      safeF64(s.FusedRuptureIndex),
				State:       snapshotState(s),
				Stress:      safeF64(s.Stress.Value),
				Fatigue:     safeF64(s.Fatigue.Value),
				Contagion:   safeF64(s.Contagion.Value),
				Mood:        safeF64(s.Mood.Value),
				Velocity:    safeF64(s.Velocity.Value),
				Entropy:     safeF64(s.Entropy.Value),
			}
			resp.Nodes = append(resp.Nodes, n)
			snapList = append(snapList, snapEntry{id: id, node: n})
		}
	}

	// Merge auto-discovered pending workloads not yet sending telemetry.
	if h.analyzer != nil {
		seen := make(map[string]bool, len(resp.Nodes))
		for _, n := range resp.Nodes {
			seen[n.ID] = true
		}
		for _, s := range h.analyzer.AllAnalyzerSnapshots() {
			if s.WorkloadStatus != "pending_telemetry" {
				continue
			}
			id := s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			if seen[id] {
				continue
			}
			resp.Nodes = append(resp.Nodes, topologyNode{
				ID:        id,
				Label:     s.Workload.Name,
				Namespace: s.Workload.Namespace,
				Kind:      s.Workload.Kind,
				State:     "pending_telemetry",
			})
		}
	}

	// Trace-confirmed edges from OTLP span parent-child relationships.
	hasTraceEdges := false
	if h.topoBuilder != nil {
		for _, e := range h.topoBuilder.Edges() {
			var errRate float64
			if e.Calls > 0 {
				errRate = float64(e.Errors) / float64(e.Calls)
			}
			resp.Edges = append(resp.Edges, topologyEdge{
				Source:       e.From,
				Target:       e.To,
				CallRate:     e.Calls,
				ErrorRate:    errRate,
				P99LatencyMS: e.AvgLatMS,
				EdgeType:     "trace",
				Strength:     1.0,
			})
			hasTraceEdges = true
		}
	}

	// Inferred edges from KPI correlation — active when no trace edges observed yet.
	// Strategy: for each pair of workloads in the same namespace with live telemetry,
	// detect co-degradation patterns using signal correlation:
	//   - If A has high contagion AND B has high stress → B likely calls A (B stresses A via calls)
	//   - If two services share similar health score trajectories (Δ < 15) → mutual dependency
	// Both patterns produce a directed edge with a strength ∈ [0, 1].
	if !hasTraceEdges && len(snapList) > 1 {
		edgeSeen := make(map[string]bool)
		for i := 0; i < len(snapList); i++ {
			for j := 0; j < len(snapList); j++ {
				if i == j {
					continue
				}
				a := snapList[i].node
				b := snapList[j].node
				// Only infer within same namespace; cross-ns edges need traces.
				if a.Namespace == "" || a.Namespace != b.Namespace {
					continue
				}
				// Skip if both are pending_telemetry.
				if a.State == "pending_telemetry" && b.State == "pending_telemetry" {
					continue
				}
				edgeKey := b.ID + "→" + a.ID
				if edgeSeen[edgeKey] {
					continue
				}

				// Signal 1: contagion spread — B (caller, high stress) → A (callee, high contagion)
				// A's contagion is elevated because B is calling it and failing.
				contagionSignal := b.Stress * a.Contagion
				if contagionSignal < 0.05 {
					contagionSignal = 0
				}

				// Signal 2: co-degradation — both services degrade together → mutual dependency
				healthDelta := math.Abs(a.HealthScore-b.HealthScore) / 100.0
				coDegradation := 0.0
				if a.HealthScore < 80 && b.HealthScore < 80 {
					// Both degraded and close in score → shared dependency likely
					coDegradation = (1 - healthDelta) * (1 - math.Min(a.HealthScore, b.HealthScore)/100)
				}

				strength := math.Max(contagionSignal, coDegradation*0.7)
				if strength < 0.08 {
					continue // too weak, skip
				}
				if strength > 1 {
					strength = 1
				}

				edgeSeen[edgeKey] = true
				resp.Edges = append(resp.Edges, topologyEdge{
					Source:   b.ID,
					Target:   a.ID,
					EdgeType: "inferred",
					Strength: strength,
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleEngineStatus serves GET /api/v2/engine/status.
func (h *Handlers) handleEngineStatus(w http.ResponseWriter, _ *http.Request) {
	uptime := time.Since(h.startTime).Seconds()

	type analyzerSection struct {
		TickIntervalMS      int `json:"tick_interval_ms"`
		LastTickAgoMS       int `json:"last_tick_ago_ms"`
		ActiveWorkloads     int `json:"active_workloads"`
		CalibratingWorkloads int `json:"calibrating_workloads"`
		PendingWorkloads    int `json:"pending_workloads"`
	}
	type ingestSection struct {
		MetricsPerSec float64 `json:"metrics_per_sec"`
		LogsPerSec    float64 `json:"logs_per_sec"`
		TracesPerSec  float64 `json:"traces_per_sec"`
	}
	type actionsSection struct {
		PendingTier1      int `json:"pending_tier1"`
		PendingTier2      int `json:"pending_tier2"`
		ExecutedLastHour  int `json:"executed_last_hour"`
	}
	type statusResp struct {
		Analyzer      analyzerSection `json:"analyzer"`
		Ingest        ingestSection   `json:"ingest"`
		Actions       actionsSection  `json:"actions"`
		Version       string          `json:"version"`
		Edition       string          `json:"edition"`
		UptimeSeconds int64           `json:"uptime_seconds"`
	}

	resp := statusResp{
		Version:       h.version,
		Edition:       h.edition,
		UptimeSeconds: int64(uptime),
		Analyzer: analyzerSection{
			TickIntervalMS: 15000,
			LastTickAgoMS:  -1,
		},
	}

	if h.analyzer != nil {
		st := h.analyzer.Stats()
		resp.Analyzer.ActiveWorkloads = st.ActiveWorkloads
		resp.Analyzer.CalibratingWorkloads = st.CalibratingWorkloads
		resp.Analyzer.PendingWorkloads = st.PendingWorkloads
	}

	if h.ingest != nil && uptime > 0 {
		m, l, t := h.ingest.IngestCounts()
		resp.Ingest = ingestSection{
			MetricsPerSec: math.Round(float64(m)/uptime*10) / 10,
			LogsPerSec:    math.Round(float64(l)/uptime*10) / 10,
			TracesPerSec:  math.Round(float64(t)/uptime*10) / 10,
		}
	}

	if h.engine != nil {
		pending := h.engine.PendingActions()
		for _, p := range pending {
			switch p.Tier {
			case 1:
				resp.Actions.PendingTier1++
			case 2:
				resp.Actions.PendingTier2++
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleEngineStorage serves GET /api/v2/engine/storage.
func (h *Handlers) handleEngineStorage(w http.ResponseWriter, _ *http.Request) {
	type badgerSection struct {
		DiskBytes     int64 `json:"disk_bytes"`
		VlogSizeBytes int64 `json:"vlog_size_bytes"`
		NumTables     int   `json:"num_tables"`
		Keys          int64 `json:"keys"`
	}
	type storageResp struct {
		Badger badgerSection `json:"badger"`
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
	})
}
