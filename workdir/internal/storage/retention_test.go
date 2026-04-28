package storage

import (
	"os"
	"testing"
	"time"
)

func openTmp(t *testing.T) (*Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ohe-retention-*")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	return s, func() {
		s.Close()
		os.RemoveAll(dir)
	}
}

func TestCompactRawTo5m(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	host, metric := "testhost", "cpu"
	now := time.Now()
	// Use 8h-old data so the query window is >6h, triggering the 5m rollup tier.
	old := now.Add(-8 * time.Hour)

	// Write 6 raw points 10 min apart, all older than CompactRawAfter (2h)
	for i := 0; i < 6; i++ {
		ts := old.Add(time.Duration(i) * 10 * time.Minute)
		if err := s.SaveMetric(host, metric, float64(i+1)*10, ts); err != nil {
			t.Fatalf("SaveMetric: %v", err)
		}
	}

	// Verify raw points exist
	rawBefore, err := s.GetMetricRange(host, metric, old.Add(-time.Minute), old.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetMetricRange: %v", err)
	}
	if len(rawBefore) != 6 {
		t.Fatalf("expected 6 raw points before compact, got %d", len(rawBefore))
	}

	s.Compact()

	// Raw points should be gone (compacted away)
	rawAfter, _ := s.GetMetricRange(host, metric, old.Add(-time.Minute), old.Add(time.Hour))
	if len(rawAfter) != 0 {
		t.Errorf("expected raw points deleted after compact, got %d", len(rawAfter))
	}

	// 5m rollup points should exist — window is ~8h which selects the 5m tier
	tiered, err := s.GetMetricRangeTiered(host, metric, old.Add(-time.Minute), now)
	if err != nil {
		t.Fatalf("GetMetricRangeTiered: %v", err)
	}
	if len(tiered) == 0 {
		t.Error("expected 5m rollup points after compact, got 0")
	}
}

func TestRetentionStats(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	if err := s.SaveMetric("h", "cpu", 42, now); err != nil {
		t.Fatal(err)
	}

	stats := s.RetentionStats()
	if stats["m:"] != 1 {
		t.Errorf("expected 1 raw metric, got %d", stats["m:"])
	}
}

func TestGetMetricRangeTiered_ShortWindow(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	host, metric := "h", "mem"
	if err := s.SaveMetric(host, metric, 55.5, now.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	pts, err := s.GetMetricRangeTiered(host, metric, now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("GetMetricRangeTiered: %v", err)
	}
	if len(pts) != 1 {
		t.Errorf("expected 1 point, got %d", len(pts))
	}
}
