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

// TestRetentionStats_InfraPrefixes covers all six FBL-A3-1 prefixes
// (is:/is5:/is1h:/gh:/gni:/prop:) — the AC's literal requirement.
func TestRetentionStats_InfraPrefixes(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	if err := s.PutInfraSignal("grp.network", "namespace", "prod", "Route", "console", "nodeStress", now, 0.4); err != nil {
		t.Fatalf("PutInfraSignal: %v", err)
	}
	if err := s.PutGroupHealth("grp.network", "prod", now, 0.9); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}
	if err := s.PutGroupNoise("grp.network", now, 0.1); err != nil {
		t.Fatalf("PutGroupNoise: %v", err)
	}
	if err := s.PutPropagationSnapshot(now, []byte(`{"namespace":"prod"}`)); err != nil {
		t.Fatalf("PutPropagationSnapshot: %v", err)
	}

	stats := s.RetentionStats()
	tests := []struct {
		prefix string
		want   int64
	}{
		{"is:", 1},
		{"gh:", 1},
		{"gni:", 1},
		{"prop:", 1},
		{"is5:", 0},
		{"is1h:", 0},
	}
	for _, tt := range tests {
		if stats[tt.prefix] != tt.want {
			t.Errorf("stats[%q] = %d, want %d", tt.prefix, stats[tt.prefix], tt.want)
		}
	}
}

func TestPutListInfraSignal_RoundTrip(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	if err := s.PutInfraSignal("grp.storage", "namespace", "prod", "PersistentVolumeClaim", "data", "pvcStall", now, 0.6); err != nil {
		t.Fatalf("PutInfraSignal: %v", err)
	}

	pts, err := s.ListInfraSignal("grp.storage", "namespace", "prod", "PersistentVolumeClaim", "data", "pvcStall", now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListInfraSignal: %v", err)
	}
	if len(pts) != 1 || pts[0].Value != 0.6 {
		t.Errorf("ListInfraSignal = %+v, want one point with value 0.6", pts)
	}
}

func TestPutListGroupHealth_RoundTrip(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	if err := s.PutGroupHealth("grp.admission", "staging", now, 0.75); err != nil {
		t.Fatalf("PutGroupHealth: %v", err)
	}
	pts, err := s.ListGroupHealth("grp.admission", "staging", now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListGroupHealth: %v", err)
	}
	if len(pts) != 1 || pts[0].Value != 0.75 {
		t.Errorf("ListGroupHealth = %+v, want one point with value 0.75", pts)
	}
}

func TestPutListGroupNoise_RoundTrip(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	if err := s.PutGroupNoise("grp.tenancy", now, 0.33); err != nil {
		t.Fatalf("PutGroupNoise: %v", err)
	}
	pts, err := s.ListGroupNoise("grp.tenancy", now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListGroupNoise: %v", err)
	}
	if len(pts) != 1 || pts[0].Value != 0.33 {
		t.Errorf("ListGroupNoise = %+v, want one point with value 0.33", pts)
	}
}

func TestPutListPropagationSnapshot_RoundTripsWithoutDoubleEncoding(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	payload := []byte(`{"namespace":"prod","prop_pressure":{"grp.workload":0.42}}`)
	if err := s.PutPropagationSnapshot(now, payload); err != nil {
		t.Fatalf("PutPropagationSnapshot: %v", err)
	}

	snaps, err := s.ListPropagationSnapshots(now.Add(-time.Minute), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListPropagationSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snaps))
	}
	// Must round-trip as the literal JSON bytes, not a base64-wrapped
	// re-encoding of them (setRaw exists specifically to prevent that).
	if string(snaps[0]) != string(payload) {
		t.Errorf("ListPropagationSnapshots[0] = %s, want %s (byte-identical, no double-encoding)", snaps[0], payload)
	}
}

// TestCompactInfraSignalsRawTo5m mirrors TestCompactRawTo5m for the is:
// prefix, proving compactTier's generic series-identity logic (built for the
// 2-segment m:/kpi: schema) also works unmodified for is:'s 6-segment key.
func TestCompactInfraSignalsRawTo5m(t *testing.T) {
	s, teardown := openTmp(t)
	defer teardown()

	now := time.Now()
	old := now.Add(-8 * time.Hour)

	for i := 0; i < 6; i++ {
		ts := old.Add(time.Duration(i) * 10 * time.Minute)
		if err := s.PutInfraSignal("grp.network", "namespace", "prod", "Route", "console", "nodeStress", ts, float64(i)*0.1); err != nil {
			t.Fatalf("PutInfraSignal: %v", err)
		}
	}

	rawBefore, err := s.ListInfraSignal("grp.network", "namespace", "prod", "Route", "console", "nodeStress", old.Add(-time.Minute), old.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListInfraSignal: %v", err)
	}
	if len(rawBefore) != 6 {
		t.Fatalf("expected 6 raw points before compact, got %d", len(rawBefore))
	}

	s.Compact()

	rawAfter, _ := s.ListInfraSignal("grp.network", "namespace", "prod", "Route", "console", "nodeStress", old.Add(-time.Minute), old.Add(time.Hour))
	if len(rawAfter) != 0 {
		t.Errorf("expected raw is: points deleted after compact, got %d", len(rawAfter))
	}

	stats := s.RetentionStats()
	if stats["is5:"] == 0 {
		t.Error("expected is5: rollup points after compact, got 0")
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
