package analyzer

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
)

const epsilon = 1e-10

// TopologySource provides live service dependency edges from trace data.
type TopologySource interface {
	Edges() []models.ServiceEdge
}

// Analyzer computes holistic KPIs from normalized metrics
type Analyzer struct {
	mu sync.RWMutex

	// Per-workload accumulated state; key = WorkloadRef.Key()
	workloads map[string]*workloadState
	snapshots map[string]models.KPISnapshot // last computed snapshot per workload

	// Default fatigue config applied to new workloads; overridable per-workload via SetFatigueConfig.
	defaultFatigueRThreshold float64
	defaultFatigueLambda     float64

	topology TopologySource
}

// SetTopology injects a live topology source for graph-based contagion computation.
func (a *Analyzer) SetTopology(t TopologySource) {
	a.mu.Lock()
	a.topology = t
	a.mu.Unlock()
}

type workloadState struct {
	// WorkloadRef for this state entry
	ref models.WorkloadRef
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

	// EWMA Pressure state (ported from composites engine)
	muLat, sigma2Lat float64 // EWMA mean and variance for latency
	muErr, sigma2Err float64 // EWMA mean and variance for error_rate

	// Throughput collapse signal
	prevRequestRate float64

	// Adaptive baseline (Welford online algorithm)
	observationCount int
	baselineReady    bool
	baselineMeans    map[string]float64 // per-signal rolling mean
	baselineM2       map[string]float64 // Welford M2 for stddev
}

// NewAnalyzer creates a new holistic analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		workloads:                make(map[string]*workloadState),
		snapshots:                make(map[string]models.KPISnapshot),
		defaultFatigueRThreshold: 0.3,
		defaultFatigueLambda:     0.05,
	}
}

// SetDefaultFatigueConfig sets the dissipative fatigue parameters applied to
// all new workloads. Existing workloads are not affected; use SetFatigueConfig for those.
func (a *Analyzer) SetDefaultFatigueConfig(rThreshold, lambda float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.defaultFatigueRThreshold = rThreshold
	a.defaultFatigueLambda = lambda
}

