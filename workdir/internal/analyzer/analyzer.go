package analyzer

import (
	"math"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra/dag"
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

	topology     TopologySource
	fingerprints *fingerprintEngine

	// v6.6: per-workload HealthScore signal weight overrides (ordered; first match wins).
	weightConfigs []models.SignalWeights

	// v8.0: infra collector registry — nil when not running in-cluster.
	infraRegistry *dag.Registry
}

// SetInfraRegistry wires the infra collector registry for namespace-level pressure signals.
// When nil (default), infraStress=0 and networkHealth=1.0 — identical to v7 behavior.
func (a *Analyzer) SetInfraRegistry(r *dag.Registry) {
	a.mu.Lock()
	a.infraRegistry = r
	a.mu.Unlock()
}

// SetTopology injects a live topology source for graph-based contagion computation.
func (a *Analyzer) SetTopology(t TopologySource) {
	a.mu.Lock()
	a.topology = t
	a.mu.Unlock()
}

// SetWeightConfigs replaces the list of per-workload HealthScore signal weight overrides.
// Entries are evaluated in order; the first selector that matches the workload key wins.
// Each entry is normalised so its weights sum to 1.0 before storing.
func (a *Analyzer) SetWeightConfigs(cfgs []models.SignalWeights) {
	normalised := make([]models.SignalWeights, len(cfgs))
	for i, c := range cfgs {
		normalised[i] = normaliseWeights(c)
	}
	a.mu.Lock()
	a.weightConfigs = normalised
	a.mu.Unlock()
}

// WeightConfigs returns the current list of weight overrides (a copy).
func (a *Analyzer) WeightConfigs() []models.SignalWeights {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]models.SignalWeights, len(a.weightConfigs))
	copy(out, a.weightConfigs)
	return out
}

// resolveWeights returns the first matching SignalWeights for the given workload key,
// falling back to DefaultSignalWeights when no entry matches.
// Caller must hold a.mu (any variant).
func (a *Analyzer) resolveWeights(workloadKey string) models.SignalWeights {
	for _, cfg := range a.weightConfigs {
		if matchSelector(cfg.Selector, workloadKey) {
			return cfg
		}
	}
	return models.DefaultSignalWeights()
}

// matchSelector matches a workload key against a selector pattern.
//   - "*"         matches any key
//   - "ns/*"      matches any key with prefix "ns/"
//   - "ns/k/name" exact match
func matchSelector(selector, key string) bool {
	if selector == "*" {
		return true
	}
	if strings.HasSuffix(selector, "/*") {
		prefix := strings.TrimSuffix(selector, "*")
		return strings.HasPrefix(key, prefix)
	}
	return selector == key
}

// normaliseWeights scales all signal weights so they sum to 1.0.
// InfraStress and NetworkHealth default to 0 (omitempty) so existing configs
// that omit infra fields normalise correctly to 1.0 unchanged.
func normaliseWeights(w models.SignalWeights) models.SignalWeights {
	total := w.Stress + w.Fatigue + w.Mood + w.Pressure + w.Humidity + w.Contagion + w.InfraStress + w.NetworkHealth
	if total < 1e-9 {
		return w
	}
	w.Stress /= total
	w.Fatigue /= total
	w.Mood /= total
	w.Pressure /= total
	w.Humidity /= total
	w.Contagion /= total
	w.InfraStress /= total
	w.NetworkHealth /= total
	return w
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
	baselineCounts   map[string]int     // observations per signal

	// v6.4: business signals
	sloConfig     *models.SLOConfig
	nearMissTimes []time.Time
	inNearMiss    bool

	// v7: set true by RegisterWorkload, cleared on first Update call.
	// AllSnapshots synthesises a pending_telemetry snapshot while this is true.
	pendingTelemetry bool
}

