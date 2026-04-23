package correlator

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
)

const (
	correlationWindow = 120 * time.Second // ±2 min burst-KPI matching
	kpiDeltaThreshold = 10.0              // health_score drop of >10 pts
	pearsonWindow     = 300               // 5-min rolling Pearson window (samples)
)

// CorrelationStore persists and queries correlation events in memory.
// A production deployment should flush these to storage.
type CorrelationStore struct {
	mu     sync.RWMutex
	events []models.CorrelationEvent
	cap    int
	pos    int
	n      int
}

// NewCorrelationStore creates a ring-buffer store.
func NewCorrelationStore(cap int) *CorrelationStore {
	return &CorrelationStore{events: make([]models.CorrelationEvent, cap), cap: cap}
}

// Push adds an event.
func (s *CorrelationStore) Push(ev models.CorrelationEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[s.pos] = ev
	s.pos = (s.pos + 1) % s.cap
	if s.n < s.cap {
		s.n++
	}
}

// Query returns events matching host since the given time.
func (s *CorrelationStore) Query(host string, since time.Time) []models.CorrelationEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.CorrelationEvent
	start := (s.pos - s.n + s.cap*2) % s.cap
	for i := 0; i < s.n; i++ {
		ev := s.events[(start+i)%s.cap]
		if ev.CreatedAt.Before(since) {
			continue
		}
		if host != "" && ev.Host != host {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// Engine correlates burst events with KPI degradation to produce CorrelationEvents.
type Engine struct {
	mu sync.Mutex

	// Recent KPI snapshots per host: ring buffer of (ts, health_score)
	kpiHistory map[string]*kpiRing

	// Recent bursts pending correlation (cleared after correlationWindow)
	pendingBursts []pendingBurst

	// Output store
	Store *CorrelationStore

	// Pearson tracker per (service, kpi)
	pearson map[string]*pearsonTracker
}

// New creates a correlation engine.
func New() *Engine {
	return &Engine{
		kpiHistory: make(map[string]*kpiRing),
		Store:      NewCorrelationStore(1000),
		pearson:    make(map[string]*pearsonTracker),
	}
}

// ObserveKPI records a new KPI value for a host.
func (e *Engine) ObserveKPI(host, kpi string, value float64, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := host + ":" + kpi
	ring, ok := e.kpiHistory[key]
	if !ok {
		ring = newKPIRing(pearsonWindow)
		e.kpiHistory[key] = ring
	}
	ring.push(value, ts)

	// Try correlating pending bursts against this KPI update
	if kpi == "health_score" {
		e.tryCorrelate(host, value, ts)
	}
}

// ObserveBurst records a burst event and attempts immediate correlation.
func (e *Engine) ObserveBurst(burst models.BurstEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingBursts = append(e.pendingBursts, pendingBurst{burst: burst, addedAt: time.Now()})
	e.evictStalePending()
}

// tryCorrelate checks all pending bursts against the current host KPI drop.
// Must be called with e.mu held.
func (e *Engine) tryCorrelate(host string, currentScore float64, ts time.Time) {
	ring, ok := e.kpiHistory[host+":health_score"]
	if !ok || ring.n < 2 {
		return
	}
	prevScore := ring.prev()
	delta := prevScore - currentScore // positive = degradation
	if delta < kpiDeltaThreshold {
		return
	}

	for i := range e.pendingBursts {
		pb := &e.pendingBursts[i]
		if pb.correlated {
			continue
		}
		diff := ts.Sub(pb.burst.StartTS)
		if diff < -correlationWindow || diff > correlationWindow {
			continue
		}

		confidence := e.computeConfidence(pb.burst.Service, host, delta, diff)
		ev := models.CorrelationEvent{
			ID:         utils.GenerateID(8),
			Host:       host,
			BurstID:    pb.burst.ID,
			KPIName:    "health_score",
			KPIDelta:   delta,
			Confidence: confidence,
			CreatedAt:  ts,
		}
		e.Store.Push(ev)
		pb.correlated = true
	}
}

// computeConfidence scores confidence [0,1] based on time proximity and delta magnitude.
func (e *Engine) computeConfidence(service, host string, delta float64, timeDiff time.Duration) float64 {
	// Time proximity score: decays linearly over correlationWindow
	absDiff := timeDiff
	if absDiff < 0 {
		absDiff = -absDiff
	}
	timeScore := 1.0 - absDiff.Seconds()/correlationWindow.Seconds()

	// Delta magnitude score: larger delta → higher confidence (cap at 1.0)
	deltaScore := math.Min(delta/50.0, 1.0)

	// Pearson correlation if available
	pearsonScore := 0.5
	pk := service + ":" + host
	if pt, ok := e.pearson[pk]; ok && pt.n >= 10 {
		pearsonScore = math.Abs(pt.corr())
	}

	return (timeScore + deltaScore + pearsonScore) / 3.0
}

func (e *Engine) evictStalePending() {
	cutoff := time.Now().Add(-correlationWindow * 2)
	var keep []pendingBurst
	for _, pb := range e.pendingBursts {
		if pb.addedAt.After(cutoff) {
			keep = append(keep, pb)
		}
	}
	e.pendingBursts = keep
}

type pendingBurst struct {
	burst      models.BurstEvent
	addedAt    time.Time
	correlated bool
}

// --- kpiRing: lightweight time-stamped ring buffer ---

type kpiPoint struct {
	value float64
	ts    time.Time
}

type kpiRing struct {
	data []kpiPoint
	cap  int
	pos  int
	n    int
}

func newKPIRing(cap int) *kpiRing {
	return &kpiRing{data: make([]kpiPoint, cap), cap: cap}
}

func (r *kpiRing) push(v float64, ts time.Time) {
	r.data[r.pos] = kpiPoint{value: v, ts: ts}
	r.pos = (r.pos + 1) % r.cap
	if r.n < r.cap {
		r.n++
	}
}

func (r *kpiRing) prev() float64 {
	if r.n < 2 {
		return 0
	}
	idx := (r.pos - 2 + r.cap*2) % r.cap
	return r.data[idx].value
}

// --- pearsonTracker: online Pearson correlation ---

type pearsonTracker struct {
	n                               int
	sumX, sumY, sumXY, sumX2, sumY2 float64
}

func (p *pearsonTracker) update(x, y float64) {
	p.n++
	p.sumX += x
	p.sumY += y
	p.sumXY += x * y
	p.sumX2 += x * x
	p.sumY2 += y * y
}

func (p *pearsonTracker) corr() float64 {
	n := float64(p.n)
	num := n*p.sumXY - p.sumX*p.sumY
	den := math.Sqrt((n*p.sumX2 - p.sumX*p.sumX) * (n*p.sumY2 - p.sumY*p.sumY))
	if den < 1e-12 {
		return 0
	}
	return num / den
}
