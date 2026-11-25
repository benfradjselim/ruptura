package failuremodes

import (
	"sort"
	"sync"
	"time"
)

// objectStale is how long an object may go without a reading before its
// finding is no longer reported.
const objectStale = 3 * time.Minute

// ObjectFinding is a failure of an object other than a pod, read from a
// catalog card: a node, a namespace quota, an autoscaler, a disruption budget.
type ObjectFinding struct {
	Card      string
	Level     string
	Target    string
	Predicted bool
	Estimate  Estimate
	// Value and Capacity are the last raw readings: usage and its capacity for
	// a predicted card, the query value for a detected one.
	Value    float64
	Capacity float64
	Unit     string
}

type objectTrend struct {
	level, unit string
	mode        Mode
	horizon     time.Duration
	detector    *Saturation
	estimate    Estimate
	active      bool
	value, cap  float64
	last        time.Time
}

type objectCondition struct {
	level    string
	detector *Condition
	estimate Estimate
	active   bool
	value    float64
	last     time.Time
}

// Objects follows catalog cards per object. It is safe for concurrent use.
type Objects struct {
	mu         sync.Mutex
	trends     map[string]*objectTrend
	conditions map[string]*objectCondition
	// polled is the time of the last successful query of each card. An object
	// missing from that poll is gone: a deleted Job, a removed node.
	polled map[string]time.Time
}

// NewObjects returns an empty object tracker.
func NewObjects() *Objects {
	return &Objects{trends: map[string]*objectTrend{}, conditions: map[string]*objectCondition{}, polled: map[string]time.Time{}}
}

func objectKey(card, target string) string { return card + "\x00" + target }

func splitObjectKey(key string) (string, string) {
	for i := 0; i < len(key); i++ {
		if key[i] == 0 {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}

// ObserveTrend records usage against capacity for a predicted card. The
// saturation detector decides whether the trend is real; a forecast further
// than horizon is kept but not reported.
func (o *Objects) ObserveTrend(card, level, target string, mode Mode, horizon time.Duration, unit string, at time.Time, usage, capacity float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	key := objectKey(card, target)
	state, ok := o.trends[key]
	if !ok {
		state = &objectTrend{level: level, unit: unit, mode: mode, horizon: horizon, detector: NewSaturation()}
		o.trends[key] = state
	}
	state.value, state.cap, state.last = usage, capacity, at
	est, active := state.detector.Observe(at, usage, capacity, 0)
	est.Mode = mode
	state.estimate, state.active = est, active
}

// ObserveDetected records the value of a detected card's query: 1 while the
// failure holds, 0 otherwise.
func (o *Objects) ObserveDetected(card, level, target string, mode Mode, raiseAfter, clearAfter time.Duration, at time.Time, value float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	key := objectKey(card, target)
	state, ok := o.conditions[key]
	if !ok {
		state = &objectCondition{level: level, detector: NewConditionFor(mode, 1, raiseAfter, clearAfter)}
		o.conditions[key] = state
	}
	state.value, state.last = value, at
	state.estimate, state.active = state.detector.Observe(at, value)
}

// Polled records a successful query of card at the given time. Call it after
// the readings of that poll.
func (o *Objects) Polled(card string, at time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.polled[card] = at
}

// gone reports whether an object missed the last successful poll of its card,
// or has had no reading for objectStale.
func (o *Objects) gone(card string, last, now time.Time) bool {
	if polled, ok := o.polled[card]; ok && last.Before(polled) {
		return true
	}
	return now.Sub(last) > objectStale
}

// Findings returns the active findings with a recent reading, most urgent
// first. A forecast beyond its card's horizon is left out.
func (o *Objects) Findings(now time.Time) []ObjectFinding {
	o.mu.Lock()
	defer o.mu.Unlock()
	var out []ObjectFinding
	for key, state := range o.trends {
		card, target := splitObjectKey(key)
		if !state.active || o.gone(card, state.last, now) || state.estimate.ETF > state.horizon {
			continue
		}
		out = append(out, ObjectFinding{Card: card, Level: state.level, Target: target, Predicted: true,
			Estimate: state.estimate, Value: state.value, Capacity: state.cap, Unit: state.unit})
	}
	for key, state := range o.conditions {
		card, target := splitObjectKey(key)
		if !state.active || o.gone(card, state.last, now) {
			continue
		}
		out = append(out, ObjectFinding{Card: card, Level: state.level, Target: target,
			Estimate: state.estimate, Value: state.value})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Predicted != out[j].Predicted {
			return !out[i].Predicted
		}
		if out[i].Estimate.ETF != out[j].Estimate.ETF {
			return out[i].Estimate.ETF < out[j].Estimate.ETF
		}
		return out[i].Target < out[j].Target
	})
	return out
}

// Prune forgets objects with no reading since before.
func (o *Objects) Prune(before time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for key, state := range o.trends {
		if state.last.Before(before) {
			delete(o.trends, key)
		}
	}
	for key, state := range o.conditions {
		if state.last.Before(before) {
			delete(o.conditions, key)
		}
	}
}
