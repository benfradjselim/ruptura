package api

import (
	"testing"

	"github.com/benfradjselim/ruptura/internal/callgraph"
)

func TestImpactText(t *testing.T) {
	tests := []struct {
		callers []string
		want    string
	}{
		{nil, ""},
		{[]string{"shop/Deployment/api"}, "Can hit its callers: api"},
		{[]string{"a/Deployment/w", "a/Deployment/x", "a/Deployment/y", "a/Deployment/z", "b/Deployment/v"}, "Can hit its callers: w, x, y and 2 more"},
	}
	for _, tc := range tests {
		if got := ImpactText(tc.callers); got != tc.want {
			t.Errorf("ImpactText(%v) = %q, want %q", tc.callers, got, tc.want)
		}
	}
}

func TestCallGraphView(t *testing.T) {
	graph := callgraph.Graph{
		Edges: []callgraph.Edge{
			{From: "shop/Deployment/load", To: "shop/Deployment/api", Source: callgraph.SourceEnv, Via: "API_URL"},
			{From: "shop/Deployment/api", To: "shop/StatefulSet/db", Source: callgraph.SourceEnv, Via: "DB_URL"},
			{From: "shop/Route/front", To: "shop/Deployment/api", Source: callgraph.SourceRoute, Via: "shop.example.com"},
		},
		Entries: []callgraph.Entry{{Kind: "Route", Namespace: "shop", Name: "front", Host: "shop.example.com", Service: "shop-api"}},
	}
	failures := []*FailureResult{
		{Workload: "shop/StatefulSet/db", State: "degraded", FailureMode: "saturation", Kind: "predicted", ETASeconds: 600},
		{Workload: "shop/StatefulSet/db", State: "critical", FailureMode: "oom_killed", Kind: "detected"},
		{Workload: "other/Deployment/x", State: "critical", FailureMode: "crash_loop", Kind: "detected"},
	}
	view := callGraphView(graph, failures)
	if len(view.Nodes) != 4 || len(view.Edges) != 3 {
		t.Fatalf("got %d nodes and %d edges", len(view.Nodes), len(view.Edges))
	}
	nodes := map[string]callGraphNode{}
	for _, n := range view.Nodes {
		nodes[n.ID] = n
	}
	db := nodes["shop/StatefulSet/db"]
	if db.State != "critical" || db.FailureMode != "oom_killed" || db.Affects != 2 {
		t.Errorf("db node %+v, want the critical failure and 2 callers", db)
	}
	if front := nodes["shop/Route/front"]; front.Type != "entry" || front.Host != "shop.example.com" {
		t.Errorf("route node %+v", front)
	}
	if api := nodes["shop/Deployment/api"]; api.State != "ok" || api.Type != "workload" {
		t.Errorf("api node %+v", api)
	}
}

func TestCallGraphViewEmpty(t *testing.T) {
	view := callGraphView(callgraph.Graph{}, nil)
	if view.Nodes == nil || view.Edges == nil || len(view.Nodes) != 0 {
		t.Errorf("an empty graph must serialise as empty lists: %+v", view)
	}
}