func (a *Analyzer) getOrCreate(ref models.WorkloadRef) *workloadState {
	key := ref.Key()
	if ws, ok := a.workloads[key]; ok {
		return ws
	}
	ws := &workloadState{
		ref:               ref,
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
	a.workloads[key] = ws
	return ws
}

// Update ingests new normalized metrics for a workload and returns a KPI snapshot.
func (a *Analyzer) Update(ref models.WorkloadRef, metrics map[string]float64) models.KPISnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()

	ws := a.getOrCreate(ref)
	// Store raw inputs for /explain endpoint
	last := make(map[string]float64, len(metrics))
	for k, v := range metrics {
		last[k] = v
	}
	ws.lastMetrics = last
	now := time.Now()
	dt := now.Sub(ws.lastUpdate).Seconds()
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

	ws.stressHistory.Push(stress)

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
	lambdaEff := ws.fatigueLambda * (dt / nominalInterval)
	ws.fatigue = math.Max(0, ws.fatigue+(stress-ws.fatigueRThreshold)-lambdaEff)
	ws.fatigue = utils.Clamp(ws.fatigue, 0, 1)

	// --- Mood ---
	// M = (Uptime × Throughput) / (Errors × Timeouts × Restarts + ε)
	// ε guards the WHOLE denominator product (not individual factors) per spec.
	// Log normalization: log(rawMood + 1) / log(max_expected + 1) maps to [0, 1].
	uptime := getMetric(metrics, "uptime_seconds")
	ws.uptime = uptime // always update (including 0) to avoid stale value

	requests := getMetric(metrics, "request_rate")
	ws.requestHistory.Push(requests)
	ws.errorHistory.Push(errors)
	ws.timeoutHistory.Push(timeouts)

	restarts := ws.restartCount + 1 // always at least 1 to avoid division by zero
	// Denominator: errors × timeouts × restarts with ε protecting against zero
	denominator := errors*timeouts*restarts + epsilon
	rawMood := (ws.uptime * (requests + epsilon)) / denominator
	// Log-normalize: log(1 + rawMood) / log(1 + expectedMax)
	// expectedMax ≈ uptime(86400s) × throughput(1) / epsilon ≈ 8.64e14 → use log ceiling 35
	const moodLogCeiling = 35.0
	mood := utils.Clamp(math.Log1p(rawMood)/moodLogCeiling, 0, 1)

	// --- Atmospheric Pressure (EWMA z-score, ported from composites engine) ---
	lat := getMetric(metrics, "latency")
	if lat == 0 {
		lat = getMetric(metrics, "load_avg_1")
	}
	errRate := getMetric(metrics, "error_rate")

	ws.muLat = 0.9*ws.muLat + 0.1*lat
	ws.sigma2Lat = 0.9*ws.sigma2Lat + 0.1*math.Pow(lat-ws.muLat, 2)
	ws.muErr = 0.9*ws.muErr + 0.1*errRate
	ws.sigma2Err = 0.9*ws.sigma2Err + 0.1*math.Pow(errRate-ws.muErr, 2)

	sigmaLat := math.Sqrt(ws.sigma2Lat)
	if sigmaLat < 1e-6 {
		sigmaLat = 1.0
	}
	sigmaErr := math.Sqrt(ws.sigma2Err)
	if sigmaErr < 1e-6 {
		sigmaErr = 1.0
	}

	latencyZ := (lat - ws.muLat) / sigmaLat
	errorZ := (errRate - ws.muErr) / sigmaErr
	rawPressure := 0.5*latencyZ + 0.5*errorZ
	pressureNorm := utils.Clamp((rawPressure+3)/6.0, 0, 1) // map [-3,+3] z-score to [0,1]

	// --- Error Humidity ---
	// H = (E × T) / Q
	throughputQ := requests + epsilon
	humidity := (errors * timeouts) / throughputQ
	humidity = utils.Clamp(humidity, 0, 1)

	// --- Contagion Index ---
	// If topology is available: max incoming error propagation across upstream callers.
	// Fallback: errors×cpu proxy when no trace edges exist for this workload.
	var contagion float64
	if a.topology != nil {
		edges := a.topology.Edges()
		for _, e := range edges {
			if e.To == ws.ref.Name && e.Calls > 0 {
				errRate := float64(e.Errors) / float64(e.Calls)
				weight := utils.Clamp(float64(e.Calls)/1000.0, 0, 1) // edge weight by call volume
				prop := errRate * weight
				if prop > contagion {
					contagion = prop
				}
			}
		}
	}
	if contagion == 0 {
		contagion = errors * cpu
	}
	contagion = utils.Clamp(contagion, 0, 1)

	// --- Throughput collapse signal (v6.1) ---
	reqRate := getMetric(metrics, "request_rate")
	throughputDrop := 0.0
	if ws.prevRequestRate > 0.01 && reqRate < ws.prevRequestRate {
		drop := (ws.prevRequestRate - reqRate) / ws.prevRequestRate
		throughputDrop = utils.Clamp(drop, 0, 1)
	}
	ws.prevRequestRate = reqRate

	// --- ETF-style Composed KPIs ---

	// Resilience: ability to absorb disruption without failing
	// High mood + low fatigue + low contagion = resilient
	// R = mood × (1 - fatigue) × (1 - contagion)
	resilience := utils.Clamp(mood*(1-ws.fatigue)*(1-contagion), 0, 1)

	// HealthScore: additive weighted penalty (avoids multiplicative collapse).
	// penalty = weighted sum of bad signals; healthScore = 1 - penalty, clamped [0,1].
	// Weights: stress=0.25, fatigue=0.20, mood-inv=0.20, pressure=0.15, humidity=0.10, contagion=0.10
	penalty := 0.25*stress + 0.20*ws.fatigue + 0.20*(1-mood) + 0.15*pressureNorm + 0.10*humidity + 0.10*contagion
	healthScore := utils.Clamp(1-penalty, 0, 1)

	// Entropy: system disorder — how much KPI values deviate from their rolling mean
	// Computed as mean absolute deviation of health history (normalized)
	ws.healthHistory.Push(healthScore)
	healthVals := ws.healthHistory.Values()
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
	if !ws.firstUpdate && dt > 0 {
		delta := math.Abs(healthScore - ws.lastHealthScore)
		// normalize by expected max change rate: 0.1 per second = extreme
		velocity = utils.Clamp(delta/(0.1*dt), 0, 1)
	}

	// --- Adaptive baseline update (Welford online algorithm) ---
	signals := map[string]float64{
		"stress":          stress,
		"fatigue":         ws.fatigue,
		"mood":            mood,
		"pressure":        pressureNorm,
		"humidity":        humidity,
		"contagion":       contagion,
		"throughput_drop": throughputDrop,
		"health_score":    healthScore,
	}
	ws.updateBaseline(signals)

	// After baseline is established, recalculate HealthScore using workload-relative
	// z-scores so global thresholds don't produce false positives for heavy-load workloads.
	// Fatigue keeps its absolute threshold — sustained effort is fatigue regardless of "normal".
	if ws.baselineReady {
		relStress := adaptiveScore(ws, "stress", stress)
		relMood := adaptiveScore(ws, "mood", 1-mood) // mood positive: penalize below-baseline mood
		relPressure := adaptiveScore(ws, "pressure", pressureNorm)
		relHumidity := adaptiveScore(ws, "humidity", humidity)
		relContagion := adaptiveScore(ws, "contagion", contagion)
		penalty = 0.25*relStress + 0.20*ws.fatigue + 0.20*relMood + 0.15*relPressure + 0.10*relHumidity + 0.10*relContagion
		healthScore = utils.Clamp(1-penalty, 0, 1)
	}

	// Update last state
	ws.lastStress = stress
	ws.lastHealthScore = healthScore
	ws.lastUpdate = now
	ws.firstUpdate = false

	key := ref.Key()
	snap := models.KPISnapshot{
		Host:      ref.Node,
		Workload:  ref,
		Timestamp: now,
		Stress: models.KPI{
			Name:      "stress",
			Value:     utils.RoundTo(stress, 4),
			State:     stressState(stress),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Fatigue: models.KPI{
			Name:      "fatigue",
			Value:     utils.RoundTo(ws.fatigue, 4),
			State:     fatigueState(ws.fatigue),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Mood: models.KPI{
			Name:      "mood",
			Value:     utils.RoundTo(mood, 4),
			State:     moodState(mood),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Pressure: models.KPI{
			Name:      "pressure",
			Value:     utils.RoundTo(pressureNorm, 4),
			State:     pressureState(pressureNorm),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Humidity: models.KPI{
			Name:      "humidity",
			Value:     utils.RoundTo(humidity, 4),
			State:     humidityState(humidity),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Contagion: models.KPI{
			Name:      "contagion",
			Value:     utils.RoundTo(contagion, 4),
			State:     contagionState(contagion),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Resilience: models.KPI{
			Name:      "resilience",
			Value:     utils.RoundTo(resilience, 4),
			State:     resilienceState(resilience),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Entropy: models.KPI{
			Name:      "entropy",
			Value:     utils.RoundTo(entropy, 4),
			State:     entropyState(entropy),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Velocity: models.KPI{
			Name:      "velocity",
			Value:     utils.RoundTo(velocity, 4),
			State:     velocityState(velocity),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		Throughput: models.KPI{
			Name:      "throughput",
			Value:     utils.RoundTo(throughputDrop, 4),
			State:     throughputState(throughputDrop),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
		HealthScore: models.KPI{
			Name:      "health_score",
			Value:     utils.RoundTo(healthScore*100, 2), // expose as 0-100
			State:     healthScoreState(healthScore),
			Timestamp: now,
			Host:      ref.Node,
			Workload:  ref,
		},
	}
	a.snapshots[key] = snap
	return snap
}

// UpdateHost is a backward-compatible wrapper for callers using the old host-string API.
func (a *Analyzer) UpdateHost(host string, metrics map[string]float64) models.KPISnapshot {
	return a.Update(models.WorkloadRefFromHost(host), metrics)
}

// updateBaseline applies the Welford online algorithm to maintain per-signal
// rolling mean and M2 (sum of squared deviations) for adaptive thresholding.
// baselineReady is set to true after 96 observations (24h at 15s intervals).
func (ws *workloadState) updateBaseline(signals map[string]float64) {
	ws.observationCount++
	if ws.baselineMeans == nil {
		ws.baselineMeans = make(map[string]float64)
		ws.baselineM2 = make(map[string]float64)
	}
	for k, v := range signals {
		n := float64(ws.observationCount)
		delta := v - ws.baselineMeans[k]
		ws.baselineMeans[k] += delta / n
		ws.baselineM2[k] += delta * (v - ws.baselineMeans[k])
	}
	if ws.observationCount >= 96 { // 24h at 15s intervals
		ws.baselineReady = true
	}
}

// adaptiveScore converts a raw signal value to a workload-relative score in [0,1].
// When the value equals the workload's baseline mean it returns 0 (healthy).
// A 3σ deviation above the mean returns 1 (max penalty).
// Falls back to the raw value when baseline data is insufficient.
func adaptiveScore(ws *workloadState, signal string, raw float64) float64 {
	if !ws.baselineReady || ws.observationCount < 2 {
		return raw
	}
	mean, ok := ws.baselineMeans[signal]
	if !ok {
		return raw
	}
	m2 := ws.baselineM2[signal]
	variance := m2 / float64(ws.observationCount-1)
	sigma := math.Sqrt(variance)
	if sigma < 1e-6 {
		// Near-zero variance: workload is very stable; any deviation is notable.
		sigma = 0.05
	}
	z := (raw - mean) / sigma
	return utils.Clamp(z/3.0, 0, 1) // 3σ above normal = max penalty
}

// BaselineReady returns true if the workload has accumulated enough observations
// to use adaptive baseline thresholding (96 observations, ~24h at 15s intervals).
func (a *Analyzer) BaselineReady(ref models.WorkloadRef) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ws, ok := a.workloads[ref.Key()]
	if !ok {
		return false
	}
	return ws.baselineReady
}

// BaselineSigma returns the standard deviation for a named signal from the
// Welford adaptive baseline. Returns 0 if the workload is unknown or the signal
// has not been observed.
func (a *Analyzer) BaselineSigma(ref models.WorkloadRef, signal string) float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ws, ok := a.workloads[ref.Key()]
	if !ok || ws.observationCount < 2 {
		return 0
	}
	m2, ok := ws.baselineM2[signal]
	if !ok {
		return 0
	}
	variance := m2 / float64(ws.observationCount-1)
	return math.Sqrt(variance)
}

// Snapshot returns the last computed KPI snapshot without mutating state (safe for GET handlers)
func (a *Analyzer) Snapshot(host string) (models.KPISnapshot, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	// Try direct lookup by host key (backward compat: host was used as map key before)
	ref := models.WorkloadRefFromHost(host)
	snap, ok := a.snapshots[ref.Key()]
	return snap, ok
}

// RecordRestart increments the restart count for a workload
func (a *Analyzer) RecordRestart(host string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ws := a.getOrCreate(models.WorkloadRefFromHost(host))
	ws.restartCount++
}

// ResetFatigue resets fatigue after maintenance/restart
func (a *Analyzer) ResetFatigue(host string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ws := a.getOrCreate(models.WorkloadRefFromHost(host))
	ws.fatigue = 0
}

// LastMetrics returns the most recent raw metric inputs for a workload.
// Used by the /explain/:kpi endpoint to compute input contributions.
func (a *Analyzer) LastMetrics(host string) (map[string]float64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ref := models.WorkloadRefFromHost(host)
	ws, ok := a.workloads[ref.Key()]
	if !ok || ws.lastMetrics == nil {
		return nil, false
	}
	cp := make(map[string]float64, len(ws.lastMetrics))
	for k, v := range ws.lastMetrics {
		cp[k] = v
	}
	return cp, true
}

// SetFatigueConfig overrides the dissipative fatigue parameters for a workload.
// Call before the first Update() for a workload to take effect from the start.
// If not called, canonical v5.0 defaults (RThreshold=0.3, Lambda=0.05) are used.
func (a *Analyzer) SetFatigueConfig(host string, rThreshold, lambda float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ws := a.getOrCreate(models.WorkloadRefFromHost(host))
	ws.fatigueRThreshold = rThreshold
	ws.fatigueLambda = lambda
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

func throughputState(t float64) string {
	switch {
	case t > 0.5:
		return "collapsing"
	case t > 0.2:
		return "declining"
	default:
		return "stable"
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

// AllHosts returns all known host names (backward-compat: returns workload keys)
func (a *Analyzer) AllHosts() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	hosts := make([]string, 0, len(a.snapshots))
	for h := range a.snapshots {
		hosts = append(hosts, h)
	}
	return hosts
}
