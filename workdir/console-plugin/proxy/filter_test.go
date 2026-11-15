package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

// mapChecker allows the namespaces of its map and counts the questions.
type mapChecker struct {
	allowed map[string]bool
	asked   int
	err     error
}

func (m *mapChecker) Allowed(_ context.Context, _ string, namespace string) (bool, error) {
	m.asked++
	if m.err != nil {
		return false, m.err
	}
	return m.allowed[namespace], nil
}

func TestNamespaceOf(t *testing.T) {
	tests := []struct{ key, want string }{
		{"shop/Deployment/api", "shop"},
		{"shop/api", ""},
		{"/Deployment/api", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := namespaceOf(tt.key); got != tt.want {
			t.Errorf("namespaceOf(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestFilterByWorkload(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		allowed map[string]bool
		want    []map[string]interface{}
		asked   int
	}{
		{
			"keeps visible namespaces, drops the rest",
			`[{"workload":"shop/Deployment/a","failureMode":"x"},{"workload":"hidden/Deployment/b"},{"workload":"shop/StatefulSet/c"}]`,
			map[string]bool{"shop": true},
			[]map[string]interface{}{{"workload": "shop/Deployment/a", "failureMode": "x"}, {"workload": "shop/StatefulSet/c"}},
			2,
		},
		{
			"drops hidden callers from affects",
			`[{"workload":"shop/Deployment/a","affects":["shop/Deployment/b","hidden/Deployment/c"]}]`,
			map[string]bool{"shop": true},
			[]map[string]interface{}{{"workload": "shop/Deployment/a", "affects": []interface{}{"shop/Deployment/b"}}},
			2,
		},
		{
			"never shows a malformed key",
			`[{"workload":"shop"},{"workload":""}]`,
			map[string]bool{"shop": true},
			[]map[string]interface{}{},
			0,
		},
		{
			"shows nothing to a user without namespaces",
			`[{"workload":"shop/Deployment/a"}]`,
			map[string]bool{},
			[]map[string]interface{}{},
			1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &mapChecker{allowed: tt.allowed}
			out, err := filterByWorkload([]byte(tt.in), newNamespaceFilter(context.Background(), "t", checker))
			if err != nil {
				t.Fatal(err)
			}
			var got []map[string]interface{}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if checker.asked != tt.asked {
				t.Fatalf("asked the checker %d times, want %d", checker.asked, tt.asked)
			}
		})
	}
}

func TestFilterByWorkload_Errors(t *testing.T) {
	f := newNamespaceFilter(context.Background(), "t", &mapChecker{err: errors.New("down")})
	if _, err := filterByWorkload([]byte(`[{"workload":"shop/Deployment/a"}]`), f); err == nil {
		t.Fatal("a checker error must not pass silently")
	}
	if _, err := filterByWorkload([]byte(`not json`), newNamespaceFilter(context.Background(), "t", &mapChecker{})); err == nil {
		t.Fatal("bad JSON must be an error")
	}
}

func TestFilterGraph(t *testing.T) {
	in := `{"nodes":[{"id":"a","namespace":"shop","kind":"Deployment"},{"id":"b","namespace":"hidden"},{"id":"c","namespace":"shop"}],
	        "edges":[{"from":"a","to":"c","via":"env"},{"from":"a","to":"b"},{"from":"b","to":"c"}]}`
	out, err := filterGraph([]byte(in), newNamespaceFilter(context.Background(), "t", &mapChecker{allowed: map[string]bool{"shop": true}}))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Nodes []map[string]interface{} `json:"nodes"`
		Edges []map[string]interface{} `json:"edges"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Nodes) != 2 || got.Nodes[0]["id"] != "a" || got.Nodes[1]["id"] != "c" {
		t.Fatalf("nodes = %v", got.Nodes)
	}
	if len(got.Edges) != 1 || got.Edges[0]["to"] != "c" || got.Edges[0]["via"] != "env" {
		t.Fatalf("edges = %v", got.Edges)
	}
}

func TestFilterGraph_EmptyStaysArrays(t *testing.T) {
	out, err := filterGraph([]byte(`{"nodes":[],"edges":[]}`), newNamespaceFilter(context.Background(), "t", &mapChecker{}))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"edges":[],"nodes":[]}` {
		t.Fatalf("got %s", out)
	}
}
