package sim

import (
	"context"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

// SeedConfig controls a demo-mode synthetic history seed.
type SeedConfig struct {
	Namespaces   []string
	PerNamespace int
	History      time.Duration
	Interval     time.Duration
	RampDuration time.Duration // how long the degrading workload takes to breach, live
}

// DefaultSeedConfig returns the demo-mode defaults from shining.md P3: 7 days
// of history across 3 namespaces with 4 workloads each, sampled every 10
// minutes, with the degrading workload breaching within ~10 minutes of
// startup.
func DefaultSeedConfig() SeedConfig {
	return SeedConfig{
		Namespaces:   []string{"prod", "staging", "infra"},
		PerNamespace: 4,
		History:      7 * 24 * time.Hour,
		Interval:     10 * time.Minute,
		RampDuration: 10 * time.Minute,
	}
}

var demoWorkloadNames = []string{
	"api", "worker", "gateway", "cache",
	"billing", "auth", "scheduler", "ingest",
	"notifier", "search", "queue", "reporter",
}

// SeedStats summarizes what Seed produced, for logging and tests.
type SeedStats struct {
	Workloads         int
	Ticks             int
	KPIWrites         int
	DegradingWorkload string
}

// AnalyzerEngine is the subset of *analyzer.Analyzer that demo seeding needs.
type AnalyzerEngine interface {
	Update(ref models.WorkloadRef, metrics map[string]float64) models.KPISnapshot
}

// SnapshotStore is the subset of *storage.Store that demo seeding needs.
type SnapshotStore interface {
	StoreSnapshot(snap models.KPISnapshot)
	PutKPI(name, host string, ts time.Time, value float64) error
}

// demoWorkloadRefs builds the fixed set of demo WorkloadRefs from cfg:
// cfg.PerNamespace workloads in each of cfg.Namespaces, named from
// demoWorkloadNames in order.
func demoWorkloadRefs(cfg SeedConfig) []models.WorkloadRef {
	refs := make([]models.WorkloadRef, 0, len(cfg.Namespaces)*cfg.PerNamespace)
	i := 0
	for _, ns := range cfg.Namespaces {
		for n := 0; n < cfg.PerNamespace; n++ {
			name := demoWorkloadNames[i%len(demoWorkloadNames)]
			i++
			refs = append(refs, models.WorkloadRef{Namespace: ns, Kind: "Deployment", Name: name})
		}
	}
	return refs
}

// Seed populates cfg.History of synthetic healthy baseline data (health
// 0.7-1.0, stress 0.05-0.3) for every demo workload, in compressed time — it
// does not sleep, so it returns in milliseconds regardless of cfg.History.
// This gives every workload enough observations to clear calibration (the
// analyzer requires ~96) so the dashboard shows "active" workloads
// immediately, with no calibration wait.
//
// The first workload returned by demoWorkloadRefs is earmarked as the
// degrading one (SeedStats.DegradingWorkload); its live breach trajectory is
// driven separately by DegradeLive, not by Seed itself — Seed only lays down
// its healthy pre-incident history.
func Seed(a AnalyzerEngine, store SnapshotStore, cfg SeedConfig) SeedStats {
	if cfg.Interval <= 0 || cfg.History <= 0 {
		cfg = DefaultSeedConfig()
	}
	refs := demoWorkloadRefs(cfg)
	stats := SeedStats{Workloads: len(refs)}
	if len(refs) > 0 {
		stats.DegradingWorkload = refs[0].Key()
	}

	totalTicks := int(cfg.History / cfg.Interval)
	now := time.Now()

	for tick := 0; tick < totalTicks; tick++ {
		ts := now.Add(-cfg.History + time.Duration(tick)*cfg.Interval)
		for _, ref := range refs {
			metrics := demoHealthyMetrics()
			snap := a.Update(ref, metrics)
			store.StoreSnapshot(snap)
			for name, val := range snapshotKPIValues(snap) {
				if err := store.PutKPI(name, ref.Key(), ts, val); err == nil {
					stats.KPIWrites++
				}
			}
			stats.Ticks++
		}
	}
	return stats
}

// DegradeLive drives the degrading workload's live breach trajectory in real
// time: metrics ramp from healthy to a memory-leak-shaped breach over
// rampDuration, then hold at the breached state. It runs until ctx is
// cancelled, ticking every 5 seconds so the dashboard shows visible movement.
func DegradeLive(ctx context.Context, a AnalyzerEngine, store SnapshotStore, ref models.WorkloadRef, rampDuration time.Duration) {
	if rampDuration <= 0 {
		rampDuration = 10 * time.Minute
	}
	const tickInterval = 5 * time.Second
	start := time.Now()
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			frac := time.Since(start).Seconds() / rampDuration.Seconds()
			if frac > 1 {
				frac = 1
			}
			snap := a.Update(ref, demoDegradingMetrics(frac))
			store.StoreSnapshot(snap)
		}
	}
}

// demoHealthyMetrics returns stable, low-noise metrics matching real signal
// shapes: health 0.7-1.0, stress 0.05-0.3.
func demoHealthyMetrics() map[string]float64 {
	return map[string]float64{
		"cpu_percent":    0.15 + jitter(0.08),
		"memory_percent": 0.25 + jitter(0.10),
		"load_avg_1":     0.10 + jitter(0.05),
		"error_rate":     0.0,
		"timeout_rate":   0.0,
		"request_rate":   80 + jitter(15),
	}
}

// demoDegradingMetrics returns memory-leak-shaped metrics for a workload
// ramping toward breach: frac 0 is healthy, frac 1 is fully breached (errors
// and timeouts climbing, memory saturated).
func demoDegradingMetrics(frac float64) map[string]float64 {
	ram := clamp(0.15+0.80*frac, 0, 1)
	errors := 0.0
	if ram > 0.60 {
		errors = clamp((ram-0.60)*1.5, 0, 1)
	}
	return map[string]float64{
		"cpu_percent":    clamp(0.20+0.50*frac, 0, 1) + jitter(0.03),
		"memory_percent": ram,
		"load_avg_1":     clamp(0.10+0.60*frac, 0, 1),
		"error_rate":     errors,
		"timeout_rate":   errors * 0.4,
		"request_rate":   clamp(80-40*frac, 10, 80) + jitter(5),
	}
}

// snapshotKPIValues extracts the named KPI series from a snapshot for
// per-metric history persistence via SnapshotStore.PutKPI.
func snapshotKPIValues(snap models.KPISnapshot) map[string]float64 {
	return map[string]float64{
		"stress":              snap.Stress.Value,
		"fatigue":             snap.Fatigue.Value,
		"mood":                snap.Mood.Value,
		"pressure":            snap.Pressure.Value,
		"humidity":            snap.Humidity.Value,
		"contagion":           snap.Contagion.Value,
		"resilience":          snap.Resilience.Value,
		"entropy":             snap.Entropy.Value,
		"velocity":            snap.Velocity.Value,
		"health_score":        snap.HealthScore.Value,
		"throughput":          snap.Throughput.Value,
		"fused_rupture_index": snap.FusedRuptureIndex,
	}
}
