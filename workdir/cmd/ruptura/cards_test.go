package main

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/callgraph"
	"github.com/benfradjselim/ruptura/internal/failuremodes"
	"github.com/benfradjselim/ruptura/internal/sources"
)

type fakeVectors map[string][]sources.Series

func (f fakeVectors) QueryVector(_ context.Context, q string) ([]sources.Series, error) {
	if v, ok := f[q]; ok {
		return v, nil
	}
	return nil, errors.New("query failed")
}

func TestCardTargetAndMode(t *testing.T) {
	card := sources.Card{Target: []string{"node", "condition"}, ModeLabel: "condition", Modes: map[string]string{"Ready": "node_not_ready"}}
	target, ok := cardTarget(card, map[string]string{"node": "n1", "condition": "Ready"})
	if !ok || target != "n1/Ready" {
		t.Fatalf("target = %q %v", target, ok)
	}
	if _, ok := cardTarget(card, map[string]string{"node": "n1"}); ok {
		t.Fatal("named a series with a missing label")
	}
	if mode, ok := cardMode(card, map[string]string{"condition": "Ready"}); !ok || mode != "node_not_ready" {
		t.Fatalf("mode = %q %v", mode, ok)
	}
	if _, ok := cardMode(card, map[string]string{"condition": "NetworkUnavailable"}); ok {
		t.Fatal("mapped an unknown condition")
	}
	plain := sources.Card{FailureMode: "job_failed"}
	if mode, ok := cardMode(plain, nil); !ok || mode != "job_failed" {
		t.Fatalf("plain mode = %q %v", mode, ok)
	}
}

func TestPollCards(t *testing.T) {
	predicted := sources.Card{ID: "fs", Kind: sources.CardPredicted, Level: "node", FailureMode: "node_filesystem_filling",
		Target: []string{"instance"}, Usage: "u", Capacity: "c", Horizon: 6 * time.Hour, Unit: "bytes"}
	detected := sources.Card{ID: "job", Kind: sources.CardDetected, Level: "workload", FailureMode: "job_failed",
		Target: []string{"namespace", "job_name"}, Value: "v"}
	broken := sources.Card{ID: "broken", Kind: sources.CardDetected, Level: "node", FailureMode: "x", Target: []string{"node"}, Value: "missing"}
	objects := failuremodes.NewObjects()
	start := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	var now time.Time
	for i := 0; i <= 40; i++ {
		now = start.Add(time.Duration(i) * 30 * time.Second)
		source := fakeVectors{
			"u": {{Labels: map[string]string{"instance": "n1"}, Value: 100 + 4*float64(i)}, {Labels: map[string]string{"instance": "n2"}, Value: 10}},
			"c": {{Labels: map[string]string{"instance": "n1"}, Value: 512}},
			"v": {{Labels: map[string]string{"namespace": "batch", "job_name": "nightly"}, Value: 1}},
		}
		pollCards(context.Background(), source, []sources.Card{predicted, detected, broken}, objects, now)
	}
	got := objects.Findings(now)
	if len(got) != 2 {
		t.Fatalf("findings = %+v, want the filling node and the failed job", got)
	}
	seen := map[string]bool{}
	for _, f := range got {
		seen[f.Card+" "+f.Target] = true
	}
	if !seen["fs n1"] || !seen["job batch/nightly"] {
		t.Fatalf("findings = %+v", got)
	}
}

func TestPollCardsDatedCardMissingSampleTime(t *testing.T) {
	card := sources.Card{ID: "pdb", Kind: sources.CardDetected, Level: "workload", FailureMode: "pdb_blocks_drain",
		Target: []string{"namespace"}, Value: "v", Sampled: "s"}
	objects := failuremodes.NewObjects()
	start := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	values := []sources.Series{{Labels: map[string]string{"namespace": "a"}, Value: 1}, {Labels: map[string]string{"namespace": "b"}, Value: 1}}
	stamped := []sources.Series{{Labels: map[string]string{"namespace": "a"}, Value: float64(start.Unix())}}
	pollCards(context.Background(), fakeVectors{"v": values, "s": stamped}, []sources.Card{card}, objects, start)
	got := objects.Findings(start)
	if len(got) != 1 || got[0].Target != "a" {
		t.Fatalf("findings = %+v, want only the dated object", got)
	}
}

