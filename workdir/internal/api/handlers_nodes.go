package api

import (
	"net/http"
	"sort"

	"github.com/gorilla/mux"
)

// nodeInfo is the per-node summary for GET /api/v2/nodes.
type nodeInfo struct {
	Name          string  `json:"name"`
	CPUPct        float64 `json:"cpu_pct"`
	MemoryPct     float64 `json:"memory_pct"`
	DiskPressure  bool    `json:"disk_pressure"`
	WorkloadCount int     `json:"workload_count"`
	WorstFusedR   float64 `json:"worst_fused_r"`
}

// nodeWorkload is a lightweight workload entry for GET /api/v2/nodes/{node}.
type nodeWorkload struct {
	Ref         string  `json:"ref"`
	HealthScore float64 `json:"health_score"`
	FusedR      float64 `json:"fused_r"`
	Status      string  `json:"status"`
}

// nodeDetail is the full response for GET /api/v2/nodes/{node}.
type nodeDetail struct {
	Name          string         `json:"name"`
	CPUPct        float64        `json:"cpu_pct"`
	MemoryPct     float64        `json:"memory_pct"`
	DiskPressure  bool           `json:"disk_pressure"`
	WorkloadCount int            `json:"workload_count"`
	WorstFusedR   float64        `json:"worst_fused_r"`
	Workloads     []nodeWorkload `json:"workloads"`
}

// handleNodes serves GET /api/v2/nodes.
func (h *Handlers) handleNodes(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusOK, []nodeInfo{})
		return
	}

	snaps := h.store.AllSnapshots()
	// Build per-node aggregation.
	type nodeAgg struct {
		info      nodeInfo
		workloads []nodeWorkload
	}
	nodes := map[string]*nodeAgg{}

	for _, s := range snaps {
		nodeName := s.Workload.Node
		if nodeName == "" {
			nodeName = s.Host
		}
		if nodeName == "" {
			continue
		}

		agg, ok := nodes[nodeName]
		if !ok {
			agg = &nodeAgg{info: nodeInfo{Name: nodeName}}
			nodes[nodeName] = agg
		}

		isHostKind := s.Workload.Kind == "host"
		if isHostKind {
			// Node-exporter style snapshot — use its signals as node-level stats.
			agg.info.CPUPct = s.Stress.Value * 100
			agg.info.MemoryPct = s.Fatigue.Value * 100
			agg.info.DiskPressure = s.Pressure.State == "storm_approaching" || s.Pressure.State == "storm"
			continue
		}

		// Regular workload on this node.
		agg.info.WorkloadCount++
		if s.FusedRuptureIndex > agg.info.WorstFusedR {
			agg.info.WorstFusedR = s.FusedRuptureIndex
		}
		// Derive cpu/memory from workload signals when no host snapshot exists.
		if s.Stress.Value*100 > agg.info.CPUPct {
			agg.info.CPUPct = s.Stress.Value * 100
		}
		if s.Fatigue.Value*100 > agg.info.MemoryPct {
			agg.info.MemoryPct = s.Fatigue.Value * 100
		}
		if !agg.info.DiskPressure {
			agg.info.DiskPressure = s.Pressure.State == "storm_approaching" || s.Pressure.State == "storm"
		}

		ref := s.Workload.Key()
		if ref == "" {
			ref = s.Host
		}
		agg.workloads = append(agg.workloads, nodeWorkload{
			Ref:         ref,
			HealthScore: s.HealthScore.Value,
			FusedR:      s.FusedRuptureIndex,
			Status:      s.WorkloadStatus,
		})
	}

	result := make([]nodeInfo, 0, len(nodes))
	for _, a := range nodes {
		result = append(result, a.info)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	writeJSON(w, http.StatusOK, result)
}

// handleNode serves GET /api/v2/nodes/{node}.
func (h *Handlers) handleNode(w http.ResponseWriter, r *http.Request) {
	nodeName := mux.Vars(r)["node"]
	if h.store == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	snaps := h.store.AllSnapshots()
	detail := nodeDetail{Name: nodeName}
	found := false

	for _, s := range snaps {
		sNode := s.Workload.Node
		if sNode == "" {
			sNode = s.Host
		}
		if sNode != nodeName {
			continue
		}
		found = true

		isHostKind := s.Workload.Kind == "host"
		if isHostKind {
			detail.CPUPct = s.Stress.Value * 100
			detail.MemoryPct = s.Fatigue.Value * 100
			detail.DiskPressure = s.Pressure.State == "storm_approaching" || s.Pressure.State == "storm"
			continue
		}

		detail.WorkloadCount++
		if s.FusedRuptureIndex > detail.WorstFusedR {
			detail.WorstFusedR = s.FusedRuptureIndex
		}
		if s.Stress.Value*100 > detail.CPUPct {
			detail.CPUPct = s.Stress.Value * 100
		}
		if s.Fatigue.Value*100 > detail.MemoryPct {
			detail.MemoryPct = s.Fatigue.Value * 100
		}
		if !detail.DiskPressure {
			detail.DiskPressure = s.Pressure.State == "storm_approaching" || s.Pressure.State == "storm"
		}

		ref := s.Workload.Key()
		if ref == "" {
			ref = s.Host
		}
		detail.Workloads = append(detail.Workloads, nodeWorkload{
			Ref:         ref,
			HealthScore: s.HealthScore.Value,
			FusedR:      s.FusedRuptureIndex,
			Status:      s.WorkloadStatus,
		})
	}

	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	sort.Slice(detail.Workloads, func(i, j int) bool {
		return detail.Workloads[i].FusedR > detail.Workloads[j].FusedR
	})

	writeJSON(w, http.StatusOK, detail)
}
