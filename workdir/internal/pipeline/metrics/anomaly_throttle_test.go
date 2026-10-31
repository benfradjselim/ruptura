package metrics

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func TestAlertThrottle_Allow(t *testing.T) {
	start := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	steps := []struct {
		name     string
		key      string
		after    time.Duration
		critical bool
		want     bool
	}{
		{"first alert passes", "a", 0, false, true},
		{"repeat in cooldown is dropped", "a", 15 * time.Second, false, false},
		{"other series passes", "b", 30 * time.Second, false, true},
		{"escalation passes", "a", 45 * time.Second, true, true},
		{"repeat critical is dropped", "a", time.Minute, true, false},
		{"warning after critical is dropped", "a", 75 * time.Second, false, false},
		{"reminder after cooldown passes", "a", 45*time.Second + alertCooldown, false, true},
	}
	th := newAlertThrottle()
	for _, s := range steps {
		severity := models.SeverityWarning
		if s.critical {
			severity = models.SeverityCritical
		}
		if got := th.allow(s.key, severity, start.Add(s.after)); got != s.want {
			t.Errorf("%s: allow = %v, want %v", s.name, got, s.want)
		}
	}
}

func TestAnomalyEngine_OneAlertPerObservation(t *testing.T) {
	e := NewAnomalyEngine()
	ts := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 400; i++ {
		ts = ts.Add(15 * time.Second)
		e.Observe("h", "m", 100+float64(i%3), ts)
	}
	got := e.Observe("h", "m", 100000, ts.Add(15*time.Second))
	if len(got) != 1 {
		t.Fatalf("alerts = %d, want 1: %+v", len(got), got)
	}
	if got[0].Severity != models.SeverityCritical {
		t.Errorf("severity = %s, want critical when methods agree", got[0].Severity)
	}
}