// NewAnalyzer creates a new holistic analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		workloads:                make(map[string]*workloadState),
		snapshots:                make(map[string]models.KPISnapshot),
		defaultFatigueRThreshold: 0.3,
		defaultFatigueLambda:     0.05,
		fingerprints:             newFingerprintEngine(),
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
	ws.pendingTelemetry = false // first real data — no longer pending
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
	cpu := readMetric(metrics, "cpu_percent")
	ram := readMetric(metrics, "memory_percent")
	latency := readMetric(metrics, "latency", "load_avg_1")
	errors := readMetric(metrics, "error_rate")
	timeouts := readMetric(metrics, "timeout_rate")

	stress := weightedMean([]weightedInput{
		{0.30, cpu}, {0.20, ram}, {0.20, latency}, {0.20, errors}, {0.10, timeouts},
	})
	if stress.available {
		stress.value = utils.Clamp(stress.value, 0, 1)
		ws.stressHistory.Push(stress.value)
	}

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
	fatigue := reading{}
	if stress.available {
		ws.fatigue = math.Max(0, ws.fatigue+(stress.value-ws.fatigueRThreshold)-lambdaEff)
		ws.fatigue = utils.Clamp(ws.fatigue, 0, 1)
		fatigue = measured(ws.fatigue)
	}

	// --- Mood ---
	// M = (Uptime × Throughput) / (Errors × Timeouts × Restarts + ε)
	// ε guards the WHOLE denominator product (not individual factors) per spec.
	// Log normalization: log(rawMood + 1) / log(max_expected + 1) maps to [0, 1].
	uptime := readMetric(metrics, "uptime_seconds")
	ws.uptime = uptime.value // always update (including 0) to avoid stale value

	requests := readMetric(metrics, "request_rate")
	ws.requestHistory.Push(requests.value)
	ws.errorHistory.Push(errors.value)
	ws.timeoutHistory.Push(timeouts.value)

	mood := reading{}
	if allAvailable(uptime, requests, errors, timeouts) {
		restarts := ws.restartCount + 1 // always at least 1 to avoid division by zero
		// Denominator: errors × timeouts × restarts with ε protecting against zero
		denominator := errors.value*timeouts.value*restarts + epsilon
		rawMood := (ws.uptime * (requests.value + epsilon)) / denominator
		// Log-normalize: log(1 + rawMood) / log(1 + expectedMax)
		// expectedMax ≈ uptime(86400s) × throughput(1) / epsilon ≈ 8.64e14 → use log ceiling 35
		const moodLogCeiling = 35.0
		mood = measured(utils.Clamp(math.Log1p(rawMood)/moodLogCeiling, 0, 1))
	}

	// --- Atmospheric Pressure (EWMA z-score, ported from composites engine) ---
	pressure := reading{}
	var zScores []weightedInput
	if latency.available {
		ws.muLat = 0.9*ws.muLat + 0.1*latency.value
		ws.sigma2Lat = 0.9*ws.sigma2Lat + 0.1*math.Pow(latency.value-ws.muLat, 2)
		sigmaLat := math.Sqrt(ws.sigma2Lat)
		if sigmaLat < 1e-6 {
			sigmaLat = 1.0
		}
		zScores = append(zScores, weightedInput{0.5, measured((latency.value - ws.muLat) / sigmaLat)})
	}
	if errors.available {
		ws.muErr = 0.9*ws.muErr + 0.1*errors.value
		ws.sigma2Err = 0.9*ws.sigma2Err + 0.1*math.Pow(errors.value-ws.muErr, 2)
		sigmaErr := math.Sqrt(ws.sigma2Err)
		if sigmaErr < 1e-6 {
			sigmaErr = 1.0
		}
		zScores = append(zScores, weightedInput{0.5, measured((errors.value - ws.muErr) / sigmaErr)})
	}
	if rawPressure := weightedMean(zScores); rawPressure.available {
		pressure = measured(utils.Clamp((rawPressure.value+3)/6.0, 0, 1)) // map [-3,+3] z-score to [0,1]
	}

	// --- Error Humidity ---
	// H = (E × T) / Q
	humidity := reading{}
	if allAvailable(errors, timeouts, requests) {
		humidity = measured(utils.Clamp((errors.value*timeouts.value)/(requests.value+epsilon), 0, 1))
	}

	// --- Contagion Index ---
	// If topology is available: max incoming error propagation across upstream callers.
	// Fallback: errors×cpu proxy when no trace edges exist for this workload.
	contagion := reading{}
	if a.topology != nil {
		edges := a.topology.Edges()
		for _, e := range edges {
			if e.To == ws.ref.Name && e.Calls > 0 {
				errRate := float64(e.Errors) / float64(e.Calls)
				weight := utils.Clamp(float64(e.Calls)/1000.0, 0, 1) // edge weight by call volume
				prop := errRate * weight
				if !contagion.available || prop > contagion.value {
					contagion = measured(prop)
				}
			}
		}
	}
	if contagion.value == 0 && allAvailable(errors, cpu) {
		contagion = measured(errors.value * cpu.value)
	}

	// --- v8.0: Infra + CGPM integration (nil-safe; defaults to v7 behavior when absent) ---
	infraStress := reading{}
	networkHealth := reading{}
	if a.infraRegistry != nil {
		nsSnap := a.infraRegistry.NamespaceSnapshot(ref.Namespace)
		infraStress = measured(nsSnap.InfraStress)
		networkHealth = measured(nsSnap.NetworkHealth)
		// Fold CGPM PropPressure into contagion via max() — existing contagion never decreases.
		if nsSnap.PropPressure > contagion.value {
			contagion = measured(nsSnap.PropPressure)
		}
	}
	if contagion.available {
		contagion.value = utils.Clamp(contagion.value, 0, 1)
	}

	// --- Throughput collapse signal (v6.1) ---
	throughputDrop := reading{}
	if requests.available {
		throughputDrop = measured(0)
		if ws.prevRequestRate > 0.01 && requests.value < ws.prevRequestRate {
			drop := (ws.prevRequestRate - requests.value) / ws.prevRequestRate
			throughputDrop = measured(utils.Clamp(drop, 0, 1))
		}
		ws.prevRequestRate = requests.value
	}

	// --- ETF-style Composed KPIs ---

	// Resilience: ability to absorb disruption without failing
	// High mood + low fatigue + low contagion = resilient
	// R = mood × (1 - fatigue) × (1 - contagion)
	resilience := reading{}
	if allAvailable(mood, fatigue, contagion) {
		resilience = measured(utils.Clamp(mood.value*(1-ws.fatigue)*(1-contagion.value), 0, 1))
	}

	// HealthScore: additive weighted penalty (avoids multiplicative collapse).
	// Weights are resolved per-workload; fall back to global defaults when no override matches.
	w := a.resolveWeights(ref.Key())
	moodPenalty := reading{value: 1 - mood.value, available: mood.available}
	networkPenalty := reading{value: 1 - networkHealth.value, available: networkHealth.available}
	healthScore := reading{}
	if penalty := weightedMean([]weightedInput{
		{w.Stress, stress}, {w.Fatigue, fatigue}, {w.Mood, moodPenalty}, {w.Pressure, pressure},
		{w.Humidity, humidity}, {w.Contagion, contagion}, {w.InfraStress, infraStress},
		{w.NetworkHealth, networkPenalty},
	}); penalty.available {
		healthScore = measured(utils.Clamp(1-penalty.value, 0, 1))
	}

	// Entropy: system disorder — how much KPI values deviate from their rolling mean
	// Computed as mean absolute deviation of health history (normalized)
	entropy := reading{}
	if healthScore.available {
		ws.healthHistory.Push(healthScore.value)
		entropy = measured(0)
		healthVals := ws.healthHistory.Values()
		if len(healthVals) > 1 {
			sum := 0.0
			for _, v := range healthVals {
				sum += v
			}
			mean := sum / float64(len(healthVals))
			mad := 0.0
			for _, v := range healthVals {
				mad += math.Abs(v - mean)
			}
			mad /= float64(len(healthVals))
			// normalize: max theoretical MAD for [0,1] values is 0.5
			entropy = measured(utils.Clamp(mad/0.5, 0, 1))
		}
	}

	// Velocity: rate of change of HealthScore (momentum)
	// High velocity = system changing fast (could be recovering or crashing)
	velocity := reading{}
	if healthScore.available {
		velocity = measured(0)
		if !ws.firstUpdate && dt > 0 {
			delta := math.Abs(healthScore.value - ws.lastHealthScore)
			// normalize by expected max change rate: 0.1 per second = extreme
			velocity = measured(utils.Clamp(delta/(0.1*dt), 0, 1))
		}
	}

	// --- Adaptive baseline update (Welford online algorithm) ---
	// mood is stored as 1-mood (penalty direction) so that adaptiveScore(ws,"mood",1-mood)
	// computes a z-score relative to the workload's normal penalty level, not its raw mood.
	// Without this, idle workloads (mood=0 always, baseline=0) produce z=(1-0)/sigma_guard=20
	// → full penalty every tick → healthScore stuck at exactly 0.80 regardless of activity.
	signals := map[string]reading{
		"stress":          stress,
		"fatigue":         fatigue,
		"mood":            moodPenalty,
		"pressure":        pressure,
		"humidity":        humidity,
		"contagion":       contagion,
		"throughput_drop": throughputDrop,
		"health_score":    healthScore,
	}
	baselineInputs := make(map[string]float64, len(signals))
	for name, signal := range signals {
		if signal.available {
			baselineInputs[name] = signal.value
		}
	}
	ws.updateBaseline(baselineInputs)

	// After baseline is established, recalculate HealthScore using workload-relative
	// z-scores so global thresholds don't produce false positives for heavy-load workloads.
	// Fatigue keeps its absolute threshold — sustained effort is fatigue regardless of "normal".
	if ws.baselineReady {
		relative := func(name string, r reading) reading {
			if !r.available {
				return r
			}
			return measured(adaptiveScore(ws, name, r.value))
		}
		// infraStress and networkHealth are registry-sourced; not adaptive per-workload baseline.
		if penalty := weightedMean([]weightedInput{
			{w.Stress, relative("stress", stress)}, {w.Fatigue, fatigue},
			{w.Mood, relative("mood", moodPenalty)}, {w.Pressure, relative("pressure", pressure)},
			{w.Humidity, relative("humidity", humidity)}, {w.Contagion, relative("contagion", contagion)},
			{w.InfraStress, infraStress}, {w.NetworkHealth, networkPenalty},
		}); penalty.available {
			healthScore = measured(utils.Clamp(1-penalty.value, 0, 1))
		}
	}

	// Update last state
	ws.lastStress = stress.value
	if healthScore.available {
		ws.lastHealthScore = healthScore.value
	}
	ws.lastUpdate = now
	ws.firstUpdate = false

	key := ref.Key()
	host := models.FirstNonEmpty(ref.Node, ref.Name)
	kpi := func(name string, r reading, state func(float64) string) models.KPI {
		k := models.KPI{Name: name, Timestamp: now, Host: host, Workload: ref}
		if !r.available {
			k.State = stateUnavailable
			k.Unavailable = true
			return k
		}
		k.Value = utils.RoundTo(r.value, 4)
		k.State = state(r.value)
		return k
	}
	snap := models.KPISnapshot{
		Host:        host,
		Workload:    ref,
		Timestamp:   now,
		Stress:      kpi("stress", stress, stressState),
		Fatigue:     kpi("fatigue", fatigue, fatigueState),
		Mood:        kpi("mood", mood, moodState),
		Pressure:    kpi("pressure", pressure, pressureState),
		Humidity:    kpi("humidity", humidity, humidityState),
		Contagion:   kpi("contagion", contagion, contagionState),
		Resilience:  kpi("resilience", resilience, resilienceState),
		Entropy:     kpi("entropy", entropy, entropyState),
		Velocity:    kpi("velocity", velocity, velocityState),
		Throughput:  kpi("throughput", throughputDrop, throughputState),
		HealthScore: kpi("health_score", healthScore, healthScoreState),
		UnavailableSignals: unavailableSignals(map[string]reading{
			"stress": stress, "fatigue": fatigue, "mood": mood, "pressure": pressure,
			"humidity": humidity, "contagion": contagion, "resilience": resilience,
			"entropy": entropy, "velocity": velocity, "throughput": throughputDrop,
			"health_score": healthScore,
		}),
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
		ws.baselineCounts = make(map[string]int)
	}
	for k, v := range signals {
		ws.baselineCounts[k]++
		n := float64(ws.baselineCounts[k])
		delta := v - ws.baselineMeans[k]
		ws.baselineMeans[k] += delta / n
		ws.baselineM2[k] += delta * (v - ws.baselineMeans[k])
	}
	if ws.observationCount >= 96 { // 24 minutes at 15s intervals
		ws.baselineReady = true
	}
}

