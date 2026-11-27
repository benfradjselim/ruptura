package sim

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

const (
	// demoKillPause is how long a demo workload reports its OOM kill before
	// its memory leak starts over.
	demoKillPause = 5 * time.Minute
	// demoCrashRestarts is the restart count a crash looping demo workload
	// reports, enough for the API to call it a crash loop.
	demoCrashRestarts = 6
)

// demoWorkloads are the workloads demo mode shows, as namespace/Kind/name.
var demoWorkloads = []string{
	"shop/Deployment/frontend", "shop/Deployment/checkout", "shop/Deployment/payments",
	"shop/Deployment/cart", "shop/Deployment/catalog",
	"data/StatefulSet/postgres", "data/StatefulSet/redis", "data/Deployment/kafka-connect",
	"ops/DaemonSet/node-agent", "ops/Deployment/batch-api", "ops/Deployment/ingress", "ops/Deployment/auth",
}

// demoFailure is one synthetic failure. A leak counts down to its deadline,
// is reported as an OOM kill for demoKillPause, then starts over.
type demoFailure struct {
	pattern  string
	lead     time.Duration
	deadline time.Time
}

// Demo is a synthetic failure source for running Ruptura without a cluster.
// It is safe for concurrent use.
type Demo struct {
	mu       sync.Mutex
	failures map[string]*demoFailure
}

// NewDemo returns a demo source seeded with four live failures.
func NewDemo(now time.Time) *Demo {
	d := &Demo{failures: map[string]*demoFailure{}}
	seeds := map[string]string{
		"shop/Deployment/checkout": PatternMemoryLeak,
		"shop/Deployment/payments": PatternCascadeFailure,
		"shop/Deployment/catalog":  PatternTrafficSurge,
		"ops/Deployment/batch-api": PatternSlowBurn,
	}
	for workload, pattern := range seeds {
		_ = d.Inject(workload, pattern, now)
	}
	return d
}

// patternLead is how far ahead a predicted pattern starts.
func patternLead(pattern string) (time.Duration, error) {
	switch pattern {
	case PatternMemoryLeak:
		return 22 * time.Minute, nil
	case PatternSlowBurn:
		return 50 * time.Minute, nil
	case PatternCascadeFailure, PatternTrafficSurge:
		return 0, nil
	}
	return 0, fmt.Errorf("unknown pattern %q, valid: %v", pattern, AllPatterns)
}

// Inject adds or replaces the synthetic failure of a workload.
func (d *Demo) Inject(workload, pattern string, now time.Time) error {
	if strings.Count(workload, "/") != 2 {
		return fmt.Errorf("workload %q must be namespace/Kind/name", workload)
	}
	lead, err := patternLead(pattern)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failures[workload] = &demoFailure{pattern: pattern, lead: lead, deadline: now.Add(lead)}
	return nil
}

// Workloads returns the demo workloads and every injected one, sorted.
func (d *Demo) Workloads(time.Time) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	seen := map[string]bool{}
	for _, w := range demoWorkloads {
		seen[w] = true
	}
	for w := range d.failures {
		seen[w] = true
	}
	out := make([]string, 0, len(seen))
	for w := range seen {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// Worst returns the synthetic failure of a workload at now.
func (d *Demo) Worst(workload string, now time.Time) (failuremodes.Estimate, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	f, ok := d.failures[workload]
	if !ok {
		return failuremodes.Estimate{}, false
	}
	switch f.pattern {
	case PatternTrafficSurge:
		return failuremodes.Estimate{Mode: failuremodes.ModeCPUThrottling, Confidence: 1, Usage: 0.62}, true
	case PatternCascadeFailure:
		return failuremodes.Estimate{}, false
	}
	left := f.deadline.Sub(now)
	if left <= 0 {
		if -left < demoKillPause {
			return failuremodes.Estimate{Mode: failuremodes.ModeOOMKilled, Confidence: 1, Reason: "OOMKilled"}, true
		}
		f.deadline = now.Add(f.lead)
		left = f.lead
	}
	band := left / 8
	usage := 1 - left.Minutes()/(f.lead.Minutes()*1.6)
	return failuremodes.Estimate{
		Mode: failuremodes.ModeSaturation, Pod: podName(workload),
		ETF: left, ETFEarly: left - band, ETFLate: left + band,
		Confidence: 0.9, Usage: usage,
	}, true
}

// RecentRestarts reports restarts for crash looping demo workloads.
func (d *Demo) RecentRestarts(workload string, _ time.Time, _ time.Duration) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	if f, ok := d.failures[workload]; ok && f.pattern == PatternCascadeFailure {
		return demoCrashRestarts
	}
	return 0
}

// podName makes a pod name for a demo workload.
func podName(workload string) string {
	name := workload[strings.LastIndex(workload, "/")+1:]
	return workload[:strings.Index(workload, "/")] + "/" + name + "-7d9f8-x2k4q"
}