func TestPollSourceCards(t *testing.T) {
	card := sources.Card{ID: "mcp", Kind: sources.CardDetected, Level: "cluster", FailureMode: "machine_config_pool_degraded",
		Target: []string{"name"}, Source: "machineconfigpools", RaiseAfter: 5 * time.Minute}
	absent := sources.Card{ID: "other", Kind: sources.CardDetected, Level: "cluster", FailureMode: "x", Target: []string{"name"}, Source: "missing"}
	readers := map[string]objectSource{"machineconfigpools": func() []objectReading {
		return []objectReading{{Target: "worker", Value: 1}, {Target: "master", Value: 0}}
	}}
	objects := failuremodes.NewObjects()
	start := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	var now time.Time
	for i := 0; i <= 12; i++ {
		now = start.Add(time.Duration(i) * 30 * time.Second)
		pollSourceCards([]sources.Card{card, absent}, readers, objects, now)
		// The Prometheus poll leaves source cards alone.
		pollCards(context.Background(), fakeVectors{}, []sources.Card{card}, objects, now)
	}
	got := objects.Findings(now)
	if len(got) != 1 || got[0].Target != "worker" || got[0].Estimate.Mode != "machine_config_pool_degraded" {
		t.Fatalf("findings = %+v, want the degraded worker pool", got)
	}
}

func TestPollSourceCardsAbove(t *testing.T) {
	card := sources.Card{ID: "calls", Kind: sources.CardDetected, Level: "workload", FailureMode: "dependency_call_failures",
		Target: []string{"edge"}, Source: "callgraph", Above: 0.05}
	readers := map[string]objectSource{"callgraph": func() []objectReading {
		return []objectReading{{Target: "shop/Deployment/web -> shop/Deployment/db", Value: 0.2}, {Target: "shop/Deployment/web -> shop/Deployment/cache", Value: 0.01}}
	}}
	objects := failuremodes.NewObjects()
	now := time.Date(2026, 11, 28, 10, 0, 0, 0, time.UTC)
	pollSourceCards([]sources.Card{card}, readers, objects, now)
	got := objects.Findings(now)
	if len(got) != 1 || got[0].Target != "shop/Deployment/web -> shop/Deployment/db" || got[0].Value != 1 {
		t.Fatalf("findings = %+v, want only the failing edge", got)
	}
}

func TestEdgeReadings(t *testing.T) {
	high, low := 0.3, 0.0
	graph := callgraph.Graph{Edges: []callgraph.Edge{
		{From: "shop/Deployment/web", To: "shop/Deployment/db", CallsPerMinute: 60, ErrorRatio: &high},
		{From: "shop/Deployment/web", To: "shop/Deployment/cache", CallsPerMinute: 60, ErrorRatio: &low},
		{From: "shop/Deployment/web", To: "shop/Deployment/rare", CallsPerMinute: 0.2, ErrorRatio: &high},
		{From: "shop/Deployment/web", To: "shop/Deployment/static"},
	}}
	got := edgeReadings(func() callgraph.Graph { return graph })()
	if len(got) != 2 || got[0].Target != "shop/Deployment/web -> shop/Deployment/db" || got[0].Value != 0.3 || got[1].Value != 0 {
		t.Fatalf("readings = %+v, want the two measured edges with enough calls", got)
	}
}

func TestBurnReadings(t *testing.T) {
	failing, healthy := 0.02, 0.0
	graph := callgraph.Graph{Edges: []callgraph.Edge{
		{From: "shop/Deployment/web", To: "shop/Deployment/api", CallsPerMinute: 60, ErrorRatio: &failing},
		{From: "shop/Deployment/cron", To: "shop/Deployment/api", CallsPerMinute: 60, ErrorRatio: &healthy},
		{From: "shop/Deployment/web", To: "shop/Deployment/rare", CallsPerMinute: 0.5, ErrorRatio: &failing},
	}}
	got := burnReadings(func() callgraph.Graph { return graph }, 99.9)()
	// 1% of the api's calls fail against a budget of 0.1%: burn 10.
	if len(got) != 1 || got[0].Target != "shop/Deployment/api" || math.Abs(got[0].Value-10) > 1e-9 {
		t.Fatalf("readings = %+v, want the api burning at 10", got)
	}
}

func TestSLOTarget(t *testing.T) {
	for raw, want := range map[string]float64{"": 99.9, "99.5": 99.5, "100": 99.9, "nines": 99.9} {
		if got := sloTarget(raw); got != want {
			t.Errorf("sloTarget(%q) = %v, want %v", raw, got, want)
		}
	}
}
