package main

import (
	"context"
	"errors"
	"testing"
	"time"

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
