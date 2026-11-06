package callgraph

import (
	"reflect"
	"testing"
)

func shop() ([]Workload, []Service) {
	workloads := []Workload{
		{Namespace: "shop", Kind: "Deployment", Name: "load", Labels: map[string]string{"app": "load"}, Env: []EnvVar{{"API_URL", "http://shop-api:8080/buy"}}},
		{Namespace: "shop", Kind: "Deployment", Name: "api", Labels: map[string]string{"app": "api"}, Env: []EnvVar{{"DB_URL", "http://shop-db:8080/q"}, {"LOG_LEVEL", "info"}}},
		{Namespace: "shop", Kind: "StatefulSet", Name: "db", Labels: map[string]string{"app": "db"}},
		{Namespace: "pay", Kind: "Deployment", Name: "checkout", Labels: map[string]string{"app": "checkout"},
			Env: []EnvVar{{"ORDERS", "shop-api.shop.svc.cluster.local:8080"}, {"BANK", "https://api.bank.example.com/v1"}}},
	}
	services := []Service{
		{Namespace: "shop", Name: "shop-api", Selector: map[string]string{"app": "api"}},
		{Namespace: "shop", Name: "shop-db", Selector: map[string]string{"app": "db"}},
		{Namespace: "shop", Name: "empty", Selector: nil},
	}
	return workloads, services
}

func TestHosts(t *testing.T) {
	tests := []struct {
		value string
		want  []string
	}{
		{"http://shop-db:8080/q", []string{"shop-db"}},
		{"postgres://user:secret@db.prod.svc:5432/app", []string{"db.prod.svc"}},
		{"redis:6379", []string{"redis"}},
		{"a:1,b.ns:2", []string{"a", "b.ns"}},
		{"cache.infra.svc.cluster.local", []string{"cache.infra.svc.cluster.local"}},
		{"info", nil},
		{"/var/lib/data", nil},
	}
	for _, tc := range tests {
		if got := hosts(tc.value); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("hosts(%q) = %v, want %v", tc.value, got, tc.want)
		}
	}
}

func TestServiceKey(t *testing.T) {
	tests := []struct {
		host, ns, want string
		ok             bool
	}{
		{"shop-db", "shop", "shop/shop-db", true},
		{"shop-db.other", "shop", "other/shop-db", true},
		{"shop-db.other.svc", "shop", "other/shop-db", true},
		{"shop-db.other.svc.cluster.local", "shop", "other/shop-db", true},
		{"api.bank.example.com", "shop", "", false},
	}
	for _, tc := range tests {
		got, ok := serviceKey(tc.host, tc.ns)
		if got != tc.want || ok != tc.ok {
			t.Errorf("serviceKey(%q, %q) = %q %v, want %q %v", tc.host, tc.ns, got, ok, tc.want, tc.ok)
		}
	}
}

func TestBuild(t *testing.T) {
	workloads, services := shop()
	entries := []Entry{
		{Kind: "Route", Namespace: "shop", Name: "front", Host: "shop.apps.example.com", Service: "shop-api"},
		{Kind: "Ingress", Namespace: "shop", Name: "gone", Host: "x", Service: "missing"},
	}
	got := Build(workloads, services, entries)
	want := []Edge{
		{From: "pay/Deployment/checkout", To: "shop/Deployment/api", Source: SourceEnv, Via: "ORDERS"},
		{From: "shop/Deployment/api", To: "shop/StatefulSet/db", Source: SourceEnv, Via: "DB_URL"},
		{From: "shop/Deployment/load", To: "shop/Deployment/api", Source: SourceEnv, Via: "API_URL"},
		{From: "shop/Route/front", To: "shop/Deployment/api", Source: SourceRoute, Via: "shop.apps.example.com"},
	}
	if !reflect.DeepEqual(got.Edges, want) {
		t.Errorf("edges\n got %+v\nwant %+v", got.Edges, want)
	}
	if len(got.Entries) != 1 || got.Entries[0].Name != "front" {
		t.Errorf("entries without a backend must be dropped: %+v", got.Entries)
	}
}

func TestBuildIgnoresSelfAndDuplicates(t *testing.T) {
	workloads := []Workload{{Namespace: "a", Kind: "Deployment", Name: "w", Labels: map[string]string{"app": "w"},
		Env: []EnvVar{{"SELF", "http://w:80"}, {"X", "http://v:80"}, {"Y", "v:80"}}},
		{Namespace: "a", Kind: "Deployment", Name: "v", Labels: map[string]string{"app": "v"}}}
	services := []Service{{Namespace: "a", Name: "w", Selector: map[string]string{"app": "w"}}, {Namespace: "a", Name: "v", Selector: map[string]string{"app": "v"}}}
	got := Build(workloads, services, nil)
	if len(got.Edges) != 1 || got.Edges[0].To != "a/Deployment/v" {
		t.Errorf("got %+v", got.Edges)
	}
}

func TestCallers(t *testing.T) {
	workloads, services := shop()
	g := Build(workloads, services, []Entry{{Kind: "Route", Namespace: "shop", Name: "front", Service: "shop-api"}})
	tests := []struct {
		target string
		want   []string
	}{
		{"shop/StatefulSet/db", []string{"shop/Deployment/api", "pay/Deployment/checkout", "shop/Deployment/load"}},
		{"shop/Deployment/api", []string{"pay/Deployment/checkout", "shop/Deployment/load"}},
		{"shop/Deployment/load", []string{}},
	}
	for _, tc := range tests {
		if got := g.Callers(tc.target); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Callers(%s) = %v, want %v", tc.target, got, tc.want)
		}
	}
}

func TestCallersStopsOnCycles(t *testing.T) {
	g := Graph{Edges: []Edge{{From: "a", To: "b", Source: SourceEnv}, {From: "b", To: "a", Source: SourceEnv}}}
	if got := g.Callers("a"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("got %v", got)
	}
}
