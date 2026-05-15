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
	HealthScore float64 `json:"health_score"`
	FusedR      float64 `json:"fused_r"`
	State       string  `json:"state"`
}

type topologyEdge struct {
	Source      string  `json:"source"`
	Target      string  `json:"target"`
	CallRate    int64   `json:"call_rate"`
	ErrorRate   float64 `json:"error_rate"`
	P99LatencyMS float64 `json:"p99_latency_ms"`
}

type topologyResponse struct {
	Nodes []topologyNode `json:"nodes"`
	Edges []topologyEdge `json:"edges"`
}

// handleTopology serves GET /api/v2/topology.
// Returns all known workload nodes (with health state) and the service-call edges
// observed from trace spans.
func (h *Handlers) handleTopology(w http.ResponseWriter, r *http.Request) {
	resp := topologyResponse{
		Nodes: []topologyNode{},
		Edges: []topologyEdge{},
	}

	// Build node set from live snapshots.
	if h.store != nil {
		snapshots := h.store.AllSnapshots()
		for i := range snapshots {
			h.enrichSnapshot(&snapshots[i])
			s := snapshots[i]
			id := s.Host
			if s.Workload.Namespace != "" {
				id = s.Workload.Namespace + "/" + s.Workload.Kind + "/" + s.Workload.Name
			}
			resp.Nodes = append(resp.Nodes, topologyNode{
				ID:          id,
				HealthScore: s.HealthScore.Value,
				FusedR:      s.FusedRuptureIndex,
				State:       snapshotState(s),
			})
		}
	}

	// Merge auto-discovered pending workloads.
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
				ID:    id,
				State: "pending_telemetry",
			})
		}
	}

	// Build edges from topology builder.
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
			})
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
