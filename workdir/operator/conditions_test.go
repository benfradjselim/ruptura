package main

import (
	"testing"
	"time"
)

func TestConditions(t *testing.T) {
	now := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	tests := []struct {
		phase string
		ready int32
		want  map[string]string
	}{
		{"Running", 1, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "False"}},
		{"Pending", 0, map[string]string{"Available": "False", "Progressing": "True", "Degraded": "False"}},
		{"Degraded", 1, map[string]string{"Available": "True", "Progressing": "False", "Degraded": "True"}},
		{"Degraded", 0, map[string]string{"Available": "False", "Progressing": "False", "Degraded": "True"}},
		{"Failed", 0, map[string]string{"Available": "False", "Progressing": "False", "Degraded": "True"}},
	}
	for _, tc := range tests {
		got := conditions(nil, tc.phase, "why", tc.ready, 3, now)
		if len(got) != 3 {
			t.Fatalf("%s: %d conditions", tc.phase, len(got))
		}
		for _, c := range got {
			if c.Status != tc.want[c.Type] || c.Reason == "" || c.ObservedGeneration != 3 || c.LastTransitionTime != "2026-11-06T22:00:00Z" {
				t.Errorf("%s ready=%d: condition %+v", tc.phase, tc.ready, c)
			}
		}
	}
}

func TestConditionsKeepTransitionTime(t *testing.T) {
	first := time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC)
	previous := conditions(nil, "Running", "", 1, 1, first)
	again := conditions(previous, "Running", "", 1, 2, first.Add(time.Hour))
	for _, c := range again {
		if c.LastTransitionTime != "2026-11-06T22:00:00Z" {
			t.Errorf("unchanged %s moved its transition time to %s", c.Type, c.LastTransitionTime)
		}
	}
	changed := conditions(previous, "Pending", "pod not ready", 0, 2, first.Add(time.Hour))
	for _, c := range changed {
		if c.LastTransitionTime != "2026-11-06T23:00:00Z" && c.Type != "Degraded" {
			t.Errorf("changed %s kept the old transition time", c.Type)
		}
		if c.Type == "Degraded" && c.LastTransitionTime != "2026-11-06T22:00:00Z" {
			t.Errorf("Degraded stayed False and must keep its time, got %s", c.LastTransitionTime)
		}
	}
}
