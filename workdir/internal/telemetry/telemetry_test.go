package telemetry

import (
    "strings"
    "testing"
    "time"
)

func TestTelemetry(t *testing.T) {
    reg := NewRegistry("6.0.0")
    reg.SetRuptureIndex("h1", "cpu", "crit", 1.0)
    reg.IncActionsTotal("act", "t1", "success")
    reg.IncIngestTotal("prom")

    rendered := reg.Render()
    if !strings.Contains(rendered, "rpt_rupture_index") {
        t.Errorf("missing rupture_index. Got:\n%s", rendered)
    }

    hc := NewHealthChecker()
    res := hc.Check(time.Now().Add(61 * time.Minute))
    if res.Status != "ready" {
        t.Errorf("expected ready, got %s", res.Status)
    }
    _, err := hc.CheckJSON(time.Now())
    if err != nil {
        t.Fatal(err)
    }
    reg.SetTimeToFailure("h", "m", 1.0)
    reg.SetPredictedValue("h", "m", "5m", 1.0)
    reg.SetConfidence("h", 1.0)
    reg.SetFusedProbability("h", 1.0)
    reg.SetKPIStress("h", 1.0)
    reg.SetKPIFatigue("h", 1.0)
    reg.SetKPIHealthscore("h", 1.0)
    reg.SetTrackerCount("t", "active", 1)
    reg.SetMemoryBytes(100)
    reg.Render()
}
