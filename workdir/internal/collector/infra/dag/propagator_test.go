package dag

import (
	"math"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/collector/infra"
)

func TestPropagator_Tick(t *testing.T) {
	now := time.Now()
	_ = now

	tests := []struct {
		name          string
		ns            string
		snapshots     []infra.GroupSnapshot
		wantWorkload  float64 // expected WorkloadPressure ±0.001
		wantWorkloadMin float64
	}{
		{
			name:         "no snapshots — workload pressure zero",
			ns:           "default",
			snapshots:    nil,
			wantWorkload: 0,
		},
		{
			name: "fully healthy cluster-scoped — no pressure",
			ns:   "app",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 1.0, 0.0),
			},
			wantWorkload: 0,
		},
		{
			name: "degraded controlplane — workload receives pressure",
			ns:   "app",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 0.5, 0.0), // activation=0.5
			},
			// controlplane→workload ω=1.0: 0.5*1.0=0.5
			wantWorkload: 0.5,
		},
		{
			name: "cluster-scoped and namespace-scoped combined",
			ns:   "payments",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupControlPlane, "", 0.8, 0.0),      // activation=0.2
				snap(infra.GroupNetwork, "payments", 0.6, 0.0),   // activation=0.4
				snap(infra.GroupStorage, "other-ns", 0.0, 0.0),   // wrong ns — excluded
			},
			// controlplane→workload: 0.2*1.0=0.2
			// network→workload: 0.4*0.9=0.36
			// workload = max(0.2, 0.36) = 0.36
			wantWorkloadMin: 0.35,
		},
		{
			name: "tick is idempotent for same input",
			ns:   "test",
			snapshots: []infra.GroupSnapshot{
				snap(infra.GroupNetwork, "test", 0.7, 0.0),
			},
			// network→workload: 0.3*0.9=0.27
			wantWorkload: 0.3 * 0.9,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewPropagator()
			result := p.Tick(tc.ns, tc.snapshots)

			if result.Namespace != tc.ns {
				t.Errorf("Namespace = %q, want %q", result.Namespace, tc.ns)
			}
			if result.Timestamp.IsZero() {
				t.Error("Timestamp must not be zero")
			}

			wp := result.WorkloadPressure()
			if tc.wantWorkloadMin > 0 {
				if wp < tc.wantWorkloadMin {
					t.Errorf("WorkloadPressure = %.4f, want >= %.4f", wp, tc.wantWorkloadMin)
				}
			} else {
				if math.Abs(wp-tc.wantWorkload) > 0.001 {
					t.Errorf("WorkloadPressure = %.4f, want %.4f", wp, tc.wantWorkload)
				}
			}

			// LastResult must return the cached result.
			cached := p.LastResult(tc.ns)
			if cached.Namespace != tc.ns {
				t.Errorf("LastResult.Namespace = %q, want %q", cached.Namespace, tc.ns)
			}
		})
	}
}

func TestPropagator_LastResult_MissingNamespace(t *testing.T) {
	p := NewPropagator()
	r := p.LastResult("nonexistent")
	if r.WorkloadPressure() != 0 {
		t.Errorf("LastResult for unknown ns: WorkloadPressure = %.4f, want 0", r.WorkloadPressure())
	}
}

func TestPropagator_AllResults(t *testing.T) {
	p := NewPropagator()
	p.Tick("ns-a", []infra.GroupSnapshot{snap(infra.GroupNetwork, "ns-a", 0.5, 0)})
	p.Tick("ns-b", []infra.GroupSnapshot{snap(infra.GroupStorage, "ns-b", 0.4, 0)})

	all := p.AllResults()
	if len(all) != 2 {
		t.Errorf("AllResults len = %d, want 2", len(all))
	}
	if _, ok := all["ns-a"]; !ok {
		t.Error("ns-a missing from AllResults")
	}
	if _, ok := all["ns-b"]; !ok {
		t.Error("ns-b missing from AllResults")
	}
}

func TestPropagator_ConcurrentTick(t *testing.T) {
	p := NewPropagator()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			for j := 0; j < 50; j++ {
				p.Tick("shared-ns", []infra.GroupSnapshot{
					snap(infra.GroupNetwork, "shared-ns", 0.9, 0),
				})
				_ = p.LastResult("shared-ns")
			}
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}
