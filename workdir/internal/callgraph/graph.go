// Package callgraph infers which workloads call which from what a cluster
// declares: container environment values that name a Service, and Routes or
// Ingresses that send outside traffic to a Service. Observed calls, from traces
// or flows, are a later source.
package callgraph

import (
	"regexp"
	"sort"
	"strings"
)

// Edge sources.
const (
	SourceEnv     = "env"
	SourceRoute   = "route"
	SourceIngress = "ingress"
)

// Workload is a Deployment, StatefulSet or DaemonSet with what its pods declare.
type Workload struct {
	Namespace string
	Kind      string
	Name      string
	// Labels are the pod template labels a Service selects on.
	Labels map[string]string
	Env    []EnvVar
}

// ID is namespace/kind/name, the key used by every engine output.
func (w Workload) ID() string { return w.Namespace + "/" + w.Kind + "/" + w.Name }

// EnvVar is one literal environment value of a container.
type EnvVar struct {
	Name  string
	Value string
}

// Service routes to the pods its selector matches.
type Service struct {
	Namespace string
	Name      string
	Selector  map[string]string
}

// Entry is a Route or an Ingress backend: traffic from outside the cluster.
type Entry struct {
	Kind      string
	Namespace string
	Name      string
	Host      string
	Service   string
}

// ID names the entry as a graph node.
func (e Entry) ID() string { return e.Namespace + "/" + e.Kind + "/" + e.Name }

// Edge says From calls To, and why Ruptura believes it.
type Edge struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Source string `json:"source"`
	Via    string `json:"via"`
	// CallsPerMinute and ErrorRatio are measured, so only observed edges carry them.
	CallsPerMinute float64  `json:"callsPerMinute,omitempty"`
	ErrorRatio     *float64 `json:"errorRatio,omitempty"`
}

// Graph is the inferred call graph.
type Graph struct {
	Edges   []Edge  `json:"edges"`
	Entries []Entry `json:"-"`
	// UnresolvedCalls counts observed calls whose services match no single
	// workload; they are left out rather than guessed.
	UnresolvedCalls int `json:"-"`
}

// hostPattern finds a host after a scheme, or a host followed by a port.
var hostPattern = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.-]*://(?:[^@/\s]+@)?([a-z0-9][a-z0-9.-]*))|(?:^|[\s,;=])([a-z0-9][a-z0-9.-]*):\d+`)

// hosts returns the host names an environment value refers to.
func hosts(value string) []string {
	var found []string
	for _, m := range hostPattern.FindAllStringSubmatch(value, -1) {
		for _, h := range m[1:] {
			if h != "" {
				found = append(found, strings.ToLower(strings.TrimSuffix(h, ".")))
			}
		}
	}
	if len(found) == 0 && strings.Contains(value, ".svc") && !strings.ContainsAny(value, " /") {
		found = append(found, strings.ToLower(value))
	}
	return found
}

// serviceKey resolves a host to namespace/service. A short name is in the
// caller's namespace; name.ns, name.ns.svc and name.ns.svc.cluster.local name
// another one. Anything else, such as an external domain, does not resolve.
func serviceKey(host, namespace string) (string, bool) {
	parts := strings.Split(host, ".")
	switch {
	case len(parts) == 1:
		return namespace + "/" + parts[0], true
	case len(parts) == 2:
		return parts[1] + "/" + parts[0], true
	case len(parts) >= 3 && parts[2] == "svc":
		return parts[1] + "/" + parts[0], true
	}
	return "", false
}

func selects(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// Build infers the call graph from what workloads declare and from calls seen
// in traces. Edges are unique per from, to and source, and sorted so the output
// is stable.
func Build(workloads []Workload, services []Service, entries []Entry, observed []Observed) Graph {
	backends := map[string][]string{}
	for _, svc := range services {
		key := svc.Namespace + "/" + svc.Name
		for _, w := range workloads {
			if w.Namespace == svc.Namespace && selects(svc.Selector, w.Labels) {
				backends[key] = append(backends[key], w.ID())
			}
		}
	}

	seen := map[string]bool{}
	edges := []Edge{}
	add := func(e Edge) {
		key := e.From + "|" + e.To + "|" + e.Source
		if e.From == e.To || seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, e)
	}

	for _, w := range workloads {
		for _, env := range w.Env {
			for _, host := range hosts(env.Value) {
				key, ok := serviceKey(host, w.Namespace)
				if !ok {
					continue
				}
				for _, target := range backends[key] {
					add(Edge{From: w.ID(), To: target, Source: SourceEnv, Via: env.Name})
				}
			}
		}
	}

	kept := []Entry{}
	for _, entry := range entries {
		targets := backends[entry.Namespace+"/"+entry.Service]
		if len(targets) == 0 {
			continue
		}
		kept = append(kept, entry)
		source := SourceIngress
		if entry.Kind == "Route" {
			source = SourceRoute
		}
		for _, target := range targets {
			add(Edge{From: entry.ID(), To: target, Source: source, Via: entry.Host})
		}
	}

	unresolved := 0
	for _, call := range observed {
		from, okFrom := resolveService(call.ClientNamespace, call.Client, workloads, backends)
		to, okTo := resolveService(call.ServerNamespace, call.Server, workloads, backends)
		if !okFrom || !okTo {
			unresolved++
			continue
		}
		if call.PerMinute <= 0 {
			continue
		}
		ratio := call.FailedPerMinute / call.PerMinute
		add(Edge{From: from, To: to, Source: SourceTraces, Via: viaServiceGraph, CallsPerMinute: call.PerMinute, ErrorRatio: &ratio})
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Source < edges[j].Source
	})
	return Graph{Edges: edges, Entries: kept, UnresolvedCalls: unresolved}
}

// resolveService maps a service name seen in traces to one workload: a
// workload of that name in the namespace, else the single workload a Service
// of that name selects. No match, or more than one, is not resolved.
func resolveService(namespace, name string, workloads []Workload, backends map[string][]string) (string, bool) {
	var found []string
	for _, w := range workloads {
		if w.Namespace == namespace && w.Name == name {
			found = append(found, w.ID())
		}
	}
	if len(found) == 0 {
		found = backends[namespace+"/"+name]
	}
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// Callers returns every workload that reaches target through the graph,
// nearest first, without entries. These are what a failure of target can hit.
func (g Graph) Callers(target string) []string {
	incoming := map[string][]string{}
	for _, e := range g.Edges {
		if e.Source == SourceEnv || e.Source == SourceTraces {
			incoming[e.To] = append(incoming[e.To], e.From)
		}
	}
	for to, from := range incoming {
		incoming[to] = unique(from)
	}
	seen := map[string]bool{target: true}
	queue, found := []string{target}, []string{}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		callers := append([]string(nil), incoming[next]...)
		sort.Strings(callers)
		for _, c := range callers {
			if !seen[c] {
				seen[c] = true
				found = append(found, c)
				queue = append(queue, c)
			}
		}
	}
	return found
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := values[:0]
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ObservedCallers returns the calls per minute of each workload seen calling
// target in traces.
func (g Graph) ObservedCallers(target string) map[string]float64 {
	rates := map[string]float64{}
	for _, e := range g.Edges {
		if e.To == target && e.Source == SourceTraces {
			rates[e.From] += e.CallsPerMinute
		}
	}
	return rates
}
