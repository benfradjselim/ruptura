package metrics

import (
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
	t.Run("rate jump alerts", func(t *testing.T) {
		e := NewAnomalyEngine()
		total, ts, _ := feed(e, 0, 1500, 400, start)
		if got := e.Observe("h", metric, total+150000, ts.Add(15*time.Second)); len(got) == 0 {
			t.Error("a 100x rate jump must alert")
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
