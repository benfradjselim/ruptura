package safety

import (
	"context"
	"testing"
	"time"

	"github.com/benfradjselim/ruptura/internal/actions/engine"
)

func TestRateLimiter_allow_block(t *testing.T) {
	l := NewRateLimiter(2)
	host := "h1"
	if !l.Allow(host, engine.Tier1) || !l.Allow(host, engine.Tier1) {
		t.Error("Should allow 2 calls")
	}
	if l.Allow(host, engine.Tier1) {
		t.Error("Should block 3rd call")
	}
}

func TestRateLimiter_tier2_noLimit(t *testing.T) {
	l := NewRateLimiter(1)
	if !l.Allow("h1", engine.Tier2) || !l.Allow("h1", engine.Tier2) {
		t.Error("Tier2 should not be limited")
	}
}

func TestCooldownTracker_inCooldown_expired(t *testing.T) {
	c := NewCooldownTracker()
	c.Record("h1", "act", 10*time.Millisecond)
	if !c.InCooldown("h1", "act") {
		t.Error("Should be in cooldown")
	}
	time.Sleep(20 * time.Millisecond)
	if c.InCooldown("h1", "act") {
		t.Error("Should NOT be in cooldown")
	}
}

func TestEmergencyStop(t *testing.T) {
	es := &EmergencyStop{}
	es.Stop()
	if !es.IsActive() {
		t.Error("Should be active")
	}
	es.Reset()
	if es.IsActive() {
		t.Error("Should NOT be active")
	}
}

type mockProvider struct{}
func (m *mockProvider) Execute(ctx context.Context, a engine.ActionRecommendation) error { return nil }
func (m *mockProvider) Name() string { return "mock" }

func TestShadowMode(t *testing.T) {
	p := &mockProvider{}
	sEnabled := NewShadowMode(true)
	if err := sEnabled.MaybeExecute(context.Background(), p, engine.ActionRecommendation{}); err != nil {
		t.Errorf("Should not error, got %v", err)
	}

	sDisabled := NewShadowMode(false)
	if err := sDisabled.MaybeExecute(context.Background(), p, engine.ActionRecommendation{}); err != nil {
		t.Errorf("Should not error, got %v", err)
	}
}

func TestRollbackTrigger(t *testing.T) {
	rt := NewRollbackTrigger()
	rt.Record("h1", "id", 2.0, time.Now())
	if !rt.ShouldRollback("h1", "id", 3.0) {
		t.Error("Should rollback")
	}
	if rt.ShouldRollback("h1", "id", 1.0) {
		t.Error("Should not rollback")
	}
}
