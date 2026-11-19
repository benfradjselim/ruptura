package failuremodes

import (
	"testing"
	"time"
)

// TestTracker_OOMIsTheCause replays the det-oom fault of the benchmark: a
// single replica killed for memory, restarting, missing, in back-off. Whatever
// symptom is also present, the reported mode is oom_killed, while a forecast of
// the next failure still comes first.
func TestTracker_OOMIsTheCause(t *testing.T) {
	const workload, pod = "shop/Deployment/det-oom", "shop/det-oom-1"
	start := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		symptom func(tr *Tracker, now time.Time)
		want    Mode
	}{
		{"OOM alone", func(*Tracker, time.Time) {}, ModeOOMKilled},
		{"OOM with its replica missing", func(tr *Tracker, now time.Time) {
			tr.ObserveReplicas(workload, 1, now.Add(-6*time.Minute))
			tr.ObserveReplicas(workload, 1, now)
		}, ModeOOMKilled},
		{"OOM with a stalled rollout", func(tr *Tracker, now time.Time) { tr.ObserveRollout(workload, true, now) }, ModeOOMKilled},
		{"OOM and a start blocked on a mount", func(tr *Tracker, now time.Time) {
			tr.ObserveEvent(workload, "shop/det-oom-2", "FailedMount", "", now)
		}, ModeOOMKilled},
		{"OOM and a forecast of the next one", func(tr *Tracker, now time.Time) {
			for i := 0; i <= 30; i++ {
				at := now.Add(time.Duration(i-30) * 30 * time.Second)
				tr.Observe(workload, "shop/det-oom-3", at, (40+6*float64(i))*benchMiB, 256*benchMiB, 0)
			}
		}, ModeSaturation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			tr.ObserveRestarts(workload, pod, start, 0)
			now := start.Add(2 * time.Minute)
			tr.ObserveRestarts(workload, pod, now, 1)
			tr.ObserveOOM(workload, pod, true, now)
			tt.symptom(tr, now)
			est, ok := tr.Worst(workload, now)
			if !ok || est.Mode != tt.want {
				t.Fatalf("Worst = %s (%v), want %s", est.Mode, ok, tt.want)
			}
		})
	}
	t.Run("old OOM no longer hides a new cause", func(t *testing.T) {
		tr := NewTracker()
		tr.ObserveRestarts(workload, pod, start, 0)
		tr.ObserveRestarts(workload, pod, start.Add(time.Minute), 1)
		tr.ObserveOOM(workload, pod, true, start.Add(time.Minute))
		later := start.Add(time.Minute + oomRecentWindow + time.Minute)
		tr.ObserveEvent(workload, "shop/det-oom-2", "FailedMount", "", later)
		if est, _ := tr.Worst(workload, later); est.Mode != ModeStartupStuck {
			t.Fatalf("Worst = %s, want startup_stuck", est.Mode)
		}
	})
}

// TestTracker_UnschedulableReplica replays the det-replicas fault: one of two
// replicas cannot be scheduled. The workload is short of a replica from the
// first FailedScheduling event, not five minutes later, and other blocked
// starts stay startup_stuck.
func TestTracker_UnschedulableReplica(t *testing.T) {
	const workload = "shop/Deployment/det-replicas"
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name, reason string
		replicas     int
		want         Mode
		wantMissing  float64
	}{
		{"unschedulable, replica count not read yet", "FailedScheduling", 0, ModeReplicasUnavailable, 1},
		{"unschedulable, two missing", "FailedScheduling", 2, ModeReplicasUnavailable, 2},
		{"mount failure", "FailedMount", 1, ModeStartupStuck, 0},
		{"image failure", "Failed", 1, ModeStartupStuck, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			if tt.replicas > 0 {
				tr.ObserveReplicas(workload, tt.replicas, now)
			}
			tr.ObserveEvent(workload, "shop/det-replicas-2", tt.reason, "Error: ErrImagePull", now)
			est, ok := tr.Worst(workload, now.Add(30*time.Second))
			if !ok || est.Mode != tt.want {
				t.Fatalf("Worst = %+v (%v), want %s", est, ok, tt.want)
			}
			if tt.want == ModeReplicasUnavailable && (est.Usage != tt.wantMissing || est.Reason != "FailedScheduling" || est.Pod != "shop/det-replicas-2") {
				t.Fatalf("estimate = %+v", est)
			}
		})
	}
}
