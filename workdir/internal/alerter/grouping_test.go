package alerter

import (
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/internal/analyzer"
	"github.com/benfradjselim/kairo-core/pkg/models"
)

func TestGroupingEngineNoSuppression(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	g := NewGroupingEngine(topo)

	// A warning alert with no critical parent → should NOT be suppressed
	alert := &models.Alert{
		ID:        "a1",
		Host:      "web-01",
		Severity:  models.SeverityWarning,
		CreatedAt: time.Now(),
	}
	g.Classify(alert, nil)
	if alert.Suppressed {
		t.Error("alert should not be suppressed with no active parent")
	}
}

func TestGroupingEngineCriticalNotSuppressed(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	g := NewGroupingEngine(topo)

	// Critical alerts are never suppressed
	alert := &models.Alert{
		ID:        "a2",
		Host:      "db-01",
		Severity:  models.SeverityCritical,
		CreatedAt: time.Now(),
	}
	active := []*models.Alert{alert}
	g.Classify(alert, active)
	if alert.Suppressed {
		t.Error("critical alert must not be suppressed")
	}
}

func TestGroupingEngineSuppression(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	// Register edge: web-01 depends on db-01
	now := time.Now()
	// Build topology edge web-01 → db-01: ingest parent (web-01) then child (db-01)
	topo.IngestSpan(models.Span{TraceID: "t1", SpanID: "s-parent", Service: "web-01", StartTime: now})
	topo.IngestSpan(models.Span{TraceID: "t1", SpanID: "s-child", ParentID: "s-parent", Service: "db-01", StartTime: now})

	g := NewGroupingEngine(topo)

	// Critical alert on db-01 (the parent/dependency)
	parentAlert := &models.Alert{
		ID:        "parent-1",
		Host:      "db-01",
		Severity:  models.SeverityCritical,
		Status:    models.StatusActive,
		CreatedAt: time.Now(),
	}
	activeAlerts := []*models.Alert{parentAlert}
	g.Classify(parentAlert, activeAlerts)

	// Warning alert on web-01 — should be suppressed because db-01 is critical
	childAlert := &models.Alert{
		ID:        "child-1",
		Host:      "web-01",
		Severity:  models.SeverityWarning,
		CreatedAt: time.Now(),
	}
	g.Classify(childAlert, activeAlerts)
	if !childAlert.Suppressed {
		t.Error("child alert should be suppressed by critical parent")
	}
	if childAlert.SuppressedBy != "parent-1" {
		t.Errorf("SuppressedBy = %q; want parent-1", childAlert.SuppressedBy)
	}
}

func TestGroupingEngineSuppressionExpiry(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	// Set start time in the past but within window
	now2 := time.Now()
	topo.IngestSpan(models.Span{TraceID: "t2", SpanID: "p2", Service: "api-01", StartTime: now2})
	topo.IngestSpan(models.Span{TraceID: "t2", SpanID: "c2", ParentID: "p2", Service: "db-01", StartTime: now2})

	g := NewGroupingEngine(topo)

	// Parent alert that is older than suppressionCeiling
	parentAlert := &models.Alert{
		ID:        "old-parent",
		Host:      "db-01",
		Severity:  models.SeverityCritical,
		CreatedAt: time.Now().Add(-10 * time.Minute), // expired
	}
	activeAlerts := []*models.Alert{parentAlert}
	g.Classify(parentAlert, activeAlerts)

	// Child should NOT be suppressed because the parent alert is too old
	childAlert := &models.Alert{
		ID:        "child-2",
		Host:      "api-01",
		Severity:  models.SeverityWarning,
		CreatedAt: time.Now(),
	}
	g.Classify(childAlert, activeAlerts)
	if childAlert.Suppressed {
		t.Error("child alert should not be suppressed by an expired parent")
	}
}

func TestGroupingEngineListGroups(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	g := NewGroupingEngine(topo)

	critical := &models.Alert{
		ID:        "grp-parent",
		Host:      "db",
		Severity:  models.SeverityCritical,
		CreatedAt: time.Now(),
	}
	g.Classify(critical, []*models.Alert{critical})

	groups := g.GetGroups()
	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

func TestGetGroupAndExpand(t *testing.T) {
	topo := analyzer.NewTopologyAnalyzer(5 * time.Minute)
	g := NewGroupingEngine(topo)

	alert := &models.Alert{
		ID:        "g-parent",
		Host:      "db",
		Severity:  models.SeverityCritical,
		CreatedAt: time.Now(),
	}
	g.Classify(alert, []*models.Alert{alert})

	groups := g.GetGroups()
	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}
	groupID := groups[0].ID

	grp, ok := g.GetGroup(groupID)
	if !ok {
		t.Fatal("GetGroup: not found")
	}
	if grp.ID != groupID {
		t.Errorf("GetGroup id mismatch: %s", grp.ID)
	}

	_, ok = g.GetGroup("nonexistent")
	if ok {
		t.Error("GetGroup should return false for missing group")
	}

	a := NewAlerter(100)
	g.ExpandGroup(groupID, a)
	g.ExpandGroup("nonexistent", a) // should not panic
}
