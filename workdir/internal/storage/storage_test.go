package storage

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestStorage(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := Open(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Metric samples
	t.Run("Metrics", func(t *testing.T) {
		host, metric := "h1", "cpu"
		ts := time.Now().Truncate(time.Second)
		if err := store.PutMetric(host, metric, ts, 1.23); err != nil {
			t.Fatal(err)
		}
		val, err := store.GetMetric(host, metric, ts)
		if err != nil || val != 1.23 {
			t.Errorf("GetMetric failed: %v, %v", val, err)
		}
		samples, err := store.ListMetrics(host, metric, ts.Add(-time.Second), ts.Add(time.Second))
		if err != nil || len(samples) != 1 || samples[0].Value != 1.23 {
			t.Errorf("ListMetrics failed: %v, %v", samples, err)
		}
	})

	// Rupture events
	t.Run("Rupture", func(t *testing.T) {
		id, payload := "r1", []byte(`{"status":"rupture"}`)
		if err := store.PutRupture(id, payload); err != nil {
			t.Fatal(err)
		}
		got, err := store.GetRupture(id)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("GetRupture failed: %s, %v", got, err)
		}
	})

	// KPI values
	t.Run("KPI", func(t *testing.T) {
		name, host := "stress", "h1"
		ts := time.Now().Truncate(time.Second)
		if err := store.PutKPI(name, host, ts, 0.5); err != nil {
			t.Fatal(err)
		}
		samples, err := store.ListKPI(name, host, ts.Add(-time.Second), ts.Add(time.Second))
		if err != nil || len(samples) != 1 || samples[0].Value != 0.5 {
			t.Errorf("ListKPI failed: %v, %v", samples, err)
		}
	})

	// Context entries
	t.Run("Context", func(t *testing.T) {
		id, payload := "c1", []byte(`{"type":"load_test"}`)
		if err := store.PutContext(id, payload); err != nil {
			t.Fatal(err)
		}
		got, err := store.GetContext(id)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("GetContext failed: %s, %v", got, err)
		}
		list, err := store.ListContexts()
		if err != nil || len(list) != 1 {
			t.Errorf("ListContexts failed: %v, %v", list, err)
		}
		if err := store.DeleteContext(id); err != nil {
			t.Fatal(err)
		}
		list, _ = store.ListContexts()
		if len(list) != 0 {
			t.Error("ListContexts not empty after delete")
		}
	})

	// Suppressions
	t.Run("Suppression", func(t *testing.T) {
		id, payload := "s1", []byte(`{"window":"60s"}`)
		if err := store.PutSuppression(id, payload); err != nil {
			t.Fatal(err)
		}
		list, err := store.ListSuppressions()
		if err != nil || len(list) != 1 {
			t.Errorf("ListSuppressions failed: %v, %v", list, err)
		}
		if err := store.DeleteSuppression(id); err != nil {
			t.Fatal(err)
		}
		list, _ = store.ListSuppressions()
		if len(list) != 0 {
			t.Error("ListSuppressions not empty after delete")
		}
	})

	t.Run("Logs", func(t *testing.T) {
		service, ts := "svc1", time.Now()
		entry := map[string]string{"msg": "hello"}
		if err := store.SaveLog(service, entry, ts); err != nil {
			t.Fatal(err)
		}
		logs, err := store.QueryLogs(service, ts.Add(-time.Second), ts.Add(time.Second), 1)
		if err != nil || len(logs) != 1 {
			t.Errorf("QueryLogs failed: %v, %v", logs, err)
		}
	})

	t.Run("Spans", func(t *testing.T) {
		span := map[string]string{"name": "op1"}
		tid, sid := "t1", "s1"
		if err := store.SaveSpan(span, tid, sid); err != nil {
			t.Fatal(err)
		}
		spans, err := store.QuerySpansByTrace(tid)
		if err != nil || len(spans) != 1 {
			t.Errorf("QuerySpansByTrace failed: %v, %v", spans, err)
		}
	})
}

// TestLoadSnapshots_SanitizesCorruptValues verifies that snapshots with
// out-of-range FusedRuptureIndex values (written before the v7.0.20 fix) are
// zeroed on load rather than served to the UI.
func TestLoadSnapshots_SanitizesCorruptValues(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// NaN and Inf cannot round-trip through JSON, so only finite values are tested here.
	// The NaN/Inf path is covered by TestSanitizeLoadedSnapshot_Unit.
	cases := []struct {
		name  string
		input float64
		want  float64
	}{
		{"large corrupt value", 163759.981, 0},
		{"negative value", -1.0, 0},
		{"at cap boundary", 10.0, 10.0},
		{"just above cap", 10.001, 0},
		{"valid mid-range", 3.5, 3.5},
		{"zero", 0, 0},
	}

	for _, tc := range cases {
		snap := models.KPISnapshot{
			Host:              tc.name,
			FusedRuptureIndex: tc.input,
		}
		store.StoreSnapshot(snap)
		// Flush to BadgerDB then reload.
		if err := store.FlushSnapshots(); err != nil {
			t.Fatalf("%s: flush: %v", tc.name, err)
		}
		// Wipe in-memory map to force a reload.
		store.snapshotsMu.Lock()
		store.snapshots = make(map[string]models.KPISnapshot)
		store.snapshotsMu.Unlock()

		if _, err := store.LoadSnapshots(); err != nil {
			t.Fatalf("%s: load: %v", tc.name, err)
		}
		got, ok := store.LatestSnapshot(tc.name)
		if !ok {
			t.Errorf("%s: snapshot not found after reload", tc.name)
			continue
		}
		if math.IsNaN(tc.want) {
			if !math.IsNaN(got.FusedRuptureIndex) {
				t.Errorf("%s: got %v, want NaN", tc.name, got.FusedRuptureIndex)
			}
		} else if got.FusedRuptureIndex != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got.FusedRuptureIndex, tc.want)
		}
	}
}

// TestSanitizeLoadedSnapshot_Unit tests the helper in isolation.
func TestSanitizeLoadedSnapshot_Unit(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{163759, 0}, {-0.1, 0}, {math.NaN(), 0}, {math.Inf(1), 0},
		{10.0, 10.0}, {3.14, 3.14}, {0, 0},
	}
	for _, tc := range cases {
		snap := &models.KPISnapshot{FusedRuptureIndex: tc.in}
		sanitizeLoadedSnapshot(snap)
		if math.IsNaN(tc.want) {
			continue // input NaN → output 0, checked implicitly
		}
		if snap.FusedRuptureIndex != tc.want {
			t.Errorf("in=%v: got %v, want %v", tc.in, snap.FusedRuptureIndex, tc.want)
		}
	}
}
