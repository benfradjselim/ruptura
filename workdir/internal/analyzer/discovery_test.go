package analyzer

import (
	"sync"
	"testing"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func ref(ns, kind, name string) models.WorkloadRef {
	return models.WorkloadRef{Namespace: ns, Kind: kind, Name: name}
}

func TestRegisterWorkload_AppearsAsPending(t *testing.T) {
	a := NewAnalyzer()
	a.RegisterWorkload(ref("prod", "Deployment", "api"))

	snaps := a.AllAnalyzerSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].WorkloadStatus != models.WorkloadStatusPending {
		t.Errorf("want status %q, got %q", models.WorkloadStatusPending, snaps[0].WorkloadStatus)
	}
	if snaps[0].Workload.Name != "api" {
		t.Errorf("want name=api, got %q", snaps[0].Workload.Name)
	}
}

func TestRegisterWorkload_Idempotent(t *testing.T) {
	a := NewAnalyzer()
	a.RegisterWorkload(ref("prod", "Deployment", "api"))
	a.RegisterWorkload(ref("prod", "Deployment", "api")) // duplicate

	if refs := a.AllWorkloadRefs(); len(refs) != 1 {
		t.Errorf("want 1 ref, got %d", len(refs))
	}
}

func TestUnregisterWorkload_RemovesIt(t *testing.T) {
	a := NewAnalyzer()
	r := ref("prod", "Deployment", "api")
	a.RegisterWorkload(r)
	a.UnregisterWorkload(r)

	if snaps := a.AllAnalyzerSnapshots(); len(snaps) != 0 {
		t.Errorf("want 0 snapshots after unregister, got %d", len(snaps))
	}
}

func TestUpdateClearsPendingTelemetry(t *testing.T) {
	a := NewAnalyzer()
	r := ref("prod", "Deployment", "worker")
	a.RegisterWorkload(r)

	// First Update call should clear pending flag.
	a.Update(r, map[string]float64{"cpu": 0.5})

	snaps := a.AllAnalyzerSnapshots()
	if len(snaps) != 1 {
		t.Fatalf("want 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].WorkloadStatus == models.WorkloadStatusPending {
		t.Errorf("WorkloadStatus should not be pending_telemetry after Update")
	}
}

func TestAllWorkloadRefs_IncludesPending(t *testing.T) {
	a := NewAnalyzer()
	a.RegisterWorkload(ref("ns", "Deployment", "svc-a"))
	a.RegisterWorkload(ref("ns", "StatefulSet", "svc-b"))

	refs := a.AllWorkloadRefs()
	if len(refs) != 2 {
		t.Errorf("want 2 refs, got %d", len(refs))
	}
}

// TestRegisterUnregister_Race exercises concurrent Register/Unregister under the race detector.
func TestRegisterUnregister_Race(t *testing.T) {
	a := NewAnalyzer()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		r := ref("ns", "Deployment", "svc")
		go func() { defer wg.Done(); a.RegisterWorkload(r) }()
		go func() { defer wg.Done(); a.UnregisterWorkload(r) }()
	}
	wg.Wait()
}
