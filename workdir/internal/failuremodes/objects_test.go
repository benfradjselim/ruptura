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
		o.ObserveTrend("node-filesystem-filling", "node", "n1//dev/loop0", "node_filesystem_filling", 6*time.Hour, "bytes", now, time.Time{}, 100+4*float64(i), size)
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
				o.ObserveTrend("c", "node", "n1", "m", tt.horizon, "bytes", now, time.Time{}, tt.usage(i), 512)
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
				o.ObserveDetected("c", "workload", "ns/x", "m", tt.raise, tt.clear, now, time.Time{}, 1)
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
		o.ObserveDetected("c", "workload", "ns/x", "m", 0, 2*time.Minute, now, time.Time{}, v)
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
	o.ObserveDetected("job-failed", "workload", "ns/job", "job_failed", 0, 0, objectsStart, time.Time{}, 1)
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
	o.ObserveDetected("c", "node", "n1/Ready", "node_not_ready", 0, 0, objectsStart, time.Time{}, 1)
	if len(o.Findings(objectsStart.Add(2*time.Minute))) != 1 {
		t.Fatal("dropped too early while queries fail")
	}
	if len(o.Findings(objectsStart.Add(4*time.Minute))) != 0 {
		t.Fatal("kept a finding with no reading for 4 minutes")
	}
}

func TestObjectsPrune(t *testing.T) {
	o := NewObjects()
	o.ObserveDetected("c", "node", "old", "m", 0, 0, objectsStart, time.Time{}, 1)
	o.ObserveTrend("t", "node", "old", "m", time.Hour, "bytes", objectsStart, time.Time{}, 1, 2)
	o.Prune(objectsStart.Add(time.Minute))
	if len(o.conditions) != 0 || len(o.trends) != 0 {
		t.Fatalf("prune kept %d conditions and %d trends", len(o.conditions), len(o.trends))
	}
}

// ksmPolls feeds a detected card polled every 30 s from a source sampled every
// 2 minutes, true from condStart on, and returns the first poll time at which
// the finding is listed.
func ksmPolls(t *testing.T, raise time.Duration, condStart time.Time, end time.Time) time.Time {
	t.Helper()
	o := NewObjects()
	for now := objectsStart; !now.After(end); now = now.Add(30 * time.Second) {
		sampled := objectsStart.Add(now.Sub(objectsStart).Truncate(2 * time.Minute))
		if sampled.Before(condStart) {
			o.Polled("c", now)
			continue
		}
		o.ObserveDetected("c", "workload", "ns/x", "m", raise, time.Minute, now, sampled, 1)
		o.Polled("c", now)
		if len(o.Findings(now)) == 1 {
			return now
		}
	}
	return time.Time{}
}

func TestObjectsSampledConditionDatesNewObjectFromSourceInterval(t *testing.T) {
	o := NewObjects()
	raisedAt := time.Time{}
	for now := objectsStart; now.Before(objectsStart.Add(20 * time.Minute)); now = now.Add(30 * time.Second) {
		sampled := objectsStart.Add(now.Sub(objectsStart).Truncate(2 * time.Minute))
		// Another object of the card shows the source's 2 minute rhythm.
		o.ObserveDetected("c", "workload", "ns/other", "m", 9*time.Minute, time.Minute, now, sampled, 0)
		if sampled.After(objectsStart) {
			o.ObserveDetected("c", "workload", "ns/x", "m", 9*time.Minute, time.Minute, now, sampled, 1)
		}
		o.Polled("c", now)
		if raisedAt.IsZero() && len(o.Findings(now)) == 1 {
			raisedAt = now
		}
	}
	// First seen in the sample at 2 min, dated at 1 min: a 9 min run is reached by
	// the sample at 10 min, not 12 min as it would be from its first sample.
	if want := objectsStart.Add(10 * time.Minute); !raisedAt.Equal(want) {
		t.Fatalf("raised at %v, want %v", raisedAt.Sub(objectsStart), want.Sub(objectsStart))
	}
}

func TestObjectsSampledConditionSurvivesOneMissedScrape(t *testing.T) {
	o := NewObjects()
	raised := false
	for now := objectsStart; now.Before(objectsStart.Add(16 * time.Minute)); now = now.Add(30 * time.Second) {
		sampled := objectsStart.Add(now.Sub(objectsStart).Truncate(2 * time.Minute))
		if sampled.Equal(objectsStart.Add(6 * time.Minute)) {
			sampled = objectsStart.Add(4 * time.Minute) // the 6 min scrape is missed
		}
		o.ObserveDetected("c", "workload", "ns/x", "m", 10*time.Minute, time.Minute, now, sampled, 1)
		o.Polled("c", now)
		raised = raised || len(o.Findings(now)) == 1
	}
	if !raised {
		t.Fatal("a missed 2 minute scrape restarted the 10 minute run")
	}
}

func TestObjectsDatedTrendGoneWhenSourceStops(t *testing.T) {
	o := NewObjects()
	var sampled time.Time
	poll := func(now time.Time) []ObjectFinding {
		o.ObserveTrend("q", "namespace", "ns/q/configmaps", "quota_exhaustion", 6*time.Hour, "count", now, sampled, float64(4*now.Sub(objectsStart)/(2*time.Minute)), 400)
		o.Polled("q", now)
		return o.Findings(now)
	}
	now := objectsStart
	for ; now.Before(objectsStart.Add(40 * time.Minute)); now = now.Add(30 * time.Second) {
		sampled = objectsStart.Add(now.Sub(objectsStart).Truncate(2 * time.Minute))
		poll(now)
	}
	if len(poll(now)) != 1 {
		t.Fatal("no forecast while the source runs")
	}
	// The source stops: Prometheus keeps serving the last sample.
	stoppedAt := now
	for ; now.Before(stoppedAt.Add(6 * time.Minute)); now = now.Add(30 * time.Second) {
		if got := poll(now); len(got) == 0 {
			if age := now.Sub(sampled); age <= 4*time.Minute {
				t.Fatalf("dropped at sample age %v, want after 2 intervals", age)
			}
			return
		}
	}
	t.Fatal("forecast kept on a sample more than 5 minutes old")
}

