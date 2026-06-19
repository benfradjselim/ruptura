// ruptura-testapp — synthetic workload simulator for Ruptura lab
// Reads ENV vars to simulate different failure scenarios and exposes
// Prometheus metrics + OTLP telemetry.
package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// ─── config ───────────────────────────────────────────────────────────────

type Config struct {
	AppName              string
	Scenario             string
	LatencyP50Ms         float64
	LatencyP99Ms         float64
	ErrorRate            float64
	CPULoad              float64
	MemoryMB             float64
	MemoryLeakMBPerMin   float64
	SpikeIntervalSeconds float64
	SpikeDurationSeconds float64
	SpikeCPUMultiplier   float64
	SpikeLatencyMult     float64
	BaseCPULoad          float64
	BaseLatencyMs        float64
	DependencyFailRate   float64
}

func envFloat(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func loadConfig() Config {
	return Config{
		AppName:              envStr("APP_NAME", "testapp"),
		Scenario:             envStr("SCENARIO", "stable"),
		LatencyP50Ms:         envFloat("LATENCY_P50_MS", 20),
		LatencyP99Ms:         envFloat("LATENCY_P99_MS", 200),
		ErrorRate:            envFloat("ERROR_RATE", 0.01),
		CPULoad:              envFloat("CPU_LOAD", 0.1),
		MemoryMB:             envFloat("MEMORY_MB", 64),
		MemoryLeakMBPerMin:   envFloat("MEMORY_LEAK_MB_PER_MIN", 0),
		SpikeIntervalSeconds: envFloat("SPIKE_INTERVAL_SECONDS", 120),
		SpikeDurationSeconds: envFloat("SPIKE_DURATION_SECONDS", 30),
		SpikeCPUMultiplier:   envFloat("SPIKE_CPU_MULTIPLIER", 3),
		SpikeLatencyMult:     envFloat("SPIKE_LATENCY_MULTIPLIER", 5),
		BaseCPULoad:          envFloat("BASE_CPU_LOAD", 0.1),
		BaseLatencyMs:        envFloat("BASE_LATENCY_MS", 20),
		DependencyFailRate:   envFloat("DEPENDENCY_FAILURE_RATE", 0),
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ─── state ────────────────────────────────────────────────────────────────

var (
	cfg Config

	totalRequests    int64
	totalErrors      int64
	totalLatencyMs   int64
	currentCPULoad   float64
	currentMemoryMB  float64
	inSpike          bool
	mu               sync.RWMutex

	// memory sink to simulate usage
	memorySink [][]byte
	memMu      sync.Mutex
)

// ─── CPU simulation ───────────────────────────────────────────────────────

func cpuBurner(ctx context.Context, loadFrac float64) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		mu.RLock()
		load := currentCPULoad
		mu.RUnlock()

		onMs := int(load * 10)
		offMs := 10 - onMs
		if onMs <= 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		deadline := time.Now().Add(time.Duration(onMs) * time.Millisecond)
		for time.Now().Before(deadline) {
			_ = math.Sqrt(rand.Float64() * 1e9)
		}
		if offMs > 0 {
			time.Sleep(time.Duration(offMs) * time.Millisecond)
		}
	}
}

// ─── memory simulation ────────────────────────────────────────────────────

func memoryManager(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.RLock()
			targetMB := currentMemoryMB
			mu.RUnlock()

			memMu.Lock()
			currentBytes := 0
			for _, b := range memorySink {
				currentBytes += len(b)
			}
			targetBytes := int(targetMB * 1024 * 1024)
			if currentBytes < targetBytes {
				chunk := targetBytes - currentBytes
				if chunk > 10*1024*1024 {
					chunk = 10 * 1024 * 1024
				}
				buf := make([]byte, chunk)
				rand.Read(buf)
				memorySink = append(memorySink, buf)
			} else if currentBytes > targetBytes+20*1024*1024 {
				memorySink = memorySink[:len(memorySink)/2]
				runtime.GC()
			}
			memMu.Unlock()
		}
	}
}

// ─── spike controller ─────────────────────────────────────────────────────

func spikeController(ctx context.Context) {
	if cfg.Scenario != "spike" {
		return
	}
	interval := time.Duration(cfg.SpikeIntervalSeconds) * time.Second
	duration := time.Duration(cfg.SpikeDurationSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			inSpike = true
			currentCPULoad = cfg.BaseCPULoad * cfg.SpikeCPUMultiplier
			mu.Unlock()
			log.Printf("[%s] SPIKE START — CPU=%.0f%% latency×%.0f", cfg.AppName, currentCPULoad*100, cfg.SpikeLatencyMult)
			time.Sleep(duration)
			mu.Lock()
			inSpike = false
			currentCPULoad = cfg.BaseCPULoad
			mu.Unlock()
			log.Printf("[%s] SPIKE END", cfg.AppName)
		}
	}
}

// ─── memory leak controller ───────────────────────────────────────────────

func memLeakController(ctx context.Context) {
	if cfg.MemoryLeakMBPerMin <= 0 {
		return
	}
	leakPerTick := cfg.MemoryLeakMBPerMin / 6 // every 10s
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mu.Lock()
			currentMemoryMB += leakPerTick
			if currentMemoryMB > 800 {
				currentMemoryMB = 800 // cap at 800MB
			}
			mu.Unlock()
		}
	}
}

// ─── request simulator ────────────────────────────────────────────────────

func requestSimulator(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go simulateRequest()
		}
	}
}

