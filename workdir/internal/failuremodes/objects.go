package failuremodes

import (
	"sort"
	"sync"
	"time"
)

// objectStale is how long an object may go without a reading before its
// finding is no longer reported.
const objectStale = 3 * time.Minute

// stallMinRises is how many rises a trend needs before a pause can end it.
const stallMinRises = 3

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
	// sampled is the time of the source sample behind the estimate.
	sampled time.Time
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
	// samples holds the latest and the previous sample time of each card, so
	// that an object first seen at one sample can be dated from the one before.
	samples map[string][2]time.Time
}

// NewObjects returns an empty object tracker.
func NewObjects() *Objects {
	return &Objects{trends: map[string]*objectTrend{}, conditions: map[string]*objectCondition{}, polled: map[string]time.Time{},
		samples: map[string][2]time.Time{}}
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

// ObserveTrend records usage against capacity for a predicted card, read at
// polled from a source sample taken at sampled (zero means polled). The
// saturation detector decides whether the trend is real; a forecast further
// than horizon is kept but not reported, and a trend that pauses for longer
// than any pause it showed while rising is withdrawn.
func (o *Objects) ObserveTrend(card, level, target string, mode Mode, horizon time.Duration, unit string, polled, sampled time.Time, usage, capacity float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if sampled.IsZero() {
		sampled = polled
	}
	key := objectKey(card, target)
	state, ok := o.trends[key]
	if !ok {
		state = &objectTrend{level: level, unit: unit, mode: mode, horizon: horizon, detector: NewSaturation()}
		o.trends[key] = state
	}
	state.last = polled
	if !sampled.After(state.sampled) {
		return
	}
	state.value, state.cap, state.sampled = usage, capacity, sampled
	est, active := state.detector.Observe(sampled, usage, capacity, 0)
	if active && usage < capacity && stalled(state.detector.points) {
		est, active = Estimate{}, false
	}
	est.Mode = mode
	state.estimate, state.active = est, active
}

// ObserveDetected records the value of a detected card's query, 1 while the
// failure holds and 0 otherwise, read at polled from a source sample taken at
// sampled (zero means polled).
func (o *Objects) ObserveDetected(card, level, target string, mode Mode, raiseAfter, clearAfter time.Duration, polled, sampled time.Time, value float64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if sampled.IsZero() {
		sampled = polled
	}
	prior := o.sample(card, sampled)
	key := objectKey(card, target)
	state, ok := o.conditions[key]
	if !ok {
		state = &objectCondition{level: level, detector: NewSampledCondition(mode, 1, raiseAfter, clearAfter, objectStale, prior)}
		o.conditions[key] = state
	}
	state.value, state.last = value, polled
	state.estimate, state.active = state.detector.Observe(sampled, value)
}

// sample records a sample time of card and returns the card's sample before
// it, or zero when none is known.
func (o *Objects) sample(card string, at time.Time) time.Time {
	times := o.samples[card]
	if at.After(times[0]) {
		times = [2]time.Time{at, times[0]}
		o.samples[card] = times
	}
	if at.Equal(times[0]) {
		return times[1]
	}
	return time.Time{}
}

// stalled reports whether usage has not risen for longer than the longest
// pause between rises in the window, plus one sample. A fill that stops is
// withdrawn within a few samples, while a leak that grows in steps keeps its
// forecast through pauses like the ones it already showed.
func stalled(points []point) bool {
	var rises []time.Time
	for i := 1; i < len(points); i++ {
		if points[i].usage > points[i-1].usage {
			rises = append(rises, points[i].at)
		}
	}
	if len(rises) < stallMinRises {
		return false
	}
	var longest time.Duration
	for i := 1; i < len(rises); i++ {
		if gap := rises[i].Sub(rises[i-1]); gap > longest {
			longest = gap
		}
	}
	last := points[len(points)-1].at
	spacing := last.Sub(points[0].at) / time.Duration(len(points)-1)
	return last.Sub(rises[len(rises)-1]) >= longest+spacing
}

// aged returns est counted down by the age of its sample at now.
func aged(est Estimate, age time.Duration) Estimate {
	down := func(d time.Duration) time.Duration {
		if d == 0 {
			return 0
		}
		if d -= age; d < 0 {
			return 0
		}
		return d
	}
	est.ETF, est.ETFEarly, est.ETFLate = down(est.ETF), down(est.ETFEarly), down(est.ETFLate)
	return est
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
		est := aged(state.estimate, now.Sub(state.sampled))
		if !state.active || o.gone(card, state.last, now) || est.ETF > state.horizon {
			continue
		}
		out = append(out, ObjectFinding{Card: card, Level: state.level, Target: target, Predicted: true,
			Estimate: est, Value: state.value, Capacity: state.cap, Unit: state.unit})
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
