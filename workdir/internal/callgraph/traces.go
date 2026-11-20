package callgraph

import (
	"context"
	"fmt"

	"github.com/benfradjselim/ruptura/internal/sources"
)

// SourceTraces marks an edge seen in traces, through the service graph
// metrics of an OpenTelemetry Collector or Tempo.
const SourceTraces = "traces"

// viaServiceGraph names what an observed edge was read from.
const viaServiceGraph = "traces_service_graph"

const serviceGraphGrouping = "client, server, client_k8s_namespace_name, server_k8s_namespace_name, namespace, connection_type"

// Calls per minute over 10 minutes. Failed calls are counted either with a
// failed="true" label on the request counter (recent OpenTelemetry connectors)
// or by a separate failed counter without that label (Tempo, older connectors);
// the label filters keep the two styles from counting the same calls twice.
var (
	serviceGraphTotal     = "sum by (" + serviceGraphGrouping + ") (rate(traces_service_graph_request_total[10m])) * 60"
	serviceGraphFailed    = "sum by (" + serviceGraphGrouping + ") (rate(traces_service_graph_request_total{failed=\"true\"}[10m])) * 60"
	serviceGraphFailedOld = "sum by (" + serviceGraphGrouping + ") (rate(traces_service_graph_request_failed_total{failed=\"\"}[10m])) * 60"
)

// Querier runs a Prometheus instant query.
type Querier interface {
	QueryVector(ctx context.Context, q string) ([]sources.Series, error)
}

// Observed is a call between two services seen in traces. Services are named
// as the traces name them; Build maps them to workloads.
type Observed struct {
	ClientNamespace string
	Client          string
	ServerNamespace string
	Server          string
	PerMinute       float64
	FailedPerMinute float64
}

type callKey struct{ clientNS, client, serverNS, server string }

// ObservedCalls reads the service graph metrics and returns one call per
// client and server. Calls to or from a peer that sends no traces, which the
// connector names "unknown", are left out.
func ObservedCalls(ctx context.Context, q Querier) ([]Observed, error) {
	total, err := q.QueryVector(ctx, serviceGraphTotal)
	if err != nil {
		return nil, fmt.Errorf("callgraph: service graph calls: %w", err)
	}
	if len(total) == 0 {
		return nil, nil
	}
	calls := map[callKey]*Observed{}
	var order []callKey
	add := func(series []sources.Series, failed bool) {
		for _, s := range series {
			key, ok := keyOf(s.Labels)
			if !ok {
				continue
			}
			call, seen := calls[key]
			if !seen {
				if failed {
					continue
				}
				call = &Observed{ClientNamespace: key.clientNS, Client: key.client, ServerNamespace: key.serverNS, Server: key.server}
				calls[key] = call
				order = append(order, key)
			}
			if failed {
				call.FailedPerMinute += s.Value
			} else {
				call.PerMinute += s.Value
			}
		}
	}
	add(total, false)
	for _, query := range []string{serviceGraphFailed, serviceGraphFailedOld} {
		failed, err := q.QueryVector(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("callgraph: service graph failures: %w", err)
		}
		add(failed, true)
	}
	out := make([]Observed, 0, len(order))
	for _, key := range order {
		out = append(out, *calls[key])
	}
	return out, nil
}

// keyOf names both sides of a call. A side without its own namespace label
// takes the other side's, then the namespace the metrics were scraped in.
func keyOf(labels map[string]string) (callKey, bool) {
	client, server := labels["client"], labels["server"]
	if client == "" || server == "" || client == "unknown" || server == "unknown" || labels["connection_type"] == "virtual_node" {
		return callKey{}, false
	}
	clientNS, serverNS := labels["client_k8s_namespace_name"], labels["server_k8s_namespace_name"]
	if clientNS == "" {
		clientNS = serverNS
	}
	if serverNS == "" {
		serverNS = clientNS
	}
	if clientNS == "" {
		clientNS, serverNS = labels["namespace"], labels["namespace"]
	}
	if clientNS == "" {
		return callKey{}, false
	}
	return callKey{clientNS, client, serverNS, server}, true
}
