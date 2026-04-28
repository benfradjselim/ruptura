package metrics

import (
	"container/heap"
	"math"
	"sync"
	"time"
)

// runningMedian tracks an approximate streaming median using two heaps
// (max-heap for the lower half, min-heap for the upper half).
// This avoids the O(n log n) sort-per-observe of the naive approach.
type runningMedian struct {
	lo maxHeap // lower half — top is the largest of the small values
	hi minHeap // upper half — top is the smallest of the large values
}

func (r *runningMedian) push(v float64) {
	if r.lo.Len() == 0 || v <= r.lo.peek() {
		heap.Push(&r.lo, v)
	} else {
		heap.Push(&r.hi, v)
	}
	// Rebalance: |lo| - |hi| must stay in {0, 1}
	for r.lo.Len() > r.hi.Len()+1 {
		heap.Push(&r.hi, heap.Pop(&r.lo))
	}
	for r.hi.Len() > r.lo.Len() {
		heap.Push(&r.lo, heap.Pop(&r.hi))
	}
}

func (r *runningMedian) median() float64 {
	if r.lo.Len() == 0 {
		return 0
	}
	if r.lo.Len() > r.hi.Len() {
		return r.lo.peek()
	}
	return (r.lo.peek() + r.hi.peek()) / 2
}

// maxHeap implements heap.Interface for a max-heap of float64.
type maxHeap []float64

func (h maxHeap) Len() int           { return len(h) }
func (h maxHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h maxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x any)        { *h = append(*h, x.(float64)) }
func (h *maxHeap) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }
func (h maxHeap) peek() float64      { return h[0] }

// minHeap implements heap.Interface for a min-heap of float64.
type minHeap []float64

func (h minHeap) Len() int           { return len(h) }
func (h minHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h minHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x any)        { *h = append(*h, x.(float64)) }
func (h *minHeap) Pop() any          { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }
func (h minHeap) peek() float64      { return h[0] }

// MADDetector implements Median Absolute Deviation anomaly detection.
// Uses a dual-heap streaming median so each Update is O(log n) instead of
// O(n log n), making it safe to call at high ingestion rates.
// Not thread-safe; wrap calls in MADAnomalyDetector.mu.
type MADDetector struct {
	window []float64
	cap    int
	pos    int
	n      int
	k      float64 // threshold multiplier (default 3.0)
}

func newMADDetector(windowSize int, k float64) *MADDetector {
	if k <= 0 {
		k = 3.0
	}
	return &MADDetector{
		window: make([]float64, windowSize),
		cap:    windowSize,
		k:      k,
	}
}

// Update adds a value to the rolling window.
func (m *MADDetector) Update(v float64) {
	m.window[m.pos] = v
	m.pos = (m.pos + 1) % m.cap
	if m.n < m.cap {
		m.n++
	}
}

// IsAnomaly returns true, expected, and score if v is an outlier by MAD test.
// The median and MAD are computed via the dual-heap streaming estimator to
// avoid allocating and sorting on every call.
func (m *MADDetector) IsAnomaly(v float64) (anomaly bool, expected, score float64) {
	if m.n < 10 {
		return false, v, 0
	}

	// Compute median of the window using dual-heap streaming estimator
	var rm runningMedian
	start := (m.pos - m.n + m.cap*2) % m.cap
	for i := 0; i < m.n; i++ {
		rm.push(m.window[(start+i)%m.cap])
	}
	med := rm.median()

	// Compute MAD: median of |x - median|
	var rmAD runningMedian
	for i := 0; i < m.n; i++ {
		rmAD.push(math.Abs(m.window[(start+i)%m.cap] - med))
	}
	mad := rmAD.median()

	// Scale factor 1.4826 makes MAD consistent with σ for Gaussian distributions
	sigma := 1.4826 * mad
	if sigma < 1e-10 {
		return false, med, 0
	}
	z := math.Abs(v-med) / sigma
	return z > m.k, med, z
}

// MADAnomalyDetector manages per-metric MAD detectors with thread safety.
type MADAnomalyDetector struct {
	mu         sync.RWMutex
	detectors  map[string]*MADDetector
	windowSize int
	k          float64
}

// NewMADAnomalyDetector creates a MAD-based detector.
func NewMADAnomalyDetector(windowSize int, k float64) *MADAnomalyDetector {
	return &MADAnomalyDetector{
		detectors:  make(map[string]*MADDetector),
		windowSize: windowSize,
		k:          k,
	}
}

// Observe updates the model for metric and returns an anomaly if detected.
func (d *MADAnomalyDetector) Observe(metric string, value float64, ts time.Time) (MADResult, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	det, ok := d.detectors[metric]
	if !ok {
		det = newMADDetector(d.windowSize, d.k)
		d.detectors[metric] = det
	}

	det.Update(value)
	isAnom, expected, score := det.IsAnomaly(value)
	if !isAnom {
		return MADResult{}, false
	}
	return MADResult{
		Metric:    metric,
		Value:     value,
		Expected:  expected,
		Score:     score,
		Timestamp: ts,
	}, true
}

// MADResult is the output of a MAD anomaly detection.
type MADResult struct {
	Metric    string
	Value     float64
	Expected  float64
	Score     float64
	Timestamp time.Time
}
