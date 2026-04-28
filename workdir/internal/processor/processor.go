package processor

import (
	"runtime"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/utils"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// Processor normalizes and aggregates raw metrics
type Processor struct {
	mu      sync.RWMutex
	buffers map[string]*utils.CircularBuffer // metric name → circular buffer
	maxSize int
}

// NewProcessor creates a processor with given buffer size per metric
func NewProcessor(bufferSize int) *Processor {
	return &Processor{
		buffers: make(map[string]*utils.CircularBuffer),
		maxSize: bufferSize,
	}
}

// Ingest accepts a batch of raw metrics and stores them normalized
func (p *Processor) Ingest(metrics []models.Metric) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, m := range metrics {
		key := m.Host + ":" + m.Name
		if _, ok := p.buffers[key]; !ok {
			p.buffers[key] = utils.NewCircularBuffer(p.maxSize)
		}
		normalized := normalize(m.Name, m.Value)
		p.buffers[key].Push(normalized)
	}
}

// GetNormalized returns the normalized current value for a metric on a host
func (p *Processor) GetNormalized(host, metricName string) (float64, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := host + ":" + metricName
	buf, ok := p.buffers[key]
	if !ok {
		return 0, false
	}
	v, ok := buf.Last()
	return v, ok
}

// GetHistory returns the buffered history for a metric
func (p *Processor) GetHistory(host, metricName string) []float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	key := host + ":" + metricName
	buf, ok := p.buffers[key]
	if !ok {
		return nil
	}
	return buf.Values()
}

// Aggregate computes aggregated statistics for a metric history
type AggregateResult struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// Aggregate returns aggregated stats for a metric
func (p *Processor) Aggregate(host, metricName string) (AggregateResult, bool) {
	history := p.GetHistory(host, metricName)
	if len(history) == 0 {
		return AggregateResult{}, false
	}
	min := history[0]
	max := history[0]
	for _, v := range history {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return AggregateResult{
		Avg: utils.Mean(history),
		Min: min,
		Max: max,
		P95: utils.Percentile(history, 95),
		P99: utils.Percentile(history, 99),
	}, true
}

// Downsample reduces a time series by averaging into buckets
func Downsample(points []models.DataPoint, bucketDuration time.Duration) []models.DataPoint {
	if len(points) == 0 {
		return nil
	}
	result := []models.DataPoint{}
	bucketStart := points[0].Timestamp.Truncate(bucketDuration)
	var bucketVals []float64

	flush := func(start time.Time, vals []float64) {
		if len(vals) == 0 {
			return
		}
		result = append(result, models.DataPoint{
			Timestamp: start,
			Value:     utils.Mean(vals),
		})
	}

	for _, p := range points {
		bucket := p.Timestamp.Truncate(bucketDuration)
		if bucket.Equal(bucketStart) {
			bucketVals = append(bucketVals, p.Value)
		} else {
			flush(bucketStart, bucketVals)
			bucketStart = bucket
			bucketVals = []float64{p.Value}
		}
	}
	flush(bucketStart, bucketVals)
	return result
}

// normalize maps a raw metric value to [0,1] based on metric type
func normalize(name string, value float64) float64 {
	switch name {
	case "cpu_percent", "memory_percent", "disk_percent":
		return utils.NormalizePercent(value)
	case "load_avg_1", "load_avg_5", "load_avg_15":
		// Normalize load avg relative to CPU count: 1.0 = fully loaded
		numCPU := float64(runtime.NumCPU())
		if numCPU < 1 {
			numCPU = 1
		}
		return utils.Clamp(value/numCPU, 0, 1)
	case "net_rx_bps", "net_tx_bps":
		// Normalize to 1Gbps = 1.0
		return utils.Clamp(value/1e9, 0, 1)
	case "memory_used_mb", "memory_total_mb":
		// Keep raw for display, normalized by percent separately
		return utils.Clamp(value/65536.0, 0, 1) // normalize to 64GB
	case "disk_used_gb", "disk_total_gb":
		return utils.Clamp(value/10240.0, 0, 1) // normalize to 10TB
	case "uptime_seconds":
		return utils.Clamp(value/2592000.0, 0, 1) // normalize to 30 days
	case "processes":
		return utils.Clamp(value/1000.0, 0, 1)
	default:
		// Unknown metric: clamp to [0,1]. If value > 1.0, assume it's a percentage
		// and divide by 100. Log a warning to aid debugging of misclassified metrics.
		if value > 1 {
			logger.Default.Debug("processor unknown metric treated as percentage", "name", name, "value", value)
			return utils.NormalizePercent(value)
		}
		return utils.Clamp(value, 0, 1)
	}
}
