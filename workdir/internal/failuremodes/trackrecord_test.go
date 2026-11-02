package failuremodes

import (
	"testing"
	"time"
)

func TestTrackRecord(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker()
	fail := func(minute int, restarts int) {
		tr.ObserveRestarts("w", "n/p", start.Add(time.Duration(minute)*time.Minute), restarts)
	}
	fail(0, 0)
	tr.NoteForecast("w", start.Add(10*time.Minute))
	tr.NoteForecast("w", start.Add(12*time.Minute))
	fail(30, 1)
	fail(60, 2)
	tr.NoteForecast("w", start.Add(80*time.Minute))
	fail(82, 3)

	got := tr.TrackRecord(start, start.Add(2*time.Hour))
	want := TrackRecord{Failures: 3, Warned: 1, MedianLead: 20 * time.Minute}
	if got != want {
		t.Errorf("record = %+v, want %+v", got, want)
	}
	if got := tr.TrackRecord(start.Add(70*time.Minute), start.Add(2*time.Hour)); got.Failures != 1 || got.Warned != 0 {
		t.Errorf("window record = %+v", got)
	}
}

func TestTrackRecord_Restored(t *testing.T) {
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	var saved []time.Duration
	tr := NewTracker()
	tr.SetOutcomeRecorder(func(_ time.Time, lead time.Duration) { saved = append(saved, lead) })
	tr.ObserveRestarts("w", "n/p", start, 0)
	tr.NoteForecast("w", start.Add(10*time.Minute))
	tr.ObserveRestarts("w", "n/p", start.Add(30*time.Minute), 1)
	if len(saved) != 1 || saved[0] != 20*time.Minute {
		t.Fatalf("saved = %v", saved)
	}

	restored := NewTracker()
	restored.RestoreOutcome(start.Add(30*time.Minute), saved[0])
	got := restored.TrackRecord(start, start.Add(time.Hour))
	if got.Failures != 1 || got.Warned != 1 || got.MedianLead != 20*time.Minute {
		t.Errorf("record = %+v", got)
	}
}
