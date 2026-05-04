// Package sim provides synthetic degradation patterns for demoing and testing Ruptura.
// It injects metrics directly into the Ruptura ingest API over HTTP so no cluster is needed.
package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// Pattern names exposed to the CLI.
const (
	PatternMemoryLeak     = "memory-leak"
	PatternCascadeFailure = "cascade-failure"
	PatternTrafficSurge   = "traffic-surge"
	PatternSlowBurn       = "slow-burn"
)

// AllPatterns lists every available pattern for help output.
var AllPatterns = []string{
	PatternMemoryLeak,
	PatternCascadeFailure,
	PatternTrafficSurge,
	PatternSlowBurn,
}

// Config controls a simulation run.
type Config struct {
	Target   string        // Ruptura API base URL, e.g. "http://localhost:8080"
	Workload string        // "namespace/deployment/name", e.g. "demo/deployment/api"
	Origin   string        // cascade-failure: the source workload
	Duration time.Duration // how long to run the pattern
	Interval time.Duration // tick interval (default 5s)
	Pattern  string        // one of AllPatterns
	Verbose  bool
}

type metricPayload struct {
	Workload string             `json:"workload"` // "namespace/kind/name"
	Metrics  map[string]float64 `json:"metrics"`
}

// Run executes the simulation until duration elapses or ctx is cancelled.
func Run(cfg Config) error {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Second
	}

	gen, err := patternGenerator(cfg.Pattern)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(cfg.Duration)
	tick := 0
	for time.Now().Before(deadline) {
		metrics := gen(tick, cfg)
		if err := send(cfg.Target, cfg.Workload, metrics); err != nil {
			fmt.Printf("warn: send failed at tick %d: %v\n", tick, err)
		}
		if cfg.Verbose {
			fmt.Printf("[tick %3d] %s  health_score_hint=%.2f\n", tick, cfg.Pattern, metrics["health_score_hint"])
		}
		tick++
		time.Sleep(cfg.Interval)
	}
	return nil
}

type generatorFn func(tick int, cfg Config) map[string]float64

func patternGenerator(name string) (generatorFn, error) {
	switch name {
	case PatternMemoryLeak:
		return memoryLeakGen, nil
	case PatternCascadeFailure:
		return cascadeFailureGen, nil
	case PatternTrafficSurge:
		return trafficSurgeGen, nil
	case PatternSlowBurn:
		return slowBurnGen, nil
	default:
		return nil, fmt.Errorf("unknown pattern %q — valid: %v", name, AllPatterns)
	}
}

// memoryLeakGen: RAM climbs 2% per tick, latency creeps up, errors appear after 60%.
func memoryLeakGen(tick int, _ Config) map[string]float64 {
	ram := clamp(0.10+float64(tick)*0.02, 0, 1)
	latency := clamp(0.05+float64(tick)*0.005, 0, 1)
	errors := 0.0
	if ram > 0.60 {
		errors = clamp((ram-0.60)*0.5, 0, 1)
	}
	return map[string]float64{
		"cpu_percent":        0.25 + jitter(0.05),
		"memory_percent":     ram,
		"load_avg_1":         latency,
		"error_rate":         errors,
		"timeout_rate":       errors * 0.3,
		"request_rate":       80 + jitter(10),
		"uptime_seconds":     float64(tick) * 5,
		"health_score_hint":  clamp(1-ram, 0, 1),
	}
}

// cascadeFailureGen: origin workload spikes errors at tick 5, contagion propagates.
func cascadeFailureGen(tick int, _ Config) map[string]float64 {
	// Simulate an upstream DB going into error storm
	errorRate := 0.0
	if tick >= 5 {
		errorRate = clamp(float64(tick-5)*0.15, 0, 0.95)
	}
	cpu := clamp(0.30+errorRate*0.4, 0, 1)
	latency := clamp(0.10+errorRate*0.6, 0, 1)
	return map[string]float64{
		"cpu_percent":       cpu,
		"memory_percent":    0.40 + jitter(0.05),
		"load_avg_1":        latency,
		"error_rate":        errorRate,
		"timeout_rate":      errorRate * 0.5,
		"request_rate":      clamp(100-errorRate*80, 5, 100),
		"uptime_seconds":    float64(tick) * 5,
		"health_score_hint": clamp(1-errorRate, 0, 1),
	}
}

// trafficSurgeGen: request rate doubles over 10 ticks, stress + pressure spike.
func trafficSurgeGen(tick int, _ Config) map[string]float64 {
	surge := math.Min(float64(tick)/10.0, 2.0)
	requests := 50 * (1 + surge)
	cpu := clamp(0.20+surge*0.30, 0, 1)
	latency := clamp(surge*0.25, 0, 1)
	return map[string]float64{
		"cpu_percent":       cpu,
		"memory_percent":    clamp(0.30+surge*0.20, 0, 1),
		"load_avg_1":        latency,
		"error_rate":        clamp((surge-1.5)*0.1, 0, 1),
		"timeout_rate":      0.01,
		"request_rate":      requests,
		"uptime_seconds":    float64(tick) * 5,
		"health_score_hint": clamp(1-cpu, 0, 1),
	}
}

// slowBurnGen: tiny stress increase each tick — designed to test detection of
// gradual degradation that threshold-based alerting misses entirely.
func slowBurnGen(tick int, _ Config) map[string]float64 {
	drift := float64(tick) * 0.003
	cpu := clamp(0.15+drift, 0, 1)
	errors := clamp(drift*0.2, 0, 1)
	return map[string]float64{
		"cpu_percent":       cpu,
		"memory_percent":    clamp(0.20+drift*0.5, 0, 1),
		"load_avg_1":        clamp(drift*0.3, 0, 1),
		"error_rate":        errors,
		"timeout_rate":      errors * 0.2,
		"request_rate":      70 + jitter(5),
		"uptime_seconds":    float64(tick) * 5,
		"health_score_hint": clamp(1-cpu-errors, 0, 1),
	}
}

func send(target, workload string, metrics map[string]float64) error {
	payload := metricPayload{Workload: workload, Metrics: metrics}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := http.Post(target+"/api/v2/sim/inject", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func jitter(scale float64) float64 {
	return (rand.Float64()*2 - 1) * scale
}
