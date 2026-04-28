package alerter

import (
	"testing"
)

func TestAlerterFiringAndResolution(t *testing.T) {
	a := NewAlerter(100)

	// Trigger stress_panic (threshold 0.8)
	a.Evaluate("host1", map[string]float64{"stress": 0.9})

	alerts := a.GetActive()
	found := false
	for _, al := range alerts {
		if al.Name == "stress_panic" && al.Host == "host1" {
			found = true
		}
	}
	if !found {
		t.Error("expected stress_panic alert to be active")
	}

	// Resolve it (value drops below threshold)
	a.Evaluate("host1", map[string]float64{"stress": 0.1})
	active := a.GetActive()
	for _, al := range active {
		if al.Name == "stress_panic" && al.Host == "host1" {
			t.Error("stress_panic should be resolved")
		}
	}
}

func TestAlerterAcknowledge(t *testing.T) {
	a := NewAlerter(100)
	a.Evaluate("host1", map[string]float64{"fatigue": 0.9})

	alerts := a.GetActive()
	if len(alerts) == 0 {
		t.Skip("no alerts fired")
	}

	id := alerts[0].ID
	if err := a.Acknowledge(id); err != nil {
		t.Fatalf("Acknowledge: %v", err)
	}

	al, ok := a.GetByID(id)
	if !ok {
		t.Fatal("alert not found after acknowledge")
	}
	if al.Status != "acknowledged" {
		t.Errorf("status = %q; want acknowledged", al.Status)
	}
}

func TestAlerterSilence(t *testing.T) {
	a := NewAlerter(100)
	a.Evaluate("host2", map[string]float64{"contagion": 0.9})

	alerts := a.GetAll()
	if len(alerts) == 0 {
		t.Skip("no alerts fired")
	}

	id := alerts[0].ID
	if err := a.Silence(id); err != nil {
		t.Fatalf("Silence: %v", err)
	}
	al, ok := a.GetByID(id)
	if !ok {
		t.Fatal("alert not found")
	}
	if al.Status != "silenced" {
		t.Errorf("status = %q; want silenced", al.Status)
	}
}

func TestAlerterDelete(t *testing.T) {
	a := NewAlerter(100)
	a.Evaluate("host3", map[string]float64{"humidity": 0.9})

	alerts := a.GetAll()
	if len(alerts) == 0 {
		t.Skip("no alerts fired")
	}

	id := alerts[0].ID
	if err := a.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := a.GetByID(id); ok {
		t.Error("deleted alert should not be found")
	}
}

func TestAlerterAlertChannel(t *testing.T) {
	a := NewAlerter(100)
	a.Evaluate("host4", map[string]float64{"pressure": 0.9})

	select {
	case al := <-a.Alerts():
		if al.Host != "host4" {
			t.Errorf("alert host = %q; want host4", al.Host)
		}
	default:
		t.Error("expected alert on channel")
	}
}

func TestAlerterAddRule(t *testing.T) {
	a := NewAlerter(100)
	a.AddRule(Rule{
		Name:      "custom_high",
		Metric:    "custom_metric",
		Threshold: 0.5,
		Severity:  "warning",
		Message:   "custom metric high",
	})

	// Fire the new custom rule
	a.Evaluate("hostX", map[string]float64{"custom_metric": 0.9})

	found := false
	for _, al := range a.GetActive() {
		if al.Name == "custom_high" && al.Host == "hostX" {
			found = true
		}
	}
	if !found {
		t.Error("expected custom_high alert to be active after AddRule")
	}
}

func TestAlerterDroppedCount(t *testing.T) {
	// Buffer size 1 — second alert should be dropped
	a := NewAlerter(1)

	// Fill the buffer with first evaluation
	a.Evaluate("hostD1", map[string]float64{"stress": 0.9})
	// Second host fires into a full channel
	a.Evaluate("hostD2", map[string]float64{"stress": 0.9})

	if a.DroppedCount() == 0 {
		t.Error("expected at least one dropped alert with buffer size 1")
	}
}

func TestAlerterDedup(t *testing.T) {
	a := NewAlerter(100)

	// Drain any existing alerts
	for len(a.Alerts()) > 0 {
		<-a.Alerts()
	}

	// First evaluation fires alerts for rules that exceed thresholds
	a.Evaluate("dedup-host", map[string]float64{"stress": 0.9})
	firstCount := 0
	for len(a.Alerts()) > 0 {
		<-a.Alerts()
		firstCount++
	}

	// Second evaluation immediately after — same rules, should NOT re-fire (dedup within 1 minute)
	a.Evaluate("dedup-host", map[string]float64{"stress": 0.9})
	secondCount := 0
	for len(a.Alerts()) > 0 {
		<-a.Alerts()
		secondCount++
	}

	if secondCount != 0 {
		t.Errorf("expected 0 alerts on second evaluation (dedup), got %d", secondCount)
	}
	if firstCount == 0 {
		t.Error("expected at least 1 alert on first evaluation")
	}
}

func TestGetUpdateDeleteRule(t *testing.T) {
	a := NewAlerter(100)
	// Clear default rules for a controlled test
	a.rules = nil
	a.AddRule(Rule{Name: "r1", Metric: "cpu", Threshold: 90, Severity: "warning"})
	a.AddRule(Rule{Name: "r2", Metric: "mem", Threshold: 80, Severity: "critical"})

	rules := a.GetRules()
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	if !a.UpdateRule("r1", Rule{Name: "r1", Metric: "cpu", Threshold: 95, Severity: "critical"}) {
		t.Error("UpdateRule should return true for existing rule")
	}
	if a.UpdateRule("nonexistent", Rule{}) {
		t.Error("UpdateRule should return false for missing rule")
	}

	if !a.DeleteRule("r2") {
		t.Error("DeleteRule should return true for existing rule")
	}
	if a.DeleteRule("r2") {
		t.Error("DeleteRule should return false after deletion")
	}
	if len(a.GetRules()) != 1 {
		t.Errorf("expected 1 rule after delete, got %d", len(a.GetRules()))
	}
}