// minBaselineSigma keeps a workload with almost no variance from turning a
// negligible move into a full penalty. Signals are in [0,1], so the score reaches
// its maximum no sooner than 0.15 above the baseline mean.
const minBaselineSigma = 0.05

// adaptiveScore converts a raw signal value to a workload-relative score in [0,1].
// When the value equals the workload's baseline mean it returns 0 (healthy).
// A 3σ deviation above the mean returns 1 (max penalty).
// Falls back to the raw value when baseline data is insufficient.
func adaptiveScore(ws *workloadState, signal string, raw float64) float64 {
	if !ws.baselineReady || ws.observationCount < 2 {
		return raw
	}
	mean, ok := ws.baselineMeans[signal]
	count := ws.baselineCounts[signal]
	if !ok || count < 2 {
		return raw
	}
	m2 := ws.baselineM2[signal]
	variance := m2 / float64(count-1)
	sigma := math.Sqrt(variance)
	if sigma < minBaselineSigma {
		sigma = minBaselineSigma
	}
	z := (raw - mean) / sigma
	return utils.Clamp(z/3.0, 0, 1) // 3σ above normal = max penalty
}

// BaselineReady returns true if the workload has accumulated enough observations
// to use adaptive baseline thresholding (96 observations, about 24 minutes at 15s intervals).
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
	count := ws.baselineCounts[signal]
	if !ok || count < 2 {
		return 0
	}
	variance := m2 / float64(count-1)
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

