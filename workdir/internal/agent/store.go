package agent

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// stale is how long a reading is kept without a new report: the agent
// reports every 30 seconds, so three missed reports.
const stale = 90 * time.Second

// Reading is the latest agent value of one object, with its limit when it has one.
type Reading struct {
	Target   string
	Value    float64
	Capacity float64
	At       time.Time
}

// Store keeps the latest reading of every object the agents report. It is
// safe for concurrent use.
type Store struct {
	mu       sync.Mutex
	readings map[string]map[string]Reading // kind -> target and node -> reading
}

// Reading kinds.
const (
	KindNodePIDs    = "node-pids"
	KindPodPIDs     = "pod-pids"
	KindPolicyDrops = "policy-drops"
)

// NewStore returns an empty store.
func NewStore() *Store { return &Store{readings: map[string]map[string]Reading{}} }

// Accept records the agent resources of an OTLP request and reports whether
// the resource was the agent's, so the caller skips it.
func (s *Store) Accept(rm models.OTLPResourceMetrics) bool {
	if rm.Resource.GetAttr(AgentAttribute) != "true" {
		return false
	}
	values := map[string]float64{}
	var at time.Time
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Gauge == nil || len(m.Gauge.DataPoints) == 0 || m.Gauge.DataPoints[0].AsDouble == nil {
				continue
			}
			dp := m.Gauge.DataPoints[0]
			values[m.Name] = *dp.AsDouble
			at = parseNanos(dp.TimeUnixNano)
		}
	}
	node, ns, pod := rm.Resource.GetAttr("k8s.node.name"), rm.Resource.GetAttr("k8s.namespace.name"), rm.Resource.GetAttr("k8s.pod.name")
	switch {
	case pod != "" && ns != "":
		s.put(KindPodPIDs, node, Reading{Target: ns + "/" + pod, Value: values[MetricPodPIDs], Capacity: values[MetricPodPIDMax], At: at})
	case ns != "":
		s.put(KindPolicyDrops, node, Reading{Target: ns, Value: values[MetricPolicyDrops], At: at})
	case node != "":
		s.put(KindNodePIDs, node, Reading{Target: node, Value: values[MetricNodePIDs], Capacity: values[MetricNodePIDMax], At: at})
	}
	return true
}

func (s *Store) put(kind, node string, r Reading) {
	if r.At.IsZero() {
		r.At = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readings[kind] == nil {
		s.readings[kind] = map[string]Reading{}
	}
	s.readings[kind][r.Target+"\x00"+node] = r
}

// Readings returns the fresh readings of a kind at now, sorted by target. The
// policy drops of a namespace, reported by each node, are summed.
func (s *Store) Readings(kind string, now time.Time) []Reading {
	s.mu.Lock()
	defer s.mu.Unlock()
	byTarget := map[string]Reading{}
	for key, r := range s.readings[kind] {
		if now.Sub(r.At) > stale {
			delete(s.readings[kind], key)
			continue
		}
		if prev, ok := byTarget[r.Target]; ok {
			prev.Value += r.Value
			r = prev
		}
		byTarget[r.Target] = r
	}
	out := make([]Reading, 0, len(byTarget))
	for _, r := range byTarget {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

func parseNanos(raw string) time.Time {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		return time.Time{}
	}
	return time.Unix(0, n).UTC()
}
