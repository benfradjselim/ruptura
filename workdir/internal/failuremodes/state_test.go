package failuremodes

import (
	"testing"
	"time"
)

// TestTracker_RestoreResumesForecast stops the engine during a leak for a gap,
// then compares the first forecast after the restart with and without the saved
// state, and with a continuous run.
func TestTracker_RestoreResumesForecast(t *testing.T) {
	const workload, pod = "ns/Deployment/leak", "ns/leak-1"
	leak := ordersLeak(20)
	start := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	at := func(i int) time.Time { return start.Add(time.Duration(i) * 30 * time.Second) }
	// firstForecast feeds samples from..to and returns the first minute with a
	// saturation forecast, or -1.
	firstForecast := func(tr *Tracker, from, to int) float64 {
		for i := from; i <= to; i++ {
			tr.Observe(workload, pod, at(i), leak(float64(i)/2), 256*benchMiB, 0)
			if est, ok := tr.Worst(workload, at(i)); ok && est.Mode == ModeSaturation {
				return float64(i) / 2
			}
		}
		return -1
	}
	tests := []struct {
		name    string
		gap     int // samples missed while the engine restarts
		restore bool
		save    func(State) State
		// want is the latest minute of the first forecast; zero means none before minute 18.
		want float64
	}{
		{"continuous run", 0, false, nil, 10},
		{"restart with saved state", 2, true, nil, 11},
		{"restart without saved state", 2, false, nil, 0},
		{"state older than the window", 70, true, nil, 0},
		{"state of another layout version", 2, true, func(s State) State { s.Version++; return s }, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := NewTracker()
			if got := firstForecast(before, 0, 15); got >= 0 && tt.gap > 0 {
				t.Fatalf("forecast at minute %v before the restart, test needs none", got)
			}
			after := before
			resume := 16
			if tt.gap > 0 {
				state := before.Snapshot(at(15))
				if tt.save != nil {
					state = tt.save(state)
				}
				data, err := EncodeState(state)
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := DecodeState(data)
				if err != nil {
					t.Fatal(err)
				}
				resume = 16 + tt.gap
				after = NewTracker()
				if tt.restore {
					after.Restore(decoded, at(resume))
				}
			}
			// The same pod keeps leaking through the outage.
			got := firstForecast(after, resume, resume+36)
			if tt.gap == 0 {
				got = firstForecast(NewTracker(), 0, 36)
			}
			relative := got
			if got >= 0 {
				relative = got - float64(resume-16)/2
			}
			t.Logf("first forecast at minute %v", got)
			if tt.want == 0 {
				if got >= 0 && relative < 18 {
					t.Fatalf("forecast at minute %v, want none before minute 18 of leak", got)
				}
				return
			}
			if got < 0 || relative > tt.want {
				t.Fatalf("first forecast at minute %v, want by minute %v", got, tt.want)
			}
		})
	}
}

func TestDecodeState_Corrupt(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("not gzip"), {0x1f, 0x8b, 0x08, 0x00}} {
		if _, err := DecodeState(data); err == nil {
			t.Errorf("DecodeState(%q) gave no error", data)
		}
	}
}

func TestTracker_SnapshotReleaseAndCondition(t *testing.T) {
	tr := NewTracker()
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 30; i++ {
		at := now.Add(time.Duration(i) * 30 * time.Second)
		tr.ObserveVersion("ns/app-a-1", "app-a")
		tr.Observe("ns/Deployment/app", "ns/app-a-1", at, 80*benchMiB, 256*benchMiB, 0)
		tr.ObserveCondition("ns/Deployment/app", "ns/app-a-1", at, 0.9)
	}
	end := now.Add(29 * 30 * time.Second)
	restored := NewTracker()
	restored.Restore(tr.Snapshot(end), end.Add(30*time.Second))
	restored.ObserveCondition("ns/Deployment/app", "ns/app-a-1", end.Add(30*time.Second), 0.9)
	if est, ok := restored.Worst("ns/Deployment/app", end.Add(30*time.Second)); !ok || est.Mode != ModeCPUThrottling {
		t.Fatalf("throttling not resumed after restore: %+v %v", est, ok)
	}
	if got := restored.releases["ns/Deployment/app"].pods["ns/app-a-1"].version; got != "app-a" {
		t.Fatalf("release pod version = %q, want app-a", got)
	}
}

func TestTracker_ObserveUnlimited(t *testing.T) {
	tr := NewTracker()
	const workload, pod = "ns/Deployment/cache", "ns/cache-1"
	start := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	var last Estimate
	found := false
	for i := 0; i <= 40; i++ {
		at := start.Add(time.Duration(i) * 30 * time.Second)
		ceiling := 400.0 * benchMiB
		if i > 25 {
			ceiling = 420 * benchMiB // the hourly refresh moved the ceiling
		}
		tr.ObserveUnlimited(workload, pod, at, (100+4*float64(i)/2)*benchMiB, ceiling, "7-day P99 + 20%", 0)
		last, found = tr.Worst(workload, at)
	}
	if !found || last.Mode != ModeMemoryBeyondUsual {
		t.Fatalf("Worst = %+v %v, want memory_beyond_usual", last, found)
	}
	if last.Ceiling != 420*benchMiB || last.CeilingSource != "7-day P99 + 20%" {
		t.Fatalf("ceiling %v from %q", last.Ceiling, last.CeilingSource)
	}
	// 180 MiB now, growing 4 MiB per minute, 240 MiB to go: about 60 minutes.
	if m := last.ETF.Minutes(); m < 55 || m > 65 {
		t.Fatalf("ETF %v min, want about 60 after the ceiling moved", m)
	}
}
