package receiver

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/models"
	"github.com/benfradjselim/ruptura/pkg/logger"
)

// DogStatsDReceiver listens on UDP 8125 for DogStatsD / StatsD protocol metrics.
// Supports: counters (c), gauges (g), timers (ms), histograms (h), sets (s).
// DogStatsD extensions: tags (#tag:value), sample rate (@0.5).
// This makes OHE a drop-in replacement for the Datadog Agent StatsD endpoint.
type DogStatsDReceiver struct {
	addr    string
	store   *storage.Store
	metrics MetricSink
	host    string
}

// NewDogStatsDReceiver creates a receiver that listens on the given UDP address (e.g. ":8125")
func NewDogStatsDReceiver(addr string, store *storage.Store, metrics MetricSink, host string) *DogStatsDReceiver {
	return &DogStatsDReceiver{addr: addr, store: store, metrics: metrics, host: host}
}

// Run starts the UDP listener and blocks until ctx is cancelled
func (r *DogStatsDReceiver) Run(ctx context.Context) error {
	conn, err := net.ListenPacket("udp", r.addr)
	if err != nil {
		return fmt.Errorf("dogstatsd listen %s: %w", r.addr, err)
	}

	// Use sync.Once to guarantee exactly one Close regardless of who triggers it
	var closeOnce sync.Once
	closeConn := func() { closeOnce.Do(func() { _ = conn.Close() }) }
	defer closeConn()

	logger.Default.Info("dogstatsd listening", "addr", r.addr)

	buf := make([]byte, 65536)

	// Background goroutine unblocks ReadFrom when ctx is cancelled
	go func() {
		<-ctx.Done()
		closeConn()
	}()

	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				logger.Default.Error("dogstatsd read error", "err", err)
				continue
			}
		}
		// Multiple metrics can be batched separated by newlines
		for _, line := range strings.Split(strings.TrimSpace(string(buf[:n])), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if sm, err := parseStatsDLine(line); err == nil {
				r.emit(sm)
			} else {
				logger.Default.Warn("dogstatsd parse error", "line", line, "err", err)
			}
		}
	}
}

func (r *DogStatsDReceiver) emit(sm models.StatsDMetric) {
	// Convert DogStatsD metric to OHE metric
	// Apply sample rate correction for counters
	value := sm.Value
	if sm.Type == "c" && sm.SampleRate > 0 && sm.SampleRate < 1 {
		value = sm.Value / sm.SampleRate
	}

	metric := models.Metric{
		Name:      sanitizeName(sm.Name),
		Value:     value,
		Timestamp: sm.Timestamp,
		Host:      r.host,
		Labels:    sm.Tags,
	}

	if r.metrics != nil {
		r.metrics.IngestMetric(metric)
	}
	if err := r.store.SaveMetric(r.host, metric.Name, metric.Value, metric.Timestamp); err != nil {
		logger.Default.Error("dogstatsd save error", "err", err)
	}
}

// parseStatsDLine parses a single DogStatsD line:
// <metric_name>:<value>|<type>[|@<sample_rate>][|#<tag1>:<val1>,<tag2>:<val2>]
func parseStatsDLine(line string) (models.StatsDMetric, error) {
	sm := models.StatsDMetric{Timestamp: time.Now(), SampleRate: 1.0}

	// Split name:rest
	colonIdx := strings.Index(line, ":")
	if colonIdx < 0 {
		return sm, fmt.Errorf("missing colon")
	}
	sm.Name = strings.TrimSpace(line[:colonIdx])
	rest := line[colonIdx+1:]

	// Split by pipe
	parts := strings.Split(rest, "|")
	if len(parts) < 2 {
		return sm, fmt.Errorf("missing type")
	}

	val, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return sm, fmt.Errorf("bad value: %w", err)
	}
	sm.Value = val
	sm.Type = strings.TrimSpace(parts[1])

	for _, part := range parts[2:] {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "@"):
			sr, err := strconv.ParseFloat(part[1:], 64)
			if err == nil && sr > 0 {
				sm.SampleRate = sr
			}
		case strings.HasPrefix(part, "#"):
			tags := strings.Split(part[1:], ",")
			sm.Tags = make(map[string]string, len(tags))
			for _, tag := range tags {
				kv := strings.SplitN(tag, ":", 2)
				if len(kv) == 2 {
					sm.Tags[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
				} else if kv[0] != "" {
					sm.Tags[kv[0]] = "true"
				}
			}
		}
	}

	return sm, nil
}