func simulateRequest() {
	mu.RLock()
	spike := inSpike
	mu.RUnlock()

	latency := sampleLatency(spike)
	isErr := rand.Float64() < effectiveErrorRate(spike)

	time.Sleep(time.Duration(latency) * time.Millisecond)
	atomic.AddInt64(&totalRequests, 1)
	atomic.AddInt64(&totalLatencyMs, int64(latency))
	if isErr {
		atomic.AddInt64(&totalErrors, 1)
	}
}

func sampleLatency(spike bool) float64 {
	p50 := cfg.LatencyP50Ms
	p99 := cfg.LatencyP99Ms
	if cfg.Scenario == "spike" {
		if spike {
			p50 *= cfg.SpikeLatencyMult
			p99 *= cfg.SpikeLatencyMult
		} else {
			p50 = cfg.BaseLatencyMs
			p99 = cfg.BaseLatencyMs * 5
		}
	}
	// Log-normal approximation: sample between p50 and p99 with exponential tail
	r := rand.Float64()
	if r < 0.5 {
		return p50 * (0.5 + rand.Float64())
	}
	if r < 0.99 {
		return p50 + (p99-p50)*rand.Float64()
	}
	return p99 * (1 + rand.Float64()*2)
}

func effectiveErrorRate(spike bool) float64 {
	r := cfg.ErrorRate
	if spike {
		r = math.Min(r*5, 0.5)
	}
	return r
}

// ─── metrics handler ──────────────────────────────────────────────────────

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	reqs := atomic.LoadInt64(&totalRequests)
	errs := atomic.LoadInt64(&totalErrors)
	latMs := atomic.LoadInt64(&totalLatencyMs)

	mu.RLock()
	cpuLoad := currentCPULoad
	memMB := currentMemoryMB
	mu.RUnlock()

	var avgLatency float64
	if reqs > 0 {
		avgLatency = float64(latMs) / float64(reqs)
	}

	appName := cfg.AppName
	ns := envStr("OTEL_RESOURCE_ATTRIBUTES", "")
	_ = ns

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP http_requests_total Total HTTP requests\n")
	fmt.Fprintf(w, "# TYPE http_requests_total counter\n")
	fmt.Fprintf(w, `http_requests_total{app="%s",namespace="test-apps"} %d`+"\n", appName, reqs)

	fmt.Fprintf(w, "# HELP http_errors_total Total HTTP errors\n")
	fmt.Fprintf(w, "# TYPE http_errors_total counter\n")
	fmt.Fprintf(w, `http_errors_total{app="%s",namespace="test-apps"} %d`+"\n", appName, errs)

	fmt.Fprintf(w, "# HELP http_request_duration_ms Average request duration in ms\n")
	fmt.Fprintf(w, "# TYPE http_request_duration_ms gauge\n")
	fmt.Fprintf(w, `http_request_duration_ms{app="%s",namespace="test-apps"} %.2f`+"\n", appName, avgLatency)

	fmt.Fprintf(w, "# HELP process_cpu_usage CPU usage fraction (0-1)\n")
	fmt.Fprintf(w, "# TYPE process_cpu_usage gauge\n")
	fmt.Fprintf(w, `process_cpu_usage{app="%s",namespace="test-apps"} %.4f`+"\n", appName, cpuLoad)

	fmt.Fprintf(w, "# HELP process_memory_mb Memory usage in MB\n")
	fmt.Fprintf(w, "# TYPE process_memory_mb gauge\n")
	fmt.Fprintf(w, `process_memory_mb{app="%s",namespace="test-apps"} %.1f`+"\n", appName, memMB)

	var errRate float64
	if reqs > 0 {
		errRate = float64(errs) / float64(reqs)
	}
	fmt.Fprintf(w, "# HELP http_error_rate Current error rate (0-1)\n")
	fmt.Fprintf(w, "# TYPE http_error_rate gauge\n")
	fmt.Fprintf(w, `http_error_rate{app="%s",namespace="test-apps"} %.6f`+"\n", appName, errRate)

	// Dependency failure metric (for contagion signal)
	fmt.Fprintf(w, "# HELP dependency_failure_rate Downstream dependency failure rate\n")
	fmt.Fprintf(w, "# TYPE dependency_failure_rate gauge\n")
	fmt.Fprintf(w, `dependency_failure_rate{app="%s",namespace="test-apps"} %.4f`+"\n", appName, cfg.DependencyFailRate)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","app":"%s","scenario":"%s"}`, cfg.AppName, cfg.Scenario)
}

// ─── main ─────────────────────────────────────────────────────────────────

func main() {
	cfg = loadConfig()
	log.Printf("[%s] starting scenario=%s cpu=%.0f%% mem=%.0fMB err=%.1f%%",
		cfg.AppName, cfg.Scenario, cfg.CPULoad*100, cfg.MemoryMB, cfg.ErrorRate*100)

	// Init state
	mu.Lock()
	currentCPULoad = cfg.CPULoad
	currentMemoryMB = cfg.MemoryMB
	if cfg.Scenario == "spike" {
		currentCPULoad = cfg.BaseCPULoad
	}
	mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start goroutines
	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go cpuBurner(ctx, cfg.CPULoad)
	}
	go memoryManager(ctx)
	go memLeakController(ctx)
	go spikeController(ctx)
	go requestSimulator(ctx)

	// HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", handleMetrics)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/readyz", handleHealth)
	mux.HandleFunc("/", handleHealth)

	addr := ":8080"
	log.Printf("[%s] listening on %s", cfg.AppName, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
