package mcpserver

import "testing"

var sample = []Failure{
	{Level: "workload", Target: "shop/Deployment/api", Workload: "shop/Deployment/api", FailureMode: "saturation", Kind: "predicted", ETASeconds: 600},
	{Level: "workload", Target: "shop/Deployment/db", Workload: "shop/Deployment/db", FailureMode: "crash_loop", Kind: "detected"},
	{Level: "node", Target: "worker-1//dev/sda1", FailureMode: "node_filesystem_filling", Kind: "predicted", ETASeconds: 7200},
	{Level: "namespace", Target: "billing/q/pods", FailureMode: "quota_exhaustion", Kind: "predicted", ETASeconds: 1800},
	{Level: "pod", Target: "shop/api-7d9-x", Workload: "shop/Deployment/api", Pod: "api-7d9-x", FailureMode: "oom_killed", Kind: "detected"},
}

func TestSelect(t *testing.T) {
	tests := []struct {
		name        string
		filter      Filter
		wantModes   []string
		wantMatched int
	}{
		{"everything", Filter{}, []string{"saturation", "crash_loop", "node_filesystem_filling", "quota_exhaustion", "oom_killed"}, 5},
		{"namespace", Filter{Namespace: "shop"}, []string{"saturation", "crash_loop", "oom_killed"}, 3},
		{"namespace ignores case", Filter{Namespace: "SHOP"}, []string{"saturation", "crash_loop", "oom_killed"}, 3},
		{"level", Filter{Level: "node"}, []string{"node_filesystem_filling"}, 1},
		{"kind", Filter{Kind: "detected"}, []string{"crash_loop", "oom_killed"}, 2},
		{"within keeps detected", Filter{WithinMinutes: 30}, []string{"saturation", "crash_loop", "quota_exhaustion", "oom_killed"}, 4},
		{"within and kind", Filter{WithinMinutes: 15, Kind: "predicted"}, []string{"saturation"}, 1},
		{"limit counts all matches", Filter{Limit: 2}, []string{"saturation", "crash_loop"}, 5},
		{"no match", Filter{Namespace: "nowhere"}, []string{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, matched := Select(sample, tt.filter)
			if matched != tt.wantMatched {
				t.Fatalf("matched = %d, want %d", matched, tt.wantMatched)
			}
			if len(got) != len(tt.wantModes) {
				t.Fatalf("got %d failures, want %d", len(got), len(tt.wantModes))
			}
			for i, f := range got {
				if f.FailureMode != tt.wantModes[i] {
					t.Fatalf("failure %d = %s, want %s", i, f.FailureMode, tt.wantModes[i])
				}
			}
		})
	}
}

func TestSelectCapsLimit(t *testing.T) {
	many := make([]Failure, maxLimit+10)
	if got, _ := Select(many, Filter{Limit: 10000}); len(got) != maxLimit {
		t.Fatalf("got %d, want the %d cap", len(got), maxLimit)
	}
	if got, _ := Select(many, Filter{}); len(got) != defaultLimit {
		t.Fatalf("got %d, want the default %d", len(got), defaultLimit)
	}
}

func TestAbout(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"workload name", "api", 2},
		{"pod name", "api-7d9", 1},
		{"node", "worker-1", 1},
		{"ignores case", "DB", 1},
		{"empty", "  ", 0},
		{"unknown", "payments", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := About(sample, tt.in); len(got) != tt.want {
				t.Fatalf("About(%q) = %d failures, want %d", tt.in, len(got), tt.want)
			}
		})
	}
}

func TestNeighbours(t *testing.T) {
	edges := []Edge{
		{From: "shop/frontend", To: "shop/api", Source: "traces", CallsPerMinute: 84},
		{From: "shop/api", To: "shop/db", Source: "config"},
		{From: "shop/checkout", To: "shop/api", Source: "config"},
		{From: "shop/apigw", To: "shop/frontend", Source: "config"},
	}
	callers, callees := Neighbours(edges, "api")
	if len(callers) != 2 || len(callees) != 1 || callees[0].To != "shop/db" {
		t.Fatalf("callers %+v callees %+v", callers, callees)
	}
	if callers, callees := Neighbours(edges, ""); len(callers)+len(callees) != 0 {
		t.Fatal("an empty name matched edges")
	}
}

func TestOnMatchesWholeNames(t *testing.T) {
	failures := []Failure{
		{Target: "shop/Deployment/web", Workload: "shop/Deployment/web", FailureMode: "saturation"},
		{Target: "shop/Deployment/webhook", Workload: "shop/Deployment/webhook", FailureMode: "crash_loop"},
		{Target: "shop/web-7d9-x", Workload: "shop/Deployment/web", Pod: "web-7d9-x", FailureMode: "oom_killed"},
	}
	got := On(failures, "web")
	if len(got) != 2 || got[0].FailureMode != "saturation" || got[1].FailureMode != "oom_killed" {
		t.Fatalf("On(web) = %+v", got)
	}
	if got := On(failures, "shop/web"); len(got) != 2 {
		t.Fatalf("On(shop/web) = %d failures, want 2", len(got))
	}
}

func TestRedact(t *testing.T) {
	tests := map[string]string{
		"http://user:pass@engine:8080": "http://engine:8080",
		"http://ruptura:8080":          "http://ruptura:8080",
		"::not a url":                  "::not a url",
	}
	for in, want := range tests {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%q) = %q, want %q", in, got, want)
		}
	}
}
