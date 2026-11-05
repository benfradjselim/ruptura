package failuremodes

import (
	"testing"
	"time"
)

func TestMemorySeries(t *testing.T) {
	start := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		observe  func(tr *Tracker)
		workload string
		want     map[string]int
	}{
		{
			name: "pods of the workload sorted, other workloads left out",
			observe: func(tr *Tracker) {
				for i := 0; i < 3; i++ {
					at := start.Add(time.Duration(i) * time.Minute)
					tr.Observe("shop/Deployment/api", "shop/api-b", at, 100, 200, 0)
					tr.Observe("shop/Deployment/api", "shop/api-a", at, 50, 200, 0)
					tr.Observe("shop/Deployment/db", "shop/db-a", at, 50, 200, 0)
				}
			},
			workload: "shop/Deployment/api",
			want:     map[string]int{"shop/api-a": 3, "shop/api-b": 3},
		},
		{
			name: "pod without a memory limit has no series",
			observe: func(tr *Tracker) {
				tr.Observe("shop/Deployment/api", "shop/api-a", start, 100, 0, 0)
			},
			workload: "shop/Deployment/api",
			want:     map[string]int{},
		},
		{
			name: "restart starts a new window",
			observe: func(tr *Tracker) {
				tr.Observe("shop/Deployment/api", "shop/api-a", start, 100, 200, 0)
				tr.Observe("shop/Deployment/api", "shop/api-a", start.Add(time.Minute), 150, 200, 0)
				tr.Observe("shop/Deployment/api", "shop/api-a", start.Add(2*time.Minute), 20, 200, 1)
			},
			workload: "shop/Deployment/api",
			want:     map[string]int{"shop/api-a": 1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTracker()
			tc.observe(tr)
			got := tr.MemorySeries(tc.workload)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d pods, want %d", len(got), len(tc.want))
			}
			for i, series := range got {
				if i > 0 && got[i-1].Pod >= series.Pod {
					t.Errorf("pods not sorted: %s before %s", got[i-1].Pod, series.Pod)
				}
				if n, ok := tc.want[series.Pod]; !ok || n != len(series.Points) {
					t.Errorf("pod %s has %d points, want %d", series.Pod, len(series.Points), n)
				}
			}
		})
	}
}

func TestMemorySeriesUsageIsShareOfLimit(t *testing.T) {
	tr := NewTracker()
	tr.Observe("shop/Deployment/api", "shop/api-a", time.Now(), 150, 200, 0)
	got := tr.MemorySeries("shop/Deployment/api")
	if len(got) != 1 || got[0].Points[0].Usage != 0.75 {
		t.Fatalf("got %+v, want usage 0.75", got)
	}
}
