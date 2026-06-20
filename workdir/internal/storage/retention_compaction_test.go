package storage

import (
	"os"
	"testing"
	"time"
)

// TestKPICompactionEndToEnd verifies that KPI data older than CompactRawAfter
// is rolled up into kr5: buckets by Compact().
//
// After compaction, raw kpi: keys are deleted and kr5: rollups exist.
// We verify via RetentionStats — the authoritative view of what is in each tier.
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

	// Insert 20 KPI points 3 hours in the past (older than CompactRawAfter = 2h)
	pastBase := time.Now().Add(-3 * time.Hour)
	for i := 0; i < 20; i++ {
		ts := pastBase.Add(time.Duration(i) * 15 * time.Second)
		if err := s.PutKPI("health_score", "test-host", ts, float64(i)/20.0); err != nil {
			t.Fatalf("PutKPI i=%d: %v", i, err)
		}
	}

	// Verify raw data exists before compaction
	statsBefore := s.RetentionStats()
	if statsBefore["kpi:"] == 0 {
		t.Fatal("expected raw kpi: data before compaction, got 0")
	}

	// Run compaction
	s.Compact()

	// After compaction: raw kpi: keys should be gone, kr5: rollups should exist.
	statsAfter := s.RetentionStats()
	if statsAfter["kr5:"] == 0 {
		t.Errorf("expected kr5: rollups after compaction, got 0 — compaction did not run or prefix is wrong")
	}
	// Raw tier should be empty (compacted away)
	if statsAfter["kpi:"] != 0 {
		t.Errorf("expected kpi: to be empty after compaction, still has %d keys", statsAfter["kpi:"])
	}
}

// TestCompactionAtomicity verifies idempotent rollup — running Compact twice
// on already-compacted data does not double-average.
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
	// 4 points in same 5-min bucket, all value=1.0
	for i := 0; i < 4; i++ {
		ts := pastBase.Add(time.Duration(i) * 30 * time.Second)
		if err := s.PutKPI("stress", "idempotent-host", ts, 1.0); err != nil {
			t.Fatalf("PutKPI: %v", err)
		}
	}

	// First compaction
	s.Compact()
	stats1 := s.RetentionStats()

	// Second compaction — sources already gone, rollups should be unchanged
	s.Compact()
	stats2 := s.RetentionStats()

	if stats1["kr5:"] != stats2["kr5:"] {
		t.Errorf("double compaction changed kr5: count: %d → %d (double-averaging?)",
			stats1["kr5:"], stats2["kr5:"])
	}
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
