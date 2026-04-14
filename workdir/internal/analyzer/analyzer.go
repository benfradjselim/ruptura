package analyzer

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
)

const epsilon = 1e-10

// Analyzer computes holistic KPIs from normalized metrics
type Analyzer struct {
	mu sync.RWMutex

	// Per-host accumulated state
	hosts     map[string]*hostState
	snapshots map[string]models.KPISnapshot // last computed snapshot per host
}

type hostState struct {
	// Stress history for fatigue integration
	stressHistory *utils.CircularBuffer
	// Error history for humidity/pressure
	errorHistory *utils.CircularBuffer
	// Timeout history
	timeoutHistory *utils.CircularBuffer
	// Request history
	requestHistory *utils.CircularBuffer
	// Restart count
	restartCount float64
	// Accumulated fatigue (integral of stress - recovery)
	fatigue float64
	// Last stress for pressure derivative
	lastStress float64
	// Last update time
	lastUpdate time.Time
	// Uptime in seconds (always updated, including 0)
	uptime float64
	// firstUpdate tracks whether lastStress is valid for derivative
	firstUpdate bool
}

// NewAnalyzer creates a new holistic analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		hosts:     make(map[string]*hostState),
		snapshots: make(map[string]models.KPISnapshot),
	}
}

func (a *Analyzer) getOrCreate(host string) *hostState {
	if hs, ok := a.hosts[host]; ok {
		return hs
	}
	hs := &hostState{
		stressHistory:  utils.NewCircularBuffer(600), // 10 min at 1s
		errorHistory:   utils.NewCircularBuffer(600),
		timeoutHistory: utils.NewCircularBuffer(600),
		requestHistory: utils.NewCircularBuffer(600),
		lastUpdate:     time.Now(),
		firstUpdate:    true,
	}
	a.hosts[host] = hs
	return hs
}

// Update ingests new normalized metrics and returns a KPI snapshot
func (a *Analyzer) Update(host string, metrics map[string]float64) models.KPISnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	hs := a.getOrCreate(host)
	now := time.Now()
	dt := now.Sub(hs.lastUpdate).Seconds()
	if dt < 0.001 {
		dt = 1
	}

	// --- Stress Index ---
	// S = α·CPU + β·RAM + γ·Latency + δ·Errors + ε·Timeouts
	cpu := getMetric(metrics, "cpu_percent")
	ram := getMetric(metrics, "memory_percent")
	latency := getMetric(metrics, "load_avg_1") // use load as latency proxy
	errors := getMetric(metrics, "error_rate")
	timeouts := getMetric(metrics, "timeout_rate")

	stress := 0.30*cpu + 0.20*ram + 0.20*latency + 0.20*errors + 0.10*timeouts
	stress = utils.Clamp(stress, 0, 1)

	hs.stressHistory.Push(stress)

	// --- Fatigue ---
	// F += (S - R) * dt, R = recovery rate = 0.1 when S < 0.3
	// Cap dt to 2× expected interval (30s) to prevent first-call fatigue spike on restart
	const maxDt = 30.0
	if dt > maxDt {
		dt = maxDt
	}
	recovery := 0.0
	if stress < 0.3 {
		recovery = 0.1
	}
	hs.fatigue += (stress - recovery) * dt / 3600.0 // normalize to hourly
	hs.fatigue = utils.Clamp(hs.fatigue, 0, 1)

	// --- Mood ---
	// M = (Uptime × Throughput) / (Errors × Timeouts × Restarts + ε)
	// ε guards the WHOLE denominator product (not individual factors) per spec.
	// Log normalization: log(rawMood + 1) / log(max_expected + 1) maps to [0, 1].
	uptime := getMetric(metrics, "uptime_seconds")
	hs.uptime = uptime // always update (including 0) to avoid stale value

	requests := getMetric(metrics, "request_rate")
	hs.requestHistory.Push(requests)
	hs.errorHistory.Push(errors)
	hs.timeoutHistory.Push(timeouts)

	restarts := hs.restartCount + 1 // always at least 1 to avoid division by zero
	// Denominator: errors × timeouts × restarts with ε protecting against zero
	denominator := errors*timeouts*restarts + epsilon
	rawMood := (hs.uptime * (requests + epsilon)) / denominator
	// Log-normalize: log(1 + rawMood) / log(1 + expectedMax)
	// expectedMax ≈ uptime(86400s) × throughput(1) / epsilon ≈ 8.64e14 → use log ceiling 35
	const moodLogCeiling = 35.0
	mood := utils.Clamp(math.Log1p(rawMood)/moodLogCeiling, 0, 1)

	// --- Atmospheric Pressure ---
	// P = dS/dt + ∫errors dt (raw integral per spec, not normalized by count)
	// Skip derivative on first call to avoid artificially high spike from zero lastStress.
	dSdt := 0.0
	if !hs.firstUpdate {
		dSdt = utils.Derivative(hs.lastStress, stress, dt)
	}
	errorIntegral := utils.TrapezoidIntegrate(hs.errorHistory.Values(), 1.0)
	pressure := dSdt + errorIntegral
	pressure = utils.Clamp(pressure, -1, 1)
	// Normalize to [0,1]: -1 → 0, 0 → 0.5, +1 → 1
	pressureNorm := utils.Clamp((pressure+1)/2.0, 0, 1)

	// --- Error Humidity ---
	// H = (E × T) / Q
	throughput := requests + epsilon
	humidity := (errors * timeouts) / throughput
	humidity = utils.Clamp(humidity, 0, 1)

	// --- Contagion Index ---
	// C = Σ(E_ij × D_ij) — simplified: use average error × load as proxy
	contagion := utils.Clamp(errors*cpu, 0, 1)

	// Update last state
	hs.lastStress = stress
	hs.lastUpdate = now
	hs.firstUpdate = false

	snap := models.KPISnapshot{
		Host:      host,
		Timestamp: now,
		Stress: models.KPI{
			Name:      "stress",
			Value:     utils.RoundTo(stress, 4),
			State:     stressState(stress),
			Timestamp: now,
			Host:      host,
		},
		Fatigue: models.KPI{
			Name:      "fatigue",
			Value:     utils.RoundTo(hs.fatigue, 4),
			State:     fatigueState(hs.fatigue),
			Timestamp: now,
			Host:      host,
		},
		Mood: models.KPI{
			Name:      "mood",
			Value:     utils.RoundTo(mood, 4),
			State:     moodState(mood),
			Timestamp: now,
			Host:      host,
		},
		Pressure: models.KPI{
			Name:      "pressure",
			Value:     utils.RoundTo(pressureNorm, 4),
			State:     pressureState(pressureNorm),
			Timestamp: now,
			Host:      host,
		},
		Humidity: models.KPI{
			Name:      "humidity",
			Value:     utils.RoundTo(humidity, 4),
			State:     humidityState(humidity),
			Timestamp: now,
			Host:      host,
		},
		Contagion: models.KPI{
			Name:      "contagion",
			Value:     utils.RoundTo(contagion, 4),
			State:     contagionState(contagion),
			Timestamp: now,
			Host:      host,
		},
	}
	a.snapshots[host] = snap
	return snap
}

