package correlator

import (
	"math"
	"sync"
	"time"

	"github.com/benfradjselim/ohe/pkg/models"
	"github.com/benfradjselim/ohe/pkg/utils"
)

// BurstDetector watches per-service log level counts and fires BurstEvents
// when the error/warn rate exceeds mean + 3σ of a 5-minute rolling baseline.
// It runs asynchronously; calls to Observe are non-blocking.
type BurstDetector struct {
	mu      sync.Mutex
	series  map[string]*burstSeries // key: "service:level"
	events  chan models.BurstEvent
	dropped int64
}

// NewBurstDetector creates a detector with an output channel of given buffer size.
func NewBurstDetector(bufSize int) *BurstDetector {
	return &BurstDetector{
		series: make(map[string]*burstSeries),
		events: make(chan models.BurstEvent, bufSize),
	}
}

// Events returns the read-only channel of detected burst events.
func (d *BurstDetector) Events() <-chan models.BurstEvent {
	return d.events
}

// DroppedCount returns the number of burst events dropped due to a full buffer.
func (d *BurstDetector) DroppedCount() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dropped
}

// Observe records a log line for the given service and level ("error"|"warn").
// It is safe for concurrent use. Detection is O(1); no allocation on the hot path.
func (d *BurstDetector) Observe(service, level string, ts time.Time) {
	if level != "error" && level != "warn" {
		return
	}
	key := service + ":" + level

	d.mu.Lock()
	s, ok := d.series[key]
	if !ok {
		s = newBurstSeries()
		d.series[key] = s
	}
	burst := s.record(ts)
	if burst != nil {
		burst.ID = utils.GenerateID(8)
		burst.Service = service
		burst.Level = level
		select {
		case d.events <- *burst:
		default:
			d.dropped++
		}
	}
	d.mu.Unlock()
}

// --- burstSeries: per-service per-level rolling window state ---

const (
	bucketSize   = 10 * time.Second // granularity of rate counting
	baselineSize = 30               // number of buckets for baseline (5 min)
	burstSigma   = 3.0
)

type burstSeries struct {
	buckets  [baselineSize + 1]int64 // circular bucket counts
	bucketTS [baselineSize + 1]int64 // bucket start unix-seconds
	pos      int
	n        int

	// active burst tracking
	inBurst    bool
	burstStart time.Time
	burstCount int64
}

func newBurstSeries() *burstSeries { return &burstSeries{} }

// record increments the current bucket and returns a BurstEvent if one is detected.
func (s *burstSeries) record(ts time.Time) *models.BurstEvent {
	bucketKey := ts.UnixNano() / int64(bucketSize)

	// If bucket has rolled over, advance
	if s.n == 0 || s.bucketTS[s.pos] != bucketKey {
		s.pos = (s.pos + 1) % len(s.buckets)
		s.buckets[s.pos] = 0
		s.bucketTS[s.pos] = bucketKey
		if s.n < len(s.buckets) {
			s.n++
		}
	}
	s.buckets[s.pos]++

	if s.n < 5 {
		return nil // not enough baseline
	}

	// Compute baseline over all buckets except current
	baseline := s.baselineRate()
	current := float64(s.buckets[s.pos])

	isBurst := current > baseline.mean+burstSigma*baseline.std

	if isBurst && !s.inBurst {
		s.inBurst = true
		s.burstStart = ts
		s.burstCount = s.buckets[s.pos]
		return nil // wait for next tick to confirm end
	}
	if s.inBurst {
		s.burstCount += s.buckets[s.pos]
		if !isBurst {
			// burst ended
			s.inBurst = false
			return &models.BurstEvent{
				StartTS:      s.burstStart,
				EndTS:        ts,
				Count:        s.burstCount,
				BaselineRate: baseline.mean,
			}
		}
	}
	return nil
}

type rateStats struct {
	mean float64
	std  float64
}

func (s *burstSeries) baselineRate() rateStats {
	n := s.n - 1 // exclude current bucket
	if n <= 0 {
		return rateStats{}
	}
	vals := make([]float64, n)
	for i := 0; i < n; i++ {
		idx := (s.pos - 1 - i + len(s.buckets)*2) % len(s.buckets)
		vals[i] = float64(s.buckets[idx])
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(n)
	var variance float64
	for _, v := range vals {
		d := v - mean
		variance += d * d
	}
	return rateStats{mean: mean, std: math.Sqrt(variance / float64(n))}
}
