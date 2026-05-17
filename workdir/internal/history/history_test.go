package history

import (
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/pkg/models"
)

func makeSnap(healthScore float64) models.KPISnapshot {
	return models.KPISnapshot{
		HealthScore: models.KPI{Value: healthScore},
		Stress:      models.KPI{Value: 0.1},
		Fatigue:     models.KPI{Value: 0.2},
	}
}

func TestMaybePush_StoresFirstPoint(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("ns/dep/svc", makeSnap(80), now, 30*time.Second)
	pts := m.Get("ns/dep/svc")
	if len(pts) != 1 {
		t.Fatalf("expected 1 point, got %d", len(pts))
	}
	if pts[0].HealthScore != 80 {
		t.Errorf("expected HealthScore=80, got %v", pts[0].HealthScore)
	}
}

func TestMaybePush_ThrottlesWithinInterval(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("svc", makeSnap(80), now, 30*time.Second)
	m.MaybePush("svc", makeSnap(90), now.Add(10*time.Second), 30*time.Second)
	pts := m.Get("svc")
	if len(pts) != 1 {
		t.Errorf("expected 1 point (throttled), got %d", len(pts))
	}
}

func TestMaybePush_AllowsAfterInterval(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("svc", makeSnap(80), now, 30*time.Second)
	m.MaybePush("svc", makeSnap(90), now.Add(31*time.Second), 30*time.Second)
	pts := m.Get("svc")
	if len(pts) != 2 {
		t.Errorf("expected 2 points, got %d", len(pts))
	}
	if pts[1].HealthScore != 90 {
		t.Errorf("expected second point HealthScore=90, got %v", pts[1].HealthScore)
	}
}

func TestMaybePush_CapsAtMaxPoints(t *testing.T) {
	m := New()
	base := time.Now()
	interval := 30 * time.Second
	// Push maxPoints+10 entries
	for i := 0; i < maxPoints+10; i++ {
		m.MaybePush("svc", makeSnap(float64(i)), base.Add(time.Duration(i)*interval), interval)
	}
	pts := m.Get("svc")
	if len(pts) != maxPoints {
		t.Errorf("expected %d points (capped), got %d", maxPoints, len(pts))
	}
	// The oldest points should have been evicted; last point has highest score
	last := pts[len(pts)-1]
	if last.HealthScore != float64(maxPoints+10-1) {
		t.Errorf("expected last HealthScore=%v, got %v", float64(maxPoints+10-1), last.HealthScore)
	}
}

func TestGet_ReturnsEmptyForUnknownKey(t *testing.T) {
	m := New()
	pts := m.Get("nonexistent/key")
	if len(pts) != 0 {
		t.Errorf("expected empty slice for unknown key, got %d points", len(pts))
	}
}

func TestGet_ReturnsCopy(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("svc", makeSnap(50), now, time.Second)
	pts := m.Get("svc")
	pts[0].HealthScore = 999
	// Original should be unaffected
	orig := m.Get("svc")
	if orig[0].HealthScore == 999 {
		t.Error("Get should return a copy, not a reference to internal state")
	}
}

func TestAll_ReturnsAllKeys(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("svc-a", makeSnap(80), now, time.Second)
	m.MaybePush("svc-b", makeSnap(60), now, time.Second)
	all := m.All()
	if len(all) != 2 {
		t.Errorf("expected 2 keys, got %d", len(all))
	}
	if _, ok := all["svc-a"]; !ok {
		t.Error("missing svc-a in All()")
	}
	if _, ok := all["svc-b"]; !ok {
		t.Error("missing svc-b in All()")
	}
}

func TestAll_ReturnsCopies(t *testing.T) {
	m := New()
	now := time.Now()
	m.MaybePush("svc", makeSnap(70), now, time.Second)
	all := m.All()
	all["svc"][0].HealthScore = 999
	// Original should be unaffected
	orig := m.Get("svc")
	if orig[0].HealthScore == 999 {
		t.Error("All should return copies, not references to internal state")
	}
}

func TestPointFromSnapshot(t *testing.T) {
	snap := models.KPISnapshot{
		HealthScore:         models.KPI{Value: 75.5},
		FusedRuptureIndex:   1.2,
		Stress:              models.KPI{Value: 0.3},
		Fatigue:             models.KPI{Value: 0.4},
		Contagion:           models.KPI{Value: 0.15},
		Pressure:            models.KPI{Value: 0.6},
		Mood:                models.KPI{Value: 0.7},
		CalibrationProgress: 45,
	}
	pt := PointFromSnapshot(snap)
	if pt.HealthScore != 75.5 {
		t.Errorf("HealthScore mismatch: got %v", pt.HealthScore)
	}
	if pt.FusedRuptureIndex != 1.2 {
		t.Errorf("FusedRuptureIndex mismatch: got %v", pt.FusedRuptureIndex)
	}
	if pt.CalibrationPct != 45 {
		t.Errorf("CalibrationPct mismatch: got %v", pt.CalibrationPct)
	}
}