// Snapshot returns the last computed KPI snapshot without mutating state (safe for GET handlers)
func (a *Analyzer) Snapshot(host string) (models.KPISnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snap, ok := a.snapshots[host]
	return snap, ok
}

// RecordRestart increments the restart count for a host
func (a *Analyzer) RecordRestart(host string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	hs := a.getOrCreate(host)
	hs.restartCount++
}

// ResetFatigue resets fatigue after maintenance/restart
func (a *Analyzer) ResetFatigue(host string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	hs := a.getOrCreate(host)
	hs.fatigue = 0
}

func getMetric(metrics map[string]float64, name string) float64 {
	if v, ok := metrics[name]; ok {
		return v
	}
	return 0
}

// --- State classification functions ---

func stressState(s float64) string {
	switch {
	case s < 0.3:
		return "calm"
	case s < 0.6:
		return "nervous"
	case s < 0.8:
		return "stressed"
	default:
		return "panic"
	}
}

func fatigueState(f float64) string {
	switch {
	case f < 0.3:
		return "rested"
	case f < 0.6:
		return "tired"
	case f < 0.8:
		return "exhausted"
	default:
		return "burnout"
	}
}

func moodState(m float64) string {
	// m is normalized [0,1]; 1.0 = happy
	switch {
	case m > 0.75:
		return "happy"
	case m > 0.50:
		return "content"
	case m > 0.25:
		return "neutral"
	case m > 0.10:
		return "sad"
	default:
		return "depressed"
	}
}

func pressureState(p float64) string {
	// p is [0,1], 0.5 = stable, > 0.6 = rising
	switch {
	case p > 0.7:
		return "storm_approaching"
	case p > 0.55:
		return "rising"
	case p < 0.45:
		return "improving"
	default:
		return "stable"
	}
}

func humidityState(h float64) string {
	switch {
	case h < 0.1:
		return "dry"
	case h < 0.3:
		return "humid"
	case h < 0.5:
		return "very_humid"
	default:
		return "storm"
	}
}

func contagionState(c float64) string {
	switch {
	case c < 0.3:
		return "low"
	case c < 0.6:
		return "moderate"
	case c < 0.8:
		return "epidemic"
	default:
		return "pandemic"
	}
}
