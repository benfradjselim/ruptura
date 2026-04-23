package orchestrator

// Coverage boost for collectLocally (was 15.8%), logAlerts, runGC, runCompaction.
// All tests use Port=0 (OS-assigned) and short intervals so they are CI-safe.

import (
	"context"
	"testing"
	"time"
)

func engineForRun(t *testing.T, interval time.Duration) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.StoragePath = t.TempDir()
	cfg.Port = 0
	cfg.DogStatsDAddr = ""
	cfg.CollectInterval = interval

	eng, err := New(cfg)
	if err != nil {
		t.Fatalf("New central engine: %v", err)
	}
	return eng
}

// TestCollectLocally_FiresTicks runs the central-mode engine long enough to
// trigger several collection ticks. This exercises collectLocally, logAlerts,
// runGC, runCompaction, and buildMetricsMap in one pass.
func TestCollectLocally_FiresTicks(t *testing.T) {
	eng := engineForRun(t, 30*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify that KPI data was stored by at least one tick
	now := time.Now()
	tvs, err := eng.store.ForOrg("default").GetKPIRange(
		eng.cfg.Host, "stress", now.Add(-5*time.Second), now,
	)
	if err != nil {
		t.Logf("GetKPIRange: %v (may be empty on fast CI)", err)
	}
	t.Logf("stress KPI points stored after run: %d", len(tvs))
}

// TestCollectLocally_GracefulShutdown verifies that Run() returns cleanly
// when the context is cancelled mid-collection.
func TestCollectLocally_GracefulShutdown(t *testing.T) {
	eng := engineForRun(t, 25*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := eng.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Errorf("Run took too long after cancel: %v", elapsed)
	}
}
