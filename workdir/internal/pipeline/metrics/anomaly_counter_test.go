package metrics

import (
	"math/rand"
	"testing"
	"time"
)

func TestAnomalyEngine_CumulativeCounters(t *testing.T) {
	const metric = "k8s.pod.network.io"
	start := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)

	feed := func(e *AnomalyEngine, total, step float64, n int, ts time.Time) (float64, time.Time, int) {
		alerts := 0
		for i := 0; i < n; i++ {
			total += step + float64(i%3)
			ts = ts.Add(15 * time.Second)
			alerts += len(e.Observe("h", metric, total, ts))
		}
		return total, ts, alerts
	}

	t.Run("steady growth never alerts", func(t *testing.T) {
		if _, _, alerts := feed(NewAnomalyEngine(), 0, 1500, 500, start); alerts != 0 {
			t.Errorf("alerts = %d, want 0", alerts)
		}
	})
	t.Run("sustained rate jump alerts", func(t *testing.T) {
		e := NewAnomalyEngine()
		total, ts, _ := feed(e, 0, 1500, 400, start)
		if _, _, alerts := feed(e, total, 15000, int(counterRateWindow/(15*time.Second)), ts); alerts == 0 {
			t.Error("a sustained 10x rate jump must alert")
		}
	})
	t.Run("counter reset does not alert", func(t *testing.T) {
		e := NewAnomalyEngine()
		total, ts, _ := feed(e, 0, 1500, 400, start)
		if total == 0 {
			t.Fatal("no data")
		}
		if got := e.Observe("h", metric, 10, ts.Add(15*time.Second)); len(got) != 0 {
			t.Errorf("reset alerted: %+v", got)
		}
	})
}

func TestAnomalyEngine_BurstyCounterStaysQuiet(t *testing.T) {
	const metric = "k8s.pod.network.io"
	const samples = 240 * 4
	const warmup = 240

	rng := rand.New(rand.NewSource(1))
	e := NewAnomalyEngine()
	ts := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	total := 0.0
	alerts := 0
	for i := 0; i < samples; i++ {
		rate := 100 + rng.Float64()*100
		if rng.Float64() < 0.25 {
			rate *= 5 + rng.Float64()*15
		}
		total += rate * 15
		ts = ts.Add(15 * time.Second)
		if got := len(e.Observe("h", metric, total, ts)); i >= warmup {
			alerts += got
		}
	}
	const maxAlertsPerHour = 2
	if limit := maxAlertsPerHour * (samples - warmup) / 240; alerts > limit {
		t.Errorf("alerts = %d on normal bursts, want at most %d", alerts, limit)
	}
}

func TestCounterRates_Rate(t *testing.T) {
	start := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	const half = counterRateWindow / 2
	steps := []struct {
		name   string
		after  time.Duration
		value  float64
		want   float64
		wantOK bool
	}{
		{"first sample", 0, 0, 0, false},
		{"window not covered", half, 150, 0, false},
		{"second point of a timestamp", half, 9999, 0, false},
		{"window covered", counterRateWindow, 300, 1, true},
		{"oldest sample slides out", counterRateWindow + half, 750, 2, true},
		{"counter reset", counterRateWindow * 2, 10, 0, false},
		{"history restarts after reset", counterRateWindow*2 + half, 160, 0, false},
		{"rate resumes", counterRateWindow * 3, 310, 1, true},
	}
	c := newCounterRates()
	for _, s := range steps {
		got, ok := c.rate("k", s.value, start.Add(s.after))
		if ok != s.wantOK || got != s.want {
			t.Errorf("%s: rate = %v, %v, want %v, %v", s.name, got, ok, s.want, s.wantOK)
		}
	}
}
