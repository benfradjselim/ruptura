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