func TestObjectsSeenKeepsObjectWithoutFeedingIt(t *testing.T) {
	o := NewObjects()
	o.ObserveDetected("c", "workload", "ns/x", "m", 0, 0, objectsStart, objectsStart, 1)
	o.Polled("c", objectsStart)
	next := objectsStart.Add(30 * time.Second)
	o.Seen("c", "ns/x", next)
	o.Polled("c", next)
	if len(o.Findings(next)) != 1 {
		t.Fatal("an object seen without a sample time was dropped")
	}
	if got := o.conditions[objectKey("c", "ns/x")].at; !got.Equal(objectsStart) {
		t.Fatalf("sample time moved to %v, want the last real sample", got)
	}
}

func TestAgedIgnoresSampleAheadOfClock(t *testing.T) {
	est := Estimate{ETF: 10 * time.Minute, ETFEarly: 8 * time.Minute, ETFLate: 12 * time.Minute}
	if got := aged(est, -30*time.Second); got != est {
		t.Fatalf("aged = %+v, want unchanged", got)
	}
	if got := aged(est, 9*time.Minute); got.ETF != time.Minute || got.ETFEarly != 0 || got.ETFLate != 3*time.Minute {
		t.Fatalf("aged = %+v", got)
	}
}

func TestObjectsSampledConditionCountsRepeatedSampleOnce(t *testing.T) {
	o := NewObjects()
	for i := 0; i < 4; i++ {
		now := objectsStart.Add(time.Duration(i) * 30 * time.Second)
		o.ObserveDetected("c", "workload", "ns/x", "m", 0, 2*time.Minute, now, objectsStart, 1)
	}
	// Dated at 1 min, one sample read four times does not make the 2 min run below.
	later := objectsStart.Add(2 * time.Minute)
	for i := 0; i < 4; i++ {
		o.ObserveDetected("c", "workload", "ns/x", "m", 0, 2*time.Minute, later.Add(time.Duration(i)*30*time.Second), later, 0)
		o.Polled("c", later.Add(time.Duration(i)*30*time.Second))
	}
	if len(o.Findings(later.Add(90*time.Second))) != 1 {
		t.Fatal("cleared on one sample read several times")
	}
}

func TestObjectsSampledConditionKeepsTwoMinuteSamplesContinuous(t *testing.T) {
	if got := ksmPolls(t, 10*time.Minute, objectsStart, objectsStart.Add(20*time.Minute)); got.IsZero() {
		t.Fatal("never raised: samples 2 minutes apart broke the run")
	}
}

func TestObjectsTrendCountsDownFromSampleTime(t *testing.T) {
	o := NewObjects()
	var now, sampled time.Time
	for i := 0; i <= 20; i++ {
		sampled = objectsStart.Add(time.Duration(i) * 2 * time.Minute)
		now = sampled.Add(90 * time.Second)
		o.ObserveTrend("q", "namespace", "ns/q/configmaps", "quota_exhaustion", 6*time.Hour, "count", now, sampled, float64(4*i), 120)
		o.Polled("q", now)
	}
	got := o.Findings(now)
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1", len(got))
	}
	// 80 of 120 at the last sample, 2 per minute: 20 min after the sample, 18.5 min after now.
	want := 18*time.Minute + 30*time.Second
	if diff := got[0].Estimate.ETF - want; diff < -15*time.Second || diff > 15*time.Second {
		t.Fatalf("ETF = %v, want about %v", got[0].Estimate.ETF, want)
	}
}

func TestObjectsTrendWithdrawnWhenFillStops(t *testing.T) {
	o := NewObjects()
	usage := 100.0
	listed := func(i int) bool {
		now := objectsStart.Add(time.Duration(i) * 30 * time.Second)
		o.ObserveTrend("c", "node", "n1", "m", 6*time.Hour, "bytes", now, now, usage, 1024)
		o.Polled("c", now)
		return len(o.Findings(now)) == 1
	}
	i := 0
	for ; i <= 40; i++ {
		usage += 4
		listed(i)
	}
	if !listed(i) {
		t.Fatal("no forecast while filling")
	}
	i++
	stoppedAt := i
	for ; i < stoppedAt+10; i++ {
		if !listed(i) {
			break
		}
	}
	if flat := i - stoppedAt; flat > 2 {
		t.Fatalf("forecast kept for %d flat samples, want at most 2", flat)
	}
}

func TestStalledKeepsStepLeak(t *testing.T) {
	var points []point
	usage := 0.1
	for i := 0; i < 60; i++ {
		// One step every 5 samples: pauses of 4 samples are the trend's own rhythm.
		if i%5 == 0 {
			usage += 0.01
		}
		points = append(points, point{objectsStart.Add(time.Duration(i) * 30 * time.Second), usage})
		if i > 20 && stalled(points) {
			t.Fatalf("step leak withdrawn at sample %d", i)
		}
	}
}
