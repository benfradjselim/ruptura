package sim

import (
	"context"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/analyzer"
	"github.com/benfradjselim/ruptura/internal/storage"
	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestSeed_PopulatesKeysAndClearsCalibration(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	a := analyzer.NewAnalyzer()

	cfg := SeedConfig{
		Namespaces:   []string{"prod", "staging"},
		PerNamespace: 2,
		Interval:     time.Minute,
		History:      100 * time.Minute, // 100 ticks, clears the 96-observation calibration bar
		RampDuration: time.Minute,
	}
	wantWorkloads := len(cfg.Namespaces) * cfg.PerNamespace
	wantTicksPerWorkload := int(cfg.History / cfg.Interval)

	stats := Seed(a, store, cfg)

	if stats.Workloads != wantWorkloads {
		t.Errorf("Workloads = %d, want %d", stats.Workloads, wantWorkloads)
	}
	if stats.Ticks != wantWorkloads*wantTicksPerWorkload {
		t.Errorf("Ticks = %d, want %d", stats.Ticks, wantWorkloads*wantTicksPerWorkload)
	}
	if stats.DegradingWorkload == "" {
		t.Error("DegradingWorkload is empty")
	}

	// One KPI write per snapshot metric (12 series, see snapshotKPIValues) per tick.
	wantKPIWrites := stats.Ticks * len(snapshotKPIValues(models.KPISnapshot{}))
	if stats.KPIWrites != wantKPIWrites {
		t.Errorf("KPIWrites = %d, want %d", stats.KPIWrites, wantKPIWrites)
	}

	rs := store.RetentionStats()
	if rs["kpi:"] != int64(wantKPIWrites) {
		t.Errorf("RetentionStats()[\"kpi:\"] = %d, want %d", rs["kpi:"], wantKPIWrites)
	}

	// Calibration bar is 96 observations; 100 ticks must clear it for every
	// seeded workload, matching the AC's "no calibration wait".
	for _, ref := range demoWorkloadRefs(cfg) {
		status, progress, _ := a.CalibrationInfo(ref)
		if status != "active" {
			t.Errorf("workload %s status = %q, want active (progress=%d)", ref.Key(), status, progress)
		}
	}
}

func TestSeed_DefaultsWhenConfigIncomplete(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	a := analyzer.NewAnalyzer()
	stats := Seed(a, store, SeedConfig{})

	def := DefaultSeedConfig()
	wantWorkloads := len(def.Namespaces) * def.PerNamespace
	if stats.Workloads != wantWorkloads {
		t.Errorf("Workloads = %d, want %d (fell back to DefaultSeedConfig)", stats.Workloads, wantWorkloads)
	}
}

func TestDegradeLive_RampsTowardBreachThenHolds(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	a := analyzer.NewAnalyzer()
	ref := models.WorkloadRef{Namespace: "prod", Kind: "Deployment", Name: "api"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		DegradeLive(ctx, a, store, ref, 20*time.Millisecond)
		close(done)
	}()

	// Let a few ticks land, then stop; DegradeLive must return promptly on
	// ctx cancellation instead of blocking forever.
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("DegradeLive did not return after ctx cancellation")
	}
}

func TestDemoDegradingMetrics_RampsRamAndErrors(t *testing.T) {
	healthy := demoDegradingMetrics(0)
	if healthy["memory_percent"] >= 0.5 {
		t.Errorf("frac=0 memory_percent = %v, want healthy baseline", healthy["memory_percent"])
	}
	if healthy["error_rate"] != 0 {
		t.Errorf("frac=0 error_rate = %v, want 0", healthy["error_rate"])
	}

	breached := demoDegradingMetrics(1)
	if breached["memory_percent"] < 0.9 {
		t.Errorf("frac=1 memory_percent = %v, want near-saturated", breached["memory_percent"])
	}
	if breached["error_rate"] <= 0 {
		t.Errorf("frac=1 error_rate = %v, want > 0", breached["error_rate"])
	}
}
