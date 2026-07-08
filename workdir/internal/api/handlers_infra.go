package api

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
	"github.com/benfradjselim/ruptura/internal/collector/infra/dag"
)

// SetInfraRegistry wires the infra collector registry into the API handler.
func (h *Handlers) SetInfraRegistry(r *dag.Registry) {
	h.infraRegistry = r
}

// handleInfraGroups returns all cached GroupSnapshots from the infra registry.
// GET /api/v2/infra/groups
func (h *Handlers) handleInfraGroups(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"groups": []interface{}{}})
		return
	}
	type groupDTO struct {
		Group       string  `json:"group"`
		Namespace   string  `json:"namespace"`
		Health      float64 `json:"health"`
		Spread      float64 `json:"spread"`
		GNI         float64 `json:"gni"`
		Agitated    bool    `json:"agitated"`
		ObjectCount int     `json:"objectCount"`
	}
	snaps := h.infraRegistry.AllGroupSnapshots()
	dtos := make([]groupDTO, len(snaps))
	for i, s := range snaps {
		dtos[i] = groupDTO{
			Group:       s.Group,
			Namespace:   s.Namespace,
			Health:      s.Health,
			Spread:      s.Spread,
			GNI:         s.GNI,
			Agitated:    s.Agitated,
			ObjectCount: s.ObjectCount,
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": dtos})
}

// handleInfraGroupHistory returns the persisted group-health timeline for one
// Object-Group (FBL-A3-1). GET /api/v2/infra/groups/{group}/history?ns=&from=&to=
// ns="" (or omitted) queries the cluster-scoped series. from/to are RFC3339;
// default window is the last 24h.
func (h *Handlers) handleInfraGroupHistory(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "storage not available"})
		return
	}
	group := mux.Vars(r)["group"]
	q := r.URL.Query()
	ns := q.Get("ns")

	to := time.Now()
	if v := q.Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	from := to.Add(-24 * time.Hour)
	if v := q.Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}

	points, err := h.store.ListGroupHealth(group, ns, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read history"})
		return
	}
	type point struct {
		Timestamp time.Time `json:"timestamp"`
		Health    float64   `json:"health"`
	}
	dtos := make([]point, len(points))
	for i, p := range points {
		dtos[i] = point{Timestamp: p.Timestamp, Health: p.Value}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"group":     group,
		"namespace": ns,
		"points":    dtos,
	})
}

// handleInfraNodes returns per-node health signals.
// GET /api/v2/infra/nodes
func (h *Handlers) handleInfraNodes(w http.ResponseWriter, r *http.Request) {
	h.handleSignalsByKind(w, "Node")
}

// handleInfraMCP returns per-MachineConfigPool health signals.
// GET /api/v2/infra/mcp
func (h *Handlers) handleInfraMCP(w http.ResponseWriter, r *http.Request) {
	h.handleSignalsByKind(w, "MachineConfigPool")
}

// handleInfraOperators returns per-ClusterOperator health signals.
// GET /api/v2/infra/operators
func (h *Handlers) handleInfraOperators(w http.ResponseWriter, r *http.Request) {
	h.handleSignalsByKind(w, "ClusterOperator")
}

// handleInfraNetwork returns network health aggregated per namespace.
// GET /api/v2/infra/network
func (h *Handlers) handleInfraNetwork(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"namespaces": map[string]interface{}{}})
		return
	}
	type nsDTO struct {
		Health      float64 `json:"health"`
		Spread      float64 `json:"spread"`
		ObjectCount int     `json:"objectCount"`
	}
	result := make(map[string]nsDTO)
	for _, s := range h.infraRegistry.AllGroupSnapshots() {
		if s.Group == infra.GroupNetwork {
			result[s.Namespace] = nsDTO{
				Health:      s.Health,
				Spread:      s.Spread,
				ObjectCount: s.ObjectCount,
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"namespaces": result})
}

