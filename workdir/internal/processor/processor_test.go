package processor

import (
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/pkg/models"
)

func TestProcessorIngestAndGetNormalized(t *testing.T) {
	p := NewProcessor(100)

	metrics := []models.Metric{
		{Name: "cpu_percent", Value: 80, Host: "host1", Timestamp: time.Now()},
		{Name: "memory_percent", Value: 60, Host: "host1", Timestamp: time.Now()},
	}
	p.Ingest(metrics)

	val, ok := p.GetNormalized("host1", "cpu_percent")
	if !ok {
		t.Fatal("cpu_percent not found after ingest")
	}
	// 80% CPU → normalized to 0.80
	if val < 0.79 || val > 0.81 {
		t.Errorf("normalized cpu_percent = %v; want ~0.80", val)
	}
}

func TestProcessorAggregate(t *testing.T) {
	p := NewProcessor(100)
	for i := 1; i <= 10; i++ {
		p.Ingest([]models.Metric{
			{Name: "cpu_percent", Value: float64(i * 10), Host: "host2", Timestamp: time.Now()},
		})
	}

	agg, ok := p.Aggregate("host2", "cpu_percent")
	if !ok {
		t.Fatal("aggregate returned not-ok")
	}
	if agg.Avg < 0 || agg.Avg > 1 {
		t.Errorf("aggregate avg out of [0,1]: %v", agg.Avg)
	}
	if agg.Min > agg.Avg {
		t.Errorf("min > avg: %v > %v", agg.Min, agg.Avg)
	}
	if agg.Max < agg.Avg {
		t.Errorf("max < avg: %v < %v", agg.Max, agg.Avg)
	}
}

func TestDownsample(t *testing.T) {
	now := time.Now().Truncate(time.Minute)
	points := []models.DataPoint{
		{Timestamp: now, Value: 1},
		{Timestamp: now.Add(10 * time.Second), Value: 2},
		{Timestamp: now.Add(20 * time.Second), Value: 3},
		{Timestamp: now.Add(60 * time.Second), Value: 4},
		{Timestamp: now.Add(70 * time.Second), Value: 5},
	}

	result := Downsample(points, time.Minute)
	if len(result) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(result))
	}
	// First bucket avg of 1,2,3 = 2.0
	if result[0].Value < 1.9 || result[0].Value > 2.1 {
		t.Errorf("first bucket avg = %v; want ~2.0", result[0].Value)
	}
}

func TestNormalizeCPU(t *testing.T) {
	// cpu_percent 100% → 1.0
	if got := normalize("cpu_percent", 100); got != 1.0 {
		t.Errorf("normalize cpu 100 = %v; want 1.0", got)
	}
	// cpu_percent 50% → 0.5
	if got := normalize("cpu_percent", 50); got != 0.5 {
		t.Errorf("normalize cpu 50 = %v; want 0.5", got)
	}
}

func TestNormalizeAllMetricTypes(t *testing.T) {
	cases := []struct {
		name  string
		value float64
		lo    float64
		hi    float64
	}{
		// percent-based
		{"memory_percent", 50, 0.49, 0.51},
		{"disk_percent", 100, 0.99, 1.01},
		// load avg (relative to NumCPU; result ∈ [0,1])
		{"load_avg_1", 0, 0, 0.001},
		{"load_avg_5", 1e9, 0.99, 1.01},  // clamp to 1
		{"load_avg_15", 1e9, 0.99, 1.01}, // clamp to 1
		// network
		{"net_rx_bps", 1e9, 0.99, 1.01},   // 1 Gbps → 1.0
		{"net_tx_bps", 5e8, 0.49, 0.51},   // 500 Mbps → 0.5
		{"net_rx_bps", 0, 0, 0.001},
		// memory raw
		{"memory_used_mb", 65536, 0.99, 1.01}, // 64 GB → 1.0
		{"memory_total_mb", 0, 0, 0.001},
		// disk raw
		{"disk_used_gb", 10240, 0.99, 1.01}, // 10 TB → 1.0
		{"disk_total_gb", 0, 0, 0.001},
		// uptime
		{"uptime_seconds", 2592000, 0.99, 1.01}, // 30 days → 1.0
		{"uptime_seconds", 0, 0, 0.001},
		// processes
		{"processes", 1000, 0.99, 1.01},
		{"processes", 500, 0.49, 0.51},
		// default: value ≤ 1 → pass through
		{"unknown_metric", 0.4, 0.39, 0.41},
		// default: value > 1 → treated as percentage
		{"unknown_big", 50, 0.49, 0.51},
	}

	for _, tc := range cases {
		got := normalize(tc.name, tc.value)
		if got < tc.lo || got > tc.hi {
			t.Errorf("normalize(%q, %v) = %v; want [%v, %v]", tc.name, tc.value, got, tc.lo, tc.hi)
		}
	}
}

func TestNormalizeClampsBounds(t *testing.T) {
	// All results must stay within [0, 1]
	inputs := []struct {
		name  string
		value float64
	}{
		{"cpu_percent", 200},
		{"cpu_percent", -10},
		{"net_rx_bps", 1e12},
		{"processes", 99999},
		{"uptime_seconds", 1e9},
		{"unknown_metric", 2.0},
	}
	for _, tc := range inputs {
		got := normalize(tc.name, tc.value)
		if got < 0 || got > 1 {
			t.Errorf("normalize(%q, %v) = %v; want in [0,1]", tc.name, tc.value, got)
		}
	}
}

func TestProcessorHistoryIsolation(t *testing.T) {
	p := NewProcessor(10)
	p.Ingest([]models.Metric{{Name: "cpu_percent", Value: 50, Host: "hostA"}})
	p.Ingest([]models.Metric{{Name: "cpu_percent", Value: 70, Host: "hostB"}})

	vA, _ := p.GetNormalized("hostA", "cpu_percent")
	vB, _ := p.GetNormalized("hostB", "cpu_percent")
	if vA >= vB {
		t.Errorf("hostA cpu (%v) should be < hostB cpu (%v)", vA, vB)
	}
}
