package agent

import (
	"strconv"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// Metric names the agent sends and the engine reads.
const (
	MetricNodePIDs    = "ruptura.agent.node.pids"
	MetricNodePIDMax  = "ruptura.agent.node.pid_max"
	MetricPodPIDs     = "ruptura.agent.pod.pids"
	MetricPodPIDMax   = "ruptura.agent.pod.pids_max"
	MetricPolicyDrops = "ruptura.agent.policy.drops"
	// AgentAttribute marks a resource sent by the agent.
	AgentAttribute = "ruptura.agent"
)

// Pod names one pod the agent reports on.
type Pod struct {
	Namespace, Name string
	PIDs            PodPIDs
}

// Report is one reading of a node.
type Report struct {
	Node              string
	At                time.Time
	NodeUsed, NodeMax float64
	Pods              []Pod
	// Drops counts packets denied by network policies per namespace since the
	// previous report.
	Drops map[string]int
}

// Request turns a report into an OTLP metrics request, one resource per object.
func (r Report) Request() models.OTLPMetricsRequest {
	ts := strconv.FormatInt(r.At.UnixNano(), 10)
	resource := func(attrs ...string) models.OTLPResource {
		res := models.OTLPResource{Attributes: []models.OTLPAttribute{attr(AgentAttribute, "true"), attr("k8s.node.name", r.Node)}}
		for i := 0; i+1 < len(attrs); i += 2 {
			res.Attributes = append(res.Attributes, attr(attrs[i], attrs[i+1]))
		}
		return res
	}
	gauges := func(res models.OTLPResource, values ...gauge) models.OTLPResourceMetrics {
		var metrics []models.OTLPMetric
		for _, g := range values {
			v := g.value
			metrics = append(metrics, models.OTLPMetric{Name: g.name,
				Gauge: &models.OTLPGauge{DataPoints: []models.OTLPNumberDataPoint{{TimeUnixNano: ts, AsDouble: &v}}}})
		}
		return models.OTLPResourceMetrics{Resource: res, ScopeMetrics: []models.OTLPScopeMetrics{{Metrics: metrics}}}
	}
	req := models.OTLPMetricsRequest{}
	if r.NodeMax > 0 {
		req.ResourceMetrics = append(req.ResourceMetrics, gauges(resource(), gauge{MetricNodePIDs, r.NodeUsed}, gauge{MetricNodePIDMax, r.NodeMax}))
	}
	for _, p := range r.Pods {
		req.ResourceMetrics = append(req.ResourceMetrics, gauges(resource("k8s.namespace.name", p.Namespace, "k8s.pod.name", p.Name),
			gauge{MetricPodPIDs, p.PIDs.Used}, gauge{MetricPodPIDMax, p.PIDs.Limit}))
	}
	for ns, n := range r.Drops {
		req.ResourceMetrics = append(req.ResourceMetrics, gauges(resource("k8s.namespace.name", ns), gauge{MetricPolicyDrops, float64(n)}))
	}
	return req
}

type gauge struct {
	name  string
	value float64
}

func attr(key, value string) models.OTLPAttribute {
	return models.OTLPAttribute{Key: key, Value: models.OTLPAnyValue{StringValue: &value}}
}