// handlePropagation returns the CGPM propagation results for all known namespaces.
// GET /api/v2/propagation
func (h *Handlers) handlePropagation(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"namespaces": map[string]interface{}{}})
		return
	}
	type nsDTO struct {
		WorkloadPressure float64            `json:"workloadPressure"`
		PropPressure     map[string]float64 `json:"propPressure"`
	}
	all := h.infraRegistry.GetPropagator().AllResults()
	result := make(map[string]nsDTO, len(all))
	for ns, pr := range all {
		result[ns] = nsDTO{
			WorkloadPressure: pr.WorkloadPressure(),
			PropPressure:     pr.PropPressure,
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"namespaces": result})
}

// handleInfraStorage returns per-namespace and cluster-wide storage signals.
// GET /api/v2/infra/storage
func (h *Handlers) handleInfraStorage(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"groups": []interface{}{}})
		return
	}
	type groupDTO struct {
		Namespace   string  `json:"namespace"`
		Health      float64 `json:"health"`
		Spread      float64 `json:"spread"`
		ObjectCount int     `json:"objectCount"`
	}
	var dtos []groupDTO
	for _, s := range h.infraRegistry.AllGroupSnapshots() {
		if s.Group == infra.GroupStorage {
			dtos = append(dtos, groupDTO{
				Namespace:   s.Namespace,
				Health:      s.Health,
				Spread:      s.Spread,
				ObjectCount: s.ObjectCount,
			})
		}
	}
	if dtos == nil {
		dtos = []groupDTO{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": dtos})
}

// handleInfraAdmission returns per-namespace admission signals.
// GET /api/v2/infra/admission
func (h *Handlers) handleInfraAdmission(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"groups": []interface{}{}})
		return
	}
	type groupDTO struct {
		Namespace   string  `json:"namespace"`
		Health      float64 `json:"health"`
		Spread      float64 `json:"spread"`
		ObjectCount int     `json:"objectCount"`
	}
	var dtos []groupDTO
	for _, s := range h.infraRegistry.AllGroupSnapshots() {
		if s.Group == infra.GroupAdmission {
			dtos = append(dtos, groupDTO{
				Namespace:   s.Namespace,
				Health:      s.Health,
				Spread:      s.Spread,
				ObjectCount: s.ObjectCount,
			})
		}
	}
	if dtos == nil {
		dtos = []groupDTO{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": dtos})
}

// handleInfraTenancy returns per-namespace tenancy signals.
// GET /api/v2/infra/tenancy
func (h *Handlers) handleInfraTenancy(w http.ResponseWriter, r *http.Request) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"groups": []interface{}{}})
		return
	}
	type groupDTO struct {
		Namespace   string  `json:"namespace"`
		Health      float64 `json:"health"`
		Spread      float64 `json:"spread"`
		ObjectCount int     `json:"objectCount"`
	}
	var dtos []groupDTO
	for _, s := range h.infraRegistry.AllGroupSnapshots() {
		if s.Group == infra.GroupTenancy {
			dtos = append(dtos, groupDTO{
				Namespace:   s.Namespace,
				Health:      s.Health,
				Spread:      s.Spread,
				ObjectCount: s.ObjectCount,
			})
		}
	}
	if dtos == nil {
		dtos = []groupDTO{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": dtos})
}

// handleSignalsByKind is shared by node/mcp/operator handlers — filters all
// collector signals by Kubernetes object kind.
func (h *Handlers) handleSignalsByKind(w http.ResponseWriter, kind string) {
	if h.infraRegistry == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"signals": []interface{}{}})
		return
	}
	type signalDTO struct {
		Name      string  `json:"name"`
		Namespace string  `json:"namespace,omitempty"`
		Value     float64 `json:"value"`
		Severity  string  `json:"severity"`
		Signal    string  `json:"signal"`
		Message   string  `json:"message,omitempty"`
	}
	var dtos []signalDTO
	for _, sig := range h.infraRegistry.AllSignals() {
		if sig.Object.Kind != kind {
			continue
		}
		dtos = append(dtos, signalDTO{
			Name:      sig.Object.Name,
			Namespace: sig.Object.Namespace,
			Value:     sig.Value,
			Severity:  sig.Severity,
			Signal:    sig.Signal,
			Message:   sig.Message,
		})
	}
	if dtos == nil {
		dtos = []signalDTO{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"signals": dtos})
}
