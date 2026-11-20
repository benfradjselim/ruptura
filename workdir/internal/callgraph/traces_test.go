package callgraph

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/benfradjselim/ruptura/internal/sources"
)

type fakeQuerier map[string][]sources.Series

func (f fakeQuerier) QueryVector(_ context.Context, q string) ([]sources.Series, error) {
	if series, ok := f[q]; ok {
		return series, nil
	}
	return nil, nil
}

type failingQuerier struct{}

func (failingQuerier) QueryVector(context.Context, string) ([]sources.Series, error) {
	return nil, errors.New("prometheus down")
}

func series(value float64, kv ...string) sources.Series {
	labels := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		labels[kv[i]] = kv[i+1]
	}
	return sources.Series{Labels: labels, Value: value}
}

func TestObservedCalls(t *testing.T) {
	tests := []struct {
		name    string
		querier fakeQuerier
		want    []Observed
	}{
		{
			name:    "no trace metrics",
			querier: fakeQuerier{},
			want:    nil,
		},
		{
			name: "unknown peers left out",
			querier: fakeQuerier{
				serviceGraphTotal: {
					series(35, "client", "checkout", "server", "payments", "client_k8s_namespace_name", "shop", "server_k8s_namespace_name", "shop", "namespace", "shop"),
					series(1, "client", "checkout", "server", "unknown", "client_k8s_namespace_name", "shop", "connection_type", "virtual_node", "namespace", "shop"),
					series(2, "client", "unknown", "server", "payments", "server_k8s_namespace_name", "shop", "namespace", "shop"),
				},
				serviceGraphFailed: {
					series(1.4, "client", "checkout", "server", "payments", "client_k8s_namespace_name", "shop", "server_k8s_namespace_name", "shop", "namespace", "shop"),
				},
			},
			want: []Observed{{ClientNamespace: "shop", Client: "checkout", ServerNamespace: "shop", Server: "payments", PerMinute: 35, FailedPerMinute: 1.4}},
		},
		{
			name: "calls and failures without namespace labels",
			querier: fakeQuerier{
				serviceGraphTotal:  {series(10, "client", "web", "server", "api", "namespace", "shop")},
				serviceGraphFailed: {series(0.5, "client", "web", "server", "api", "namespace", "shop")},
			},
			want: []Observed{{ClientNamespace: "shop", Client: "web", ServerNamespace: "shop", Server: "api", PerMinute: 10, FailedPerMinute: 0.5}},
		},
		{
			name: "a side without its namespace takes the other side's",
			querier: fakeQuerier{
				serviceGraphTotal: {series(4, "client", "web", "server", "api", "client_k8s_namespace_name", "front", "namespace", "monitoring")},
			},
			want: []Observed{{ClientNamespace: "front", Client: "web", ServerNamespace: "front", Server: "api", PerMinute: 4}},
		},
		{
			name: "one call scraped in two namespaces is summed",
			querier: fakeQuerier{
				serviceGraphTotal: {
					series(3, "client", "web", "server", "api", "client_k8s_namespace_name", "shop", "server_k8s_namespace_name", "shop", "namespace", "a"),
					series(2, "client", "web", "server", "api", "client_k8s_namespace_name", "shop", "server_k8s_namespace_name", "shop", "namespace", "b"),
				},
			},
			want: []Observed{{ClientNamespace: "shop", Client: "web", ServerNamespace: "shop", Server: "api", PerMinute: 5}},
		},
		{
			name: "failures without a matching call are ignored",
			querier: fakeQuerier{
				serviceGraphTotal:  {series(3, "client", "web", "server", "api", "namespace", "shop")},
				serviceGraphFailed: {series(9, "client", "other", "server", "api", "namespace", "shop")},
			},
			want: []Observed{{ClientNamespace: "shop", Client: "web", ServerNamespace: "shop", Server: "api", PerMinute: 3}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ObservedCalls(context.Background(), tc.querier)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestObservedCallsError(t *testing.T) {
	if _, err := ObservedCalls(context.Background(), failingQuerier{}); err == nil {
		t.Fatal("want an error when Prometheus fails")
	}
}

func demoWorkloads() ([]Workload, []Service) {
	names := []string{"loadgen", "frontend", "checkout", "payments", "inventory", "fraud"}
	env := map[string][]EnvVar{
		"loadgen":  {{Name: "FRONTEND_URL", Value: "http://frontend:8080"}},
		"frontend": {{Name: "CHECKOUT_URL", Value: "http://checkout:8080"}},
		"checkout": {{Name: "PAYMENTS_URL", Value: "http://payments:8080"}},
		"payments": {{Name: "FRAUD_URL", Value: "http://fraud:8080"}},
	}
	var workloads []Workload
	var services []Service
	for _, n := range names {
		workloads = append(workloads, Workload{Namespace: "td", Kind: "Deployment", Name: n, Labels: map[string]string{"app": n}, Env: env[n]})
		if n != "loadgen" {
			services = append(services, Service{Namespace: "td", Name: n, Selector: map[string]string{"app": n}})
		}
	}
	return workloads, services
}

func call(client, server string, perMinute, failed float64) Observed {
	return Observed{ClientNamespace: "td", Client: client, ServerNamespace: "td", Server: server, PerMinute: perMinute, FailedPerMinute: failed}
}

func TestBuildObserved(t *testing.T) {
	workloads, services := demoWorkloads()
	workloads = append(workloads,
		Workload{Namespace: "td", Kind: "Deployment", Name: "blue", Labels: map[string]string{"tier": "web"}},
		Workload{Namespace: "td", Kind: "Deployment", Name: "green", Labels: map[string]string{"tier": "web"}})
	services = append(services,
		Service{Namespace: "td", Name: "web", Selector: map[string]string{"tier": "web"}},
		Service{Namespace: "td", Name: "store", Selector: map[string]string{"app": "inventory"}})
	observed := []Observed{
		call("loadgen", "frontend", 35, 0),
		call("frontend", "checkout", 37, 0),
		call("checkout", "payments", 36, 1.08),
		call("checkout", "inventory", 37, 0),
		call("frontend", "store", 2, 0),
		call("frontend", "web", 5, 0),
		call("frontend", "ghost", 5, 0),
		call("checkout", "fraud", 0, 0),
	}
	g := Build(workloads, services, nil, observed)

	type key struct{ from, to, source string }
	got := map[key]Edge{}
	for _, e := range g.Edges {
		got[key{e.From, e.To, e.Source}] = e
	}
	id := func(n string) string { return "td/Deployment/" + n }
	for _, want := range []key{
		{id("loadgen"), id("frontend"), SourceTraces},
		{id("frontend"), id("checkout"), SourceTraces},
		{id("checkout"), id("payments"), SourceTraces},
		{id("checkout"), id("inventory"), SourceTraces},
		{id("frontend"), id("inventory"), SourceTraces},
		{id("payments"), id("fraud"), SourceEnv},
		{id("checkout"), id("payments"), SourceEnv},
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing edge %v", want)
		}
	}
	for _, absent := range []key{
		{id("payments"), id("fraud"), SourceTraces},
		{id("checkout"), id("fraud"), SourceTraces},
		{id("checkout"), id("inventory"), SourceEnv},
	} {
		if _, ok := got[absent]; ok {
			t.Errorf("unexpected edge %v", absent)
		}
	}
	pay := got[key{id("checkout"), id("payments"), SourceTraces}]
	if pay.CallsPerMinute != 36 || pay.ErrorRatio == nil || math.Abs(*pay.ErrorRatio-0.03) > 1e-9 {
		t.Errorf("checkout -> payments rate %v ratio %v, want 36 and 0.03", pay.CallsPerMinute, pay.ErrorRatio)
	}
	if env := got[key{id("payments"), id("fraud"), SourceEnv}]; env.ErrorRatio != nil || env.CallsPerMinute != 0 {
		t.Errorf("a declared edge must carry no measure, got %+v", env)
	}
	if g.UnresolvedCalls != 2 {
		t.Errorf("unresolved %d, want 2 (web selects two workloads, ghost matches none)", g.UnresolvedCalls)
	}
}

func TestCallersFollowObservedEdges(t *testing.T) {
	workloads, services := demoWorkloads()
	g := Build(workloads, services, nil, []Observed{
		call("loadgen", "frontend", 35, 0),
		call("frontend", "checkout", 37, 0),
		call("checkout", "inventory", 37, 0),
	})
	want := []string{"td/Deployment/checkout", "td/Deployment/frontend", "td/Deployment/loadgen"}
	if got := g.Callers("td/Deployment/inventory"); !reflect.DeepEqual(got, want) {
		t.Errorf("Callers(inventory) = %v, want %v", got, want)
	}
	if got := g.ObservedCallers("td/Deployment/inventory"); !reflect.DeepEqual(got, map[string]float64{"td/Deployment/checkout": 37}) {
		t.Errorf("ObservedCallers(inventory) = %v", got)
	}
	if got := g.Callers("td/Deployment/checkout"); !reflect.DeepEqual(got, []string{"td/Deployment/frontend", "td/Deployment/loadgen"}) {
		t.Errorf("a caller both declared and observed must be listed once, got %v", got)
	}
}

func TestServiceGraphQueries(t *testing.T) {
	if !strings.Contains(serviceGraphTotal, "traces_service_graph_request_total") || strings.Contains(serviceGraphTotal, "failed=") {
		t.Errorf("calls must sum the request counter across the failed label: %s", serviceGraphTotal)
	}
	if !strings.Contains(serviceGraphFailed, "traces_service_graph_request_failed_total") {
		t.Errorf("failures must come from the failed counter: %s", serviceGraphFailed)
	}
}
