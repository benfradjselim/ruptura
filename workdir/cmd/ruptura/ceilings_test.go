package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/sources"
)

type fakePods map[string]string

func (f fakePods) WorkloadForPodName(ns, pod string) (string, string, bool) {
	name, ok := f[ns+"/"+pod]
	return "Deployment", name, ok
}

type fakeCollector struct {
	results map[string][]sources.Reading
	err     error
}

func (f fakeCollector) Collect(_ context.Context, sig sources.Signal) ([]sources.Reading, error) {
	return f.results[sig.Usage], f.err
}

func TestInferCeilings(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	old := float64(now.Add(-3 * 24 * time.Hour).Unix())
	young := float64(now.Add(-2 * time.Hour).Unix())
	pods := fakePods{"shop/web-1": "web", "shop/web-2": "web", "shop/new-1": "new"}
	p99 := []sources.Reading{
		{Namespace: "shop", Pod: "web-1", Usage: 100},
		{Namespace: "shop", Pod: "web-2", Usage: 150},
		{Namespace: "shop", Pod: "new-1", Usage: 80},
		{Namespace: "shop", Pod: "orphan", Usage: 500},
	}
	starts := []sources.Reading{
		{Namespace: "shop", Pod: "web-1", Usage: young},
		{Namespace: "shop", Pod: "web-2", Usage: old},
		{Namespace: "shop", Pod: "new-1", Usage: young},
	}
	got := inferCeilings(p99, starts, pods, now)
	if len(got) != 1 || got["shop/Deployment/web"] != 180 {
		t.Fatalf("inferCeilings = %v, want web at 180 only", got)
	}
}

func TestCeilings_LookupAndRefresh(t *testing.T) {
	now := time.Now()
	c := newCeilings("shop=512Mi")
	if v, from, ok := c.lookup("shop", "shop/Deployment/web"); !ok || v != 512<<20 || from != ceilingSourceDefault {
		t.Fatalf("namespace default: %v %q %v", v, from, ok)
	}
	src := fakeCollector{results: map[string][]sources.Reading{
		ceilingP99Query:   {{Namespace: "dev", Pod: "api-1", Usage: 1000}},
		ceilingStartQuery: {{Namespace: "dev", Pod: "api-1", Usage: float64(now.Add(-48 * time.Hour).Unix())}},
	}}
	if err := c.refresh(context.Background(), src, fakePods{"dev/api-1": "api"}, now); err != nil {
		t.Fatal(err)
	}
	if v, from, ok := c.lookup("dev", "dev/Deployment/api"); !ok || v != 1200 || from != ceilingSourceInferred {
		t.Fatalf("inferred: %v %q %v", v, from, ok)
	}
	// A failed refresh keeps the previous ceilings.
	if err := c.refresh(context.Background(), fakeCollector{err: errors.New("timeout")}, fakePods{}, now); err == nil {
		t.Fatal("refresh error not reported")
	}
	if _, _, ok := c.lookup("dev", "dev/Deployment/api"); !ok {
		t.Fatal("ceiling lost after a failed refresh")
	}
	if _, _, ok := c.lookup("other", "other/Deployment/x"); ok {
		t.Fatal("ceiling for an unknown workload")
	}
}

func TestParseDefaultLimits(t *testing.T) {
	tests := []struct {
		raw     string
		want    map[string]float64
		wantErr bool
	}{
		{"", map[string]float64{}, false},
		{"shop=512Mi, batch=2Gi", map[string]float64{"shop": 512 << 20, "batch": 2 << 30}, false},
		{"shop=1G", map[string]float64{"shop": 1e9}, false},
		{"shop=1048576", map[string]float64{"shop": 1 << 20}, false},
		{"shop", nil, true},
		{"shop=lots", nil, true},
		{"shop=-1Mi", nil, true},
		{"=1Gi", nil, true},
	}
	for _, tt := range tests {
		got, err := parseDefaultLimits(tt.raw)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDefaultLimits(%q) error = %v", tt.raw, err)
			continue
		}
		if tt.wantErr {
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("parseDefaultLimits(%q) = %v, want %v", tt.raw, got, tt.want)
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("parseDefaultLimits(%q)[%s] = %v, want %v", tt.raw, k, got[k], v)
			}
		}
	}
}
