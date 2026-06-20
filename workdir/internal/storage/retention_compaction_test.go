package storage

import (
	"os"
	"testing"
	"time"
)

// TestKPICompactionEndToEnd verifies that KPI data older than CompactRawAfter
// is correctly rolled up into 5-minute buckets and accessible via GetKPIRange.
func TestKPICompactionEndToEnd(t *testing.T) {
	dir, err := os.MkdirTemp("", "ruptura-compact-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Insert 20 KPI points 3 hours in the past (older than CompactRawAfter=2h)
	pastBase := time.Now().Add(-3 * time.Hour)
	for i := 0; i < 20; i++ {
		ts := pastBase.Add(time.Duration(i) * 15 * time.Second)
		if err := s.PutKPI("health_score", "test-host", ts, float64(i)/20.0); err != nil {
			t.Fatalf("PutKPI: %v", err)
		}
	}

	// Run compaction
	s.Compact()

	// Verify rollups exist via RetentionStats
	stats := s.RetentionStats()
	if stats["kr5:"] == 0 {
		t.Error("expected kr5: rollups after compaction, got 0")
	}

	// Verify data is accessible via GetKPIRange over the rolled-up window
	from := pastBase.Add(-time.Minute)
	to := pastBase.Add(10 * time.Minute)
	vals, err := s.GetKPIRange("test-host", "health_score", from, to)
	if err != nil {
		t.Fatalf("GetKPIRange: %v", err)
	}
	if len(vals) == 0 {
		t.Error("expected KPI rollup data, got empty slice — check compaction prefix logic")
	}
}

// TestCompactionAtomicity verifies that if we compact the same data twice,
// values are not double-averaged (idempotent rollup).
func TestCompactionAtomicity(t *testing.T) {
	dir, err := os.MkdirTemp("", "ruptura-atomic-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	pastBase := time.Now().Add(-3 * time.Hour)
	// Insert 4 points all in the same 5-min bucket, value=1.0
	for i := 0; i < 4; i++ {
		ts := pastBase.Add(time.Duration(i) * 30 * time.Second)
		if err := s.PutKPI("stress", "idempotent-host", ts, 1.0); err != nil {
			t.Fatalf("PutKPI: %v", err)
		}
	}

	// First compaction: should produce rollup avg=1.0
	s.Compact()

	from := pastBase.Add(-time.Minute)
	to := pastBase.Add(10 * time.Minute)
	vals1, _ := s.GetKPIRange("idempotent-host", "stress", from, to)

	// Second compaction: sources are already deleted, rollup should stay at 1.0
	s.Compact()

	vals2, _ := s.GetKPIRange("idempotent-host", "stress", from, to)

	if len(vals1) != len(vals2) {
		t.Errorf("double compaction changed result count: %d -> %d", len(vals1), len(vals2))
	}
	if len(vals1) > 0 && len(vals2) > 0 {
		if absF(vals1[0].Value-vals2[0].Value) > 0.001 {
			t.Errorf("double compaction changed value: %.4f -> %.4f (double-averaging bug)",
				vals1[0].Value, vals2[0].Value)
		}
	}
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