// CalibrationInfo returns the calibration status, progress (0–100), and ETA in minutes
// for the given workload. A workload is "active" once it has >= 96 observations (~24h).
func (a *Analyzer) CalibrationInfo(ref models.WorkloadRef) (status string, progress int, etaMinutes int) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ws, ok := a.workloads[ref.Key()]
	if !ok {
		return "calibrating", 0, 24*60
	}
	const required = 96
	if ws.baselineReady {
		return "active", 100, 0
	}
	pct := ws.observationCount * 100 / required
	if pct > 99 {
		pct = 99
	}
	remaining := required - ws.observationCount
	if remaining < 0 {
		remaining = 0
	}
	etaMinutes = remaining * 15 / 60
	return "calibrating", pct, etaMinutes
}

// ForecastHealthScore computes a linear trend projection over the rolling health history.
// Returns nil when fewer than 10 observations are available or the workload is still calibrating.
func (a *Analyzer) ForecastHealthScore(ref models.WorkloadRef) *models.HealthForecast {
	a.mu.RLock()
	defer a.mu.RUnlock()
	ws, ok := a.workloads[ref.Key()]
	if !ok || !ws.baselineReady {
		return nil
	}
	vals := ws.healthHistory.Values()
	n := len(vals)
	if n < 10 {
		return nil
	}

	// Ordinary least squares: y = m*x + b over indices 0..n-1
	fn := float64(n)
	var sumX, sumY, sumXY, sumX2 float64
	for i, v := range vals {
		x := float64(i)
		sumX += x
		sumY += v
		sumXY += x * v
		sumX2 += x * x
	}
	denom := fn*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-12 {
		return &models.HealthForecast{Trend: "stable", In15Min: vals[n-1], In30Min: vals[n-1]}
	}
	slope := (fn*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / fn

	// Project: each tick ≈ 15s. 15min = 60 ticks, 30min = 120 ticks from index 0.
	project := func(ticks float64) float64 {
		raw := slope*(float64(n-1)+ticks) + intercept
		return utils.Clamp(raw, 0, 1)
	}
	in15 := project(60)
	in30 := project(120)

	trend := "stable"
	switch {
	case slope > 0.001:
		trend = "improving"
	case slope < -0.001:
		trend = "degrading"
	}

	var critETA int
	if slope < -1e-6 {
		// Solve slope*(n-1+k) + intercept = 0.40 (poor threshold in raw [0,1])
		k := (0.40 - intercept - slope*float64(n-1)) / slope
		if k > 0 {
			critETA = int(k * 15 / 60) // ticks × 15s / 60 = minutes
		}
	}

	return &models.HealthForecast{
		Trend:              trend,
		In15Min:            math.Round(in15*1000) / 1000,
		In30Min:            math.Round(in30*1000) / 1000,
		CriticalETAMinutes: critETA,
		ConfidenceWindow:   n,
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

// RegisterWorkload pre-registers a workload discovered via the k8s informer.
// Idempotent — if the workload already has telemetry, this is a no-op.
// The workload will appear in AllSnapshots with status "pending_telemetry" until
// the first Update() call transitions it to "calibrating".
func (a *Analyzer) RegisterWorkload(ref models.WorkloadRef) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := ref.Key()
	if _, ok := a.workloads[key]; ok {
		return // already known (either from telemetry or prior registration)
	}
	ws := a.getOrCreate(ref)
	ws.pendingTelemetry = true
}

// UnregisterWorkload removes a workload that was deleted from the cluster.
// Existing snapshots and telemetry history are discarded.
func (a *Analyzer) UnregisterWorkload(ref models.WorkloadRef) {
	a.mu.Lock()
	defer a.mu.Unlock()
	key := ref.Key()
	delete(a.workloads, key)
	delete(a.snapshots, key)
}

// AllWorkloadRefs returns every registered WorkloadRef, including pending ones.
func (a *Analyzer) AllWorkloadRefs() []models.WorkloadRef {
	a.mu.RLock()
	defer a.mu.RUnlock()
	refs := make([]models.WorkloadRef, 0, len(a.workloads))
	for _, ws := range a.workloads {
		refs = append(refs, ws.ref)
	}
	return refs
}

// AnalyzerStats holds aggregate workload counts for the engine status endpoint.
type AnalyzerStats struct {
	TotalWorkloads       int
	ActiveWorkloads      int
	CalibratingWorkloads int
	PendingWorkloads     int
}

// Stats returns aggregate counts across all registered workloads.
func (a *Analyzer) Stats() AnalyzerStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	var s AnalyzerStats
	s.TotalWorkloads = len(a.workloads)
	for _, ws := range a.workloads {
		if ws.pendingTelemetry {
			s.PendingWorkloads++
			continue
		}
		if ws.observationCount >= 96 {
			s.ActiveWorkloads++
		} else {
			s.CalibratingWorkloads++
		}
	}
	return s
}

// AllAnalyzerSnapshots returns one KPISnapshot per registered workload.
// For pending workloads (no telemetry yet) it synthesises a zero-value snapshot
// with WorkloadStatus = "pending_telemetry".
func (a *Analyzer) AllAnalyzerSnapshots() []models.KPISnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]models.KPISnapshot, 0, len(a.workloads))
	for key, ws := range a.workloads {
		if ws.pendingTelemetry {
			host := ws.ref.Name
			if ws.ref.Node != "" {
				host = ws.ref.Node
			}
			out = append(out, models.KPISnapshot{
				Host:           host,
				Workload:       ws.ref,
				Timestamp:      time.Now(),
				WorkloadStatus: models.WorkloadStatusPending,
				HealthScore: models.KPI{
					Name:  "health_score",
					Value: 0,
					State: "pending",
				},
			})
			continue
		}
		if snap, ok := a.snapshots[key]; ok {
			out = append(out, snap)
		}
	}
	return out
}
