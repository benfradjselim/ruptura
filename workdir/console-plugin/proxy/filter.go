package main

import (
	"context"
	"encoding/json"
	"strings"
)

// namespaceOf returns the namespace of a namespace/Kind/name key, or "" when
// the key has another shape.
func namespaceOf(workload string) string {
	parts := strings.Split(workload, "/")
	if len(parts) != 3 || parts[0] == "" {
		return ""
	}
	return parts[0]
}

// namespaceFilter decides per namespace, asking the checker once per
// namespace for one request.
type namespaceFilter struct {
	ctx     context.Context
	token   string
	checker AccessChecker
	known   map[string]bool
}

func newNamespaceFilter(ctx context.Context, token string, checker AccessChecker) *namespaceFilter {
	return &namespaceFilter{ctx: ctx, token: token, checker: checker, known: map[string]bool{}}
}

// allowed reports whether the user may see the namespace. An unparsable
// workload is never shown.
func (f *namespaceFilter) allowed(namespace string) (bool, error) {
	if namespace == "" {
		return false, nil
	}
	if ok, seen := f.known[namespace]; seen {
		return ok, nil
	}
	ok, err := f.checker.Allowed(f.ctx, f.token, namespace)
	if err != nil {
		return false, err
	}
	f.known[namespace] = ok
	return ok, nil
}

// keyed is the part of a failure or a closest pod the filter reads.
type keyed struct {
	Workload string   `json:"workload"`
	Affects  []string `json:"affects"`
}

// filterByWorkload keeps the items whose workload namespace the user may see,
// and drops from their affects list the callers it may not see. Other fields
// are passed through untouched.
func filterByWorkload(raw []byte, f *namespaceFilter) ([]byte, error) {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	out := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		var k keyed
		if err := json.Unmarshal(item, &k); err != nil {
			return nil, err
		}
		ok, err := f.allowed(namespaceOf(k.Workload))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if len(k.Affects) == 0 {
			out = append(out, item)
			continue
		}
		kept, err := keepAllowed(k.Affects, f)
		if err != nil {
			return nil, err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(item, &fields); err != nil {
			return nil, err
		}
		affects, err := json.Marshal(kept)
		if err != nil {
			return nil, err
		}
		fields["affects"] = affects
		rewritten, err := json.Marshal(fields)
		if err != nil {
			return nil, err
		}
		out = append(out, rewritten)
	}
	return json.Marshal(out)
}

func keepAllowed(workloads []string, f *namespaceFilter) ([]string, error) {
	kept := make([]string, 0, len(workloads))
	for _, w := range workloads {
		ok, err := f.allowed(namespaceOf(w))
		if err != nil {
			return nil, err
		}
		if ok {
			kept = append(kept, w)
		}
	}
	return kept, nil
}

type graphNode struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
}

type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// filterGraph keeps the nodes in namespaces the user may see and the edges
// between them.
func filterGraph(raw []byte, f *namespaceFilter) ([]byte, error) {
	var graph struct {
		Nodes []json.RawMessage `json:"nodes"`
		Edges []json.RawMessage `json:"edges"`
	}
	if err := json.Unmarshal(raw, &graph); err != nil {
		return nil, err
	}
	visible := map[string]bool{}
	nodes := make([]json.RawMessage, 0, len(graph.Nodes))
	for _, item := range graph.Nodes {
		var n graphNode
		if err := json.Unmarshal(item, &n); err != nil {
			return nil, err
		}
		ok, err := f.allowed(n.Namespace)
		if err != nil {
			return nil, err
		}
		if ok {
			visible[n.ID] = true
			nodes = append(nodes, item)
		}
	}
	edges := make([]json.RawMessage, 0, len(graph.Edges))
	for _, item := range graph.Edges {
		var e graphEdge
		if err := json.Unmarshal(item, &e); err != nil {
			return nil, err
		}
		if visible[e.From] && visible[e.To] {
			edges = append(edges, item)
		}
	}
	return json.Marshal(map[string]interface{}{"nodes": nodes, "edges": edges})
}
