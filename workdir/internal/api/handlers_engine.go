package api

import (
	"net/http"

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
