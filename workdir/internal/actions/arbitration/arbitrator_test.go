package arbitration

import (
	"testing"
	"time"

	"github.com/benfradjselim/kairo-core/internal/actions/engine"
)

func TestArbitrator_submit_dedup(t *testing.T) {
	arb := NewArbitrator(5 * time.Minute)
	rec := engine.ActionRecommendation{Host: "h1", ActionType: "restart"}
	if !arb.Submit(rec) {
		t.Error("First submit should be allowed")
	}
	if arb.Submit(rec) {
		t.Error("Second submit should be deduped")
	}
}

func TestArbitrator_submit_afterCooldown(t *testing.T) {
	arb := NewArbitrator(10 * time.Millisecond)
	rec := engine.ActionRecommendation{Host: "h1", ActionType: "restart"}
	arb.Submit(rec)
	time.Sleep(20 * time.Millisecond)
	if !arb.Submit(rec) {
		t.Error("Submit after cooldown should be allowed")
	}
}

func TestArbitrator_drain_order(t *testing.T) {
	arb := NewArbitrator(0)
	arb.Submit(engine.ActionRecommendation{Tier: engine.Tier2, Confidence: 0.5})
	arb.Submit(engine.ActionRecommendation{Tier: engine.Tier1, Confidence: 0.9})
	arb.Submit(engine.ActionRecommendation{Tier: engine.Tier1, Confidence: 0.8})

	recs := arb.Drain()
	if len(recs) != 3 {
		t.Fatalf("Expected 3 recs, got %d", len(recs))
	}
	if recs[0].Tier != engine.Tier1 || recs[0].Confidence != 0.9 {
		t.Errorf("Wrong order: %v", recs[0])
	}
	if recs[2].Tier != engine.Tier2 {
		t.Errorf("Wrong order: %v", recs[2])
	}
}

func TestArbitrator_drain_clears(t *testing.T) {
	arb := NewArbitrator(0)
	arb.Submit(engine.ActionRecommendation{Tier: engine.Tier1})
	arb.Drain()
	if len(arb.Drain()) != 0 {
		t.Error("Queue should be empty after second Drain")
	}
}
