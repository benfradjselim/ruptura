package sim

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/failuremodes"
)

var demoStart = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

func TestNewDemoSeedsFourFailures(t *testing.T) {
	d := NewDemo(demoStart)
	tests := []struct {
		workload string
		mode     failuremodes.Mode
		found    bool
		restarts int
	}{
		{"shop/Deployment/checkout", failuremodes.ModeSaturation, true, 0},
		{"ops/Deployment/batch-api", failuremodes.ModeSaturation, true, 0},
		{"shop/Deployment/catalog", failuremodes.ModeCPUThrottling, true, 0},
		{"shop/Deployment/payments", "", false, demoCrashRestarts},
		{"shop/Deployment/frontend", "", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.workload, func(t *testing.T) {
			est, found := d.Worst(tt.workload, demoStart)
			if found != tt.found || est.Mode != tt.mode {
				t.Fatalf("Worst = %v %v, want %v %v", est.Mode, found, tt.mode, tt.found)
			}
			if got := d.RecentRestarts(tt.workload, demoStart, time.Hour); got != tt.restarts {
				t.Fatalf("RecentRestarts = %d, want %d", got, tt.restarts)
			}
		})
	}
	if n := len(d.Workloads(demoStart)); n != len(demoWorkloads) {
		t.Fatalf("Workloads = %d, want %d", n, len(demoWorkloads))
	}
}

func TestDemoLeakCountsDownThenKillsThenStartsOver(t *testing.T) {
	d := NewDemo(demoStart)
	w := "shop/Deployment/checkout"

	est, _ := d.Worst(w, demoStart.Add(10*time.Minute))
	if est.ETF != 12*time.Minute || est.ETFEarly >= est.ETF || est.ETFLate <= est.ETF {
		t.Fatalf("after 10 min: ETF %v [%v, %v], want 12m inside its range", est.ETF, est.ETFEarly, est.ETFLate)
	}
	if est, _ := d.Worst(w, demoStart.Add(23*time.Minute)); est.Mode != failuremodes.ModeOOMKilled {
		t.Fatalf("after the deadline: %v, want an OOM kill", est.Mode)
	}
	if est, _ := d.Worst(w, demoStart.Add(28*time.Minute)); est.Mode != failuremodes.ModeSaturation || est.ETF != 22*time.Minute {
		t.Fatalf("after the kill pause: %v %v, want a new 22m leak", est.Mode, est.ETF)
	}
}

func TestDemoInject(t *testing.T) {
	d := NewDemo(demoStart)
	if err := d.Inject("lab/Deployment/api", PatternMemoryLeak, demoStart); err != nil {
		t.Fatal(err)
	}
	if est, ok := d.Worst("lab/Deployment/api", demoStart); !ok || est.Mode != failuremodes.ModeSaturation {
		t.Fatalf("injected leak not reported: %v %v", est.Mode, ok)
	}
	if err := d.Inject("api", PatternMemoryLeak, demoStart); err == nil {
		t.Fatal("accepted a workload without namespace and kind")
	}
	if err := d.Inject("lab/Deployment/api", "meteor", demoStart); err == nil {
		t.Fatal("accepted an unknown pattern")
	}
}
