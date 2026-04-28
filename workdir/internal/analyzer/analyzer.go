package analyzer

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
	"github.com/benfradjselim/kairo-core/pkg/utils"
)

const epsilon = 1e-10

// Analyzer computes holistic KPIs from normalized metrics
type Analyzer struct {
	mu sync.RWMutex

	// Per-host accumulated state
	hosts     map[string]*hostState
	snapshots map[string]models.KPISnapshot // last computed snapshot per host

	// Default fatigue config applied to new hosts; overridable per-host via SetFatigueConfig.
	defaultFatigueRThreshold float64
	defaultFatigueLambda     float64
}

type hostState struct {
	// Last raw metric inputs (stored for /explain endpoint)
	lastMetrics map[string]float64
	// Stress history for fatigue integration
	stressHistory *utils.CircularBuffer
	// Error history for humidity/pressure
	errorHistory *utils.CircularBuffer
	// Timeout history
	timeoutHistory *utils.CircularBuffer
	// Request history
	requestHistory *utils.CircularBuffer
	// KPI velocity: rolling history of composite health for entropy/velocity
	healthHistory *utils.CircularBuffer
	// Restart count
	restartCount float64
	// Accumulated fatigue — v5.0 dissipative formula
	fatigue float64
	// v5.0 fatigue config (defaults applied at creation)
	fatigueRThreshold float64
	fatigueLambda     float64
	// Last stress for pressure derivative
	lastStress float64
	// Last KPI vector for velocity (rate of change)
	lastHealthScore float64
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
		hosts:                    make(map[string]*hostState),
		snapshots:                make(map[string]models.KPISnapshot),
		defaultFatigueRThreshold: 0.3,
		defaultFatigueLambda:     0.05,
	}
}

// SetDefaultFatigueConfig sets the dissipative fatigue parameters applied to
// all new hosts. Existing hosts are not affected; use SetFatigueConfig for those.
func (a *Analyzer) SetDefaultFatigueConfig(rThreshold, lambda float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.defaultFatigueRThreshold = rThreshold
	a.defaultFatigueLambda = lambda
}

func (a *Analyzer) getOrCreate(host string) *hostState {
	if hs, ok := a.hosts[host]; ok {
		return hs
	}
	hs := &hostState{
		stressHistory:     utils.NewCircularBuffer(600),
		errorHistory:      utils.NewCircularBuffer(600),
		timeoutHistory:    utils.NewCircularBuffer(600),
		requestHistory:    utils.NewCircularBuffer(600),
		healthHistory:     utils.NewCircularBuffer(60),
		lastUpdate:        time.Now(),
		firstUpdate:       true,
		fatigueRThreshold: a.defaultFatigueRThreshold,
		fatigueLambda:     a.defaultFatigueLambda,
	}
	a.hosts[host] = hs
	return hs
}

