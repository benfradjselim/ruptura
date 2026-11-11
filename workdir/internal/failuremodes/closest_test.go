package failuremodes

import (
	"math"
	"testing"
	"time"
)

func TestTracker_Closest(t *testing.T) {
	tr := NewTracker()
	start := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	var now time.Time
	for i := 0; i < 20; i++ {
		now = start.Add(time.Duration(i) * 30 * time.Second)
		m := float64(i) / 2
		tr.Observe("ns/Deployment/leaky", "ns/leaky-1", now, (100+6*m)*mib, 256*mib, 0)
		tr.Observe("ns/Deployment/flat", "ns/flat-1", now, 50*mib, 256*mib, 0)
		tr.Observe("ns/Deployment/nolimit", "ns/nolimit-1", now, 500*mib, 0, 0)
	}
	tr.Observe("ns/Deployment/gone", "ns/gone-1", start, 250*mib, 256*mib, 0)

	got := tr.Closest(now, 10)
	if len(got) != 2 {
		t.Fatalf("got %d pods %+v, want the 2 fresh pods with a limit", len(got), got)
	}
	if got[0].Pod != "ns/leaky-1" || got[1].Pod != "ns/flat-1" {
		t.Fatalf("order = %s, %s; want leaky then flat", got[0].Pod, got[1].Pod)
	}
	wantTrend := 6.0 * 60 / 256 // 6 MiB per minute of a 256 MiB limit, per hour
	if math.Abs(got[0].TrendPerHour-wantTrend) > 0.01 || got[1].TrendPerHour != 0 {
		t.Fatalf("trends = %.3f, %.3f; want %.3f and 0", got[0].TrendPerHour, got[1].TrendPerHour, wantTrend)
	}
	if one := tr.Closest(now, 1); len(one) != 1 || one[0].Pod != "ns/leaky-1" {
		t.Fatalf("Closest(1) = %+v", one)
	}
}
