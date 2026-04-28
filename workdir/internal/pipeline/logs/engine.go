package logs

import (
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/pipeline/metrics"
)

type LogPipeline interface {
	IngestLine(service string, line []byte, ts time.Time)
	Flush()
}

type Engine struct {
	pipeline    metrics.MetricPipeline
	bucketSize  time.Duration
	patterns    map[string]*regexp.Regexp
	novelty     bool
	
	mu          sync.Mutex
	// service -> bucket data
	buckets     map[string]*bucket
}

type bucket struct {
	errorCount    int64
	totalVolume   int64
	keywordCounts map[string]int64
	start         time.Time
}

// Store baseline per service
var (
	baselines = make(map[string]float64)
	bMu       sync.Mutex
)

func (e *Engine) emit(service string, b *bucket, ts time.Time) {
	// ErrorRate
	rate := float64(b.errorCount) / e.bucketSize.Seconds()
	e.pipeline.Ingest(service, "log_error_rate", rate, ts)
	
	// KeywordCounter
	for name, count := range b.keywordCounts {
		e.pipeline.Ingest(service, "log_keyword_"+name, float64(count), ts)
	}
	
	// BurstDetector
	bMu.Lock()
	baseline := baselines[service]
	if baseline == 0 {
		baseline = float64(b.totalVolume)
	}
	baselines[service] = 0.9*baseline + 0.1*float64(b.totalVolume)
	bMu.Unlock()
	
	burstIndex := float64(b.totalVolume) / max(baseline, 1.0)
	e.pipeline.Ingest(service, "log_burst_index", burstIndex, ts)
	
	if e.novelty {
		e.pipeline.Ingest(service, "log_novelty", 0.0, ts)
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

type Option func(*Engine)

func WithBucketSize(d time.Duration) Option {
	return func(e *Engine) { e.bucketSize = d }
}

func WithKeywordPatterns(patterns map[string]string) Option {
	return func(e *Engine) {
		e.patterns = make(map[string]*regexp.Regexp)
		for name, p := range patterns {
			e.patterns[name] = regexp.MustCompile(p)
		}
	}
}

func WithNovelty(enabled bool) Option {
	return func(e *Engine) { e.novelty = enabled }
}

func NewEngine(pipeline metrics.MetricPipeline, opts ...Option) *Engine {
	e := &Engine{
		pipeline:   pipeline,
		bucketSize: 15 * time.Second,
		buckets:    make(map[string]*bucket),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *Engine) IngestLine(service string, line []byte, ts time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	b, ok := e.buckets[service]
	if !ok || ts.Sub(b.start) >= e.bucketSize {
		if ok {
			e.emit(service, b, ts)
		}
		b = &bucket{
			start:         ts.Truncate(e.bucketSize),
			keywordCounts: make(map[string]int64),
		}
		e.buckets[service] = b
	}
	
	lineStr := string(line)
	// ErrorRate
	if isError(lineStr) {
		b.errorCount++
	}
	
	// KeywordCounter
	for name, re := range e.patterns {
		if re.MatchString(lineStr) {
			b.keywordCounts[name]++
		}
	}
	
	// BurstDetector
	b.totalVolume++
}

func isError(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "error") || 
	       strings.Contains(lower, "fatal") || 
	       strings.Contains(lower, "critical")
}

func (e *Engine) Flush() {
	e.mu.Lock()
	defer e.mu.Unlock()
	for service, b := range e.buckets {
		e.emit(service, b, time.Now())
		delete(e.buckets, service)
	}
}

func (e *Engine) ErrorRate(service string) float64 { return 0.0 }
func (e *Engine) BurstIndex(service string) float64 { return 0.0 }