// Update ingests new normalized metrics and returns a KPI snapshot
func (a *Analyzer) Update(host string, metrics map[string]float64) models.KPISnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	hs := a.getOrCreate(host)
	// Store raw inputs for /explain endpoint
	last := make(map[string]float64, len(metrics))
	for k, v := range metrics {
		last[k] = v
	}
	hs.lastMetrics = last
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

	// --- Fatigue (v5.0 — Dissipative) ---
	// F_t = max(0, F_{t−1} + (S_t − R_threshold) − λ_eff)
	// λ is specified per 15s interval; scale by actual dt for variable-rate robustness.
	// Cap dt to avoid first-call spike on restart.
	const (
		maxDt           = 30.0 // cap to 2× expected interval
		nominalInterval = 15.0 // seconds per collection cycle
	)
	if dt > maxDt {
		dt = maxDt
	}
	lambdaEff := hs.fatigueLambda * (dt / nominalInterval)
	hs.fatigue = math.Max(0, hs.fatigue+(stress-hs.fatigueRThreshold)-lambdaEff)
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

	// --- ETF-style Composed KPIs ---

	// Resilience: ability to absorb disruption without failing
	// High mood + low fatigue + low contagion = resilient
	// R = mood × (1 - fatigue) × (1 - contagion)
	resilience := utils.Clamp(mood*(1-hs.fatigue)*(1-contagion), 0, 1)

	// HealthScore: single executive composite [0, 1] (mapped to [0,100] in API)
	// Weighted average: stress inverted (calm is healthy), mood positive, fatigue inverted,
	// pressure inverted, humidity inverted, contagion inverted
	healthScore := utils.Clamp(
		0.25*(1-stress)+
			0.20*mood+
			0.20*(1-hs.fatigue)+
			0.15*(1-pressureNorm)+
			0.10*(1-humidity)+
			0.10*(1-contagion),
		0, 1)

	// Entropy: system disorder — how much KPI values deviate from their rolling mean
	// Computed as mean absolute deviation of health history (normalized)
	hs.healthHistory.Push(healthScore)
	healthVals := hs.healthHistory.Values()
	entropy := 0.0
	if len(healthVals) > 1 {
		// mean
		sum := 0.0
		for _, v := range healthVals {
			sum += v
		}
		mean := sum / float64(len(healthVals))
		// mean absolute deviation
		mad := 0.0
		for _, v := range healthVals {
			mad += math.Abs(v - mean)
		}
		mad /= float64(len(healthVals))
		// normalize: max theoretical MAD for [0,1] values is 0.5
		entropy = utils.Clamp(mad/0.5, 0, 1)
	}

	// Velocity: rate of change of HealthScore (momentum)
	// High velocity = system changing fast (could be recovering or crashing)
	velocity := 0.0
	if !hs.firstUpdate && dt > 0 {
		delta := math.Abs(healthScore - hs.lastHealthScore)
		// normalize by expected max change rate: 0.1 per second = extreme
		velocity = utils.Clamp(delta/(0.1*dt), 0, 1)
	}

	// Update last state
	hs.lastStress = stress
	hs.lastHealthScore = healthScore
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
		Resilience: models.KPI{
			Name:      "resilience",
			Value:     utils.RoundTo(resilience, 4),
			State:     resilienceState(resilience),
			Timestamp: now,
			Host:      host,
		},
		Entropy: models.KPI{
			Name:      "entropy",
			Value:     utils.RoundTo(entropy, 4),
			State:     entropyState(entropy),
			Timestamp: now,
			Host:      host,
		},
		Velocity: models.KPI{
			Name:      "velocity",
			Value:     utils.RoundTo(velocity, 4),
			State:     velocityState(velocity),
			Timestamp: now,
			Host:      host,
		},
		HealthScore: models.KPI{
			Name:      "health_score",
			Value:     utils.RoundTo(healthScore*100, 2), // expose as 0-100
			State:     healthScoreState(healthScore),
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

// LastMetrics returns the most recent raw metric inputs for a host.
// Used by the /explain/:kpi endpoint to compute input contributions.
func (a *Analyzer) LastMetrics(host string) (map[string]float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	hs, ok := a.hosts[host]
	if !ok || hs.lastMetrics == nil {
		return nil, false
	}
	cp := make(map[string]float64, len(hs.lastMetrics))
	for k, v := range hs.lastMetrics {
		cp[k] = v
	}
	return cp, true
}

// SetFatigueConfig overrides the dissipative fatigue parameters for a host.
// Call before the first Update() for a host to take effect from the start.
// If not called, canonical v5.0 defaults (RThreshold=0.3, Lambda=0.05) are used.
func (a *Analyzer) SetFatigueConfig(host string, rThreshold, lambda float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	hs := a.getOrCreate(host)
	hs.fatigueRThreshold = rThreshold
	hs.fatigueLambda = lambda
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

func resilienceState(r float64) string {
	switch {
	case r > 0.7:
		return "robust"
	case r > 0.4:
		return "stable"
	case r > 0.2:
		return "fragile"
	default:
		return "critical"
	}
}

func entropyState(e float64) string {
	switch {
	case e < 0.1:
		return "ordered"
	case e < 0.3:
		return "fluctuating"
	case e < 0.6:
		return "chaotic"
	default:
		return "turbulent"
	}
}

func velocityState(v float64) string {
	switch {
	case v < 0.1:
		return "steady"
	case v < 0.3:
		return "shifting"
	case v < 0.6:
		return "accelerating"
	default:
		return "volatile"
	}
}

func healthScoreState(h float64) string {
	// h is in [0,1] (raw, before *100 scaling)
	switch {
	case h > 0.80:
		return "excellent"
	case h > 0.60:
		return "good"
	case h > 0.40:
		return "fair"
	case h > 0.20:
		return "poor"
	default:
		return "critical"
	}
}

// AllHosts returns all known host names
func (a *Analyzer) AllHosts() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	hosts := make([]string, 0, len(a.snapshots))
	for h := range a.snapshots {
		hosts = append(hosts, h)
	}
	return hosts
}
