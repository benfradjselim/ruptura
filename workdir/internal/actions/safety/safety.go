package safety

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benfradjselim/kairo-core/internal/actions/engine"
	"github.com/benfradjselim/kairo-core/internal/actions/providers"
)

type RateLimiter interface {
	Allow(host string, tier engine.ActionTier) bool
	Reset(host string)
}

type TokenBucketLimiter struct {
	maxPerHour int
	state      sync.Map
}

func NewRateLimiter(maxPerHour int) *TokenBucketLimiter {
	if maxPerHour == 0 {
		maxPerHour = 6
	}
	return &TokenBucketLimiter{maxPerHour: maxPerHour}
}

func (l *TokenBucketLimiter) Allow(host string, tier engine.ActionTier) bool {
	if tier != engine.Tier1 {
		return true
	}
	now := time.Now()
	val, _ := l.state.LoadOrStore(host, &hostState{windowStart: now, count: 0})
	s := val.(*hostState)
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.Sub(s.windowStart) > time.Hour {
		s.windowStart = now
		s.count = 0
	}
	if s.count < l.maxPerHour {
		s.count++
		return true
	}
	return false
}

func (l *TokenBucketLimiter) Reset(host string) {
	l.state.Delete(host)
}

type hostState struct {
	mu          sync.Mutex
	windowStart time.Time
	count       int
}

type CooldownTracker interface {
	InCooldown(host, actionType string) bool
	Record(host, actionType string, cooldown time.Duration)
}

type DefaultCooldownTracker struct {
	cooldowns sync.Map
}

func NewCooldownTracker() *DefaultCooldownTracker {
	return &DefaultCooldownTracker{}
}

func (c *DefaultCooldownTracker) InCooldown(host, actionType string) bool {
	key := host + ":" + actionType
	val, ok := c.cooldowns.Load(key)
	if !ok {
		return false
	}
	return time.Now().Before(val.(time.Time))
}

func (c *DefaultCooldownTracker) Record(host, actionType string, cooldown time.Duration) {
	key := host + ":" + actionType
	c.cooldowns.Store(key, time.Now().Add(cooldown))
}

type EmergencyStop struct {
	stopped int32 // atomic
}

func (e *EmergencyStop) Stop() {
	atomic.StoreInt32(&e.stopped, 1)
}

func (e *EmergencyStop) Reset() {
	atomic.StoreInt32(&e.stopped, 0)
}

func (e *EmergencyStop) IsActive() bool {
	return atomic.LoadInt32(&e.stopped) == 1
}

type ShadowMode struct {
	enabled bool
}

func NewShadowMode(enabled bool) *ShadowMode {
	return &ShadowMode{enabled: enabled}
}

func (s *ShadowMode) MaybeExecute(ctx context.Context, p providers.Provider, a engine.ActionRecommendation) error {
	if s.enabled {
		return nil
	}
	return p.Execute(ctx, a)
}

type RollbackTrigger interface {
	Record(host, actionID string, r float64, ts time.Time)
	ShouldRollback(host, actionID string, rNew float64) bool
}

type DefaultRollbackTrigger struct {
	lastR sync.Map
}

func NewRollbackTrigger() *DefaultRollbackTrigger {
	return &DefaultRollbackTrigger{}
}

func (r *DefaultRollbackTrigger) Record(host, actionID string, rVal float64, ts time.Time) {
	r.lastR.Store(host+":"+actionID, rVal)
}

func (r *DefaultRollbackTrigger) ShouldRollback(host, actionID string, rNew float64) bool {
	val, ok := r.lastR.Load(host + ":" + actionID)
	if !ok {
		return false
	}
	return rNew > val.(float64)
}
