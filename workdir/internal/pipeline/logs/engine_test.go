package logs

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/pipeline/metrics"
)

type mockPipeline struct {
	metrics.MetricPipeline
	ingested []struct {
		host, name string
		value      float64
	}
}

func (m *mockPipeline) Ingest(host, metric string, value float64, ts time.Time) {
	m.ingested = append(m.ingested, struct {
		host, name string
		value      float64
	}{host, metric, value})
}

func TestErrorRate_basicCounting(t *testing.T) {
	mp := &mockPipeline{}
	e := NewEngine(mp, WithBucketSize(15*time.Second))
	
	now := time.Now()
	e.IngestLine("svc1", []byte("ERROR"), now)
	e.IngestLine("svc1", []byte("ERROR"), now)
	e.IngestLine("svc1", []byte("ERROR"), now)
	
	e.Flush()
	
	found := false
	for _, m := range mp.ingested {
		if m.name == "log_error_rate" && m.value == 3.0/15.0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("error rate not found or incorrect: %v", mp.ingested)
	}
}

func TestErrorRate_caseSensitivity(t *testing.T) {
	mp := &mockPipeline{}
	e := NewEngine(mp, WithBucketSize(15*time.Second))
	
	now := time.Now()
	e.IngestLine("svc1", []byte("fatal"), now)
	e.IngestLine("svc1", []byte("CRITICAL"), now)
	e.IngestLine("svc1", []byte("[ERROR]"), now)
	
	e.Flush()
	
	found := false
	for _, m := range mp.ingested {
		if m.name == "log_error_rate" && m.value == 3.0/15.0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("error rate not found or incorrect: %v", mp.ingested)
	}
}

func TestKeywordCounter_match(t *testing.T) {
	mp := &mockPipeline{}
	e := NewEngine(mp, WithKeywordPatterns(map[string]string{"timeout": "timeout"}))
	
	now := time.Now()
	e.IngestLine("svc1", []byte("timeout"), now)
	e.IngestLine("svc1", []byte("no match"), now)
	e.IngestLine("svc1", []byte("timeout here"), now)
	
	e.Flush()
	
	found := false
	for _, m := range mp.ingested {
		if m.name == "log_keyword_timeout" && m.value == 2.0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("keyword count not found or incorrect: %v", mp.ingested)
	}
}
