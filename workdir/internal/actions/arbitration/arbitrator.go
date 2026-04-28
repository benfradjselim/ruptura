package arbitration

import (
	"sort"
	"sync"
	"time"

	"github.com/benfradjselim/kairo-core/internal/actions/engine"
)

type Arbitrator interface {
	Submit(rec engine.ActionRecommendation) bool
	Drain() []engine.ActionRecommendation
}

type DefaultArbitrator struct {
	cooldownWindow time.Duration
	lastSubmitted  sync.Map
	queue          []engine.ActionRecommendation
	mu             sync.Mutex
}

func NewArbitrator(cooldownWindow time.Duration) *DefaultArbitrator {
	return &DefaultArbitrator{cooldownWindow: cooldownWindow}
}

func (a *DefaultArbitrator) Submit(rec engine.ActionRecommendation) bool {
	key := rec.Host + ":" + rec.ActionType
	last, ok := a.lastSubmitted.Load(key)
	if ok {
		lastTime := last.(time.Time)
		if time.Since(lastTime) < a.cooldownWindow {
			return false
		}
	}
	a.lastSubmitted.Store(key, time.Now())

	a.mu.Lock()
	defer a.mu.Unlock()
	a.queue = append(a.queue, rec)
	return true
}

func (a *DefaultArbitrator) Drain() []engine.ActionRecommendation {
	a.mu.Lock()
	defer a.mu.Unlock()

	recs := a.queue
	a.queue = nil

	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Tier != recs[j].Tier {
			return recs[i].Tier < recs[j].Tier
		}
		return recs[i].Confidence > recs[j].Confidence
	})

	return recs
}
