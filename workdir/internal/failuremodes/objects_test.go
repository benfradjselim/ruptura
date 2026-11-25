package failuremodes

import (
	"testing"
	"time"
)

var objectsStart = time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

func TestObjectsTrendForecastsFillingFilesystem(t *testing.T) {
	o := NewObjects()
	const size = 512.0
	var now time.Time
	for i := 0; i <= 40; i++ {
		now = objectsStart.Add(time.Duration(i) * 30 * time.Second)
		o.ObserveTrend("node-filesystem-filling", "node", "n1//dev/loop0", "node_filesystem_filling", 6*time.Hour, "bytes", now, 100+4*float64(i), size)
		o.Polled("node-filesystem-filling", now)
	}
	got := o.Findings(now)
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1", len(got))
	}
	f := got[0]
	if !f.Predicted || f.Level != "node" || f.Target != "n1//dev/loop0" || f.Estimate.Mode != "node_filesystem_filling" {
		t.Fatalf("finding = %+v", f)
	}
	// 260 of 512 used, growing 8 per minute: full in 31.5 minutes.
	want := 31*time.Minute + 30*time.Second
	if diff := f.Estimate.ETF - want; diff < -time.Minute || diff > time.Minute {
		t.Fatalf("ETF = %v, want about %v", f.Estimate.ETF, want)
	}
}

func TestObjectsTrendQuietCases(t *testing.T) {
	tests := []struct {
		name    string
		horizon time.Duration
		usage   func(i int) float64
	}{
		{"flat usage", 6 * time.Hour, func(int) float64 { return 300 }},
		{"beyond horizon", 10 * time.Minute, func(i int) float64 { return 100 + 4*float64(i) }},
		{"falling usage", 6 * time.Hour, func(i int) float64 { return 400 - 4*float64(i) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewObjects()
			var now time.Time
			for i := 0; i <= 40; i++ {
				now = objectsStart.Add(time.Duration(i) * 30 * time.Second)
				o.ObserveTrend("c", "node", "n1", "m", tt.horizon, "bytes", now, tt.usage(i), 512)
				o.Polled("c", now)
			}
			if got := o.Findings(now); len(got) != 0 {
				t.Fatalf("findings = %+v, want none", got)
			}
		})
	}
}

func TestObjectsDetectedRaiseAndClear(t *testing.T) {
	tests := []struct {
		name        string
		raise       time.Duration
		clear       time.Duration
		on          int // polls at 1, 30 s apart
		wantRaiseAt int // first poll index reporting the finding, -1 for never
	}{
		{"raise after ten minutes", 10 * time.Minute, 2 * time.Minute, 30, 20},
		{"too short to raise", 10 * time.Minute, 2 * time.Minute, 15, -1},
		{"immediate", 0, 0, 3, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewObjects()
			raisedAt := -1
			for i := 0; i < tt.on; i++ {
				now := objectsStart.Add(time.Duration(i) * 30 * time.Second)
				o.ObserveDetected("c", "workload", "ns/x", "m", tt.raise, tt.clear, now, 1)
				o.Polled("c", now)
				if raisedAt < 0 && len(o.Findings(now)) == 1 {
					raisedAt = i
				}
			}
			if raisedAt != tt.wantRaiseAt {
				t.Fatalf("raised at poll %d, want %d", raisedAt, tt.wantRaiseAt)
			}
		})
	}
}

func TestObjectsDetectedClearsAfterDelay(t *testing.T) {
	o := NewObjects()
	i := 0
	poll := func(v float64) []ObjectFinding {
		now := objectsStart.Add(time.Duration(i) * 30 * time.Second)
		i++
		o.ObserveDetected("c", "workload", "ns/x", "m", 0, 2*time.Minute, now, v)
		o.Polled("c", now)
		return o.Findings(now)
	}
	if len(poll(1)) != 1 {
		t.Fatal("not raised")
	}
	for k := 0; k < 4; k++ {
		if len(poll(0)) != 1 {
			t.Fatalf("cleared after %d polls below, want 2 minutes", k+1)
		}
	}
	if len(poll(0)) != 0 {
		t.Fatal("still raised after 2 minutes below")
	}
}

func TestObjectsGoneWhenMissingFromPoll(t *testing.T) {
	o := NewObjects()
	o.ObserveDetected("job-failed", "workload", "ns/job", "job_failed", 0, 0, objectsStart, 1)
	o.Polled("job-failed", objectsStart)
	if len(o.Findings(objectsStart)) != 1 {
		t.Fatal("not raised")
	}
	next := objectsStart.Add(30 * time.Second)
	o.Polled("job-failed", next)
	if got := o.Findings(next); len(got) != 0 {
		t.Fatalf("deleted job still reported: %+v", got)
	}
}

func TestObjectsStaleWithoutPoll(t *testing.T) {
	o := NewObjects()
	o.ObserveDetected("c", "node", "n1/Ready", "node_not_ready", 0, 0, objectsStart, 1)
	if len(o.Findings(objectsStart.Add(2*time.Minute))) != 1 {
		t.Fatal("dropped too early while queries fail")
	}
	if len(o.Findings(objectsStart.Add(4*time.Minute))) != 0 {
		t.Fatal("kept a finding with no reading for 4 minutes")
	}
}

func TestObjectsPrune(t *testing.T) {
	o := NewObjects()
	o.ObserveDetected("c", "node", "old", "m", 0, 0, objectsStart, 1)
	o.ObserveTrend("t", "node", "old", "m", time.Hour, "bytes", objectsStart, 1, 2)
	o.Prune(objectsStart.Add(time.Minute))
	if len(o.conditions) != 0 || len(o.trends) != 0 {
		t.Fatalf("prune kept %d conditions and %d trends", len(o.conditions), len(o.trends))
	}
}
