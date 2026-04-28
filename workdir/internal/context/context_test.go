package context

import (
    "math"
    "testing"
    "time"
)

func TestContext(t *testing.T) {
    t.Run("TimeOfDayManager", func(t *testing.T) {
        mgr := NewTimeOfDayManager()
        now := time.Now()
        _ = mgr.BucketOf(now)
        mgr.Update(now, 1.0)
        if math.Abs(mgr.Baseline(now)-0.01) > 1e-9 {
            t.Errorf("expected 0.01, got %f", mgr.Baseline(now))
        }
    })

    t.Run("DeploymentDetector", func(t *testing.T) {
        det := NewDeploymentDetector()
        now := time.Now()
        det.Register(DeploymentEvent{ID: "d1", StartedAt: now})
        if len(det.Active(now)) != 1 {
            t.Error("expected active deployment")
        }
    })

    t.Run("ManualContextStore", func(t *testing.T) {
        store := NewManualContextStore()
        e := ContextEntry{ID: "c1", Type: ContextLoadTest, ExpiresAt: time.Now().Add(-time.Hour)}
        store.Add(e)
        if err := store.Delete("c1"); err != nil {
            t.Fatal(err)
        }
        store.Prune(time.Now())
    })


    t.Run("DayOfWeekManager", func(t *testing.T) {
        mgr := NewDayOfWeekManager()
        // Saturday is Weekend
        if !mgr.IsWeekend(time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)) {
            t.Error("expected weekend")
        }
        // Monday is Weekday
        if mgr.IsWeekend(time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)) {
            t.Error("expected weekday")
        }
    })

    t.Run("DeploymentDetector", func(t *testing.T) {
        det := NewDeploymentDetector()
        now := time.Now()
        det.Register(DeploymentEvent{ID: "d1", StartedAt: now})
        if !det.IsSuppressed(now) {
            t.Error("expected suppressed")
        }
    })

    t.Run("ManualContextStore", func(t *testing.T) {
        store := NewManualContextStore()
        e := ContextEntry{ID: "c1", Type: ContextLoadTest}
        if err := store.Add(e); err != nil {
            t.Fatal(err)
        }
        if _, exists := store.Get("c1"); !exists {
            t.Error("expected context entry")
        }
        store.Prune(time.Now().Add(time.Hour))
        if len(store.List()) != 1 {
            t.Error("expected 1 entry")
        }
    })

    t.Run("BaselineAdapter", func(t *testing.T) {
        det := NewDeploymentDetector()
        store := NewManualContextStore()
        adapter := NewBaselineAdapter(det, store)
        // Default
        if adapter.Lambda(time.Now()) != 0.99 {
            t.Error("expected 0.99")
        }
        // AbnormalTraffic
        store.Add(ContextEntry{ID: "c2", Type: ContextAbnormalTraffic})
        if adapter.Lambda(time.Now()) != 0.80 {
            t.Error("expected 0.80")
        }
    })
}
